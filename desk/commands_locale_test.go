package desk

import (
	"testing"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

func TestCommandLabelsFollowLocale(t *testing.T) {
	tag := "en"
	i18n.SetLocaleFunc(func() string { return tag })
	t.Cleanup(func() { i18n.SetLocaleFunc(testLocale) })
	a := &App{}
	for _, c := range []struct{ tag, name, group, does string }{
		{"en", "New Chapter", "Write", "New chapter after this one"},
		{"zh-Hans", "新建章节", "正文", "在本章之后新建章节"},
	} {
		tag = c.tag
		if got := cmdNewChapter.name(a); got != c.name {
			t.Errorf("%s command: %q", tag, got)
		}
		row := commandHelp[1]
		if got := row.group(); got != c.group {
			t.Errorf("%s group: %q", tag, got)
		}
		if got := row.does(); got != c.does {
			t.Errorf("%s description: %q", tag, got)
		}
	}
}

func TestCommandPlatformBindings(t *testing.T) {
	cmdFocus := focusCommand(t)
	for _, c := range []struct {
		cmd      *command
		win, mac string
	}{
		{cmdRedo, "Ctrl+Shift+Z", "⇧⌘Z"},
		{cmdFocus, "F11", "⌃⌘F"},
		{cmdRename, "F2", ""},
		{cmdNextChapter, "Ctrl+PgDn", "⌘⇟"},
		{cmdPrevChapter, "Ctrl+PgUp", "⌘⇞"},
		{cmdDeleteWordBack, "Ctrl+Backspace", "⌥⌫"},
		{cmdDeleteWordForward, "Ctrl+Delete", "⌥⌦"},
		{cmdSend, "Ctrl+Enter", "⌘↩"},
		{cmdNextTab, "Ctrl+Tab", "⌃Tab"},
	} {
		for _, mac := range []bool{false, true} {
			want := c.win
			if mac {
				want = c.mac
			}
			if got := c.cmd.keysOn(mac); got != want {
				t.Errorf("%s mac=%v: %q want %q", c.cmd.name(&App{}), mac, got, want)
			}
		}
	}
	for _, row := range commandHelp {
		for _, cmd := range row.cmds {
			if cmd == cmdRename && row.keysOn(true) != "" {
				t.Fatal("mac help includes rename")
			}
		}
	}
}

func TestMacFocusDoesNotTriggerFind(t *testing.T) {
	cmdFocus := focusCommand(t)
	find, focus := false, false
	tt := ui.NewTester(func(c *ui.Context) {
		// Preserve commandList's order: Find is checked first.
		find = (&command{key: ui.KeyF, mods: modCmd}).pressedOn(c, true) || find
		focus = cmdFocus.pressedOn(c, true) || focus
	}, 800, 600)
	tt.Frame()
	tt.Key(ui.Ctrl|ui.Super, ui.KeyF)
	tt.Frame()
	if find || !focus {
		t.Fatalf("Ctrl+Super+F: find=%v focus=%v", find, focus)
	}
}
