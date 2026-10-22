package desk

import (
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/aiport"
	"writer/editor"
	"writer/prefs"
	"writer/workspace"
)

type writeView struct {
	entered bool            // the page read the tree as it showed
	folded  map[string]bool // folders folded, by ID
	node    string          // the node shown
	// back and forward are the nodes shown before and after it, for the
	// toolbar's arrows; stepping is set while they open one.
	back, forward []string
	stepping      bool
	// tabs are the nodes open, left to right, node the one of them shown;
	// parked is what the others keep while they are not; closing, the tabs
	// whose closing waits for an answer about the words not saved; blank,
	// that the author closed them all; tabSeen, the tab the row last
	// showed as the one open, which it scrolled to.
	tabs    []string
	parked  map[string]*sheet
	closing []string
	blank   bool
	tabSeen string
	ed      *editor.Editor
	// What the disk has of the node shown, as of the last read or write.
	saved    string
	savedVer int
	// counted is the text the open chapter's count was last taken of.
	counted    *editor.Doc
	countedVer int
	stamp      workspace.Stamp
	lastSnap   time.Time
	checked    time.Time
	// The outline, saved as the author pauses like the text.
	outline, outlineSaved, outlinePrev string
	outlineAt                          time.Time
	outlineOpen                        bool
	// changed, whether the prose of the chapter shown was saved since it
	// opened.
	changed  bool
	counter  prefs.Counter
	goalText string // the goal as typed
	goalOpen bool
	offered  string            // the numbered title a new node's name began with
	size     int               // the text's size, or 0 for the default
	gists    map[string]string // a line on each chapter of the folder open
	carets   map[string]int    // where the caret was in each chapter, as it was left
	// The disk's text, changed outside while the node was open; lost, the
	// node gone from the disk.
	external  *workspace.Chapter
	lost      bool
	lostTitle string

	renaming, title string
	titleWas        string // the name kept when the title is left empty
	titleHeld       bool   // the box for the title has had the caret: losing it settles the name
	titleAll        bool   // the name the box opens with is selected, to be typed over
	focus           bool   // the page alone, without the tree and the column
	finding         bool
	findAgain       bool // the caret goes to the find box, its words selected
	query, with     string
	current         int
	wide            bool              // finding in the whole book
	sure            bool              // asking before replacing through the book
	texts           map[string]string // the chapters' text while finding in the book
	hits            []bookHit
	hitsFor         [3]any // the query, chapter and edit the hits were found for

	// named is the editor last given the names, as of namedRevs revisions
	// of the setting.
	named     *editor.Editor
	namedRevs int

	// What is pending: the list open, showing the open chapter's alone, the
	// groups listed whole, and the keys of what the author ignored.
	pendOpen bool
	pendHere int
	pendMore map[string]bool
	ignored  map[string]bool

	// The foreshadowings: what their column and popovers show, and how
	// many chapters may follow one before it is buried long (0: the
	// default).
	fore      foreState
	lateAfter int

	snapsOpen bool
	snaps     []workspace.Snapshot
	snap      string // the snapshot chosen
	snapText  string
	compare   bool
	snapShown snapView

	trashOpen bool
	trash     []workspace.Trashed
	purge     string // the trashed node whose purge awaits confirmation

	takeOpen            bool
	takeQuote, takeText string
	takeCategory        string
	exporting           exportState

	// The writing model: the writer the editors predict with, and why
	// there is none.
	pen         atomic.Pointer[aiport.Pen]
	penKey      *penKey // nil when there is no key to compare with
	penErr      error
	autoPredict bool

	// What the AI does in the prose: the rewrite asked for, the dialog
	// that asks for a draft, and the changes proposed in talks, the calls
	// of the turn in flight taken in among them by talk and call, as last
	// placed in the prose; and what waits on letting them go.
	rewriting   bool
	instruction string
	draftOpen   bool
	words       string
	draftMode   string
	draftNote   string
	draftCards  int // the chapter's cards when the draft dialog opened
	props       []*prop
	propVer     int
	seen        map[string]bool
	placed      placedKey
	leaving     func()
}

// warn shows err, leaving an error shown alone when there is none.
func (a *App) warn(err error) {
	if err != nil {
		a.SetError(err)
	}
}

// writingTurn is the writing page's work before its keys and its build:
// the tree as the page shows, the editor, the counts and the names, and
// saving as the author pauses.
func (a *App) writingTurn(c *ui.Context) {
	if !a.write.entered {
		a.write.entered = true
		a.enterWriting()
	}
	a.keepPen()
	a.pruneProps()
	a.countAsTyped()
	a.feedNames()
	if a.write.ed != nil {
		a.write.ed.SetTypewriter(a.write.focus) // focused, the line written on stays mid-view
	}
	a.watch(c)
	a.checkWhilePausing()
	a.markDoubts()
}

// writing is the writing page: the node shown between the chapter tree in
// the sidebar and the setting.
func (a *App) writing(c *ui.Context) {
	// Escape backs out of the find bar first, then out of a pick in the
	// prose, so the bar over the pick goes with it.
	if ed := a.write.ed; ed != nil && c.Shortcut(0, ui.KeyEscape) {
		if s, e := ed.Selection(); a.write.finding {
			a.closeFind()
			ed.Focus()
		} else if s != e && ed.Focused() {
			ed.Select(ed.Caret(), ed.Caret())
		}
	}
	ui.Row(c).FillWidth().Grow(1).AlignItems(ui.Stretch).Children(func() {
		main := ui.Column(c).Grow(1).Shrink(1).FillHeight()
		a.LastWrite = main.Bounds()
		main.Children(func() {
			a.writeTop(c)
			a.sheets(c)
			n, ok := a.book.Tree().Node(a.write.node)
			switch {
			case a.write.lost && a.write.ed != nil:
				a.chapterView(c)
			case !ok && (a.book.Tree().Root == nil || len(a.book.Tree().Root.Children) == 0):
				a.emptyWriting(c)
				a.footBar(c, false)
			case !ok:
				a.pal.blank(c, icBook, "从左侧选择一章开始写作", "", nil)
				a.footBar(c, false)
			case n.Folder:
				a.folderView(c, n)
				a.footBar(c, false)
			default:
				a.chapterView(c)
			}
		})
		a.settingColumn(c)
	})
	if a.write.snapsOpen {
		a.snapshots(c)
	}
	if a.write.takeOpen {
		a.take(c)
	}
	if a.write.draftOpen {
		a.draftView(c)
	}
	a.askToSave(c)
}

// settingColumn is the setting beside the page, unless the author hid it.
func (a *App) settingColumn(c *ui.Context) {
	if a.page == pagePlan && !a.plan.pane || a.page != pagePlan && a.doc.hidden || a.page == pageWrite && a.write.focus {
		return
	}
	win, _ := c.Size()
	width := a.panelWidth(win)
	ui.Box(c).Width(1).FillHeight().Background(a.pal.line)
	panel := ui.Column(c).Width(width).FillHeight().Background(a.pal.side).Shrink(0)
	a.LastSetting = panel.Bounds()
	panel.Children(func() {
		if a.page == pageWrite && a.doc.tab == panePhone {
			a.phonePanel(c)
		} else if a.page == pageWrite && a.doc.tab == paneFore {
			a.forePanel(c)
		} else if a.page == pageWrite && a.talkTab() {
			a.talkPanel(c)
		} else {
			a.settingPanel(c)
		}
		a.resizer(c, width)
	})
}

// paneTop is the top row of the right pane.
func paneTop(c *ui.Context) ui.Element {
	return ui.Row(c).FillWidth().Height(titleHeight).Padding(0, 20).Gap(4).AlignItems(ui.Center).Shrink(0)
}

func (a *App) writeTop(c *ui.Context) {
	t := a.pal.theme
	w := &a.write
	n, ok := a.book.Tree().Node(w.node)
	top := ui.Row(c).FillWidth().Height(titleHeight).Padding(0, 16, 0, 24).Gap(8).AlignItems(ui.Center).Shrink(0)
	if w.focus {
		// No title bar over it: this row holds the window, and keeps clear
		// of the window's buttons.
		bar := c.TitleBar()
		top.Padding(0, 16+bar.Right, 0, 24+bar.Left).DragWindow()
	}
	top.Children(func() {
		ui.Row(c).Gap(2).AlignItems(ui.Center).Margin(0, 0, 0, -8).Shrink(0).Children(func() {
			if a.stepButton(c, icBack, "上一个打开的章节").Disabled(len(w.back) == 0).Clicked() {
				a.stepHistory(false)
			}
			if a.stepButton(c, icForward, "下一个打开的章节").Disabled(len(w.forward) == 0).Clicked() {
				a.stepHistory(true)
			}
		})
		if ok {
			var path []string
			for p := n.Parent; p != nil && p.Parent != nil; p = p.Parent {
				path = append([]string{p.Title}, path...)
			}
			for _, title := range path {
				ui.Text(c, title).FontSize(fsControl).TextColor(t.TextMuted).SingleLine().Shrink(1)
				glyph(c, icChevron, iconSm, a.pal.faint)
			}
			ui.Text(c, n.Title).FontSize(fsControl).FontWeight(600).SingleLine().Ellipsis("…").Shrink(1)
		} else if w.lost {
			ui.Text(c, w.lostTitle).FontSize(fsControl).FontWeight(600)
		} else {
			ui.Text(c, "正文").FontSize(fsControl).FontWeight(600)
		}
		if ok && !n.Folder && w.ed != nil {
			if w.ed.Predicting() {
				ui.Text(c, "续写中…").FontSize(fsNote).TextColor(a.pal.faint).SingleLine().Shrink(0).Margin(0, 0, 0, 6)
			} else if w.ed.Ghost() != "" {
				ui.Text(c, "Tab 采纳 · Esc 放弃").FontSize(fsNote).TextColor(a.pal.faint).SingleLine().Shrink(0).Margin(0, 0, 0, 6)
			}
		}
		ui.Spacer(c)
		if ok && !n.Folder {
			busy := w.ed.ReadOnly() || w.lost || w.external != nil
			color := ui.Color{}
			if w.finding {
				color = a.pal.ink
			}
			if iconButton(c, icSearch, "查找替换", color).Tooltip("查找替换（" + modKey + "F）").Disabled(busy).Clicked() {
				if w.finding {
					a.closeFind()
				} else {
					a.openFind()
				}
			}
		}
		if a.doc.hidden && !w.focus {
			if iconButton(c, icPanel, "展开右栏", ui.Color{}).Tooltip("展开右栏").Clicked() {
				a.setDocHidden(false)
			}
		}
		if w.focus {
			if iconButton(c, icShrink, "退出专注", a.pal.ink).Tooltip("退出专注，恢复章节树和右栏（F11）").Clicked() {
				w.focus = false
			}
		} else if iconButton(c, icExpand, "专注写作", ui.Color{}).Tooltip("仅显示正文，收起章节树和右栏（F11）").Clicked() {
			w.focus = true
		}
		a.backupChip(c)
		a.pendingButton(c)
	})
	a.pal.hairline(c)
}

// stepButton goes to the chapter opened before the one shown, or after it:
// smaller than the tools at the row's other end.
func (a *App) stepButton(c *ui.Context, s *ui.SVG, label string) ui.Element {
	t := a.pal.theme
	b := ui.ButtonBase(c).Size(iconLg, iconLg).Center().Radius(rControl).Shrink(0).Label(label).Tooltip(label)
	fg := t.TextMuted
	if b.Hovered() {
		b.Background(t.SurfacePressed)
		fg = t.Text
	}
	if b.IsDisabled() {
		pale(b)
	}
	b.Children(func() { glyph(c, s, iconSm, fg) })
	return b
}

func (a *App) emptyWriting(c *ui.Context) {
	a.pal.blank(c, icBook, "暂无章节", "每章是作品文件夹 chapters 中的一个纯文本文件，可用其他软件打开。", func() {
		ui.Row(c).Gap(8).Padding(10, 0, 0, 0).Justify(ui.End).Children(func() {
			if a.pal.secondary(c, "新建文件夹").Clicked() {
				a.newNode(true)
			}
			if a.pal.primary(c, "新建第一章").Clicked() {
				a.newNode(false)
			}
		})
	})
}

// outlineBox is the outline of the node shown: 章纲 of a document, 卷纲 of
// a folder.
func (a *App) outlineBox(c *ui.Context, label string, height float32) {
	w := &a.write
	ui.TextAreaBase(c, &w.outline).Label(label).Placeholder("写下这里要发生的事，作为提纲").FillWidth().Height(height).
		FontSize(fsBody).LineHeight(lhLoose).Padding(10, 12).Radius(rControl).Background(a.pal.side).Disabled(w.lost)
}

func (a *App) chapterView(c *ui.Context) {
	t := a.pal.theme
	w := &a.write
	if w.finding && !w.ed.ReadOnly() {
		a.findBar(c)
	}
	if w.external != nil {
		a.notice2(c, t.Warning, "本章文件已被其他程序修改。请选择保留的版本，另一版本将存为快照。", func() {
			if action(c, "使用磁盘版本", t.Warning, false).Clicked() {
				a.takeDisk()
			}
			if action(c, "保留当前版本", t.Warning, true).Clicked() {
				a.keepMine()
			}
		})
	}
	if w.lost {
		a.notice2(c, t.Danger, "磁盘上已找不到本章文件：在软件外改名、移动或删除，均视为删除。编辑器中的内容仍保留。", func() {
			if action(c, "存为新章节", t.Danger, true).Clicked() {
				a.saveLost()
			}
		})
	}
	// The chapter's title, then its outline, opening under it. Keyed, as
	// the bars over it come and go.
	overProse(c.Key("head"), 24, 6, func() {
		ui.Column(c).FillWidth().Gap(8).Children(func() {
			ui.Text(c, heading(a.shownTitle())).FontSize(fsTitle).FontWeight(700).SingleLine().Ellipsis("…")
			label := "章纲"
			if strings.TrimSpace(w.outline) == "" {
				label = "章纲 · 空"
			}
			var outline ui.CollapsibleParts
			ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
				outline = a.disclosure(c, label, &w.outlineOpen)
				a.chapterStory(c)
			})
			outline.Panel(func() {
				ui.Box(c).FillWidth().Padding(2, 0, 4, 0).Children(func() { a.outlineBox(c, "章纲", 96) })
			})
		})
	})
	a.placeProps()
	a.aiBar(c)
	// Keyed, as the bars over it come and go: the prose keeps the keyboard.
	ui.Box(c.Key("prose")).Grow(1).FillWidth().Children(func() {
		prose := w.ed.View(c).Fill()
		a.LastProse = prose.Bounds()
		a.propActs(c)
		a.selectionBar(c)
		a.forePopover(c)
		a.menuAt(c, "正文", a.textMenu)
	})
	a.footBar(c, true)
}

// shownTitle is the title of the chapter shown.
func (a *App) shownTitle() string {
	w := &a.write
	if n, ok := a.book.Tree().Node(w.node); ok {
		return n.Title
	}
	if w.lost {
		return w.lostTitle
	}
	return "正文"
}

// heading is a chapter's title over its prose: the name after 第几章, which
// the breadcrumb above already says.
func heading(title string) string {
	if _, name, ok := strings.Cut(title, "："); ok && strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return title
}

// disclosure is a quiet toggle over the prose, its panel built apart from it
// so that several sit on one row.
func (a *App) disclosure(c *ui.Context, label string, open *bool) ui.CollapsibleParts {
	t := a.pal.theme
	p := ui.CollapsibleBase(c, open)
	color := t.TextMuted
	if p.Trigger.Hovered() || *open {
		color = t.Text
	}
	arrow := " ▾"
	if *open {
		arrow = " ▴"
	}
	p.Trigger.Label(label).Padding(2, 0).Children(func() {
		ui.Text(c, label+arrow).FontSize(fsNote).TextColor(color).SingleLine()
	})
	return p
}

// notice2 is a banner over the editor that asks the author something.
func (a *App) notice2(c *ui.Context, color ui.Color, text string, buttons func()) {
	t := a.pal.theme
	ui.Row(c).FillWidth().Padding(10, 20, 0, 20).Shrink(0).Children(func() {
		ui.Row(c).FillWidth().Padding(8, 8, 8, 12).Gap(10).Radius(rCard).AlignItems(ui.Center).Background(color.Alpha(0.08)).Border(1, color.Alpha(0.2)).Children(func() {
			glyph(c, icAlert, iconMd, color)
			ui.Text(c, text).FontSize(fsControl).TextColor(color.Mix(t.Text, 0.35)).Grow(1).Shrink(1)
			buttons()
		})
	})
}

// folderView shows a folder: its outline and what it holds.
func (a *App) folderView(c *ui.Context, n *workspace.Node) {
	t := a.pal.theme
	w := &a.write
	ui.Scroll(c.Key("folder")).Grow(1).FillWidth().Children(func() {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(32, editor.PadX, 32, editor.PadX).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(720).Gap(14).Children(func() {
				ui.Text(c, n.Title).FontSize(fsTitle).FontWeight(700)
				ui.Text(c, "卷纲").FontSize(fsControl).FontWeight(600).TextColor(t.TextMuted)
				a.outlineBox(c, "卷纲", 160)
				ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(8, 0, 0, 0).Children(func() {
					ui.Textf(c, "%d 项", len(n.Children)).FontSize(fsControl).FontWeight(600).TextColor(t.TextMuted).Grow(1)
					if a.pal.ghost(c, "在此新建章节", ui.Color{}).Clicked() {
						a.newNode(false)
					}
				})
				for _, child := range n.Children {
					row := ui.ButtonBase(c.Key("f-"+child.ID)).Column().FillWidth().Gap(4).Padding(10, 12).Radius(rControl).Label("打开-" + child.Title)
					if row.Hovered() {
						row.Background(t.SurfaceHover)
					}
					row.Children(func() {
						ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(10).Children(func() {
							glyph(c, nodeIcon(child), iconMd, t.TextMuted)
							ui.Text(c, child.Title).FontSize(fsBody).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
							if !child.Folder {
								ui.Textf(c, "%d 字", a.book.Count(child.ID)).FontSize(fsNote).TextColor(a.pal.faint)
							}
						})
						if gist := w.gists[child.ID]; gist != "" {
							ui.Text(c, gist).FontSize(fsNote).TextColor(t.TextMuted).FillWidth().Padding(0, 0, 0, 16+10).MaxLines(2).Ellipsis("…") // under the title, past the icon
						}
					})
					if row.Clicked() {
						a.openNode(child.ID)
					}
				}
			})
		})
	})
}

// overProse lays fn out as wide as the prose under it and in line with it:
// the chapter's head and the AI's bar over the text start where it starts.
func overProse(c *ui.Context, top, bottom float32, fn func()) {
	ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(top, editor.PadX, bottom, editor.PadX).Shrink(0).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(editor.MaxTextWidth).Children(fn)
	})
}

// footBar is the line at the foot of the writing page: of a chapter, with
// the AI on, where its check stands and when chapters are checked; at the
// right, the words.
func (a *App) footBar(c *ui.Context, chapter bool) {
	t := a.pal.theme
	w := &a.write
	k := &a.ck
	a.pal.hairline(c)
	foot := ui.Row(c).FillWidth().Height(hRow).Gap(8).Padding(0, 24).AlignItems(ui.Center).Shrink(0)
	roomy := foot.Bounds().W == 0 || foot.Bounds().W >= 600 // the counts give way to the check
	wide := foot.Bounds().W == 0 || foot.Bounds().W >= 760  // and today's to the book's
	foot.Children(func() {
		if !chapter || !a.aiOn() {
			ui.Spacer(c)
			a.wordCount(c, roomy, wide)
			return
		}
		line, found, can, tip := a.checkState()
		if found > 0 {
			b := a.pal.ghost(c, line, ui.Color{}).Label("查看检查结果").Tooltip("前往「检查」页逐条查看").Shrink(1000)
			if b.Clicked() {
				for _, it := range a.checkItems(false) {
					if it.here {
						a.openCheck(it.f.ID)
						break
					}
				}
			}
		} else {
			said := ui.Text(c, line).FontSize(fsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Shrink(1000)
			if tip != "" {
				said.Tooltip(tip)
			}
		}
		if can {
			b := ui.ButtonBase(c).Label("立即检查").Tooltip("调用一次 AI，对照设定检查本章").Shrink(0)
			color := t.TextMuted
			if b.Hovered() {
				color = t.Text
			}
			b.Children(func() { ui.Text(c, "立即检查").FontSize(fsNote).TextColor(color).Underline().SingleLine() })
			if b.Clicked() {
				if err := a.saveText(); err != nil {
					a.warn(err)
				} else {
					a.ck.queue.Enqueue(w.node)
				}
			}
		}
		ui.Text(c, "·").FontSize(fsNote).TextColor(a.pal.faint)
		short := checkWhens[0].short
		for _, o := range checkWhens {
			if o.when == k.when {
				short = o.short
			}
		}
		b := ui.ButtonBase(c).Label("检查时机").Shrink(0)
		color := t.TextMuted
		if b.Hovered() {
			color = t.Text
		}
		b.Children(func() { ui.Text(c, short+" ▴").FontSize(fsNote).TextColor(color).SingleLine() })
		if b.Clicked() {
			k.whenOpen = !k.whenOpen
		}
		ui.PopoverBase(c, b, &k.whenOpen, func(panel ui.Element) {
			panel.AttachTo(b, ui.AnchorTopLeft, ui.AnchorBottomLeft).Margin(0, 0, 4, 0).Width(270).Radius(rCard).Padding(10, 6, 6, 6).Gap(4).
				Background(a.pal.raised).Border(1, t.Border)
			a.pal.pop.on(panel)
			ui.Text(c, "检查时机").FontSize(fsBody).FontWeight(600).Padding(0, 8, 4, 8)
			ui.Column(c).FillWidth().Gap(2).Children(func() { a.whenRows(c) })
		})
		ui.Spacer(c)
		a.wordCount(c, roomy, wide)
	})
}
