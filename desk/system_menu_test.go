package desk

import (
	"reflect"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
	"writer/internal/testshot"
	"writer/locales"
)

func flattenSystemMenu(rows []SystemMenuItem) map[string]SystemMenuItem {
	out := map[string]SystemMenuItem{}
	var visit func([]SystemMenuItem)
	visit = func(rows []SystemMenuItem) {
		for _, r := range rows {
			if _, ok := out[r.ID]; ok {
				panic("duplicate menu ID")
			}
			out[r.ID] = r
			visit(r.Children)
		}
	}
	visit(rows)
	return out
}

func TestSystemMenuStructureAndState(t *testing.T) {
	a, tt, _ := writingFixture(t, "雨一直下。")
	rows := a.SystemMenu()
	want := [][]string{
		{"about", "app-settings-separator", "settings", "app-hide-separator", "hide", "hide-others", "app-quit-separator", "quit"},
		{"new-chapter", "new-folder", "file-save-separator", "save", "snapshots", "export", "file-organize-separator", "categories", "trash"},
		{"undo", "redo", "edit-find-separator", "find", "rename", "edit-ai-separator", "continue", "draft"},
		{"side", "pane", "focus", "phone", "view-zoom-separator", "zoom-label", "zoom-in", "zoom-out", "zoom-reset", "view-look-separator", "look-label", "look-system", "look-light", "look-dark"},
		{"minimize", "window-zoom", "window-front-separator", "front"}, {"shortcuts"},
	}
	for i, id := range []string{"app", "file", "edit", "view", "window", "help"} {
		if rows[i].ID != id {
			t.Fatal(rows)
		}
		var ids []string
		for _, r := range rows[i].Children {
			ids = append(ids, r.ID)
		}
		if !reflect.DeepEqual(ids, want[i]) {
			t.Fatal(id, ids)
		}
	}
	all := flattenSystemMenu(rows)
	for id, role := range map[string]string{"app": "appMenu", "about": "about", "hide": "hide", "hide-others": "hideOthers", "quit": "quit", "undo": "undo", "redo": "redo", "window": "windowMenu", "minimize": "minimize", "window-zoom": "zoom", "front": "front", "help": "help"} {
		if all[id].Role != role || (id != "window" && all[id].Label != "") {
			t.Fatal(id, all[id])
		}
	}
	for _, entry := range systemCommands {
		if all[entry.id].Enabled != entry.cmd.available(a) || all[entry.id].Accelerator != systemAccelerator(entry.cmd) {
			t.Fatal(entry.id)
		}
	}
	for _, v := range []string{"", "light", "dark"} {
		a.shelf.Theme = v
		all = flattenSystemMenu(a.SystemMenu())
		n := 0
		for _, id := range []string{"look-system", "look-light", "look-dark"} {
			if all[id].Checked {
				n++
			}
		}
		if n != 1 {
			t.Fatal(v, n)
		}
	}
	a.sideHidden = true
	a.shelf.NoAI = true
	all = flattenSystemMenu(a.SystemMenu())
	if all["side"].Label != locales.CmdShowSidebar() {
		t.Fatal(all["side"])
	}
	for _, id := range []string{"edit-ai-separator", "continue", "draft"} {
		if all[id].Visible {
			t.Fatal(id)
		}
	}
	a.QueueSystemMenu("shortcuts")
	if a.keysOpen {
		t.Fatal("ran outside frame")
	}
	tt.Frame()
	if !a.keysOpen {
		t.Fatal("queued click lost")
	}
}

func TestSystemMenuAccelerators(t *testing.T) {
	for _, tc := range []struct {
		cmd  *command
		want string
	}{{cmdNewChapter, "Cmd+N"}, {cmdRedo, "Shift+Cmd+Z"}, {cmdFocus, "Ctrl+Cmd+F"}, {cmdZoomIn, "Cmd+="}, {cmdZoomOut, "Cmd+-"}, {cmdZoomReset, "Cmd+0"}, {cmdHelp, "Cmd+/"}, {cmdSettings, "Cmd+,"}, {cmdRename, ""}} {
		if got := systemAccelerator(tc.cmd); got != tc.want {
			t.Errorf("got %q want %q", got, tc.want)
		}
	}
}

func TestSystemMenuPlatformTitleAndKeys(t *testing.T) {
	old := onMacOS
	defer func() { onMacOS = old }()
	for _, mac := range []bool{false, true} {
		onMacOS = mac
		a, tt, _ := writingFixture(t, "雨。")
		tt.Frame()
		for _, name := range []string{locales.MenuFile(), locales.MenuEdit(), locales.MenuView(), locales.MenuHelp()} {
			if slices.Contains(tt.Texts(), "菜单栏-"+name) == mac {
				t.Fatal(mac, name)
			}
		}
		if mac {
			tt.Key(ui.Super, ui.KeySlash)
			tt.Frame()
			if a.keysOpen {
				t.Fatal("window dispatched menu shortcut")
			}
			if !systemMenuOwns(cmdHelp) || !systemMenuOwns(cmdSave) || systemMenuOwns(cmdRename) {
				t.Fatal("shortcut ownership")
			}
			// The menu holds ⌘=; ⌘+ (Shift with =) is still the window's.
			zoom := a.Zoom()
			tt.Key(ui.Super, ui.KeyEqual)
			tt.Frame()
			if a.Zoom() != zoom {
				t.Fatal("window ran the menu's ⌘=")
			}
			tt.Key(ui.Super|ui.Shift, ui.KeyEqual)
			tt.Frame()
			if a.Zoom() <= zoom {
				t.Fatal("⌘+ lost on macOS")
			}
		}
	}
}

func TestShotMacTitleBar(t *testing.T) {
	testshot.Dir(t)
	old := onMacOS
	onMacOS = true
	defer func() { onMacOS = old }()
	_, tt, _ := writingFixture(t, "雨从傍晚开始下，一直没有停。")
	tt.SetTitleBar(ui.TitleBar{Height: 40, Left: 80})
	tt.Frame()
	shot(t, tt, "titlebar-macos.png")
}

func TestSystemMenuDisabledShortcutDoesNotFallThrough(t *testing.T) {
	oldMac, oldCan, oldRun := onMacOS, cmdHelp.can, cmdHelp.run
	defer func() { onMacOS, cmdHelp.can, cmdHelp.run = oldMac, oldCan, oldRun }()
	onMacOS = true
	a, _, _ := fixture(t)
	calls := 0
	cmdHelp.run = func(*App) { calls++ }
	for _, enabled := range []bool{true, false} {
		cmdHelp.can = func(*App) bool { return enabled }
		tt := stillTester(a.commandKeys, 800, 600)
		tt.Frame()
		tt.Key(ui.Super, ui.KeySlash)
		tt.Frame()
		if calls != 0 {
			t.Fatal("window ran menu command", enabled)
		}
		if flattenSystemMenu(a.SystemMenu())["shortcuts"].Enabled != enabled {
			t.Fatal("menu availability differs")
		}
	}
}
