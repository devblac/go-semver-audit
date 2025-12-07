// Package newlib is a sample library v2.0.0 for testing
package newlib

import "context"

// Config represents configuration
type Config struct {
	Name    string
	Value   int
	Enabled bool // New field
}

// Option is a functional option for configuration
type Option func(*Config)

// ParseConfig parses configuration from a path with options
// BREAKING: Signature changed - added options parameter
func ParseConfig(path string, opts ...Option) (*Config, error) {
	return &Config{Name: "test", Value: 42, Enabled: true}, nil
}

// OldHelper was removed in v2.0.0

// Transform transforms a string
func Transform(s string) string {
	return s
}

// NewHelper is a new helper function added in v2
func NewHelper() string {
	return "new"
}

// Metadata represents request metadata
type Metadata struct {
	RequestID string
}

// Handler defines a handler interface
// BREAKING: Method signature changed
type Handler interface {
	HandleWithContext(ctx context.Context, meta Metadata) error
	Close() error
}

// Result represents a result
type Result struct {
	Success bool
	Data    string
	Error   error // New field
}

