package names

import (
	"path/filepath"
	"testing"
	"writer/desk/book"
	"writer/workspace"
)

func TestNameCacheSteadyFramesBuildNothingAndRenameAndHighlightsRebuildOnce(t *testing.T) {
	s, err := workspace.Create(filepath.Join(t.TempDir(), "book"), "book", "novel", "zh-Hans")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	b := book.Open(s)
	rev, err := b.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "name", Names: []string{"林晚"}}}})
	if err != nil {
		t.Fatal(err)
	}
	cache := nameCache{}
	show := true
	cache.get(b, show)
	steady := func() {
		t.Helper()
		before := cache.builds
		for range 10 {
			cache.get(b, show)
		}
		if cache.builds != before {
			t.Fatal("steady frames rebuilt names")
		}
	}
	steady()
	for _, change := range []struct {
		name string
		run  func()
	}{
		{"rename", func() {
			_, err := b.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{Op: workspace.OpName, Entry: rev.Ops[0].Entry, Names: []string{"苏晴"}}}})
			if err != nil {
				t.Fatal(err)
			}
		}},
		{"highlights off", func() { show = false }},
		{"highlights on", func() { show = true }},
		{"reload without a setting revision", func() {
			if err := b.ReloadSetting(); err != nil {
				t.Fatal(err)
			}
		}},
		{"new book object", func() { b = book.Open(s) }},
		{"shelf", func() { b = nil }},
	} {
		t.Run(change.name, func(t *testing.T) {
			before := cache.builds
			change.run()
			value := cache.get(b, show)
			if cache.builds != before+1 || value.Revision != cache.builds {
				t.Fatal("change must rebuild once", cache.builds-before)
			}
			if b != nil && (len(value.Entities) != 1 || value.Entities[0].Name != "苏晴" || (len(value.Links) != 0) != show) {
				t.Fatal(value)
			}
			steady()
		})
	}
}
