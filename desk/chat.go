package desk

import (
	"context"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"writer/aiport"
	"writer/desk/kit"
	"writer/editor"
	"writer/tokens"
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
// whole each frame made streaming and scrolling stutter. Where the talk was
// compacted last, its digest lies under the last turn it covers.
func (a *App) chatList(c *ui.Context, side bool) {
	rows := a.rows[:0]
	if !a.hasMessages() && !a.replying() {
		rows = append(rows, -1)
	}
	digest, through := a.compacted()
	for n, e := range a.events {
		if said(e) {
			rows = append(rows, n)
		}
		if n == through {
			rows = append(rows, digest)
		}
	}
	if a.replying() {
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
					if a.events[r].Role == workspace.RoleDigest {
						a.digest(c, a.events[r], through)
					} else {
						a.message(c, a.events[r], a.events[r].ID == last)
					}
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

// said tells the author's words and the replies from what a reply looked
// up on its way, which folds into the reply.
func said(e workspace.Event) bool {
	return e.Role == workspace.RoleUser || e.Role == workspace.RoleAssistant
}

// compacted is where the latest digest of the talk is among its events,
// and the event it runs through; -1 for both when there is none.
func (a *App) compacted() (int, int) {
	for n := len(a.events) - 1; n >= 0; n-- {
		if d := a.events[n]; d.Role == workspace.RoleDigest {
			for m, e := range a.events {
				if e.ID == d.Through {
					return n, m
				}
			}
			break
		}
	}
	return -1, -1
}

// digest is a line where the talk above it was compacted, which opens the
// digest that stands for it: to read, not to change.
func (a *App) digest(c *ui.Context, d workspace.Event, through int) {
	t := a.pal.Theme
	turns := 0
	for _, e := range a.events[:through+1] {
		if e.Role == workspace.RoleAssistant {
			turns++
		}
	}
	key := "digest-" + d.ID
	open := a.thoughts[key]
	if open == nil {
		open = new(bool)
		a.thoughts[key] = open
	}
	ui.Column(c.Key(d.ID)).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
		label := fmt.Sprintf("以上 %d 轮已整理成摘要", turns)
		head := ui.ButtonBase(c).Row().Gap(4).AlignItems(ui.Center).Label(label)
		fg, mark := a.pal.Faint, kit.IcChevron
		if head.Hovered() {
			fg = a.pal.Ink
		}
		if *open {
			mark = kit.IcChevronDown
		}
		head.Children(func() {
			ui.Text(c, label).FontSize(kit.FsNote).TextColor(fg)
			kit.Glyph(c, mark, kit.IconXs, fg)
		})
		if head.Clicked() {
			*open = !*open
		}
		if !*open {
			return
		}
		ui.Column(c).FillWidth().Padding(12, 14).Gap(8).Radius(kit.RCard).Background(a.pal.Side).Border(1, a.pal.Line).Children(func() {
			for _, s := range workspace.DigestSections(d.Text) {
				ui.Column(c).FillWidth().Children(func() {
					if s[0] != "" {
						ui.Text(c, s[0]).FontSize(kit.FsNote).FontWeight(600).LineHeight(kit.LhBody).FillWidth()
					}
					a.markdown(c, s[1], kit.DigestText)
				})
			}
			ui.Text(c, "摘要由 AI 写成，有误可在对话中指出。").FontSize(kit.FsNote).LineHeight(kit.LhBody).TextColor(t.TextMuted).FillWidth()
		})
	})
}

// spoken is how many events of the talk are its words and replies.
func (a *App) spoken() int {
	n := 0
	for _, e := range a.events {
		if said(e) {
			n++
		}
	}
	return n
}

func (a *App) hasMessages() bool { return a.spoken() > 0 }

func (a *App) lastMessage() string {
	for n := len(a.events) - 1; n >= 0; n-- {
		if said(a.events[n]) {
			return a.events[n].ID
		}
	}
	return ""
}

func (a *App) chatTop(c *ui.Context) {
	t := a.pal.Theme
	words := a.titles[a.session]
	ui.Row(c).FillWidth().Height(kit.TitleHeight).Padding(0, 24).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Text(c, "总讨论").FontSize(kit.FsControl).TextColor(t.TextMuted)
		kit.Glyph(c, kit.IcChevron, kit.IconSm, a.pal.Faint)
		ui.Text(c, words).FontSize(kit.FsBody).FontWeight(600).SingleLine().Ellipsis("…").Shrink(1)
		ui.Spacer(c)
		if n := a.spoken(); n > 0 {
			ui.Textf(c, "%d 条发言", n).FontSize(kit.FsNote).TextColor(a.pal.Faint)
		}
	})
	a.pal.Hairline(c)
}

// openUnit is what the writing page has open: a chapter, or a folder,
// which the author thinks of as a volume.
func (a *App) openUnit() string {
	if n, ok := a.book.Tree().Node(a.write.node); ok && n.Folder {
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
	a.SetError(a.refresh())
	a.chat = ui.ListState{} // a list's place is its session's
}

func (a *App) emptyChat(c *ui.Context) {
	t := a.pal.Theme
	title, about := "从一个初步的想法开始", "写下初步的想法。讨论中定下的内容，AI 会当场记进右侧的设定文档；已确定的设定将作为后续讨论的依据，已放弃的不再提及。"
	starts := []string{"我想写一个总在下雨的城市", "主角有个说不出口的秘密", "这个故事的冲突在哪"}
	if len(a.book.Setting().Entries) > 0 { // a work begun goes on from what it has
		title, about = "接着聊这部作品", "AI 会参考已有的设定和正文。"
		starts = []string{"接下来的情节怎么走", "哪些设定还没想清楚", "梳理一下人物关系"}
	}
	if a.page == pageWrite { // beside the prose the talk is about the page in front of the author
		unit := a.openUnit()
		starts = []string{"这一" + unit + "的节奏怎么样", "接下来可以怎么写", "人物的动机说得通吗"}
		// In the column they are rows, as its lists are.
		ui.Column(c).FillWidth().Margin(0, -10).Children(func() {
			a.pal.PaneGroup(c, "示例问题")
			for _, s := range starts {
				if a.pal.PaneRow(c, s, s, "", "", "", false).Clicked() {
					a.input, a.focusTalk = s, true
				}
			}
		})
		return
	}
	a.pal.Blank(c, kit.IcSparkle, title, about, func() {
		ui.Row(c).Gap(8).Padding(10, 0, 0, 0).Wrap().Justify(ui.Center).Children(func() {
			for _, s := range starts {
				b := ui.ButtonBase(c).Padding(6, 12).Radius(kit.RRound).Border(1, t.Border)
				fg := t.TextMuted
				if b.Hovered() {
					b.Background(t.SurfaceHover)
					fg = t.Text
				}
				b.Children(func() { ui.Text(c, s).FontSize(kit.FsControl).TextColor(fg) })
				if b.Clicked() {
					a.input, a.focusTalk = s, true
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
	ui.Box(c).Size(kit.HControlSm, kit.HControlSm).Radius(kit.RRound).Background(a.pal.Theme.SurfacePressed).Center().Shrink(0).Children(func() {
		kit.Glyph(c, kit.IcSparkle, kit.IconSm, a.pal.Theme.TextMuted)
	})
}

func (a *App) userBubble(c *ui.Context, text string) {
	ui.Column(c).MaxWidthPercent(82).Padding(8, 12).Radius(kit.RDialog).Background(a.pal.Theme.SurfacePressed).Children(func() {
		ui.Text(c, text).FontSize(kit.FsRead).LineHeight(kit.LhLoose).Selectable()
	})
}

// message shows the author's words as a bubble on the right and the AI's as
// plain prose, the way a reply reads in a letter. Its actions fade in on hover.
func (a *App) message(c *ui.Context, m workspace.Event, last bool) {
	t := a.pal.Theme
	user := m.Role == workspace.RoleUser
	align := ui.Start
	if user {
		align = ui.End
	}
	box := ui.Column(c.Key(m.ID)).FillWidth().AlignItems(align).Gap(8)
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
					if found := workspace.Looks(a.events, m.ReplyTo); len(found) > 0 {
						a.looked(c, "look-"+m.ID, found, false)
					}
					for _, e := range proposed(a.events, m.ReplyTo) {
						a.propCard(c, a.session, e)
					}
					a.markdown(c, m.Text, kit.ReplyText)
					if m.Capped {
						ui.Text(c, "已达到本次查阅上限，以上回答仅依据已查到的内容。").FontSize(kit.FsNote).TextColor(t.TextMuted).FillWidth()
					}
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
				foot := kit.When(m.At, time.Now())
				if who := a.replyLabel(m.Model, m.Effort); who != "" {
					foot += " · " + who
				}
				if u, ok := workspace.Spent(a.events, m); ok {
					foot += " · " + tokens.Round(u.Input+u.Output) + " token"
					if cost := a.turnCost(a.events, m); cost != "" {
						foot += " · " + cost
					}
				}
				ui.Text(c, foot).FontSize(kit.FsSmall).TextColor(a.pal.Faint).Padding(0, 6, 0, 0)
			}
			if kit.IconButton(c, kit.IcQuote, "摘取-"+m.ID, ui.Color{}).Tooltip("摘取原文").KeepFocus().Disabled(a.held()).Clicked() {
				a.openSource(m, 0, 0)
			}
			if !user {
				// Its Markdown, which keeps its structure pasted elsewhere.
				if kit.IconButton(c, kit.IcCopy, "复制-"+m.ID, ui.Color{}).Tooltip("复制全文").Clicked() {
					c.WriteClipboard(m.Text)
				}
				on := a.contextReply == m.ID
				color := ui.Color{}
				if on {
					color = a.pal.Ink
				}
				if kit.IconButton(c, kit.IcEye, "上下文-"+m.ID, color).Tooltip("AI 参考内容").Clicked() {
					if on {
						a.contextReply = ""
					} else {
						a.contextReply = m.ID
					}
				}
			}
			if user && !m.At.IsZero() {
				ui.Text(c, kit.When(m.At, time.Now())).FontSize(kit.FsSmall).TextColor(a.pal.Faint).Padding(0, 0, 0, 6)
			}
		})
		if user && !a.answered(m.ID) && !a.replying() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				kit.Glyph(c, kit.IcAlert, kit.IconSm, t.Warning)
				ui.Text(c, "该消息尚未收到回复").FontSize(kit.FsNote).TextColor(t.Warning)
				if a.pal.Ghost(c, "重试回复", ui.Color{}).Disabled(a.busy != "").Clicked() {
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
// its reasoning and what it looked up, which stays open while it looks.
func (a *App) thinking(c *ui.Context) {
	t := a.pal.Theme
	if a.pending != "" && !a.saved() {
		ui.Column(c).FillWidth().AlignItems(ui.End).Gap(8).Children(func() {
			for n, q := range a.pendingQuotes {
				ui.Column(c).MaxWidthPercent(82).Children(func() {
					a.quoteBlock(c, q, fmt.Sprintf("pending-%d", n), nil)
				})
			}
			a.userBubble(c, a.pending)
		})
	}
	reasoning, text, steps, compacting := a.live.read()
	found := workspace.Looks(steps, "")
	proposing, writing := len(a.live.proposals()) > 0 || len(proposed(steps, "")) > 0, a.writingProps()
	ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Start).Children(func() {
		a.aiMark(c)
		ui.Column(c).Grow(1).Shrink(1).Gap(10).Children(func() {
			if reasoning != "" {
				a.thought(c, "live-reply", "思考过程", reasoning, text == "" && len(found) == 0 && !proposing)
			}
			if len(found) > 0 {
				a.looked(c, "live-look", found, text == "" && !proposing)
			}
			a.turnCards(c, steps)
			if text != "" {
				a.liveMarkdown(c, text, kit.ReplyText)
			}
			status := "正在回复…"
			switch {
			case compacting:
				status = "正在压缩上下文…"
			case writing:
				status = "正在正文中提议修改…"
			case text != "":
			case len(found) > 0:
				status = "正在查阅…"
			default:
				status = "正在思考…"
			}
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c)
				ui.Text(c, status).FontSize(kit.FsControl).TextColor(t.TextMuted).Grow(1).Shrink(1)
				if a.pal.Ghost(c, "取消请求", ui.Color{}).Clicked() && a.cancel != nil {
					a.cancel()
				}
			})
		})
	})
}

// saved tells whether the words in flight are in the talk shown, read
// back by what refreshed it as the reply went on.
func (a *App) saved() bool {
	for n := len(a.events) - 1; n >= 0; n-- {
		if e := a.events[n]; said(e) {
			return e.Role == workspace.RoleUser && e.Text == a.pending
		}
	}
	return false
}

// thought folds what a model reasoned before it answered; it stays open
// while the reasoning is all that is streaming.
func (a *App) thought(c *ui.Context, key, label, reasoning string, running bool) {
	t := a.pal.Theme
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
			ui.Box(c).Width(2).Radius(kit.RRound).Background(a.pal.Line).Shrink(0)
			if !running {
				ui.Text(c, reasoning).FontSize(kit.FsControl).LineHeight(kit.LhLoose).TextColor(t.TextMuted).Selectable().Grow(1).Shrink(1)
				return
			}
			// While it streams, the paragraphs written keep their layout
			// and only the last is laid out again, as liveMarkdown does.
			ui.Column(c).Grow(1).Shrink(1).Children(func() {
				cut := strings.LastIndex(reasoning, "\n") + 1
				for _, s := range []string{strings.TrimSuffix(reasoning[:cut], "\n"), reasoning[cut:]} {
					if s != "" {
						ui.Text(c, s).FontSize(kit.FsControl).LineHeight(kit.LhLoose).TextColor(t.TextMuted).FillWidth()
					}
				}
			})
		})
	}).FillWidth()
}

// proposed are the results of the calls by which the reply to the
// author's words userID proposed changes in the prose, in the order it
// proposed them; for "" all of them, as the turn in flight has them.
func proposed(events []workspace.Event, userID string) []workspace.Event {
	var out []workspace.Event
	for _, e := range events {
		if e.Role == workspace.RoleResult && e.Proposal != nil && (userID == "" || e.ReplyTo == userID) {
			out = append(out, e)
		}
	}
	return out
}

// turnCost is what the requests of the turn reply ends cost by the prices
// built in, or nothing when one of them is of a model with no price.
func (a *App) turnCost(events []workspace.Event, reply workspace.Event) string {
	costs := map[string]float64{}
	for _, e := range events {
		if e.Usage == nil || e.ID != reply.ID && (e.Role != workspace.RoleCall || e.ReplyTo != reply.ReplyTo) {
			continue
		}
		cost, currency, ok := a.ai.Cost(e.Model, *e.Usage)
		if !ok {
			return ""
		}
		costs[currency] += cost
	}
	return costText(costs)
}

// looked folds what a reply looked up above it, as thought folds its
// reasoning: the head says what it did, and open, each thing shows what
// came back. What it noted links to the entry. It stays open while the
// reply looks.
func (a *App) looked(c *ui.Context, key string, found []workspace.Look, running bool) {
	t := a.pal.Theme
	open := a.thoughts[key]
	if open == nil || running {
		v := running
		open = &v
		a.thoughts[key] = open
	}
	label := "正在查阅"
	if !running {
		labels := make([]string, len(found))
		for n, l := range found {
			labels[n] = l.Label
		}
		label = strings.Join(labels, "，")
	}
	ui.Collapsible(c, label, open, func() {
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Stretch).Padding(4, 0, 2, 2).Children(func() {
			ui.Box(c).Width(2).Radius(kit.RRound).Background(a.pal.Line).Shrink(0)
			ui.Column(c).Grow(1).Shrink(1).Gap(8).Children(func() {
				for n, l := range found {
					if r := l.Result.Revision; r != "" {
						if a.pal.PaneLink(c, l.Label, "查看设定", fmt.Sprintf("查看设定-%s-%d", key, n)) {
							a.showNoted(r)
						}
						continue
					}
					ui.Column(c).FillWidth().Gap(2).Children(func() {
						ui.Text(c, l.Label).FontSize(kit.FsControl).TextColor(t.TextMuted).Selectable().FillWidth()
						if l.Result.Text != "" {
							ui.Text(c, l.Result.Text).FontSize(kit.FsNote).LineHeight(kit.LhBody).TextColor(a.pal.Faint).Selectable().FillWidth()
						}
					})
				}
			})
		})
	}).FillWidth()
}

// showNoted opens in the setting the entry revision r of a talk changed.
func (a *App) showNoted(r string) {
	var entry string
	for _, rev := range a.book.Setting().Revisions {
		if rev.ID == r && len(rev.Ops) > 0 {
			entry = rev.Ops[0].Entry
		}
	}
	if e, ok := a.book.Setting().Entry(entry); ok {
		a.showEntry(e)
	}
}

// showEntry opens e in the setting: in what waits on the author when it is
// open, or on its page.
func (a *App) showEntry(e workspace.Entry) {
	a.setDocHidden(false)
	a.doc.tab, a.doc.talks = paneSetting, false
	if e.Place == workspace.PlaceOpen {
		a.doc.route.jumpToWaiting()
	} else {
		a.doc.route.jumpToEntry(e.ID)
	}
}

// replyLabel names the model and effort a reply came from.
func (a *App) replyLabel(name, effort string) string {
	ref, _ := workspace.ParseModelRef(name)
	name = ref.Model
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if effort != "" && name != "" {
		name += " · " + a.ai.EffortLabel(effort)
	}
	return name
}

// partNames name the parts of a talk request as the author reads them.
var partNames = map[string]string{
	workspace.PartRules:    "规则",
	workspace.PartIndex:    "设定目录",
	workspace.PartDigest:   "讨论摘要",
	workspace.PartOutline:  "章纲和卷纲",
	workspace.PartCards:    "本章的卡片",
	workspace.PartOutlines: "本卷前面各章章纲",
	workspace.PartPrevious: "上一章全文",
	workspace.PartChapter:  "本章全文",
	workspace.PartRecap:    "前情提要",
	workspace.PartLooked:   "查阅结果",
	workspace.PartTalk:     "之前的对话",
	workspace.PartProse:    "附上的正文",
	workspace.PartInput:    "本次发言",
}

// audit is what a reply was sent, by the request that replied when the
// turn took more: each part's tokens as estimated, what the provider
// counted, and the setting, whole or as its index.
func (a *App) audit(c *ui.Context, m workspace.Event) {
	t := a.pal.Theme
	var setting, beside []workspace.ContextItem
	for _, item := range m.Context {
		if item.ID == "" {
			beside = append(beside, item)
		} else {
			setting = append(setting, item)
		}
	}
	index := m.Sent != nil && m.Sent.Part(workspace.PartIndex) > 0
	note := func(s string) {
		ui.Text(c, s).FontSize(kit.FsNote).TextColor(t.TextMuted).Selectable().FillWidth()
	}
	row := func(text string, tick bool) {
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			if tick {
				kit.Glyph(c, kit.IcCheck, kit.IconSm, t.Success)
			}
			ui.Text(c, text).FontSize(kit.FsNote).LineHeight(kit.LhBody).Selectable().Grow(1).Shrink(1)
		})
	}
	ui.Column(c).FillWidth().Padding(0, 0, 0, replyIndent).Children(func() {
		ui.Column(c).FillWidth().Padding(12, 14).Gap(8).Radius(kit.RCard).Background(a.pal.Side).Border(1, a.pal.Line).Children(func() {
			if s := m.Sent; s != nil {
				total := 0
				var parts []string
				for _, p := range s.Parts {
					total += p.Tokens
					name := partNames[p.Name]
					if p.Name == workspace.PartSetting {
						name = fmt.Sprintf("设定 %d 条", len(setting))
					}
					parts = append(parts, name+" "+tokens.Round(p.Tokens))
				}
				ui.Textf(c, "本次发送约 %s token", tokens.Round(total)).FontSize(kit.FsNote).FontWeight(600)
				if n := workspace.Requests(a.events, m); n > 1 { // what it says is of the request that replied
					note(fmt.Sprintf("这一轮共 %d 次请求，这是最后一次。", n))
				}
				note(strings.Join(parts, " · "))
			}
			if u := m.Usage; u != nil {
				used := "实际用量：输入 " + tokens.Round(u.Input)
				if u.CacheRead > 0 {
					used += "，其中缓存命中 " + tokens.Round(u.CacheRead)
				}
				note(used + "；输出 " + tokens.Round(u.Output))
			}
			for _, item := range beside {
				row(item.Text, true)
			}
			head := fmt.Sprintf("附带全部 %d 条设定", len(setting))
			switch {
			case index:
				head = "设定较多，本次只附目录"
			case len(m.Omitted) > 0: // a reply from before the setting went whole or as an index
				head = fmt.Sprintf("回复时附带 %d 条设定，因上下文上限省略 %d 条", len(setting), len(m.Omitted))
			case m.Sent != nil && m.Sent.AsOf != "": // a talk that keeps to a chapter has the setting as of it
				head = fmt.Sprintf("附带 %d 条设定，按「%s」时的样子", len(setting), m.Sent.AsOf)
			}
			ui.Text(c, head).FontSize(kit.FsNote).FontWeight(600)
			if index {
				note("目录只有分类、名字和第一句；需要时 AI 会查全文。")
			}
			if m.Sent != nil && m.Sent.Ahead > 0 {
				note(fmt.Sprintf("后面还有 %d 处变化，未列出。", m.Sent.Ahead))
			}
			if len(setting) == 0 && len(m.Omitted) == 0 {
				note("设定文档暂无内容。")
			}
			for _, item := range setting {
				row(item.Text, !index)
				// How the entry reads from chapters on, as it was sent under it.
				for _, later := range item.Changes {
					ui.Text(c, later).FontSize(kit.FsNote).LineHeight(kit.LhBody).TextColor(t.TextMuted).Selectable().FillWidth().Padding(0, 0, 0, kit.IconSm+8)
				}
			}
			for _, item := range m.Omitted {
				ui.Row(c).FillWidth().Gap(8).Children(func() {
					kit.Glyph(c, kit.IcX, kit.IconSm, a.pal.Faint)
					ui.Text(c, item.Text+" · "+item.Reason).FontSize(kit.FsNote).TextColor(t.TextMuted).Selectable().Grow(1).Shrink(1)
				})
			}
		})
	})
}

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
			panel.AttachTo(chip, ui.AnchorTopRight, ui.AnchorBottomRight).Margin(0, 0, 4, 0).Width(220)
		}, func(m *kit.Menu) { a.modelMenu(m, work) })
	}
	send := func() {
		can := a.busy == "" && kit.Trim(a.input) != ""
		send := ui.ButtonBase(c).Size(kit.HControl, kit.HControl).Radius(kit.RRound).Center().Label("发送").Tooltip("发送（" + cmdSend.keys() + "）").Disabled(!can).Shrink(0)
		bg := a.pal.Ink
		if !can {
			bg = t.SurfacePressed
		} else if send.Hovered() {
			bg = t.AccentHover
		}
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
					field(hint+"，"+modKey+"Enter 发送", 16, 1).Grow(1).Shrink(1)
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
			ui.Text(c, modKey+"Enter").FontSize(kit.FsSmall).TextColor(a.pal.Faint)
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

func (a *App) send(c *ui.Context) {
	if kit.Trim(a.input) == "" {
		return
	}
	input, quotes := a.input, a.quotes
	words := workspace.Event{Text: input, Quotes: quotes}
	if a.page == pageWrite {
		words.Chapter = a.write.node
	}
	a.input, a.quotes = "", nil
	a.say(c, a.session, words, func() {
		if a.input == "" {
			a.input, a.quotes = input, quotes // nothing was saved: give the words back
		}
	})
}

// say sends the author's words to the talk session, showing them there
// until they are saved; unsaid runs when they were not.
func (a *App) say(c *ui.Context, session string, words workspace.Event, unsaid func()) {
	talk, err := a.talk(c)
	if err != nil {
		a.SetError(err)
		if unsaid != nil {
			unsaid()
		}
		return
	}
	words.Role = workspace.RoleUser // as they are kept
	a.pending, a.pendingQuotes = words.Text, words.Quotes
	a.chat.ScrollToEnd()
	a.run(c, "AI 正在思考…", func(ctx context.Context) (func(), error) {
		saved, err := talk.Say(ctx, session, words)
		return func() {
			a.pending, a.pendingQuotes = "", nil
			if !saved && unsaid != nil {
				unsaid()
			}
			if a.session == session { // or the author went on to another talk
				a.chat.ScrollToEnd()
			}
		}, err
	})
	a.talking, a.talkSession, a.talkChapter = a.busy != "", session, originChapter(append(slices.Clip(a.events), words))
}

func (a *App) retry(c *ui.Context, userID string) {
	talk, err := a.talk(c)
	if err != nil {
		a.SetError(err)
		return
	}
	session := a.session
	a.run(c, "正在重新请求回复…", func(ctx context.Context) (func(), error) {
		err := talk.Retry(ctx, session, userID)
		return func() {
			if a.session == session {
				a.chat.ScrollToEnd()
			}
		}, err
	})
	a.talking, a.talkSession, a.talkChapter = a.busy != "", session, originChapter(a.events)
}

// keptTo is the chapter the talk shown keeps to, when it was opened to
// draft or rewrite it; its first words may be on their way still.
func (a *App) keptTo() string {
	if a.talking && a.talkSession == a.session {
		return a.talkChapter
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
				m.Item("复制").Hint(modKey + "C").Disabled(start == end).Do(func() { a.source.Copy() })
				m.Item("全选").Hint(modKey + "A").Do(func() { a.source.SelectAll() })
			})
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Select(c, &a.sourceCategory, a.categoryNames()).Label("摘取分类").Width(140)
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
