package desk

import (
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui"
	"writer/locales"
)

// commandBy says who takes a command's key. The desk's are dispatched from
// the page shown; the editor's and the talk box's keep their own keys, and
// are in the table for their menus and the help.
type commandBy int

const (
	byDesk commandBy = iota
	byEditor
	byBox
)

// pageAny is the page of a command that runs on every page.
const pageAny page = -1

// command is one thing the author can do, as the menus, the shortcuts and
// the help name it. run is what it does from its key or its menu item;
// where a menu does something of its own, such as closing the tab clicked,
// it gives that instead.
type command struct {
	label    func() string     // what the menus and the help call it, said when shown
	labelNow func(*App) string // what the state calls it, as 收起侧栏 / 展开侧栏
	key      ui.Key
	mods     ui.Modifiers // modCmd for the system's command key
	// macKey and macMods are its key on macOS where onMac says it has one
	// of its own; a zero macKey is none there.
	onMac   bool
	macKey  ui.Key
	macMods ui.Modifiers
	shifted bool // its key with Shift as well: = is + under Shift
	page    page // the page whose keys include it; pageAny for all
	ai      bool
	by      commandBy
	// systemMenu marks extension commands whose shortcuts the system menu owns.
	systemMenu bool
	// can says whether it can run now, for its menu. A menu with a finer
	// state of its own gives that instead, with Disabled.
	can func(*App) bool
	// when is what its key needs beyond the page, where that differs from
	// what a menu can say. Nil is the page alone.
	when func(*App) bool
	// hint is a menu's right text where it is not the key: a reason the row
	// cannot run.
	hint func(*App) string
	run  func(*App)
}

var (
	// The title bar's 文件.
	cmdNewChapter = &command{label: locales.CmdNewChapter, key: ui.KeyN, mods: modCmd, page: pageWrite, by: byDesk,
		can:  func(a *App) bool { return writingPage(a) && a.write.renaming == "" },
		when: func(a *App) bool { return a.write.renaming == "" },
		run:  func(a *App) { a.newNode(false) }}
	cmdNewFolder = &command{label: locales.CmdNewFolder, page: pageAny,
		can: func(a *App) bool { return writingPage(a) && a.write.renaming == "" },
		run: func(a *App) { a.newNode(true) }}
	cmdSave = &command{label: locales.CmdSave, key: ui.KeyS, mods: modCmd, page: pageWrite, by: byDesk,
		can: func(a *App) bool { return chapterShown(a) },
		run: func(a *App) { a.saveNow() }}
	cmdSettings = &command{label: locales.AppSettings, onMac: true, macKey: ui.KeyComma, macMods: modCmd, page: pageAny,
		run: func(a *App) { a.openPrefs(a.settings.tab) }}

	// The title bar's 编辑, and the prose's menu where it shares them.
	cmdUndo = &command{label: locales.CmdUndo, key: ui.KeyZ, mods: modCmd, page: pageWrite, by: byEditor,
		can: func(a *App) bool { open, busy := a.chapterOpen(); return open && !busy && a.write.cur().ed.CanUndo() },
		run: func(a *App) { a.write.cur().ed.Undo() }}
	cmdRedo = &command{label: locales.CmdRedo, key: ui.KeyZ, mods: modCmd | ui.Shift, page: pageWrite, by: byEditor,
		can: func(a *App) bool { open, busy := a.chapterOpen(); return open && !busy && a.write.cur().ed.CanRedo() },
		run: func(a *App) { a.write.cur().ed.Redo() }}
	cmdRename = &command{label: locales.CmdRename, key: ui.KeyF2, onMac: true, page: pageWrite, by: byDesk,
		can: func(a *App) bool {
			if !writingPage(a) || a.write.renaming != "" {
				return false
			}
			_, shown := a.book.Tree().Node(a.write.node) // a folder is renamed too
			return shown
		},
		when: func(a *App) bool { return a.write.renaming == "" },
		run:  func(a *App) { a.renameOpen() }}
	cmdContinue = &command{label: locales.CmdContinue, key: ui.KeyJ, mods: modCmd, page: pageWrite, by: byEditor, ai: true,
		can: func(a *App) bool { open, busy := a.chapterOpen(); return open && !busy },
		run: func(a *App) { a.continueText() }}
	cmdDraft = &command{label: locales.CmdDraft, page: pageAny, ai: true,
		can: func(a *App) bool {
			open, busy := a.chapterOpen()
			return open && !busy && strings.TrimSpace(a.chapterOutline(a.write.node)) != ""
		},
		hint: func(a *App) string {
			if open, _ := a.chapterOpen(); open && strings.TrimSpace(a.chapterOutline(a.write.node)) == "" {
				return locales.CmdDraftNeedsOutline()
			}
			return ""
		},
		run: func(a *App) { a.openDraft() }}

	// The title bar's 视图.
	cmdSide = &command{label: locales.CmdHideSidebar, page: pageAny,
		labelNow: func(a *App) string {
			if a.sideHidden {
				return locales.CmdShowSidebar()
			}
			return locales.CmdHideSidebar()
		},
		can: func(a *App) bool { return a.Store != nil },
		run: func(a *App) { a.fold.requested = true; a.setSideHidden(!a.sideHidden) }}
	cmdPane = &command{label: locales.CmdHidePane, page: pageAny,
		labelNow: func(a *App) string {
			if !a.paneOpen() {
				return locales.CmdShowPane()
			}
			return locales.CmdHidePane()
		},
		can: func(a *App) bool { return a.hasPane() },
		run: func(a *App) { a.fold.requested = true; a.togglePane() }}
	cmdHelp = &command{label: locales.CmdShortcuts, key: ui.KeySlash, mods: modCmd, page: pageAny, by: byDesk,
		run: func(a *App) { a.keysOpen = true }}

	// The prose's menu.
	cmdCut = &command{label: locales.CommonCut, key: ui.KeyX, mods: modCmd, page: pageWrite, by: byEditor,
		can: func(a *App) bool { return selected(a) && editable(a) },
		run: func(a *App) { a.write.cur().ed.Cut() }}
	cmdCopy = &command{label: locales.CommonCopy, key: ui.KeyC, mods: modCmd, page: pageWrite, by: byEditor,
		can: func(a *App) bool { return selected(a) },
		run: func(a *App) { a.write.cur().ed.Copy() }}
	cmdPaste = &command{label: locales.CommonPaste, key: ui.KeyV, mods: modCmd, page: pageWrite, by: byEditor,
		can: func(a *App) bool { return editable(a) },
		run: func(a *App) { a.write.cur().ed.Paste() }}
	cmdSelectAll = &command{label: locales.CommonSelectAll, key: ui.KeyA, mods: modCmd, page: pageWrite, by: byEditor,
		can: func(a *App) bool { return true },
		run: func(a *App) { a.write.cur().ed.SelectAll() }}
	cmdRewrite = &command{label: locales.EditorAIRewriteMore, page: pageWrite, ai: true,
		can: func(a *App) bool { return a.talkOn() && selected(a) && editable(a) },
		run: func(a *App) { a.openRewrite() }}
	cmdQuote = &command{label: locales.EditorQuote, key: ui.KeyL, mods: modCmd, page: pageWrite, by: byDesk, ai: true,
		can: func(a *App) bool { return a.talkOn() && selected(a) },
		run: func(a *App) { a.quoteToTalk() }}
	cmdTake = &command{label: locales.EditorAddToLore, page: pageWrite,
		can: func(a *App) bool { return a.hasEntries() && selected(a) },
		run: func(a *App) { a.openTake() }}
	cmdBury = &command{label: locales.EditorMarkForeshadow, page: pageWrite,
		can: func(a *App) bool { return selected(a) && editable(a) },
		run: func(a *App) { a.openBury() }}
	cmdPay = &command{label: locales.EditorMarkPay, page: pageWrite,
		can: func(a *App) bool { return selected(a) && editable(a) },
		run: func(a *App) { a.openPay("") }}
	cmdViewFore = &command{label: locales.EditorSeeMark, page: pageWrite,
		can: func(a *App) bool { return a.foreAt() != "" }}
	cmdUnmark = &command{label: locales.CmdUnmark, page: pageWrite,
		can: func(a *App) bool { return editable(a) && a.write.cur().ed.Marked() },
		run: func(a *App) { a.write.cur().ed.Unmark() }}

	// The tab's menu.
	cmdCloseTab = &command{label: locales.CommonClose, key: ui.KeyW, mods: modCmd, page: pageWrite, by: byDesk,
		when: func(a *App) bool { return a.write.node != "" },
		run:  func(a *App) { a.shutTabs([]string{a.write.node}) }}
	cmdCloseOthers = &command{label: locales.SideCloseOthers, page: pageWrite}
	cmdCloseRight  = &command{label: locales.SideCloseRight, page: pageWrite}

	// The writing page's other keys.
	cmdNextChapter = &command{label: locales.CmdNextChapter, key: ui.KeyPageDown, mods: modCmd, page: pageWrite, by: byDesk,
		run: func(a *App) { a.stepChapter(1) }}
	cmdPrevChapter = &command{label: locales.CmdPrevChapter, key: ui.KeyPageUp, mods: modCmd, page: pageWrite, by: byDesk,
		run: func(a *App) { a.stepChapter(-1) }}
	cmdNextTab = &command{label: locales.CmdNextTab, key: ui.KeyTab, mods: ui.Ctrl, page: pageWrite, by: byDesk,
		when: func(a *App) bool { return len(a.write.tabs) > 1 && tabAt(a) >= 0 },
		run:  func(a *App) { stepTab(a, 1) }}
	cmdPrevTab = &command{label: locales.CmdPrevTab, key: ui.KeyTab, mods: ui.Ctrl | ui.Shift, page: pageWrite, by: byDesk,
		when: func(a *App) bool { return len(a.write.tabs) > 1 && tabAt(a) >= 0 },
		run:  func(a *App) { stepTab(a, -1) }}
	cmdZoomIn = &command{label: locales.CmdZoomIn, key: ui.KeyEqual, mods: modCmd, shifted: true, page: pageAny, by: byDesk,
		can: func(a *App) bool { return a.zoomNext(1) != 0 },
		run: func(a *App) { a.zoomBy(1) }}
	cmdZoomOut = &command{label: locales.CmdZoomOut, key: ui.KeyMinus, mods: modCmd, page: pageAny, by: byDesk,
		can: func(a *App) bool { return a.zoomNext(-1) != 0 },
		run: func(a *App) { a.zoomBy(-1) }}
	cmdZoomReset = &command{label: locales.CmdZoomReset, key: ui.Key0, mods: modCmd, page: pageAny, by: byDesk,
		can: func(a *App) bool { return a.Zoom() != 1 },
		run: func(a *App) { a.zoomBy(0) }}

	// Keys the editor takes.
	cmdDeleteWordBack    = &command{label: locales.CmdDeleteWordBack, key: ui.KeyBackspace, mods: modCmd, onMac: true, macKey: ui.KeyBackspace, macMods: ui.Alt, page: pageWrite, by: byEditor}
	cmdDeleteWordForward = &command{label: locales.CmdDeleteWordForward, key: ui.KeyDelete, mods: modCmd, onMac: true, macKey: ui.KeyDelete, macMods: ui.Alt, page: pageWrite, by: byEditor}
	cmdTakePrediction    = &command{label: locales.CmdTakePrediction, key: ui.KeyTab, page: pageWrite, by: byEditor, ai: true}
	cmdDropPrediction    = &command{label: locales.CmdDropPrediction, key: ui.KeyEscape, page: pageWrite, by: byEditor, ai: true}

	// The talk's box keeps its own key.
	cmdSend = &command{label: locales.HelpSend, key: ui.KeyEnter, mods: modCmd, page: pageTalk, ai: true, by: byBox,
		can: func(a *App) bool { return a.busy == "" }}
)

// commandList is every command, in the order the desk tries its keys.
var commandList = []*command{
	cmdHelp,
	cmdQuote, cmdNewChapter, cmdRename,
	cmdZoomIn, cmdZoomOut, cmdZoomReset, cmdSave, cmdPrevChapter, cmdNextChapter,
	cmdCloseTab, cmdNextTab, cmdPrevTab,
	cmdUndo, cmdRedo, cmdContinue, cmdCut, cmdCopy, cmdPaste, cmdSelectAll,
	cmdDeleteWordBack, cmdDeleteWordForward, cmdTakePrediction, cmdDropPrediction,
	cmdNewFolder, cmdSettings, cmdDraft,
	cmdSide, cmdPane, cmdRewrite, cmdTake, cmdUnmark,
	cmdBury, cmdPay, cmdViewFore, cmdCloseOthers, cmdCloseRight, cmdSend,
}

// helpRow is one row of the help: the keys as the help writes them, what
// they do, and the commands it speaks for. Some rows join several
// commands, as Ctrl+PgDn / PgUp does.
type helpRow struct {
	group, does func() string
	ai          bool
	cmds        []*command
}

// keysOn is the row's keys as macOS or another system writes them, or ""
// when one of its commands has no key there. The row without commands is
// the wheel's.
func (r *helpRow) keysOn(mac bool) string {
	if len(r.cmds) == 0 {
		return shortcutOn(mac, cmdKeyOn(mac), 0) + locales.HelpWheel()
	}
	keys := make([]string, 0, len(r.cmds))
	for _, c := range r.cmds {
		k := c.keysOn(mac)
		if k == "" {
			return ""
		}
		keys = append(keys, k)
	}
	return strings.Join(keys, " / ")
}

// keys is the row's keys on the system the app runs on.
func (r *helpRow) keys() string { return r.keysOn(onMacOS) }

var commandHelp = []*helpRow{
	{group: locales.HelpGroupWrite, does: locales.HelpSave, cmds: []*command{cmdSave}},
	{group: locales.HelpGroupWrite, does: locales.HelpNewChapter, cmds: []*command{cmdNewChapter}},
	{group: locales.HelpGroupWrite, does: locales.HelpRename, cmds: []*command{cmdRename}},
	{group: locales.HelpGroupWrite, does: locales.HelpChapterNav, cmds: []*command{cmdNextChapter, cmdPrevChapter}},
	{group: locales.HelpGroupWrite, does: locales.HelpTabNav, cmds: []*command{cmdNextTab, cmdPrevTab}},
	{group: locales.HelpGroupWrite, does: locales.HelpCloseTab, cmds: []*command{cmdCloseTab}},
	{group: locales.HelpGroupWrite, does: locales.HelpContinue, ai: true, cmds: []*command{cmdContinue}},
	{group: locales.HelpGroupWrite, does: locales.HelpAcceptDrop, ai: true, cmds: []*command{cmdTakePrediction, cmdDropPrediction}},
	{group: locales.HelpGroupWrite, does: locales.HelpCite, ai: true, cmds: []*command{cmdQuote}},
	{group: locales.HelpGroupWrite, does: locales.HelpUndoRedo, cmds: []*command{cmdUndo, cmdRedo}},
	{group: locales.HelpGroupWrite, does: locales.HelpDeleteWord, cmds: []*command{cmdDeleteWordBack, cmdDeleteWordForward}},
	{group: locales.HelpGroupWrite, does: locales.HelpZoomText},
	{group: locales.HelpGroupChat, does: locales.HelpSend, ai: true, cmds: []*command{cmdSend}},
	{group: locales.HelpGroupGlobal, does: locales.HelpZoomUI, cmds: []*command{cmdZoomIn, cmdZoomOut, cmdZoomReset}},
	{group: locales.HelpGroupGlobal, does: locales.HelpOpenKeys, cmds: []*command{cmdHelp}},
}

// name is what a menu calls the command, or the help where it has no menu.
func (c *command) name(a *App) string {
	if c.labelNow != nil {
		return c.labelNow(a)
	}
	return c.label()
}

// available is whether the command can run now.
func (c *command) available(a *App) bool { return c.present(a) && (c.can == nil || c.can(a)) }

func (c *command) present(a *App) bool {
	if c == cmdDraft {
		return a.talkOn() && a.hasOutlines()
	}
	if c == cmdRewrite || c == cmdQuote {
		return a.talkOn()
	}
	return true
}

// menuHint is the right text of its menu row: the key, or a reason it
// cannot run.
func (c *command) menuHint(a *App) string {
	if c.hint != nil {
		return c.hint(a)
	}
	return c.keys()
}

// onMacOS says the app runs on macOS, whose keys the table writes its way.
var onMacOS = runtime.GOOS == "darwin"

// modCmd in the table is the system's command key: ⌘ on macOS, Ctrl
// elsewhere. ui.Cmd is that key only for the system the app runs on, and
// is Ctrl itself elsewhere; the table keeps a bit of its own, outside the
// toolkit's four, so that either system's keys can be written and tested
// from one, and Ctrl+Tab stays ⌃Tab on macOS.
const modCmd ui.Modifiers = 1 << 7

// cmdKeyOn is the command key on macOS or another system.
func cmdKeyOn(mac bool) ui.Modifiers {
	if mac {
		return ui.Super
	}
	return ui.Ctrl
}

// bindingOn is the command's key and modifiers on macOS or another
// system, modCmd made the system's own key.
func (c *command) bindingOn(mac bool) (ui.Modifiers, ui.Key) {
	mods, key := c.mods, c.key
	if mac && c.onMac {
		mods, key = c.macMods, c.macKey
	}
	if mods&modCmd != 0 {
		mods = mods&^modCmd | cmdKeyOn(mac)
	}
	return mods, key
}

// keysOn is the command's shortcut as the menus and the help write it on
// macOS or another system, or "" where it has none.
func (c *command) keysOn(mac bool) string {
	mods, key := c.bindingOn(mac)
	if key == 0 {
		return ""
	}
	return shortcutOn(mac, mods, key)
}

// keys is the command's shortcut on the system the app runs on.
func (c *command) keys() string { return c.keysOn(onMacOS) }

// pressed reports whether its key was pressed in the frame.
func (c *command) pressed(ctx *ui.Context) bool { return c.pressedOn(ctx, onMacOS) }

// pressedOn is pressed with the keys of macOS or another system.
func (c *command) pressedOn(ctx *ui.Context, mac bool) bool {
	mods, key := c.bindingOn(mac)
	return key != 0 && (ctx.Shortcut(mods, key) || c.shifted && ctx.Shortcut(mods|ui.Shift, key))
}

// act is what picking its menu row does.
func (c *command) act(a *App) func() {
	return func() {
		if c.run != nil {
			c.run(a)
		}
	}
}

// shortcutText writes a key and its modifiers as the menus and the help do,
// the way the system writes them.
func shortcutText(mods ui.Modifiers, key ui.Key) string {
	return shortcutOn(onMacOS, mods, key)
}

// shortcutOn is shortcutText on macOS or elsewhere. macOS writes the keys
// held down as symbols, in its own order, and some keys too: ⌃⇧Tab, ⇧⌘Z,
// ⌘↩, ⌘⇟. mods is the modifiers themselves, modCmd already made Super or
// Ctrl (bindingOn), so the same command reads ⌘S or Ctrl+S.
func shortcutOn(mac bool, mods ui.Modifiers, key ui.Key) string {
	name := keyText(key)
	if key == 0 {
		name = "" // the modifiers alone, before a word such as 滚轮
	}
	if mac {
		switch key {
		case ui.KeyEnter:
			name = "↩"
		case ui.KeyPageDown:
			name = "⇟"
		case ui.KeyPageUp:
			name = "⇞"
		case ui.KeyBackspace:
			name = "⌫"
		case ui.KeyDelete:
			name = "⌦"
		}
		held := ""
		for _, m := range []struct {
			mod ui.Modifiers
			sym string
		}{{ui.Ctrl, "⌃"}, {ui.Alt, "⌥"}, {ui.Shift, "⇧"}, {ui.Super, "⌘"}} {
			if mods&m.mod != 0 {
				held += m.sym
			}
		}
		return held + name
	}
	held := ""
	for _, m := range []struct {
		mod  ui.Modifiers
		name string
	}{{ui.Ctrl, "Ctrl+"}, {ui.Alt, "Alt+"}, {ui.Shift, "Shift+"}} {
		if mods&m.mod != 0 {
			held += m.name
		}
	}
	return held + name
}

// keyText names the keys the table uses, as the menus and the help do.
func keyText(key ui.Key) string {
	switch key {
	case ui.KeyF2:
		return "F2"
	case ui.KeyF11:
		return "F11"
	case ui.KeyPageUp:
		return "PgUp"
	case ui.KeyPageDown:
		return "PgDn"
	case ui.KeyBackspace:
		return "Backspace"
	case ui.KeyDelete:
		return "Delete"
	case ui.KeyEqual:
		return "="
	case ui.KeyMinus:
		return "-"
	case ui.Key0:
		return "0"
	case ui.KeyEnter:
		return "Enter"
	case ui.KeySlash:
		return "/"
	case ui.KeyTab:
		return "Tab"
	case ui.KeyEscape:
		return "Esc"
	case ui.KeyA:
		return "A"
	case ui.KeyC:
		return "C"
	case ui.KeyF:
		return "F"
	case ui.KeyJ:
		return "J"
	case ui.KeyL:
		return "L"
	case ui.KeyN:
		return "N"
	case ui.KeyS:
		return "S"
	case ui.KeyV:
		return "V"
	case ui.KeyW:
		return "W"
	case ui.KeyX:
		return "X"
	case ui.KeyY:
		return "Y"
	case ui.KeyZ:
		return "Z"
	}
	return "?"
}

// writingPage says the writing page is shown, with a work open.
func writingPage(a *App) bool { return a.Store != nil && a.page == pageWrite }

// chapterShown says a chapter, not a folder, is open on the writing page.
func chapterShown(a *App) bool {
	open, _ := a.chapterOpen()
	return open
}

// selected says some of the prose is chosen.
func selected(a *App) bool {
	if a.write.cur().ed == nil {
		return false
	}
	s, e := a.write.cur().ed.Selection()
	return s != e
}

// editable says the prose may be changed.
func editable(a *App) bool { return a.write.cur().ed != nil && !a.write.cur().ed.ReadOnly() }

// tabAt is the place of the shown node among the open tabs, or -1.
func tabAt(a *App) int {
	for i, id := range a.write.tabs {
		if id == a.write.node {
			return i
		}
	}
	return -1
}

// stepTab opens the tab d places from the one shown, round the row.
func stepTab(a *App, d int) {
	w := &a.write
	at := tabAt(a)
	if at < 0 || len(w.tabs) < 2 {
		return
	}
	a.openNode(w.tabs[(at+d+len(w.tabs))%len(w.tabs)])
}

// commandKeys dispatches the shortcuts of the page shown from the table,
// before the page builds: a key of the desk's is taken before the editor
// sees it. The AI's run only while it is on.
func (a *App) commandKeys(c *ui.Context) {
	for _, cmd := range a.commandList() {
		_, key := cmd.bindingOn(onMacOS)
		if key == 0 || cmd.by != byDesk || cmd.ai && !a.aiOn() {
			continue
		}
		if cmd.page != pageAny && cmd.page != a.page {
			continue
		}
		// The key is asked for before the condition, as each page asked
		// for it: while the condition does not hold the key is still the
		// desk's and goes nowhere, not on to the editor.
		pressed := false
		if onMacOS && systemMenuOwns(cmd) {
			// The system menu runs its rows' keys, so one that reaches
			// the window has its row greyed out, and goes nowhere. The
			// menu holds ⌘= and not ⌘+: a shifted key under Shift is
			// still the window's.
			mods, key := cmd.bindingOn(true)
			c.Shortcut(mods, key)
			pressed = cmd.shifted && c.Shortcut(mods|ui.Shift, key)
		} else {
			pressed = cmd.pressed(c)
		}
		if pressed && (cmd.when == nil || cmd.when(a)) && cmd.run != nil {
			cmd.run(a)
		}
	}
}

// pageTurn does the page's work before its build, and then dispatches the
// keys of the page shown.
func (a *App) pageTurn(c *ui.Context) {
	a.turnPage()
	if a.page == pageWrite {
		a.writingTurn(c)
	}
	a.commandKeys(c)
}
