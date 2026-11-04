package desk

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
)

// Every command the desk dispatches is in the help, in one row; every row
// speaks for keyed commands of the table or the prose's wheel gesture. The
// editor's cut, copy, paste and select all are hints of the prose's menu
// only: the help has never listed them, and its text stays as it is.
func TestTableHelpListsEachKeyedCommand(t *testing.T) {
	inHelp := map[*command]int{}
	for _, row := range commandHelp {
		for _, cmd := range row.cmds {
			inHelp[cmd]++
		}
	}
	for _, cmd := range commandList {
		switch {
		case cmd.key == 0 && inHelp[cmd] != 0:
			t.Errorf("%q has no key but is in the help", cmd.label())
		case cmd.key != 0 && inHelp[cmd] > 1:
			t.Errorf("%q is in %d help rows, want 1", cmd.label(), inHelp[cmd])
		case cmd.key != 0 && cmd.by == byDesk && inHelp[cmd] == 0:
			t.Errorf("the desk's %q is not in the help", cmd.label())
		}
	}
	for cmd := range inHelp {
		if !slices.Contains(commandList, cmd) {
			t.Errorf("the help names %q, which is not in the table", cmd.label())
		}
		if cmd.key == 0 || cmd.keys() == "" {
			t.Errorf("the help names %q, which has no key", cmd.label())
		}
	}
}

// No two commands of one page take the same key.
func TestTableOneKeyAtMostPerPage(t *testing.T) {
	type key struct {
		page page
		mods ui.Modifiers
		key  ui.Key
	}
	a := newApp("")
	defer a.Close()
	seen := map[key]*command{}
	for _, cmd := range a.commandList() {
		if cmd.key == 0 {
			continue
		}
		for _, p := range []page{pageTalk, pageWrite, pagePlan, pageCheck, pageCategories} {
			if cmd.page != pageAny && cmd.page != p {
				continue
			}
			k := key{p, cmd.mods, cmd.key}
			if was := seen[k]; was != nil {
				t.Errorf("%q and %q share a key on page %d", was.label(), cmd.label(), p)
			}
			seen[k] = cmd
		}
	}
}

// The help shows the table's rows in order, and the AI's only while it is
// on.
func TestHelpShowsTheTableRows(t *testing.T) {
	a, tt, _ := fixture(t)
	a.keysOpen = true
	tt.Frame()
	texts := tt.Texts()
	at := 0
	for _, row := range commandHelp {
		i := indexFrom(texts, at, row.keys())
		if i < 0 {
			t.Fatalf("the help does not list %q", row.keys())
		}
		if i+1 >= len(texts) || texts[i+1] != row.does() {
			t.Fatalf("%q does not say %q", row.keys(), row.does())
		}
		at = i + 1
	}
	a.turnAI(!(true))
	tt.Frame()
	texts = tt.Texts()
	for _, row := range commandHelp {
		if row.ai {
			if slices.Contains(texts, row.keys()) {
				t.Errorf("with the AI off the help still lists %q", row.keys())
			}
		} else if !slices.Contains(texts, row.keys()) {
			t.Errorf("with the AI off the help does not list %q", row.keys())
		}
	}
}

// Every hint a menu shows is its command's, from the table.
func TestMenuHintsComeFromCommands(t *testing.T) {
	a, _, id := writingFixture(t, "第一章的字。")
	shownEditor(a).Select(0, 2)
	later := func(do func()) func() { return do }
	check := func(m *kit.Menu) {
		t.Helper()
		for _, it := range m.Rows {
			if it.Kind != kit.RowItem || it.HintText() == "" {
				continue
			}
			var cmd *command
			for _, candidate := range a.commandList() {
				if candidate.name(a) == it.Label {
					cmd = candidate
					break
				}
			}
			if cmd == nil {
				t.Errorf("%q's hint %q is not from the table", it.Label, it.HintText())
				continue
			}
			if want := cmd.menuHint(a); it.HintText() != want {
				t.Errorf("%q shows %q, the table says %q", it.Label, it.HintText(), want)
			}
			if want := cmd.name(a); it.Label != want {
				t.Errorf("%q is called %q in the table", it.Label, want)
			}
		}
	}
	for _, bar := range []struct {
		name  string
		build func(m *kit.Menu)
	}{
		{"文件", func(m *kit.Menu) { a.fileMenu(m, later) }},
		{"编辑", func(m *kit.Menu) { a.editMenu(m, later) }},
		{"视图", func(m *kit.Menu) { a.viewMenu(m, later) }},
		{"帮助", func(m *kit.Menu) { a.helpMenu(m, later) }},
		{"正文", func(m *kit.Menu) { a.textMenu(m) }},
		{"标签页", func(m *kit.Menu) { shut := []string(nil); a.tabMenu(m, id, &shut) }},
	} {
		m := &kit.Menu{}
		bar.build(m)
		if len(m.Rows) == 0 {
			t.Errorf("the %s menu is empty", bar.name)
		}
		check(m)
	}
}

// indexFrom is the first place of s in texts at or after from.
func indexFrom(texts []string, from int, s string) int {
	for i := from; i < len(texts); i++ {
		if texts[i] == s {
			return i
		}
	}
	return -1
}

// Shortcuts are written as each system writes them: Ctrl+Shift+Tab on
// Windows, ⌃⇧Tab on macOS, never a Windows name on a Mac. ui.Cmd is
// Super on macOS and Ctrl elsewhere.
func TestShortcutsWrittenPerSystem(t *testing.T) {
	for _, c := range []struct {
		mac  bool
		mods ui.Modifiers
		key  ui.Key
		want string
	}{
		{false, ui.Ctrl, ui.KeyTab, "Ctrl+Tab"},
		{false, ui.Ctrl | ui.Shift, ui.KeyTab, "Ctrl+Shift+Tab"},
		{false, ui.Ctrl | ui.Shift, ui.KeyZ, "Ctrl+Shift+Z"},
		{false, 0, ui.KeyF2, "F2"},
		{true, ui.Ctrl, ui.KeyTab, "⌃Tab"},
		{true, ui.Ctrl | ui.Shift, ui.KeyTab, "⌃⇧Tab"},
		{true, ui.Super, ui.KeyS, "⌘S"},
		{true, ui.Super | ui.Shift, ui.KeyZ, "⇧⌘Z"},
	} {
		if got := shortcutOn(c.mac, c.mods, c.key); got != c.want {
			t.Errorf("mac=%v: %q, want %q", c.mac, got, c.want)
		}
	}
}
