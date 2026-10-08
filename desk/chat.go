package desk

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui"
	"writer/discussion"
	"writer/editor"
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
			ui.Scroll(c).Key("chat-" + a.session).TrackScroll(&a.chatScroll).Grow(1).FillWidth().Children(func() {
				ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(28, 28, 24, 28).Children(func() {
					ui.Column(c).FillWidth().MaxWidth(column).Gap(26).Children(func() {
						if !a.hasMessages() && a.pending == "" {
							a.emptyChat(c)
						}
						last := a.lastMessage()
						for _, m := range a.events {
							if m.Kind == workspace.KindMessage {
								a.message(c, m, m.ID == last)
							}
						}
						if a.talking {
							a.thinking(c)
						}
					})
				})
			})
			ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(0, 28, 20, 28).Children(func() {
				a.composer(c)
			})
		})
		ui.Box(c).Width(1).FillHeight().Background(a.pal.line)
		pool := ui.Column(c).Width(392).FillHeight().Padding(18, 18, 12, 18).Gap(12).Background(a.pal.side).Shrink(0)
		a.LastPool = pool.Bounds()
		pool.Children(func() { a.pool(c) })
	})
}

func (a *App) hasMessages() bool { return a.lastMessage() != "" }

func (a *App) lastMessage() string {
	for n := len(a.events) - 1; n >= 0; n-- {
		if a.events[n].Kind == workspace.KindMessage {
			return a.events[n].ID
		}
	}
	return ""
}

func (a *App) chatTop(c *ui.Context) {
	t := a.pal.theme
	_, words := a.sessionParts(a.session)
	ui.Row(c).FillWidth().Height(52).Padding(0, 24).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Text(c, "总讨论").FontSize(13).TextColor(t.TextMuted)
		glyph(c, icChevron, 14, a.pal.faint)
		ui.Text(c, words).FontSize(13.5).FontWeight(600).SingleLine().Ellipsis("…").Shrink(1)
		ui.Spacer(c)
		n := 0
		for _, e := range a.events {
			if e.Kind == workspace.KindMessage {
				n++
			}
		}
		if n > 0 {
			ui.Textf(c, "%d 条发言", n).FontSize(12).TextColor(a.pal.faint)
		}
	})
	a.pal.hairline(c)
}

func (a *App) switchSession(id string) {
	if id == "" || id == a.session {
		return
	}
	a.session = id
	a.source, a.sourceOpen, a.contextReply, a.turn = nil, false, "", ""
	a.setError(a.refresh())
	a.chatScroll.Y = math.MaxFloat32
}

func (a *App) emptyChat(c *ui.Context) {
	t := a.pal.theme
	ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(72, 0, 0, 0).Gap(14).Children(func() {
		ui.Box(c).Size(44, 44).Radius(12).Background(a.pal.brand.Alpha(0.1)).Center().Children(func() {
			glyph(c, icSparkle, 22, a.pal.brand)
		})
		ui.Text(c, "从一个模糊的念头聊起").FontSize(22).FontWeight(600)
		ui.Text(c, "写下脑子里的那一点想法。你和 AI 提到的思路会整理到右边，\n留下的会成为之后讨论的依据，放下的不再被提起。").FontSize(13.5).TextColor(t.TextMuted).TextAlign(ui.Center).LineHeight(1.7)
		ui.Box(c).Height(6)
		ui.Row(c).Gap(8).Wrap().Justify(ui.Center).Children(func() {
			for _, s := range []string{"我想写一个总在下雨的城市", "主角有个说不出口的秘密", "这个故事的冲突在哪"} {
				b := ui.ButtonBase(c).Padding(7, 14).Radius(999).Border(1, t.Border).Background(a.pal.raised)
				if b.Hovered() {
					b.Background(t.SurfaceHover)
				}
				b.Children(func() { ui.Text(c, s).FontSize(13).TextColor(t.Text) })
				if b.Clicked() {
					a.input = s
				}
			}
		})
	})
}

func (a *App) answered(userID string) bool {
	for _, e := range a.events {
		if e.Kind == workspace.KindMessage && e.Role == workspace.RoleAssistant && e.ReplyTo == userID {
			return true
		}
	}
	return false
}
func (a *App) aiMark(c *ui.Context) {
	ui.Box(c).Size(24, 24).Radius(7).Background(a.pal.brand.Alpha(0.12)).Center().Shrink(0).Children(func() {
		glyph(c, icSparkle, 14, a.pal.brand)
	})
}

func (a *App) userBubble(c *ui.Context, text string) {
	ui.Column(c).MaxWidthPercent(82).Padding(10, 15).Radius(14).Background(a.pal.bubble).Children(func() {
		ui.Text(c, text).FontSize(14.5).LineHeight(1.7).Selectable()
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
			a.userBubble(c, m.Text)
		} else {
			ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Start).Children(func() {
				a.aiMark(c)
				ui.Text(c, m.Text).FontSize(14.5).LineHeight(1.8).Selectable().Grow(1).Shrink(1)
			})
		}
		acts := ui.Row(c).Gap(2).AlignItems(ui.Center)
		if !user {
			acts.Padding(0, 0, 0, 30)
		}
		if !shown {
			acts.Opacity(0)
		}
		acts.Children(func() {
			if !user && !m.At.IsZero() {
				ui.Text(c, m.At.Local().Format("15:04")).FontSize(11.5).TextColor(a.pal.faint).Padding(0, 6, 0, 0)
			}
			if iconButton(c, icQuote, "摘取-"+m.ID, ui.Color{}).Tooltip("摘取原文").Disabled(a.busy != "").Clicked() {
				a.openSource(m, 0, 0)
			}
			if !user {
				on := a.contextReply == m.ID
				color := ui.Color{}
				if on {
					color = a.pal.brand
				}
				if iconButton(c, icEye, "上下文-"+m.ID, color).Tooltip("AI 看到了什么").Clicked() {
					if on {
						a.contextReply = ""
					} else {
						a.contextReply = m.ID
					}
				}
			}
			if user && !m.At.IsZero() {
				ui.Text(c, m.At.Local().Format("15:04")).FontSize(11.5).TextColor(a.pal.faint).Padding(0, 0, 0, 6)
			}
		})
		if user && !a.answered(m.ID) && !a.talking {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				glyph(c, icAlert, 14, t.Warning)
				ui.Text(c, "这条还没有得到回复").FontSize(12.5).TextColor(t.Warning)
				if ghost(c, "重试回复", t.Text).Disabled(a.busy != "").Clicked() {
					a.retry(c, m.ID)
				}
			})
		}
		if !user {
			if a.contextReply == m.ID {
				a.audit(c, m)
			}
			a.turnSummary(c, m)
		}
	})
}

// thinking stands in for the reply while a turn is in flight.
func (a *App) thinking(c *ui.Context) {
	t := a.pal.theme
	if a.pending != "" {
		ui.Column(c).FillWidth().AlignItems(ui.End).Children(func() { a.userBubble(c, a.pending) })
	}
	ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
		a.aiMark(c)
		ui.Spinner(c)
		ui.Text(c, a.busy).FontSize(13).TextColor(t.TextMuted).Grow(1).Shrink(1)
		if ghost(c, "取消请求", ui.Color{}).Clicked() && a.cancel != nil {
			a.cancel()
		}
	})
}

// turnIdeas are the cards bookkeeping drew from one exchange.
func (a *App) turnIdeas(reply workspace.Event) []workspace.Idea {
	var found []workspace.Idea
	for _, i := range a.ideas {
		if i.Origin.Session == a.session && (i.Origin.Message == reply.ID || i.Origin.Message == reply.ReplyTo) {
			found = append(found, i)
		}
	}
	return found
}

// turnSummary is one quiet line under a reply; decisions happen in the pool,
// which it filters to this exchange.
func (a *App) turnSummary(c *ui.Context, m workspace.Event) {
	t := a.pal.theme
	ideas := a.turnIdeas(m)
	ui.Row(c).Padding(0, 0, 0, 36).Gap(8).AlignItems(ui.Center).Children(func() {
		switch {
		case !a.booked(m.ID):
			glyph(c, icAlert, 14, t.Warning)
			ui.Text(c, "这一轮的思路还没整理完").FontSize(12.5).TextColor(t.Warning)
			if ghost(c, "重试记账", t.Text).Disabled(a.busy != "").Clicked() {
				a.bookkeep(c, m.ID)
			}
		case len(ideas) == 0:
			ui.Text(c, "这一轮没有新的思路").FontSize(12.5).TextColor(a.pal.faint)
		default:
			waiting := 0
			for _, i := range ideas {
				if i.Status == workspace.Unprocessed {
					waiting++
				}
			}
			b := ui.ButtonBase(c).Row().Gap(7).AlignItems(ui.Center).Padding(5, 10).Radius(999).Border(1, a.pal.brand.Alpha(0.22)).Background(a.pal.brand.Alpha(0.06)).Label("本轮思路-" + m.ID)
			if b.Hovered() || a.turn == m.ID {
				b.Background(a.pal.brand.Alpha(0.13))
			}
			b.Children(func() {
				glyph(c, icSparkle, 13, a.pal.brand)
				text := fmt.Sprintf("整理出 %d 条思路", len(ideas))
				if waiting > 0 {
					text += fmt.Sprintf("，%d 条等你决定", waiting)
				} else {
					text += "，都已处理"
				}
				ui.Text(c, text).FontSize(12.5).TextColor(a.pal.brand)
				if a.turn == m.ID {
					glyph(c, icX, 12, a.pal.brand)
				} else {
					glyph(c, icChevron, 12, a.pal.brand)
				}
			})
			if b.Clicked() {
				if a.turn == m.ID {
					a.turn = ""
				} else {
					a.turn, a.filter, a.extra, a.search = m.ID, len(filters)-1, extras[0], ""
				}
			}
		}
	})
}

func (a *App) audit(c *ui.Context, m workspace.Event) {
	t := a.pal.theme
	ui.Column(c).FillWidth().Padding(0, 0, 0, 36).Children(func() {
		ui.Column(c).FillWidth().Padding(12, 14).Gap(7).Radius(10).Background(a.pal.side).Border(1, a.pal.line).Children(func() {
			ui.Textf(c, "回复时带上了 %d 条思路，预算不足省略 %d 条", len(m.Context), len(m.Omitted)).FontSize(12.5).FontWeight(600)
			if len(m.Context) == 0 && len(m.Omitted) == 0 {
				ui.Text(c, "只读了最近几轮对话，思路池里还没有可作依据的思路。").FontSize(12.5).TextColor(t.TextMuted)
			}
			for _, item := range m.Context {
				ui.Row(c).FillWidth().Gap(8).Children(func() {
					glyph(c, icCheck, 13, t.Success)
					ui.Text(c, item.Text).FontSize(12.5).Selectable().Grow(1).Shrink(1)
				})
			}
			for _, item := range m.Omitted {
				ui.Row(c).FillWidth().Gap(8).Children(func() {
					glyph(c, icX, 13, a.pal.faint)
					ui.Text(c, item.Text+" · "+item.Reason).FontSize(12.5).TextColor(t.TextMuted).Selectable().Grow(1).Shrink(1)
				})
			}
		})
	})
}

func (a *App) booked(reply string) bool {
	have := map[string]bool{}
	for _, idea := range a.ideas {
		have[idea.ID] = true
	}
	for _, event := range a.events {
		if event.Kind == workspace.KindBookkeeping && event.ReplyTo == reply {
			for _, id := range event.IdeaIDs {
				if !have[id] {
					return false
				}
			}
			return true
		}
	}
	return false
}

// composer is one rounded field: the text, then the model it goes to and a
// round send button. It grows with the text up to a few lines.
func (a *App) composer(c *ui.Context) {
	t := a.pal.theme
	work := a.Store.Work()
	ready := work.Discussion.Provider != "" && work.Discussion.Model != ""
	box := ui.Column(c).FillWidth().MaxWidth(column).Padding(12, 12, 10, 16).Gap(6).Radius(16).Background(a.pal.raised).Border(1, t.Border).Shadow(0, 4, 18, 0, ui.RGBA(0, 0, 0, 0.06))
	if box.FocusWithin() {
		box.BorderColor(t.TextMuted.Alpha(0.55))
	}
	box.Children(func() {
		lines := strings.Count(a.input, "\n") + 1 + len([]rune(a.input))/46
		h := float32(min(max(lines, 2), 7)) * 25
		ui.TextAreaBase(c, &a.input).Label("讨论输入").Placeholder("说说你的想法，或问 AI 一个问题…").FillWidth().Height(h).FontSize(14.5).LineHeight(1.7).Disabled(a.busy != "")
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			chip := ui.ButtonBase(c).Row().Gap(6).AlignItems(ui.Center).Padding(4, 8).Radius(7).Label("当前模型").Tooltip("在模型设置里更换").Shrink(1)
			if chip.Hovered() {
				chip.Background(t.SurfaceHover)
			}
			chip.Children(func() {
				if ready {
					color := t.Success
					if !isLocal(a.providerURL(work.Discussion.Provider)) {
						color = a.pal.brand
					}
					dot(c, color, 6)
					ui.Text(c, a.destination(work.Discussion)).FontSize(12).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Shrink(1)
				} else {
					dot(c, t.Warning, 6)
					ui.Text(c, "还没有选择讨论模型，去设置").FontSize(12).TextColor(t.Warning).SingleLine()
				}
			})
			if chip.Clicked() {
				a.page = 1
			}
			ui.Spacer(c)
			key := "Ctrl + Enter"
			if runtime.GOOS == "darwin" {
				key = "⌘ + Enter"
			}
			ui.Text(c, key).FontSize(11.5).TextColor(a.pal.faint)
			can := a.busy == "" && trim(a.input) != ""
			send := ui.ButtonBase(c).Size(32, 32).Radius(999).Center().Label("发送").Tooltip("发送").Disabled(!can).Shrink(0)
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
				glyph(c, icSend, 16, fg)
			})
			if a.busy == "" && (send.Clicked() || c.Shortcut(ui.Cmd, ui.KeyEnter)) {
				a.send(c)
			}
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
	eng, err := a.engine()
	if err != nil {
		a.setError(err)
		return
	}
	input, session := a.input, a.session
	a.pending, a.input = input, ""
	a.chatScroll.Y = math.MaxFloat32
	a.run(c, "AI 正在思考，并整理这一轮的思路…", func(ctx context.Context) (func(), error) {
		turn, err := eng.Send(ctx, session, input)
		return func() {
			a.pending = ""
			if turn.User.ID == "" && a.input == "" {
				a.input = input // nothing was saved: give the words back
			}
			a.chatScroll.Y = math.MaxFloat32
		}, err
	})
	a.talking = a.busy != ""
}

func (a *App) retry(c *ui.Context, userID string) {
	eng, err := a.engine()
	if err != nil {
		a.setError(err)
		return
	}
	session := a.session
	a.run(c, "正在重新请求回复…", func(ctx context.Context) (func(), error) {
		_, err := eng.Retry(ctx, session, userID)
		return func() { a.chatScroll.Y = math.MaxFloat32 }, err
	})
	a.talking = a.busy != ""
}

// bookkeep replays a saved plan even without a model; only a missing plan
// needs one.
func (a *App) bookkeep(c *ui.Context, replyID string) {
	eng, _ := a.engine()
	session := a.session
	a.run(c, "正在整理这一轮的思路…", func(ctx context.Context) (func(), error) {
		_, err := eng.Bookkeep(ctx, session, replyID)
		return nil, err
	})
	a.talking = a.busy != ""
}

func (a *App) openSource(m workspace.Event, start, end int) {
	a.source = editor.New(editor.NewDoc(m.Text), editor.Options{ReadOnly: true})
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
	ui.Modal(c, &a.sourceOpen, func() {
		ui.Column(c).Width(min(620, max(a.LastChat.W, 480))).Gap(14).Children(func() {
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "摘取原文").FontSize(17).FontWeight(600)
				ui.Text(c, "拖选一段原话，存成一条新思路。原话不会被改动。").FontSize(13).TextColor(t.TextMuted)
			})
			a.source.View(c).Height(220).FillWidth().Border(1, t.Border).Radius(10)
			start, end := a.source.Selection()
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Select(c, &a.sourceCategory, a.categoryNames()).Label("摘取分类").Width(140)
				if start == end {
					ui.Text(c, "还没有选中文字").FontSize(12.5).TextColor(a.pal.faint)
				} else {
					ui.Textf(c, "已选 %d 字", end-start).FontSize(12.5).TextColor(a.pal.brand)
				}
				ui.Spacer(c)
				if ghost(c, "收起原文", ui.Color{}).Clicked() {
					a.sourceOpen = false
				}
				if a.pal.primary(c, "摘为新思路").Disabled(a.busy != "" || start == end).Clicked() {
					eng := &discussion.Engine{Store: a.Store}
					_, err := eng.Manual(a.session, a.sourceMessage, start, end, a.sourceCategory)
					if err == nil {
						a.sourceOpen = false
						a.filter, a.extra, a.turn = 0, extras[0], ""
						a.notice = "已摘为新思路，放在“待处理”里"
						err = a.refresh()
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
