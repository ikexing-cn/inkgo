package outline

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestExternalChapterAndVolumeOutlinesReloadThroughPagePolling(t *testing.T) {
	a, tt, f := outlineFixture(t)
	tt.Frame()
	for _, id := range []string{f.one, f.volume} {
		path := filepath.Join(a.Store.Root(), "ext", "outline", id+".txt")
		if err := os.WriteFile(path, []byte("external outline"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	a.plan.checked = time.Time{}
	tt.Frame()
	for _, id := range []string{f.one, f.volume} {
		if got := a.book.Outlines()[id]; got != "external outline" {
			t.Fatal(id, got)
		}
	}
}

func TestExternalWritingOutlineRefreshKeepsUnsavedDraft(t *testing.T) {
	for _, folder := range []bool{false, true} {
		t.Run(map[bool]string{false: "chapter", true: "volume"}[folder], func(t *testing.T) {
			a := newTestHost(t)
			id, err := a.Store.NewChapter("", "node", folder)
			if err != nil {
				t.Fatal(err)
			}
			if err := a.book.ReloadTree(); err != nil {
				t.Fatal(err)
			}
			a.OpenWriting(id)
			tt := ui.NewTester(func(c *ui.Context) { a.plan.WatchChapter(c) }, 1000, 730)
			tt.Frame()
			path := filepath.Join(a.Store.Root(), "ext", "outline", id+".txt")
			if err := os.WriteFile(path, []byte("external"), 0600); err != nil {
				t.Fatal(err)
			}
			a.plan.chapterChecked = time.Time{}
			tt.Frame()
			d := a.plan.Draft(id)
			if d.Text != "external" || d.Saved != "external" {
				t.Fatal(d)
			}
			d.Text = "unsaved"
			if err := os.WriteFile(path, []byte("another external edit"), 0600); err != nil {
				t.Fatal(err)
			}
			a.plan.chapterChecked = time.Time{}
			tt.Frame()
			if d.Text != "unsaved" {
				t.Fatal("unsaved draft overwritten", d)
			}
		})
	}
}
