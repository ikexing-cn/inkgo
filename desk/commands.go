package desk

import (
	"strings"

	"github.com/egoist/mygo/ui"
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
	label    string
	labelNow func(*App) string // what the state calls it, as 收起侧栏 / 展开侧栏
	key      ui.Key
	mods     ui.Modifiers
	page     page // the page whose keys include it; pageAny for all
	ai       bool
	by       commandBy
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
	cmdNewChapter = &command{label: "新建章节", key: ui.KeyN, mods: ui.Cmd, page: pageWrite, by: byDesk,
		can:  func(a *App) bool { return writingPage(a) && a.write.renaming == "" },
		when: func(a *App) bool { return a.write.renaming == "" },
		run:  func(a *App) { a.newNode(false) }}
	cmdNewFolder = &command{label: "新建文件夹", page: pageAny,
		can: func(a *App) bool { return writingPage(a) && a.write.renaming == "" },
		run: func(a *App) { a.newNode(true) }}
	cmdSave = &command{label: "保存", key: ui.KeyS, mods: ui.Cmd, page: pageWrite, by: byDesk,
		can: func(a *App) bool { return chapterShown(a) },
		run: func(a *App) { a.saveNow() }}
	cmdSnapshots = &command{label: "快照", page: pageAny,
		can: func(a *App) bool { open, busy := a.chapterOpen(); return open && !busy },
		run: func(a *App) { a.openSnapshots() }}
	cmdExport = &command{label: "导出…", page: pageAny,
		can: func(a *App) bool { return a.Store != nil },
		run: func(a *App) { a.exporting.Show() }}
	cmdSettings = &command{label: "设置…", page: pageAny,
		run: func(a *App) { a.openPrefs(a.bk.tab) }}
	cmdCategories = &command{label: "作品分类", page: pageAny,
		can: func(a *App) bool { return a.Store != nil },
		run: func(a *App) { a.page = pageCategories }}
	cmdTrash = &command{label: "回收站…", page: pageAny,
		can: func(a *App) bool { return a.Store != nil },
		run: func(a *App) { a.openTrash() }}

	// The title bar's 编辑, and the prose's menu where it shares them.
	cmdUndo = &command{label: "撤销", key: ui.KeyZ, mods: ui.Cmd, page: pageWrite, by: byEditor,
		can: func(a *App) bool { open, busy := a.chapterOpen(); return open && !busy && a.write.cur().ed.CanUndo() },
		run: func(a *App) { a.write.cur().ed.Undo() }}
	cmdRedo = &command{label: "重做", key: ui.KeyY, mods: ui.Cmd, page: pageWrite, by: byEditor,
		can: func(a *App) bool { open, busy := a.chapterOpen(); return open && !busy && a.write.cur().ed.CanRedo() },
		run: func(a *App) { a.write.cur().ed.Redo() }}
	cmdFind = &command{label: "查找替换", key: ui.KeyF, mods: ui.Cmd, page: pageWrite, by: byDesk,
		can: func(a *App) bool { open, busy := a.chapterOpen(); return open && !busy },
		run: func(a *App) { a.openFind() }}
	cmdRename = &command{label: "重命名", key: ui.KeyF2, page: pageWrite, by: byDesk,
		can: func(a *App) bool {
			if !writingPage(a) || a.write.renaming != "" {
				return false
			}
			_, shown := a.book.Tree().Node(a.write.node) // a folder is renamed too
			return shown
		},
		when: func(a *App) bool { return a.write.renaming == "" },
		run:  func(a *App) { a.renameOpen() }}
	cmdContinue = &command{label: "续写一句", key: ui.KeyJ, mods: ui.Cmd, page: pageWrite, by: byEditor, ai: true,
		can: func(a *App) bool { open, busy := a.chapterOpen(); return open && !busy },
		run: func(a *App) { a.continueText() }}
	cmdDraft = &command{label: "起草本章", page: pageAny, ai: true,
		can: func(a *App) bool {
			open, busy := a.chapterOpen()
			return open && !busy && strings.TrimSpace(a.write.cur().outline.Text) != ""
		},
		hint: func(a *App) string {
			if open, _ := a.chapterOpen(); open && strings.TrimSpace(a.write.cur().outline.Text) == "" {
				return "请先填写章纲"
			}
			return ""
		},
		run: func(a *App) { a.openDraft() }}

	// The title bar's 视图.
	cmdSide = &command{label: "收起侧栏", page: pageAny,
		labelNow: func(a *App) string {
			if a.sideHidden {
				return "展开侧栏"
			}
			return "收起侧栏"
		},
		can: func(a *App) bool { return a.Store != nil },
		run: func(a *App) { a.setSideHidden(!a.sideHidden) }}
	cmdPane = &command{label: "收起右栏", page: pageAny,
		labelNow: func(a *App) string {
			if !a.paneOpen() {
				return "展开右栏"
			}
			return "收起右栏"
		},
		can: func(a *App) bool { return a.hasPane() },
		run: func(a *App) { a.togglePane() }}
	cmdFocus = &command{label: "专注写作", key: ui.KeyF11, page: pageWrite, by: byDesk,
		can: func(a *App) bool { return writingPage(a) },
		run: func(a *App) { a.write.focus = !a.write.focus }}
	cmdPhone = &command{label: "手机预览", page: pageAny,
		can: func(a *App) bool { return chapterShown(a) },
		run: func(a *App) { a.openPhone() }}
	cmdHelp = &command{label: "快捷键…", key: ui.KeySlash, mods: ui.Cmd, page: pageAny, by: byDesk,
		run: func(a *App) { a.keysOpen = true }}

	// The prose's menu.
	cmdCut = &command{label: "剪切", key: ui.KeyX, mods: ui.Cmd, page: pageWrite, by: byEditor,
		can: func(a *App) bool { return selected(a) && editable(a) },
		run: func(a *App) { a.write.cur().ed.Cut() }}
	cmdCopy = &command{label: "复制", key: ui.KeyC, mods: ui.Cmd, page: pageWrite, by: byEditor,
		can: func(a *App) bool { return selected(a) },
		run: func(a *App) { a.write.cur().ed.Copy() }}
	cmdPaste = &command{label: "粘贴", key: ui.KeyV, mods: ui.Cmd, page: pageWrite, by: byEditor,
		can: func(a *App) bool { return editable(a) },
		run: func(a *App) { a.write.cur().ed.Paste() }}
	cmdSelectAll = &command{label: "全选", key: ui.KeyA, mods: ui.Cmd, page: pageWrite, by: byEditor,
		can: func(a *App) bool { return true },
		run: func(a *App) { a.write.cur().ed.SelectAll() }}
	cmdRewrite = &command{label: "AI 改写…", page: pageWrite, ai: true,
		can: func(a *App) bool { return selected(a) && editable(a) },
		run: func(a *App) { a.openRewrite() }}
	cmdQuote = &command{label: "引用到讨论", key: ui.KeyL, mods: ui.Cmd, page: pageWrite, by: byDesk, ai: true,
		can: func(a *App) bool { return selected(a) },
		run: func(a *App) { a.quoteToTalk() }}
	cmdTake = &command{label: "加入设定", page: pageWrite,
		can: func(a *App) bool { return selected(a) },
		run: func(a *App) { a.openTake() }}
	cmdBury = &command{label: "记为伏笔…", page: pageWrite,
		can: func(a *App) bool { return selected(a) && editable(a) },
		run: func(a *App) { a.openBury() }}
	cmdPay = &command{label: "记为回收…", page: pageWrite,
		can: func(a *App) bool { return selected(a) && editable(a) },
		run: func(a *App) { a.openPay("") }}
	cmdViewFore = &command{label: "查看伏笔", page: pageWrite,
		can: func(a *App) bool { return a.foreAt() != "" }}
	cmdUnmark = &command{label: "移除此处标记", page: pageWrite,
		can: func(a *App) bool { return editable(a) && a.write.cur().ed.Marked() },
		run: func(a *App) { a.write.cur().ed.Unmark() }}

	// The tab's menu.
	cmdCloseTab = &command{label: "关闭", key: ui.KeyW, mods: ui.Cmd, page: pageWrite, by: byDesk,
		when: func(a *App) bool { return a.write.node != "" },
		run:  func(a *App) { a.shutTabs([]string{a.write.node}) }}
	cmdCloseOthers = &command{label: "关闭其他", page: pageWrite}
	cmdCloseRight  = &command{label: "关闭右侧", page: pageWrite}

	// The writing page's other keys.
	cmdNextChapter = &command{label: "下一章", key: ui.KeyPageDown, mods: ui.Cmd, page: pageWrite, by: byDesk,
		run: func(a *App) { a.stepChapter(1) }}
	cmdPrevChapter = &command{label: "上一章", key: ui.KeyPageUp, mods: ui.Cmd, page: pageWrite, by: byDesk,
		run: func(a *App) { a.stepChapter(-1) }}
	cmdNextTab = &command{label: "下一个标签页", key: ui.KeyTab, mods: ui.Ctrl, page: pageWrite, by: byDesk,
		when: func(a *App) bool { return len(a.write.tabs) > 1 && tabAt(a) >= 0 },
		run:  func(a *App) { stepTab(a, 1) }}
	cmdPrevTab = &command{label: "上一个标签页", key: ui.KeyTab, mods: ui.Ctrl | ui.Shift, page: pageWrite, by: byDesk,
		when: func(a *App) bool { return len(a.write.tabs) > 1 && tabAt(a) >= 0 },
		run:  func(a *App) { stepTab(a, -1) }}
	cmdZoomIn = &command{label: "增大字号", key: ui.KeyEqual, mods: ui.Cmd, page: pageWrite, by: byDesk,
		run: func(a *App) { a.sizeText(1) }}
	cmdZoomOut = &command{label: "减小字号", key: ui.KeyMinus, mods: ui.Cmd, page: pageWrite, by: byDesk,
		run: func(a *App) { a.sizeText(-1) }}
	cmdZoomReset = &command{label: "复原字号", key: ui.Key0, mods: ui.Cmd, page: pageWrite, by: byDesk,
		run: func(a *App) { a.sizeText(0) }}

	// Keys the editor takes.
	cmdDeleteWordBack    = &command{label: "向前删除一个词", key: ui.KeyBackspace, mods: ui.Cmd, page: pageWrite, by: byEditor}
	cmdDeleteWordForward = &command{label: "向后删除一个词", key: ui.KeyDelete, mods: ui.Cmd, page: pageWrite, by: byEditor}
	cmdTakePrediction    = &command{label: "采纳续写", key: ui.KeyTab, page: pageWrite, by: byEditor, ai: true}
	cmdDropPrediction    = &command{label: "放弃续写", key: ui.KeyEscape, page: pageWrite, by: byEditor, ai: true}

	// The talk's box keeps its own key.
	cmdSend = &command{label: "发送", key: ui.KeyEnter, mods: ui.Cmd, page: pageTalk, ai: true, by: byBox,
		can: func(a *App) bool { return a.busy == "" }}
)

// commandList is every command, in the order the desk tries its keys.
var commandList = []*command{
	cmdHelp,
	cmdFind, cmdQuote, cmdFocus, cmdNewChapter, cmdRename,
	cmdZoomIn, cmdZoomOut, cmdZoomReset, cmdSave, cmdPrevChapter, cmdNextChapter,
	cmdCloseTab, cmdNextTab, cmdPrevTab,
	cmdUndo, cmdRedo, cmdContinue, cmdCut, cmdCopy, cmdPaste, cmdSelectAll,
	cmdDeleteWordBack, cmdDeleteWordForward, cmdTakePrediction, cmdDropPrediction,
	cmdNewFolder, cmdSnapshots, cmdExport, cmdSettings, cmdDraft,
	cmdCategories, cmdTrash,
	cmdSide, cmdPane, cmdPhone, cmdRewrite, cmdTake, cmdUnmark,
	cmdBury, cmdPay, cmdViewFore, cmdCloseOthers, cmdCloseRight, cmdSend,
}

// helpRow is one row of the help: the keys as the help writes them, what
// they do, and the commands it speaks for. Some rows join several
// commands, as Ctrl+PgDn / PgUp does.
type helpRow struct {
	keys, does string
	group      string
	ai         bool
	cmds       []*command
}

var commandHelp = []*helpRow{
	{group: "正文", keys: modKey + "S", does: "立即保存（停笔片刻也会自动保存）", cmds: []*command{cmdSave}},
	{group: "正文", keys: modKey + "F", does: "查找替换，可切换到全书", cmds: []*command{cmdFind}},
	{group: "正文", keys: modKey + "N", does: "在本章之后新建章节", cmds: []*command{cmdNewChapter}},
	{group: "正文", keys: "F2", does: "重命名当前章节", cmds: []*command{cmdRename}},
	{group: "正文", keys: modKey + "PgDn / PgUp", does: "下一章 / 上一章", cmds: []*command{cmdNextChapter, cmdPrevChapter}},
	{group: "正文", keys: "Ctrl+Tab / Ctrl+Shift+Tab", does: "下一个 / 上一个标签页", cmds: []*command{cmdNextTab, cmdPrevTab}},
	{group: "正文", keys: modKey + "W", does: "关闭当前标签页", cmds: []*command{cmdCloseTab}},
	{group: "正文", keys: "F11", does: "专注写作：仅显示正文", cmds: []*command{cmdFocus}},
	{group: "正文", keys: modKey + "J", does: "在光标处续写一句", ai: true, cmds: []*command{cmdContinue}},
	{group: "正文", keys: "Tab / Esc", does: "采纳 / 放弃续写", ai: true, cmds: []*command{cmdTakePrediction, cmdDropPrediction}},
	{group: "正文", keys: modKey + "L", does: "将选中的原文引用到讨论", ai: true, cmds: []*command{cmdQuote}},
	{group: "正文", keys: modKey + "Z / " + modKey + "Y", does: "撤销 / 重做", cmds: []*command{cmdUndo, cmdRedo}},
	{group: "正文", keys: modKey + "Backspace / " + modKey + "Delete", does: "向前 / 向后删除一个词", cmds: []*command{cmdDeleteWordBack, cmdDeleteWordForward}},
	{group: "正文", keys: modKey + "滚轮 · " + modKey + "= / " + modKey + "- / " + modKey + "0", does: "正文字号增大 / 减小 / 复原", cmds: []*command{cmdZoomIn, cmdZoomOut, cmdZoomReset}},
	{group: "讨论", keys: modKey + "Enter", does: "发送", ai: true, cmds: []*command{cmdSend}},
	{group: "全局", keys: modKey + "/", does: "打开快捷键列表", cmds: []*command{cmdHelp}},
}

// name is what a menu calls the command, or the help where it has no menu.
func (c *command) name(a *App) string {
	if c.labelNow != nil {
		return c.labelNow(a)
	}
	return c.label
}

// available is whether the command can run now.
func (c *command) available(a *App) bool { return c.can == nil || c.can(a) }

// menuHint is the right text of its menu row: the key, or a reason it
// cannot run.
func (c *command) menuHint(a *App) string {
	if c.hint != nil {
		return c.hint(a)
	}
	return c.keys()
}

// keys is the command's shortcut as the menus and the help write it.
func (c *command) keys() string {
	if c.key == 0 {
		return ""
	}
	return shortcutText(c.mods, c.key)
}

// pressed reports whether its key was pressed in the frame.
func (c *command) pressed(ctx *ui.Context) bool {
	return c.key != 0 && ctx.Shortcut(c.mods, c.key)
}

// act is what picking its menu row does.
func (c *command) act(a *App) func() {
	return func() {
		if c.run != nil {
			c.run(a)
		}
	}
}

// shortcutText writes a key and its modifiers as the menus and the help do.
func shortcutText(mods ui.Modifiers, key ui.Key) string {
	name := keyText(key)
	switch {
	case mods&ui.Cmd != 0:
		return modKey + name
	case mods&ui.Ctrl != 0 && mods&ui.Shift != 0:
		return "Ctrl+Shift+" + name
	case mods&ui.Ctrl != 0:
		return "Ctrl+" + name
	case mods&ui.Shift != 0:
		return "Shift+" + name
	}
	return name
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
	for _, cmd := range commandList {
		if cmd.key == 0 || cmd.by != byDesk || cmd.ai && !a.aiOn() {
			continue
		}
		if cmd.page != pageAny && cmd.page != a.page {
			continue
		}
		// The key is asked for before the condition, as each page asked
		// for it: while the condition does not hold the key is still the
		// desk's and goes nowhere, not on to the editor.
		if cmd.pressed(c) && (cmd.when == nil || cmd.when(a)) && cmd.run != nil {
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

func (a *App) openPhone() {
	if open, _ := a.chapterOpen(); !open {
		return
	}
	a.doc.hidden, a.doc.tab, a.write.focus = false, panePhone, false
	a.savePrefs()
}
