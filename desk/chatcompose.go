package desk

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"strings"
	"time"
	"unicode/utf8"
	"writer/aiport"
	"writer/desk/kit"
	"writer/editor"
	"writer/locales"
	"writer/workspace"
)

// composer is one rounded field: the text, then the model it goes to and a
// round send button. It grows with the text up to a few lines.
func (a *App) composer(c *ui.Context) {
	t := a.pal.Theme
	work := a.Store.Work()
	ready := work.Discussion.Provider != "" && work.Discussion.Model != ""
	side := a.page == pageWrite
	// The chip names the model and effort the next turn goes to, and
	// opens a menu to change either; its tooltip says where the words go.
	chip := func() {
		chip := ui.ButtonBase(c).Row().Gap(6).AlignItems(ui.Center).Padding(4, 8).Radius(kit.RControl).Label("当前模型").Tooltip(a.destination(work.Discussion)).Shrink(1).Disabled(a.held())
		if chip.Hovered() || a.menu.MenuOpen("讨论模型") {
			chip.Background(t.SurfaceHover)
		}
		chip.Children(func() {
			if ready {
				color := t.Success
				if !isLocal(a.providerURL(work.Discussion.Provider)) {
					color = a.pal.Ink
				}
				kit.Dot(c, color, 6)
				ui.Text(c, work.Discussion.Model).FontSize(kit.FsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Shrink(1)
				ui.Text(c, a.ai.EffortLabel(work.Discussion.Effort)).FontSize(kit.FsNote).TextColor(a.pal.Faint).SingleLine()
			} else {
				kit.Dot(c, t.Warning, 6)
				ui.Text(c, "选择讨论模型").FontSize(kit.FsNote).TextColor(t.Warning).SingleLine()
			}
			kit.Glyph(c, kit.IcChevronUp, kit.IconXs, a.pal.Faint)
		})
		a.menu.DropMenu(c, a.pal, chip, "讨论模型", func(panel ui.Element) {
			panel.TransformOrigin(1, 1)
			panel.AttachTo(chip, ui.AnchorTopRight, ui.AnchorBottomRight).Margin(0, 0, 4, 0).Width(220)
		}, func(m *kit.Menu) { a.modelMenu(m, work) })
	}
	send := func() {
		can := a.busy == "" && kit.Trim(a.input) != ""
		send := ui.ButtonBase(c).Size(kit.HControl, kit.HControl).Radius(kit.RRound).Center().Label("发送").Tooltip(locales.TalkSend(cmdSend.keys())).Disabled(!can).Shrink(0)
		bg := a.pal.Ink
		if !can {
			bg = t.SurfacePressed
		} else if send.Hovered() {
			bg = t.AccentHover
		}
		kit.Squeeze(send)
		send.Background(bg).Children(func() {
			fg := a.pal.OnInk
			if !can {
				fg = a.pal.Faint
			}
			kit.Glyph(c, kit.IcSend, kit.IconMd, fg)
		})
		if a.busy == "" && (send.Clicked() || cmdSend.pressed(c)) {
			a.send(c)
		}
	}
	field := func(hint string, per, least int) ui.Element {
		lines := strings.Count(a.input, "\n") + 1 + len([]rune(a.input))/per
		h := float32(min(max(lines, least), 7)) * 25
		in := ui.TextAreaBase(c, &a.input).Label("讨论输入").Placeholder(hint).Height(h).FontSize(kit.FsRead).LineHeight(kit.LhLoose).Disabled(a.held())
		if a.focusTalk { // after the words offered, to go on with them
			n := utf8.RuneCountInString(a.input)
			in.Focus().SetTextSelection(n, n)
			a.focusTalk = false
		}
		return in
	}
	quotes := func() {
		for n, q := range a.quotes {
			a.quoteBlock(c, q, fmt.Sprintf("new-%d", n), func() { a.quotes = append(a.quotes[:n:n], a.quotes[n+1:]...) })
		}
	}
	if side {
		ui.Column(c).FillWidth().Gap(8).Children(func() {
			box := ui.Column(c).FillWidth().Padding(10, 10, 10, 14).Gap(6).Radius(kit.RDialog).Background(a.pal.Canvas).Border(1, t.Border)
			if box.FocusWithin() {
				box.BorderColor(t.TextMuted.Alpha(0.55))
			}
			box.Children(func() {
				quotes()
				ui.Row(c).FillWidth().Gap(10).AlignItems(ui.End).Children(func() {
					// Enter is a new line here as on the page: the key that
					// sends is said, there being no room to show it beside.
					hint := "输入消息"
					if !a.hasMessages() {
						hint = "输入想讨论的内容"
					}
					field(locales.ChatSendHint(hint, cmdSend.keys()), 16, 1).Grow(1).Shrink(1)
					send()
				})
			})
			ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
				// A talk opened to draft or rewrite keeps to its chapter: there
				// is nothing to choose.
				if kept := a.keptTo(); kept != "" {
					ui.Text(c, "跟着「"+a.nodeTitle(kept)+"」").FontSize(kit.FsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Shrink(1).
						Tooltip("这段讨论附上的正文和设定都按这一章")
				} else {
					label, tip := "附上本章内容", "将章纲和光标前约 2000 字一并发送给 AI"
					if a.openUnit() == "卷" {
						label, tip = "附上本卷内容", "将卷纲一并发送给 AI"
					}
					if ui.Checkbox(c, &a.doc.attach, label).Tooltip(tip).Changed() {
						a.savePrefs()
					}
				}
				ui.Spacer(c)
				chip()
			})
		})
		return
	}
	box := ui.Column(c).FillWidth().MaxWidth(column).Padding(12, 12, 10, 16).Gap(6).Radius(kit.RDialog).Background(a.pal.Raised).Border(1, t.Border)
	a.pal.Raise.On(box)
	if box.FocusWithin() {
		box.BorderColor(t.TextMuted.Alpha(0.55))
	}
	box.Children(func() {
		quotes()
		field("输入想法，或向 AI 提问…", 46, 2).FillWidth()
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			chip()
			ui.Spacer(c)
			ui.Text(c, cmdSend.keys()).FontSize(kit.FsSmall).TextColor(a.pal.Faint)
			send()
		})
	})
}

func (a *App) providerURL(id string) string {
	for _, p := range a.ai.Providers() {
		if p.ID == id {
			return p.BaseURL
		}
	}
	return ""
}

// keptTo is the chapter the talk shown keeps to, when it was opened to
// draft or rewrite it; its first words may be on their way still.
func (a *App) keptTo() string {
	if a.turn.active && a.turn.session == a.session {
		return a.turn.chapter
	}
	return originChapter(a.events)
}

// originChapter is the chapter the talk of events keeps to, when it was
// opened to draft or rewrite it: its first words say.
func originChapter(events []workspace.Event) string {
	for _, e := range events {
		if e.Role == workspace.RoleUser {
			if e.Origin != "" {
				return e.Chapter
			}
			return ""
		}
	}
	return ""
}

func (a *App) openSource(m workspace.Event, start, end int) {
	a.source = editor.New(editor.NewDoc(m.Text), editor.Options{ReadOnly: true, LineHeight: kit.LhRead,
		OwnMenu: func(x, y float32) { a.menu = kit.MenuState{Owner: "原文", X: x, Y: y} }})
	if end > start {
		a.source.MoveTo(start, false)
		a.source.MoveTo(end, true)
	}
	a.sourceMessage = m.ID
	a.sourceCategory = workspace.Other
	a.sourceOpen = true
}

func (a *App) extraction(c *ui.Context) {
	t := a.pal.Theme
	a.pal.Modal(c, &a.sourceOpen, func() {
		ui.Column(c).Width(kit.DialogSource).Padding(kit.DialogPad).Gap(16).Children(func() {
			a.pal.DialogHead(c, "摘取原文", "拖选一段原文，作为已确定的设定加入正文。原文不会被修改。")
			a.source.View(c).Height(220).FillWidth().Border(1, t.Border).Radius(kit.RCard)
			start, end := a.source.Selection()
			a.menu.MenuAt(c, a.pal, "原文", func(m *kit.Menu) {
				m.Item("复制").Hint(cmdCopy.keys()).Disabled(start == end).Do(func() { a.source.Copy() })
				m.Item("全选").Hint(cmdSelectAll.keys()).Do(func() { a.source.SelectAll() })
			})
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				a.pal.Select(c, &a.sourceCategory, a.categoryNames()).Label("摘取分类").Width(140)
				if start == end {
					ui.Text(c, "尚未选中文字").FontSize(kit.FsNote).TextColor(a.pal.Faint)
				} else {
					ui.Textf(c, "已选 %d 字", end-start).FontSize(kit.FsNote).TextColor(a.pal.Ink)
				}
				ui.Spacer(c)
				if a.pal.Ghost(c, "收起原文", ui.Color{}).Clicked() {
					a.sourceOpen = false
				}
				if a.pal.Primary(c, "加入设定").Disabled(a.held() || start == end).Clicked() {
					entry, err := a.ai.Extract(a.Store, a.session, a.sourceMessage, start, end, a.sourceCategory)
					if err == nil {
						a.sourceOpen = false
						a.status.Did("已加入设定正文")
						err = a.refresh()
						a.flash[entry.ID] = time.Now()
					}
					a.SetError(err)
				}
			})
		})
	})
	if !a.sourceOpen {
		a.source = nil
	}
}

func (a *App) categoryNames() []string {
	var names []string
	for _, cat := range a.Store.Work().Categories {
		names = append(names, cat.Name)
	}
	return names
}

// firstCategory is where an entry the author writes or takes from the
// prose starts: the work's first category, 人物 unless they changed it.
// What is taken from the talk starts in 其他 instead.
func (a *App) firstCategory() string {
	if cats := a.Store.Work().Categories; len(cats) > 0 {
		return cats[0].Name
	}
	return workspace.Other
}

// modelMenu changes the discussion model and its effort without leaving the
// conversation. Models come from each provider's catalog; one outside it
// is set in the model settings.
func (a *App) modelMenu(m *kit.Menu, work workspace.Work) {
	var with []aiport.Provider
	for _, p := range a.ai.Providers() {
		if len(p.Models) > 0 {
			with = append(with, p)
		}
	}
	for _, p := range with {
		// One service's models need no name over them.
		head := "模型"
		if len(with) > 1 {
			head = p.Name
		}
		m.Head(head)
		for _, id := range modelIDs(p) {
			on := work.Discussion.Provider == p.ID && work.Discussion.Model == id
			m.Item(id).On(on).Do(func() { a.setTalk(func(s *workspace.Selection) { s.Provider, s.Model = p.ID, id }) })
		}
	}
	m.Item("AI 配置…").Do(func() { a.openPrefs("AI") })
	m.Separator()
	m.Head("思考程度")
	for _, e := range a.ai.Efforts() {
		m.Item(a.ai.EffortLabel(e)).On(work.Discussion.Effort == e).Do(func() {
			a.setTalk(func(s *workspace.Selection) { s.Effort = e })
		})
	}
}

// setTalk changes the work's discussion selection and saves it.
func (a *App) setTalk(change func(*workspace.Selection)) {
	w := a.Store.Work()
	change(&w.Discussion)
	err := a.Store.SaveWork(w)
	if err == nil {
		a.loadWork()
	}
	a.SetError(err)
}
