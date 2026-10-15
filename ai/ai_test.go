package ai

import (
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"

	"writer/workspace"
	"writer/writing"
)

// Off, nothing that sends is built: the catalog of a service, the talk
// and the pen fail at once. On again, they are built as before.
func TestOffBuildsNothing(t *testing.T) {
	st, err := workspace.Create(filepath.Join(t.TempDir(), "work"), "测试作品", "网文")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := New("")
	var built atomic.Int32
	s.NewWriter = func(*workspace.Store) (*writing.Writer, error) {
		built.Add(1)
		return &writing.Writer{Store: st}, nil
	}
	s.SetOn(false)
	if _, err := s.Catalog("deepseek"); !errors.Is(err, ErrOff) {
		t.Fatalf("catalog: %v", err)
	}
	if _, err := s.Talk(st, nil, nil, nil); !errors.Is(err, ErrOff) {
		t.Fatalf("talk: %v", err)
	}
	if _, err := s.Pen(st); !errors.Is(err, ErrOff) {
		t.Fatalf("pen: %v", err)
	}
	if n := built.Load(); n != 0 {
		t.Fatalf("built %d with the AI off", n)
	}
	s.SetOn(true)
	if _, err := s.Pen(st); err != nil {
		t.Fatal(err)
	}
	if n := built.Load(); n != 1 {
		t.Fatalf("built %d with the AI on again", n)
	}
}
