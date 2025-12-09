package report

import (
	"bytes"
	"html/template"
	"strings"

	"github.com/devblac/go-semver-audit/internal/analyzer"
)

// FormatHTML generates a self-contained HTML report.
func FormatHTML(result *analyzer.Result) (string, error) {
	data := buildHTMLData(result)

	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"join": join,
	}).Parse(htmlTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

type htmlRemoved struct {
	Name   string
	Type   string
	UsedIn string
}

type htmlChanged struct {
	Name         string
	OldSignature string
	NewSignature string
	UsedIn       string
}

type htmlInterface struct {
	Name           string
	AddedMethods   []string
	RemovedMethods []string
	UsedIn         string
}

type htmlAdded struct {
	Name string
	Type string
}

type htmlData struct {
	Module            string
	OldVersion        string
	NewVersion        string
	Breaking          bool
	SummaryCount      int
	AffectedLocations int
	Removed           []htmlRemoved
	Changed           []htmlChanged
	Interfaces        []htmlInterface
	Added             []htmlAdded
	UnusedDeps        []string
	HasUnusedDeps     bool
}

func buildHTMLData(result *analyzer.Result) htmlData {
	data := htmlData{
		Module:            result.Module,
		OldVersion:        result.OldVersion,
		NewVersion:        result.NewVersion,
		Breaking:          result.HasBreakingChanges(),
		SummaryCount:      len(result.Changes.Removed) + len(result.Changes.Changed) + len(result.Changes.InterfaceChanges),
		AffectedLocations: countAffectedLocations(result.Changes),
		HasUnusedDeps:     len(result.UnusedDeps) > 0,
		UnusedDeps:        result.UnusedDeps,
	}

	for _, removed := range result.Changes.Removed {
		data.Removed = append(data.Removed, htmlRemoved{
			Name:   removed.Name,
			Type:   removed.Type,
			UsedIn: formatLocations(removed.UsedIn, 5),
		})
	}

	for _, changed := range result.Changes.Changed {
		data.Changed = append(data.Changed, htmlChanged{
			Name:         changed.Name,
			OldSignature: changed.OldSignature,
			NewSignature: changed.NewSignature,
			UsedIn:       formatLocations(changed.UsedIn, 5),
		})
	}

	for _, iface := range result.Changes.InterfaceChanges {
		data.Interfaces = append(data.Interfaces, htmlInterface{
			Name:           iface.Name,
			AddedMethods:   iface.AddedMethods,
			RemovedMethods: iface.RemovedMethods,
			UsedIn:         formatLocations(iface.UsedIn, 5),
		})
	}

	for _, added := range result.Changes.Added {
		data.Added = append(data.Added, htmlAdded{
			Name: added.Name,
			Type: added.Type,
		})
	}

	return data
}

// Minimal, self-contained HTML with light styling for quick sharing.
const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>go-semver-audit: {{.Module}} {{.OldVersion}} → {{.NewVersion}}</title>
  <style>
    :root { color-scheme: light dark; }
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 0; padding: 24px; line-height: 1.5; background: #0f1116; color: #e7ecf3; }
    section { margin-bottom: 24px; background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.08); border-radius: 12px; padding: 16px; }
    h1 { margin: 0 0 12px; font-size: 22px; }
    h2 { margin: 12px 0; font-size: 18px; }
    h3 { margin: 8px 0; font-size: 15px; }
    .pill { display: inline-block; padding: 4px 10px; border-radius: 999px; font-size: 12px; font-weight: 600; }
    .pill.ok { background: rgba(46,204,113,0.15); color: #2ecc71; border: 1px solid rgba(46,204,113,0.4); }
    .pill.warn { background: rgba(241,196,15,0.15); color: #f1c40f; border: 1px solid rgba(241,196,15,0.4); }
    .summary { display: flex; flex-wrap: wrap; gap: 12px; }
    .card { padding: 12px; border-radius: 10px; background: rgba(255,255,255,0.04); border: 1px solid rgba(255,255,255,0.08); min-width: 160px; }
    .label { color: #9aa4b5; font-size: 12px; text-transform: uppercase; letter-spacing: 0.05em; }
    ul { margin: 6px 0 0 18px; }
    code { background: rgba(255,255,255,0.06); padding: 2px 5px; border-radius: 6px; }
    .muted { color: #9aa4b5; }
    .stacked { margin: 8px 0 0; }
  </style>
</head>
<body>
  <section>
    <h1>go-semver-audit</h1>
    <div class="muted">{{.Module}} {{.OldVersion}} → {{.NewVersion}}</div>
    {{if .Breaking}}<span class="pill warn">Breaking changes detected</span>{{else}}<span class="pill ok">No breaking changes</span>{{end}}
  </section>

  <section>
    <h2>Summary</h2>
    <div class="summary">
      <div class="card">
        <div class="label">Breaking changes</div>
        <div>{{.SummaryCount}}</div>
      </div>
      <div class="card">
        <div class="label">Affected locations</div>
        <div>{{.AffectedLocations}}</div>
      </div>
      <div class="card">
        <div class="label">Unused dependencies</div>
        <div>{{len .UnusedDeps}}</div>
      </div>
    </div>
  </section>

  {{if .Removed}}
  <section>
    <h2>Removed symbols</h2>
    {{range .Removed}}
      <div class="stacked">
        <strong>{{.Name}}</strong> <span class="muted">({{.Type}})</span><br>
        {{if .UsedIn}}<span class="muted">Used in:</span> {{.UsedIn}}{{else}}<span class="muted">Not detected in use</span>{{end}}
      </div>
    {{end}}
  </section>
  {{end}}

  {{if .Changed}}
  <section>
    <h2>Changed signatures</h2>
    {{range .Changed}}
      <div class="stacked">
        <strong>{{.Name}}</strong><br>
        <span class="muted">Old:</span> <code>{{.OldSignature}}</code><br>
        <span class="muted">New:</span> <code>{{.NewSignature}}</code><br>
        {{if .UsedIn}}<span class="muted">Used in:</span> {{.UsedIn}}{{else}}<span class="muted">Not detected in use</span>{{end}}
      </div>
    {{end}}
  </section>
  {{end}}

  {{if .Interfaces}}
  <section>
    <h2>Modified interfaces</h2>
    {{range .Interfaces}}
      <div class="stacked">
        <strong>{{.Name}}</strong><br>
        {{if .RemovedMethods}}<div><span class="muted">Removed:</span> {{join .RemovedMethods ", "}}</div>{{end}}
        {{if .AddedMethods}}<div><span class="muted">Added:</span> {{join .AddedMethods ", "}}</div>{{end}}
        {{if .UsedIn}}<span class="muted">Used in:</span> {{.UsedIn}}{{else}}<span class="muted">Not detected in use</span>{{end}}
      </div>
    {{end}}
  </section>
  {{end}}

  {{if .Added}}
  <section>
    <h2>Added symbols (informational)</h2>
    {{range .Added}}
      <div class="stacked">
        <strong>{{.Name}}</strong> <span class="muted">({{.Type}})</span>
      </div>
    {{end}}
  </section>
  {{end}}

  {{if .HasUnusedDeps}}
  <section>
    <h2>Unused dependencies</h2>
    <ul>
      {{range .UnusedDeps}}<li>{{.}}</li>{{end}}
    </ul>
  </section>
  {{end}}
</body>
</html>
`

// join provides comma-separated lists inside templates.
func join(items []string, sep string) string {
	return strings.Join(items, sep)
}
