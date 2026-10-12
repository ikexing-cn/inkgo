package desk

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/discussion"
	"writer/editor"
	"writer/model"
	"writer/workspace"
)

// column is the reading width of the conversation.
const column = 720

func (a *App) discussion(c *ui.Context) {
	ui.Row(c).FillWidth().Grow(1).AlignItems(ui.Stretch).Children(func() {
		chat := ui.Column(c).Grow(1).Shrink(1).FillHeight()
		a.LastChat = chat.Bounds()
		chat.Children(func() {
			a.chatTop(c)
			a.chatList(c, false)
			ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(0, 32, 20, 32).Children(func() {
				a.composer(c)
			})
		})
		a.settingColumn(c)
	})
}

// chatList builds only the messages in view: laying out a long session
// whole each frame made streaming and scrolling stutter.
func (a *App) chatList(c *ui.Context, side bool) {
	rows := a.rows[:0]
	if !a.hasMessages() && a.pending == "" {
		rows = append(rows, -1)
	}
	for n := range a.events {
		rows = append(rows, n)
	}
	if a.talking {
		rows = append(rows, -2)
	}
	a.rows = rows
	if !side && len(rows) == 1 && rows[0] == -1 {
		// Nothing said yet: the page opens in the middle of its room.
		ui.Column(c).Grow(1).FillWidth().Padding(24, 32).Children(func() { a.emptyChat(c) })
		return
	}
	last := a.lastMessage()
	a.chat.FollowEnd = true
	a.chat.Key = func(i int) any {
		if rows[i] < 0 {
			return rows[i]
		}
		return a.events[rows[i]].ID
	}
	list := ui.List(c, &a.chat, len(rows), func(i int) {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(column).Children(func() {
				switch r := rows[i]; r {
				case -1:
					a.emptyChat(c)
				case -2:
					a.thinking(c)
				default:
					a.message(c, a.events[r], a.events[r].ID == last)
				}
			})
		})
	})
	if side {
		list.Grow(1).FillWidth().Gap(20).Padding(8, 2, 12, 2)
	} else {
		list.Grow(1).FillWidth().Gap(24).Padding(32, 32, 24, 32)
	}
}

func (a *App) hasMessages() bool { return len(a.events) > 0 }

func (a *App) lastMessage() string {
	if len(a.events) == 0 {
		return ""
	}
	return a.events[len(a.events)-1].ID
}

func (a *App) chatTop(c *ui.Context) {
	t := a.pal.theme
	_, words := a.sessionParts(a.session)
	left, right := a.chromeRoom(c)
	ui.Row(c).FillWidth().Height(titleHeight).Padding(0, 24+right, 0, 24+left).Gap(8).AlignItems(ui.Center).Shrink(0).DragWindow().Children(func() {
		ui.Text(c, "总讨论").FontSize(fsControl).TextColor(t.TextMuted)
		glyph(c, icChevron, iconSm, a.pal.faint)
		ui.Text(c, words).FontSize(fsBody).FontWeight(600).SingleLine().Ellipsis("…").Shrink(1)
		ui.Spacer(c)
		if n := len(a.events); n > 0 {
			ui.Textf(c, "%d 条发言", n).FontSize(fsNote).TextColor(a.pal.faint)
		}
		if a.doc.hidden {
			a.bookDot(c)
		}
		if a.doc.hidden && iconButton(c, icPanel, "展开设定", ui.Color{}).Tooltip("展开设定文档").Clicked() {
			a.setDocHidden(false)
		}
	})
	a.pal.hairline(c)
}

// openUnit is what the writing page has open: a chapter, or a folder,
// which the author thinks of as a volume.
func (a *App) openUnit() string {
	if n, ok := a.write.tree.Node(a.write.node); ok && n.Folder {
		return "卷"
	}
	return "章"
}

// modKey names the command key the way the system shows it.
var modKey = map[bool]string{true: "⌘", false: "Ctrl+"}[runtime.GOOS == "darwin"]

func (a *App) switchSession(id string) {
	if id == "" || id == a.session {
		return
	}
	a.session = id
	a.source, a.sourceOpen, a.contextReply = nil, false, ""
	a.setError(a.refresh())
	a.chat = ui.ListState{} // a list's place is its session's
}

func (a *App) emptyChat(c *ui.Context) {
	t := a.pal.theme
	title, about := "从一个初步的想法开始", "写下初步的想法。讨论内容会在后台整理为右侧的设定文档；已确定的设定将作为后续讨论的依据，已放弃的不再提及。"
	starts := []string{"我想写一个总在下雨的城市", "主角有个说不出口的秘密", "这个故事的冲突在哪"}
	if a.page == 3 { // beside the prose the talk is about the page in front of the author
		unit := a.openUnit()
		starts = []string{"这一" + unit + "的节奏怎么样", "接下来可以怎么写", "人物的动机说得通吗"}
		// In the column they are rows, as its lists are.
		ui.Column(c).FillWidth().Margin(0, -10).Children(func() {
			a.paneGroup(c, "示例问题")
			for _, s := range starts {
				if a.paneRow(c, s, s, "", "", "", false).Clicked() {
					a.input, a.focusTalk = s, true
				}
			}
		})
		return
	}
	a.pal.blank(c, icSparkle, title, about, func() {
		ui.Row(c).Gap(8).Padding(10, 0, 0, 0).Wrap().Justify(ui.Center).Children(func() {
			for _, s := range starts {
				b := ui.ButtonBase(c).Padding(6, 12).Radius(rRound).Border(1, t.Border)
				fg := t.TextMuted
				if b.Hovered() {
					b.Background(t.SurfaceHover)
					fg = t.Text
				}
				b.Children(func() { ui.Text(c, s).FontSize(fsControl).TextColor(fg) })
				if b.Clicked() {
					a.input = s
				}
			}
		})
	})
}

func (a *App) answered(userID string) bool {
	for _, e := range a.events {
		if e.Role == workspace.RoleAssistant && e.ReplyTo == userID {
			return true
		}
	}
	return false
}

// replyIndent is where a reply's text starts: past the mark and the gap
// after it.
const replyIndent = 24 + 12

func (a *App) aiMark(c *ui.Context) {
	ui.Box(c).Size(hControlSm, hControlSm).Radius(rRound).Background(a.pal.theme.SurfacePressed).Center().Shrink(0).Children(func() {
		glyph(c, icSparkle, iconSm, a.pal.theme.TextMuted)
	})
}

func (a *App) userBubble(c *ui.Context, text string) {
	ui.Column(c).MaxWidthPercent(82).Padding(8, 12).Radius(rDialog).Background(a.pal.theme.SurfacePressed).Children(func() {
		ui.Text(c, text).FontSize(fsRead).LineHeight(lhLoose).Selectable()
	})
}

// message shows the author's words as a bubble on the right and the AI's as
// plain prose, the way a reply reads in a letter. Its actions fade in on hover.
func (a *App) message(c *ui.Context, m workspace.Event, last bool) {
	t := a.pal.theme
	user := m.Role == workspace.RoleUser
	align := ui.Start
	if user {
		align = ui.End
	}
	box := ui.Column(c).Key(m.ID).FillWidth().AlignItems(align).Gap(8)
	shown := last || box.Hovered() || box.FocusWithin()
	box.Children(func() {
		if user {
			for n, q := range m.Quotes {
				ui.Column(c).MaxWidthPercent(82).Children(func() {
					a.quoteBlock(c, q, fmt.Sprintf("%s-%d", m.ID, n), nil)
				})
			}
			a.userBubble(c, m.Text)
		} else {
			ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Start).Children(func() {
				a.aiMark(c)
				ui.Column(c).Grow(1).Shrink(1).Gap(10).Children(func() {
					if m.Reasoning != "" {
						a.thought(c, m.ID, "思考过程", m.Reasoning, false)
					}
					a.markdown(c, m.Text, 14.5)
				})
			})
		}
		acts := ui.Row(c).Gap(2).AlignItems(ui.Center)
		if !user {
			acts.Padding(0, 0, 0, replyIndent-6) // the icons, not their buttons, under the reply
		}
		if !shown {
			acts.Opacity(0)
		}
		acts.Children(func() {
			if !user && !m.At.IsZero() {
				when := m.At.Local().Format("15:04")
				if who := modelLabel(m.Model, m.Effort); who != "" {
					when += " · " + who
				}
				ui.Text(c, when).FontSize(fsSmall).TextColor(a.pal.faint).Padding(0, 6, 0, 0)
			}
			if iconButton(c, icQuote, "摘取-"+m.ID, ui.Color{}).Tooltip("摘取原文").Disabled(a.busy != "").Clicked() {
				a.openSource(m, 0, 0)
			}
			if !user {
				// Its Markdown, which keeps its structure pasted elsewhere.
				if iconButton(c, icCopy, "复制-"+m.ID, ui.Color{}).Tooltip("复制全文").Clicked() {
					c.WriteClipboard(m.Text)
				}
				on := a.contextReply == m.ID
				color := ui.Color{}
				if on {
					color = a.pal.ink
				}
				if iconButton(c, icEye, "上下文-"+m.ID, color).Tooltip("AI 参考内容").Clicked() {
					if on {
						a.contextReply = ""
					} else {
						a.contextReply = m.ID
					}
				}
			}
			if user && !m.At.IsZero() {
				ui.Text(c, m.At.Local().Format("15:04")).FontSize(fsSmall).TextColor(a.pal.faint).Padding(0, 0, 0, 6)
			}
		})
		if user && !a.answered(m.ID) && !a.talking {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				glyph(c, icAlert, iconSm, t.Warning)
				ui.Text(c, "该消息尚未收到回复").FontSize(fsNote).TextColor(t.Warning)
				if ghost(c, "重试回复", t.Text).Disabled(a.busy != "").Clicked() {
					a.retry(c, m.ID)
				}
			})
		}
		if !user {
			if a.contextReply == m.ID {
				a.audit(c, m)
			}
		}
	})
}

// thinking shows a turn in flight as it streams: the reply forming under
// its reasoning.
func (a *App) thinking(c *ui.Context) {
	t := a.pal.theme
	if a.pending != "" {
		ui.Column(c).FillWidth().AlignItems(ui.End).Children(func() { a.userBubble(c, a.pending) })
	}
	reasoning, text := a.live.read()
	ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Start).Children(func() {
		a.aiMark(c)
		ui.Column(c).Grow(1).Shrink(1).Gap(10).Children(func() {
			if reasoning != "" {
				a.thought(c, "live-reply", "思考过程", reasoning, text == "")
			}
			if text != "" {
				a.liveMarkdown(c, text, 14.5)
			}
			status := "正在回复…"
			if text == "" {
				status = "正在思考…"
			}
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c)
				ui.Text(c, status).FontSize(fsControl).TextColor(t.TextMuted).Grow(1).Shrink(1)
				if ghost(c, "取消请求", ui.Color{}).Clicked() && a.cancel != nil {
					a.cancel()
				}
			})
		})
	})
}

// thought folds what a model reasoned before it answered; it stays open
// while the reasoning is all that is streaming.
func (a *App) thought(c *ui.Context, key, label, reasoning string, running bool) {
	t := a.pal.theme
	open := a.thoughts[key]
	if open == nil || running {
		v := running
		open = &v
		a.thoughts[key] = open
	}
	if running {
		label = "正在思考"
	}
	ui.Collapsible(c, label, open, func() {
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Stretch).Padding(4, 0, 2, 2).Children(func() {
			ui.Box(c).Width(2).Radius(rRound).Background(a.pal.line).Shrink(0)
			if !running {
				ui.Text(c, reasoning).FontSize(fsControl).LineHeight(lhLoose).TextColor(t.TextMuted).Selectable().Grow(1).Shrink(1)
				return
			}
			// While it streams, the paragraphs written keep their layout
			// and only the last is laid out again, as liveMarkdown does.
			ui.Column(c).Grow(1).Shrink(1).Children(func() {
				cut := strings.LastIndex(reasoning, "\n") + 1
				for _, s := range []string{strings.TrimSuffix(reasoning[:cut], "\n"), reasoning[cut:]} {
					if s != "" {
						ui.Text(c, s).FontSize(fsControl).LineHeight(lhLoose).TextColor(t.TextMuted).FillWidth()
					}
				}
			})
		})
	}).FillWidth()
}

// modelLabel names the model and effort a reply came from.
func modelLabel(name, effort string) string {
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if effort != "" && name != "" {
		name += " · " + model.Effort(effort).Label()
	}
	return name
}

func (a *App) audit(c *ui.Context, m workspace.Event) {
	t := a.pal.theme
	ui.Column(c).FillWidth().Padding(0, 0, 0, replyIndent).Children(func() {
		ui.Column(c).FillWidth().Padding(12, 14).Gap(8).Radius(rCard).Background(a.pal.side).Border(1, a.pal.line).Children(func() {
			ui.Textf(c, "回复时附带 %d 条设定，因预算不足省略 %d 条", len(m.Context), len(m.Omitted)).FontSize(fsNote).FontWeight(600)
			if len(m.Context) == 0 && len(m.Omitted) == 0 {
				ui.Text(c, "仅参考了最近几轮对话，设定文档暂无内容。").FontSize(fsNote).TextColor(t.TextMuted)
			}
			for _, item := range m.Context {
				ui.Row(c).FillWidth().Gap(8).Children(func() {
					glyph(c, icCheck, iconSm, t.Success)
					ui.Text(c, item.Text).FontSize(fsNote).Selectable().Grow(1).Shrink(1)
				})
			}
			for _, item := range m.Omitted {
				ui.Row(c).FillWidth().Gap(8).Children(func() {
					glyph(c, icX, iconSm, a.pal.faint)
					ui.Text(c, item.Text+" · "+item.Reason).FontSize(fsNote).TextColor(t.TextMuted).Selectable().Grow(1).Shrink(1)
				})
			}
		})
	})
}

// composer is one rounded field: the text, then the model it goes to and a
// round send button. It grows with the text up to a few lines.
func (a *App) composer(c *ui.Context) {
	t := a.pal.theme
	work := a.Store.Work()
	ready := work.Discussion.Provider != "" && work.Discussion.Model != ""
	side := a.page == 3
	// The chip names the model and effort the next turn goes to, and
	// opens a menu to change either; its tooltip says where the words go.
	chip := func() {
		chip := ui.ButtonBase(c).Row().Gap(6).AlignItems(ui.Center).Padding(4, 8).Radius(rControl).Label("当前模型").Tooltip(a.destination(work.Discussion)).Shrink(1).Disabled(a.busy != "")
		if chip.Hovered() {
			chip.Background(t.SurfaceHover)
		}
		chip.Children(func() {
			if ready {
				color := t.Success
				if !isLocal(a.providerURL(work.Discussion.Provider)) {
					color = a.pal.ink
				}
				dot(c, color, 6)
				ui.Text(c, work.Discussion.Model).FontSize(fsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Shrink(1)
				ui.Text(c, model.Effort(work.Discussion.Effort).Label()).FontSize(fsNote).TextColor(a.pal.faint).SingleLine()
			} else {
				dot(c, t.Warning, 6)
				ui.Text(c, "选择讨论模型").FontSize(fsNote).TextColor(t.Warning).SingleLine()
			}
			glyph(c, icChevronUp, iconXs, a.pal.faint)
		})
		chip.Menu(func(m *ui.Menu) { a.modelMenu(m, work) })
	}
	send := func() {
		can := a.busy == "" && trim(a.input) != ""
		send := ui.ButtonBase(c).Size(hControl, hControl).Radius(rRound).Center().Label("发送").Tooltip("发送").Disabled(!can).Shrink(0)
		bg := a.pal.ink
		if !can {
			bg = t.SurfacePressed
		} else if send.Hovered() {
			bg = t.AccentHover
		}
		send.Background(bg).Children(func() {
			fg := a.pal.onInk
			if !can {
				fg = a.pal.faint
			}
			glyph(c, icSend, iconMd, fg)
		})
		if a.busy == "" && (send.Clicked() || c.Shortcut(ui.Cmd, ui.KeyEnter)) {
			a.send(c)
		}
	}
	field := func(hint string, per, least int) ui.Element {
		lines := strings.Count(a.input, "\n") + 1 + len([]rune(a.input))/per
		h := float32(min(max(lines, least), 7)) * 25
		in := ui.TextAreaBase(c, &a.input).Label("讨论输入").Placeholder(hint).Height(h).FontSize(fsRead).LineHeight(lhLoose).Disabled(a.busy != "")
		if a.focusTalk {
			in.Focus()
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
			box := ui.Column(c).FillWidth().Padding(10, 10, 10, 14).Gap(6).Radius(rDialog).Background(a.pal.canvas).Border(1, t.Border)
			if box.FocusWithin() {
				box.BorderColor(t.TextMuted.Alpha(0.55))
			}
			box.Children(func() {
				quotes()
				ui.Row(c).FillWidth().Gap(10).AlignItems(ui.End).Children(func() {
					hint := "输入消息"
					if !a.hasMessages() {
						hint = "输入想讨论的内容"
					}
					field(hint, 16, 1).Grow(1).Shrink(1)
					send()
				})
			})
			ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
				label, tip := "附上本章内容", "将章纲和光标前约 2000 字一并发送给 AI"
				if a.openUnit() == "卷" {
					label, tip = "附上本卷内容", "将卷纲一并发送给 AI"
				}
				if ui.Checkbox(c, &a.doc.attach, label).Tooltip(tip).Changed() {
					a.savePrefs()
				}
				ui.Spacer(c)
				chip()
			})
		})
		return
	}
	box := ui.Column(c).FillWidth().MaxWidth(column).Padding(12, 12, 10, 16).Gap(6).Radius(rDialog).Background(a.pal.raised).Border(1, t.Border)
	a.pal.raise.on(box)
	if box.FocusWithin() {
		box.BorderColor(t.TextMuted.Alpha(0.55))
	}
	box.Children(func() {
		quotes()
		field("输入想法，或向 AI 提问…", 46, 2).FillWidth()
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			chip()
			ui.Spacer(c)
			ui.Text(c, modKey+"Enter").FontSize(fsSmall).TextColor(a.pal.faint)
			send()
		})
	})
}

func (a *App) providerURL(id string) string {
	for _, p := range a.Config.Providers {
		if p.ID == id {
			return p.BaseURL
		}
	}
	return ""
}

func (a *App) send(c *ui.Context) {
	if trim(a.input) == "" {
		return
	}
	eng, err := a.engine(c)
	if err != nil {
		a.setError(err)
		return
	}
	input, quotes, session := a.input, a.quotes, a.session
	words := workspace.Event{Text: input, Quotes: quotes}
	if a.page == 3 {
		words.Chapter = a.write.node
		eng.Beside = a.beside()
	}
	a.pending, a.input, a.quotes = input, "", nil
	a.chat.ScrollToEnd()
	a.run(c, "AI 正在思考…", func(ctx context.Context) (func(), error) {
		turn, err := eng.Say(ctx, session, words)
		return func() {
			a.pending = ""
			if turn.User.ID == "" && a.input == "" {
				a.input, a.quotes = input, quotes // nothing was saved: give the words back
			}
			a.chat.ScrollToEnd()
		}, err
	})
	a.talking = a.busy != ""
}

func (a *App) retry(c *ui.Context, userID string) {
	eng, err := a.engine(c)
	if err != nil {
		a.setError(err)
		return
	}
	session := a.session
	if a.page == 3 {
		eng.Beside = a.beside()
	}
	a.run(c, "正在重新请求回复…", func(ctx context.Context) (func(), error) {
		_, err := eng.Retry(ctx, session, userID)
		return func() { a.chat.ScrollToEnd() }, err
	})
	a.talking = a.busy != ""
}

func (a *App) openSource(m workspace.Event, start, end int) {
	a.source = editor.New(editor.NewDoc(m.Text), editor.Options{ReadOnly: true, LineHeight: lhRead})
	if end > start {
		a.source.MoveTo(start, false)
		a.source.MoveTo(end, true)
	}
	a.sourceMessage = m.ID
	a.sourceCategory = workspace.Other
	a.sourceOpen = true
}

func (a *App) extraction(c *ui.Context) {
	t := a.pal.theme
	a.modal(c, &a.sourceOpen, func() {
		ui.Column(c).Width(dialogSource).Padding(dialogPad).Gap(16).Children(func() {
			a.pal.dialogHead(c, "摘取原文", "拖选一段原文，作为已确定的设定加入正文。原文不会被修改。")
			a.source.View(c).Height(220).FillWidth().Border(1, t.Border).Radius(rCard)
			start, end := a.source.Selection()
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Select(c, &a.sourceCategory, a.categoryNames()).Label("摘取分类").Width(140)
				if start == end {
					ui.Text(c, "尚未选中文字").FontSize(fsNote).TextColor(a.pal.faint)
				} else {
					ui.Textf(c, "已选 %d 字", end-start).FontSize(fsNote).TextColor(a.pal.ink)
				}
				ui.Spacer(c)
				if ghost(c, "收起原文", ui.Color{}).Clicked() {
					a.sourceOpen = false
				}
				if a.pal.primary(c, "加入设定").Disabled(a.busy != "" || start == end).Clicked() {
					eng := &discussion.Engine{Store: a.Store}
					entry, err := eng.Extract(a.session, a.sourceMessage, start, end, a.sourceCategory)
					if err == nil {
						a.sourceOpen = false
						a.notice = "已加入设定正文"
						err = a.refresh()
						a.flash[entry.ID] = time.Now()
					}
					a.setError(err)
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

// modelMenu changes the discussion model and its effort without leaving the
// conversation. Models come from each provider's catalog; one outside it
// is set in the model settings.
func (a *App) modelMenu(m *ui.Menu, work workspace.Work) {
	m.Submenu("模型", func(m *ui.Menu) {
		for _, p := range a.Config.Providers {
			if len(p.Models) == 0 {
				continue
			}
			m.Submenu(p.Name, func(m *ui.Menu) {
				for _, id := range p.Models {
					on := work.Discussion.Provider == p.ID && work.Discussion.Model == id
					if m.Item(id).Checked(on).Chosen() {
						a.setTalk(func(s *workspace.Selection) { s.Provider, s.Model = p.ID, id })
					}
				}
			})
		}
		m.Separator()
		if m.Item("模型设置…").Chosen() {
			a.openPrefs("模型")
		}
	})
	m.Submenu("思考程度", func(m *ui.Menu) {
		for _, e := range model.Efforts {
			if m.Item(e.Label()).Checked(work.Discussion.Effort == string(e)).Chosen() {
				a.setTalk(func(s *workspace.Selection) { s.Effort = string(e) })
			}
		}
	})
}

// setTalk changes the work's discussion selection and saves it.
func (a *App) setTalk(change func(*workspace.Selection)) {
	w := a.Store.Work()
	change(&w.Discussion)
	err := a.Store.SaveWork(w)
	if err == nil {
		a.loadWork()
	}
	a.setError(err)
}
