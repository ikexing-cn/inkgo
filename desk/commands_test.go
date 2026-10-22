package desk

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

// Every command the desk dispatches is in the help, in one row; every row
// speaks for a keyed command of the table, and for nothing else. The
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
			t.Errorf("%q has no key but is in the help", cmd.label)
		case cmd.key != 0 && inHelp[cmd] > 1:
			t.Errorf("%q is in %d help rows, want 1", cmd.label, inHelp[cmd])
		case cmd.key != 0 && cmd.by == byDesk && inHelp[cmd] == 0:
			t.Errorf("the desk's %q is not in the help", cmd.label)
		}
	}
	for cmd := range inHelp {
		if !slices.Contains(commandList, cmd) {
			t.Errorf("the help names %q, which is not in the table", cmd.label)
		}
		if cmd.key == 0 || cmd.keys() == "" {
			t.Errorf("the help names %q, which has no key", cmd.label)
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
	seen := map[key]*command{}
	for _, cmd := range commandList {
		if cmd.key == 0 {
			continue
		}
		for _, p := range []page{pageTalk, pageWrite, pagePlan, pageCheck, pageCategories} {
			if cmd.page != pageAny && cmd.page != p {
				continue
			}
			k := key{p, cmd.mods, cmd.key}
			if was := seen[k]; was != nil {
				t.Errorf("%q and %q share a key on page %d", was.label, cmd.label, p)
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
		i := indexFrom(texts, at, row.keys)
		if i < 0 {
			t.Fatalf("the help does not list %q", row.keys)
		}
		if i+1 >= len(texts) || texts[i+1] != row.does {
			t.Fatalf("%q does not say %q", row.keys, row.does)
		}
		at = i + 1
	}
	a.shelf.NoAI = true
	tt.Frame()
	texts = tt.Texts()
	for _, row := range commandHelp {
		if row.ai {
			if slices.Contains(texts, row.keys) {
				t.Errorf("with the AI off the help still lists %q", row.keys)
			}
		} else if !slices.Contains(texts, row.keys) {
			t.Errorf("with the AI off the help does not list %q", row.keys)
		}
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
