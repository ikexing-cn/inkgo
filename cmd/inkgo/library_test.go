package main

import (
	"os"
	"testing"
	"writer/desk"
)

// TestMain keeps a first shelf from finding the author's own works.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "library")
	if err != nil {
		panic(err)
	}
	desk.Library = func() string { return dir }
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
