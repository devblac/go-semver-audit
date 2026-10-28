#!/usr/bin/env bash
# Audits every direct dependency a pull request upgrades and reports the result
# as a pull request comment, a job summary and step outputs.
#
# The analysis runs against the pull request's *base* commit: that is the code
# and go.mod the upgrade is applied to. At the head commit go.mod already holds
# the new versions, and if an upgrade really is breaking the project may not
# even compile there.
#
# Configuration comes from GSA_* environment variables (see action.yml). Setting
# GSA_COMMENT=false and the GitHub-provided variables by hand runs it locally.
set -euo pipefail

: "${GSA_WORKING_DIRECTORY:=.}"
: "${GSA_COMMENT:=true}"
: "${GSA_FAIL_ON_BREAKING:=true}"
: "${GSA_FAIL_ON_ERROR:=false}"

# GitHub rejects comments over 65536 characters
readonly MAX_COMMENT_BYTES=60000

if [ -z "${GSA_BASE_SHA:-}" ] || [ -z "${GSA_HEAD_SHA:-}" ]; then
  echo "::notice::go-semver-audit only runs on pull_request events; skipping"
  exit 0
fi

action_path="${GITHUB_ACTION_PATH:-$(cd "$(dirname "$0")/.." && pwd)}"
# A fresh directory per invocation: the action may run several times in one
# job (once per module in a monorepo), and each run's report must survive the
# next one
tmp="$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/go-semver-audit.XXXXXX")"

set_output() {
  if [ -n "${GITHUB_OUTPUT:-}" ]; then
    echo "$1=$2" >> "$GITHUB_OUTPUT"
  fi
}

# --- Build the tool from the same ref as this action -----------------------

bin="${GSA_BIN:-}"
if [ -z "$bin" ]; then
  bin="$tmp/go-semver-audit"
  if [ "${RUNNER_OS:-}" = "Windows" ]; then
    bin="$bin.exe"
  fi
  echo "::group::Build go-semver-audit"
  # The tool's own go.mod may need a newer Go than the workflow set up, and
  # setup-go pins GOTOOLCHAIN=local; let only this build switch toolchains
  (cd "$action_path" && GOTOOLCHAIN=auto GOFLAGS= go build -o "$bin" ./cmd/go-semver-audit)
  echo "::endgroup::"
fi

# --- Locate the module -----------------------------------------------------------

cd "${GITHUB_WORKSPACE:-.}"
cd "$GSA_WORKING_DIRECTORY"
# Path of the working directory relative to the repository root, e.g. "api/"
prefix="$(git rev-parse --show-prefix)"

# Each module gets its own comment, so a monorepo running the action once per
# module does not have every run overwrite the same comment
marker="<!-- go-semver-audit dir=${prefix:-.} -->"
title="## go-semver-audit"
if [ -n "$prefix" ]; then
  title="$title: \`${prefix%/}\`"
fi

# --- Pull request comment ----------------------------------------------------------

# find_comment prints the id of this module's previous comment, if any
find_comment() {
  gh api --paginate "repos/$GSA_REPOSITORY/issues/$GSA_PR_NUMBER/comments" \
    --jq ".[] | select(.body | contains(\"$marker\")) | .id" | head -n 1
}

# write_comment creates or updates this module's comment with the given file.
# With "update-only", it never creates one.
write_comment() {
  local body=$1 mode=${2:-}
  local existing
  existing="$(find_comment)" || return 1

  if [ -n "$existing" ]; then
    gh api --method PATCH "repos/$GSA_REPOSITORY/issues/comments/$existing" -F "body=@$body" > /dev/null
  elif [ "$mode" != "update-only" ]; then
    gh api --method POST "repos/$GSA_REPOSITORY/issues/$GSA_PR_NUMBER/comments" -F "body=@$body" > /dev/null
  fi
}

commenting() {
  if [ "$GSA_COMMENT" != "true" ]; then
    return 1
  fi
  if [ -z "${GSA_PR_NUMBER:-}" ] || [ -z "${GSA_REPOSITORY:-}" ]; then
    echo "::warning::Pull request number or repository unknown; not commenting"
    return 1
  fi
}

# finish_without_upgrades ends a run that has nothing to analyze. A report
# posted by an earlier run - before the upgrades were reverted, say - would
# otherwise stay on the pull request claiming something that is no longer true.
finish_without_upgrades() {
  echo "$1"
  set_output upgrades 0
  set_output breaking false

  if commenting; then
    {
      echo "$marker"
      echo "$title"
      echo
      echo "✅ This pull request no longer upgrades any direct dependency of this module; the earlier report no longer applies."
    } > "$tmp/comment.md"
    if ! write_comment "$tmp/comment.md" update-only; then
      echo "::warning::Could not update the earlier pull request comment."
    fi
  fi
  exit 0
}

# --- Check out the base commit and read the head go.mod ---------------------

echo "::group::Fetch base and head commits"
for sha in "$GSA_BASE_SHA" "$GSA_HEAD_SHA"; do
  # actions/checkout makes a shallow clone of the merge commit by default
  if ! git cat-file -e "$sha^{commit}" 2>/dev/null; then
    git fetch --no-tags --depth=1 origin "$sha"
  fi
done
echo "::endgroup::"

if ! git show "$GSA_HEAD_SHA:${prefix}go.mod" > "$tmp/head.go.mod" 2>/dev/null; then
  finish_without_upgrades "::notice::No go.mod at ${prefix:-the repository root} in the pull request head; nothing to audit"
fi

base_dir="$tmp/base"
git worktree add --detach --quiet "$base_dir" "$GSA_BASE_SHA"
trap 'git worktree remove --force "$base_dir" >/dev/null 2>&1 || true' EXIT
project="$base_dir/$prefix"

if [ ! -f "$project/go.mod" ]; then
  finish_without_upgrades "::notice::No go.mod at ${prefix:-the repository root} in the base commit; nothing to audit"
fi

"$bin" -path "$project" -list-upgrades "$tmp/head.go.mod" > "$tmp/upgrades.txt"
if [ ! -s "$tmp/upgrades.txt" ]; then
  finish_without_upgrades "No direct dependency upgrades in this pull request; nothing to audit."
fi

# --- Analyze each upgrade ------------------------------------------------------

count=0
broken=0
errors=0
sections="$tmp/sections.md"
: > "$sections"

while IFS= read -r upgrade; do
  if [ -z "$upgrade" ]; then
    continue
  fi
  count=$((count + 1))

  echo "::group::$upgrade"
  set +e
  # stdin is redirected so nothing in the analysis can consume the loop's input
  "$bin" -path "$project" -upgrade "$upgrade" -markdown > "$tmp/out.md" 2> "$tmp/err.txt" < /dev/null
  status=$?
  set -e
  cat "$tmp/err.txt" >&2
  echo "::endgroup::"

  case "$status" in
    0)
      cat "$tmp/out.md" >> "$sections"
      ;;
    1)
      broken=$((broken + 1))
      cat "$tmp/out.md" >> "$sections"
      ;;
    *)
      errors=$((errors + 1))
      echo "::warning::Could not analyze $upgrade"
      {
        echo "### \`${upgrade%@*}\` → ${upgrade##*@}"
        echo
        echo "❔ This upgrade could not be analyzed:"
        echo
        echo '```'
        tail -n 30 "$tmp/err.txt"
        echo '```'
        echo
      } >> "$sections"
      ;;
  esac
done < "$tmp/upgrades.txt"

# --- Assemble the report -------------------------------------------------------

report="$tmp/report.md"
{
  echo "$marker"
  echo "$title"
  echo
  if [ "$broken" -gt 0 ]; then
    echo "⚠️ **$broken of $count** dependency upgrades break code in this project."
  elif [ "$errors" -eq 0 ]; then
    echo "✅ None of the **$count** dependency upgrades break code in this project."
  fi
  if [ "$errors" -gt 0 ]; then
    echo "❔ **$errors of $count** upgrades could not be analyzed."
  fi
  echo
  cat "$sections"
  echo "---"
  echo "<sub>Static analysis of the exported APIs this project uses. Behavior changes behind an unchanged signature are not detected. Generated by <a href=\"https://github.com/devblac/go-semver-audit\">go-semver-audit</a>.</sub>"
} > "$report"

set_output upgrades "$count"
set_output breaking "$([ "$broken" -gt 0 ] && echo true || echo false)"
set_output report "$report"

if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
  cat "$report" >> "$GITHUB_STEP_SUMMARY"
fi

if commenting; then
  body="$tmp/comment.md"
  if [ "$(wc -c < "$report")" -gt "$MAX_COMMENT_BYTES" ]; then
    head -c "$MAX_COMMENT_BYTES" "$report" > "$body"
    printf '\n\n_Report truncated; the full report is in the job summary._\n' >> "$body"
  else
    cp "$report" "$body"
  fi
  # Update the previous report rather than adding a comment on every push
  if ! write_comment "$body"; then
    echo "::warning::Could not post the pull request comment. Does the workflow grant 'pull-requests: write'? The report is in the job summary."
  fi
elif [ "$GSA_COMMENT" != "true" ]; then
  cat "$report"
fi

# --- Exit status -----------------------------------------------------------------

if [ "$broken" -gt 0 ] && [ "$GSA_FAIL_ON_BREAKING" = "true" ]; then
  echo "::error::$broken of $count dependency upgrades break code in this project"
  exit 1
fi
if [ "$errors" -gt 0 ] && [ "$GSA_FAIL_ON_ERROR" = "true" ]; then
  echo "::error::$errors of $count dependency upgrades could not be analyzed"
  exit 1
fi
