package main

import (
	"os"
	"path/filepath"
	"testing"

	"writer/desk"
	"writer/internal/testtmp"
)

// TestMain keeps a first shelf from finding the author's own works: the
// library is in the run's own temporary folder, which os.TempDir gives
// once testtmp.Main has set it, so it is asked for when called.
func TestMain(m *testing.M) {
	desk.Library = func() string { return filepath.Join(os.TempDir(), "library") }
	testtmp.Main(m)
}
