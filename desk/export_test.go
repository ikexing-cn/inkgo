package desk

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/ext"
	exportext "writer/ext/export"
	"writer/ext/export/exporttext"
	"writer/internal/testshot"
	"writer/workspace"
)

func exportOpen(tt *ui.Tester) bool { return tt.HasText("Word") && tt.HasText("纯文本") }

func TestSharedTestAssemblyRegistersExportForEveryApp(t *testing.T) {
	a := newApp("")
	defer a.Close()
	if a.extensions.Ledger("export").State != ext.On {
		t.Fatal("export missing from shared assembly")
	}
	for _, c := range a.extensionCommands() {
		if c.id == "export.open" {
			if c.cmd.available(a) {
				t.Fatal("export available on shelf")
			}
			return
		}
	}
	t.Fatal("export command missing from shared assembly")
}

func closeExport(t *testing.T, tt *ui.Tester) {
	t.Helper()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	tt.Frame()
}

func runExport(t *testing.T, a *App) {
	t.Helper()
	for _, c := range a.extensionCommands() {
		if c.id == "export.open" {
			c.cmd.run(a)
			return
		}
	}
	t.Fatal("export command missing")
}

func choicesOf(t *testing.T, a *App) exportext.Choices {
	t.Helper()
	data, ok := a.extStore.Load(ext.Book{}, "export.choices")
	if !ok {
		t.Fatal("export choices missing")
	}
	var v exportext.Choices
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func saveChoices(t *testing.T, a *App, v exportext.Choices) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	a.extStore.Save(ext.Book{}, "export.choices", data)
}

func TestExportSwitchUpdatesMenusCommandsAndOpenDialogWithoutRestart(t *testing.T) {
	a, tt, _ := writingFixture(t, "灯灭了。")
	for _, on := range []bool{true, false, true, false, true} {
		if err := a.extensions.Set("export", on); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		m := &kit.Menu{}
		a.fileMenu(m, func(f func()) func() { return f })
		title := false
		for _, r := range m.Rows {
			title = title || r.Label == "导出…"
		}
		_, system := flattenSystemMenu(a.SystemMenu())["export.open"]
		command := false
		for _, c := range a.commandList() {
			command = command || c.name(a) == "导出…"
		}
		if title != on || system != on || command != on {
			t.Fatalf("on=%v title=%v system=%v command=%v", on, title, system, command)
		}
		if on {
			runExport(t, a)
			tt.Frame()
			if !exportOpen(tt) {
				t.Fatal("dialog did not open")
			}
		} else if exportOpen(tt) {
			t.Fatal("disabled overlay remains")
		}
	}
}

func TestLegacyExportChoicesMoveOnceWithoutReplacingExistingValue(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "move", true: "keep"}[existing], func(t *testing.T) {
			dir := t.TempDir()
			legacy := []byte("{\"outline\":true,\"text\":true,\"pen\":\"雨声\"}")
			works := filepath.Join(dir, "works.json")
			if err := os.WriteFile(works, append(append([]byte("{\"schema\":1,\"export\":"), legacy...), '}'), 0644); err != nil {
				t.Fatal(err)
			}
			want := exportext.Choices{Pen: "雨声"}
			want.Outline, want.Text = true, true
			if existing {
				want = exportext.Choices{Pen: "已有"}
				want.Prose = true
				data, _ := json.Marshal(want)
				newExtensionStore(dir, func(err error) { t.Fatal(err) }).Save(ext.Book{}, "export.choices", data)
			}
			a := newApp(filepath.Join(dir, "providers.yaml"))
			defer a.Close()
			if got := choicesOf(t, a); got != want {
				t.Fatalf("got %+v want %+v", got, want)
			}
			data, err := os.ReadFile(works)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(data, &fields); err != nil {
				t.Fatal(err)
			}
			if _, ok := fields["export"]; ok {
				t.Fatal("legacy export retained")
			}
			b := newApp(filepath.Join(dir, "providers.yaml"))
			defer b.Close()
			if got := choicesOf(t, b); got != want {
				t.Fatal("second load lost choices", got)
			}
		})
	}
}

func TestExportTracksPageRefreshAndWorkChanges(t *testing.T) {
	a, tt, id := writingFixture(t, "一二三")
	runExport(t, a)

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
	if tt.HasText("本章") {
		t.Fatal("page change left a stale current chapter", tt.Texts())
	}
	entry := setting(a).Entries[0].ID
	if _, err := a.Store.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{Op: workspace.OpMove, Entry: entry, Place: workspace.PlaceDropped}}}); err != nil {
		t.Fatal(err)
	}
	sessionPath := filepath.Join(a.Store.Root(), "ext", "talk", "sessions", talkOf(a).Session+".jsonl")
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionPath, []byte("{broken\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := a.refresh(); err == nil {
		t.Fatal("a later refresh error left the setting plan stale", err, tt.Texts())
	}
	tt.Frame()
	if err := tt.Click("设定"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("还没有已定的设定，不导出设定。") {
		t.Fatal("refresh error left the plan stale", tt.Texts())
	}
	if err := os.WriteFile(sessionPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := a.refresh(); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("还没有已定的设定，不导出设定。") {
		t.Fatal("setting refresh did not update the plan", tt.Texts())
	}
	other, err := workspace.Create(filepath.Join(t.TempDir(), "other"), "另一部", "网文", "zh-Hans")
	if err != nil {
		t.Fatal(err)
	}
	other.Close()
	if err := a.Open(other.Root()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if exportOpen(tt) {
		t.Fatal("switching works left the export dialog open")
	}
}

func TestExportOutlineFollowsGlobalAndBookSwitchWithoutLosingTick(t *testing.T) {
	for _, perBook := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "book"}[perBook], func(t *testing.T) {
			a, tt, _ := writingFixture(t, "Text")
			setChapterOutline(a, "Outline")
			v := exportext.Choices{}
			v.Outline, v.Text = true, true
			saveChoices(t, a, v)
			runExport(t, a)
			tt.Frame()
			flip := func(on bool) {
				t.Helper()
				var err error
				if perBook {
					err = a.extensions.SetBook("outline", on)
				} else {
					err = a.extensions.Set("outline", on)
				}
				if err != nil {
					t.Fatal(err)
				}
				tt.Frame()
			}
			flip(false)
			if tt.HasText("大纲") || !tt.HasText(exporttext.ExportAboutText()) || tt.HasText("范围") {
				t.Fatal(tt.Texts())
			}
			// Clicking the disabled export button must create no output.
			r, ok := tt.Find("取消")
			if !ok {
				t.Fatal(tt.Texts())
			}
			tt.ClickAt(r.X+r.W+32, r.Y+r.H/2)
			tt.Frame()
			if _, err := os.Stat(filepath.Join(a.Store.Root(), "导出")); !os.IsNotExist(err) {
				t.Fatal("disabled export wrote files", err)
			}
			if !choicesOf(t, a).Outline {
				t.Fatal("stored tick cleared")
			}
			flip(true)
			if !tt.HasText("大纲") {
				t.Fatal(tt.Texts())
			}
			r, _ = tt.Find("取消")
			tt.ClickAt(r.X+r.W+32, r.Y+r.H/2)
			tt.Frame()
			if _, err := os.Stat(filepath.Join(a.Store.Root(), "导出", "测试作品-大纲.txt")); err != nil {
				t.Fatal("restored tick did not export", err)
			}
		})
	}
}

func TestShotExportOutlineOff(t *testing.T) {
	testshot.Dir(t)
	a, tt, _ := writingFixture(t, "雨一直没有停。")
	a.setExtension("outline", false, false)
	runExport(t, a)
	shot(t, tt, "ext-export-outline-off.png")
}
