// Package oldlib is a sample library v1.0.0 for testing
package oldlib

import "context"

// Config represents configuration
type Config struct {
	Name  string
	Value int
}

// ParseConfig parses configuration from a path
func ParseConfig(path string) (*Config, error) {
	return &Config{Name: "test", Value: 42}, nil
}

// OldHelper is a helper function that will be removed
func OldHelper() string {
	return "old"
}

// Transform transforms a string
func Transform(s string) string {
	return s
}

// Handler defines a handler interface
type Handler interface {
	Handle(ctx context.Context) error
	Close() error
}

// Result represents a result
type Result struct {
	Success bool
	Data    string
}
