package main

import (
	"context"

	"example.com/oldlib"
)

// MyHandler implements the Handler interface from oldlib
// This will break in v2 because the interface changes
type MyHandler struct {
	name string
}

func (h *MyHandler) Handle(ctx context.Context) error {
	return nil
}

func (h *MyHandler) Close() error {
	return nil
}

// Verify interface implementation
var _ oldlib.Handler = (*MyHandler)(nil)
