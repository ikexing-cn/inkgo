package desk

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"writer/export"
	"writer/workspace"
)

func TestExportTracksPageRefreshAndWorkChanges(t *testing.T) {
	a, tt, id := writingFixture(t, "一二三")
	a.exporting.Show()
	x := &a.exporting
	if err := a.Store.RenameChapter(id, "磁盘新名"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Store.NewChapter("", "磁盘新章", false); err != nil {
		t.Fatal(err)
	}
	a.checkDisk()
	tt.Frame()
	if err := a.Store.TrashChapter(id); err != nil {
		t.Fatal(err)
	}
	a.checkDisk()
	tt.Frame()
	showPage(a, pageTalk)
	tt.Frame()
	if x.Plan().Current != "" || slices.Contains(x.Plan().Scopes, export.ScopeChapter) {
		t.Fatal("page change left a stale current chapter", x.Plan())
	}
	entry := setting(a).Entries[0].ID
	if _, err := a.Store.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{Op: workspace.OpMove, Entry: entry, Place: workspace.PlaceDropped}}}); err != nil {
		t.Fatal(err)
	}
	sessionPath := filepath.Join(a.Store.Root(), "sessions", a.session+".jsonl")
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionPath, []byte("{broken\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := a.refresh(); err == nil || x.Plan().Skip[export.SettingContent] != "还没有已定的设定，不导出设定。" {
		t.Fatal("a later refresh error left the setting plan stale", err, x.Plan())
	}
	if err := os.WriteFile(sessionPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := a.refresh(); err != nil {
		t.Fatal(err)
	}
	if x.Plan().Skip[export.SettingContent] != "还没有已定的设定，不导出设定。" {
		t.Fatal("setting refresh did not update the plan", x.Plan())
	}
	other, err := workspace.Create(filepath.Join(t.TempDir(), "other"), "另一部", "网文")
	if err != nil {
		t.Fatal(err)
	}
	other.Close()
	if err := a.Open(other.Root()); err != nil {
		t.Fatal(err)
	}
	if a.exporting.Open {
		t.Fatal("switching works left the export dialog open")
	}
}
