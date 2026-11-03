package desk

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"strings"
	"time"
	"writer/desk/kit"
	"writer/locales"
	"writer/tokens"
	"writer/workspace"
)

// replyIndent is where a reply's text starts: past the mark and the gap
// after it.
const replyIndent = 24 + 12

func (a *App) aiMark(c *ui.Context) {
	ui.Box(c).Size(kit.HControlSm, kit.HControlSm).Center().Shrink(0).Children(func() {
		kit.Glyph(c, kit.IcSparkle, kit.IconSm, a.pal.Faint)
	})
}

func (a *App) userBubble(c *ui.Context, text string) {
	ui.Column(c).MaxWidthPercent(82).Padding(8, 12).Radius(kit.RDialog).Background(a.pal.Theme.SurfacePressed).Children(func() {
		ui.Text(c, text).FontSize(kit.FsRead).LineHeight(kit.LhLoose).Selectable()
	})
}

// message shows the author's words as a bubble on the right and the AI's as
// plain prose, the way a reply reads in a letter. Its actions fade in on hover.
func (a *App) message(c *ui.Context, m workspace.Event) {
	t := a.pal.Theme
	user := m.Role == workspace.RoleUser
	align := ui.Start
	if user {
		align = ui.End
	}
	box := ui.Column(c.Key(m.ID)).FillWidth().AlignItems(align).Gap(8)
	shown := box.Hovered() || box.FocusWithin() || a.contextReply == m.ID
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
						a.thought(c, m.ID, locales.ChatThought(), m.Reasoning, false)
					}
					if found := workspace.Looks(a.events, m.ReplyTo); len(found) > 0 {
						a.looked(c, "look-"+m.ID, found, false)
					}
					for _, e := range proposed(a.events, m.ReplyTo) {
						a.propCard(c, a.session, e)
					}
					a.markdown(c, m.Text, kit.ReplyText)
					if m.Capped {
						ui.Text(c, locales.ChatCapped()).FontSize(kit.FsNote).TextColor(t.TextMuted).FillWidth()
					}
				})
			})
		}
		acts := ui.Row(c).MaxWidthPercent(100).MinWidth(0).Gap(2).AlignItems(ui.Center)
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
				ui.Text(c, foot).FontSize(kit.FsSmall).TextColor(a.pal.Faint).Padding(0, 6, 0, 0).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0).Tooltip(foot)
			}
			if kit.IconButton(c, kit.IcQuote, "摘取-"+m.ID, ui.Color{}).Tooltip(locales.ChatPull()).KeepFocus().Disabled(a.held()).Clicked() {
				a.openSource(m, 0, 0)
			}
			if !user {
				// Its Markdown, which keeps its structure pasted elsewhere.
				if kit.IconButton(c, kit.IcCopy, "复制-"+m.ID, ui.Color{}).Tooltip(locales.ChatCopyAll()).Clicked() {
					c.WriteClipboard(m.Text)
				}
				on := a.contextReply == m.ID
				color := ui.Color{}
				if on {
					color = a.pal.Ink
				}
				if kit.IconButton(c, kit.IcEye, "上下文-"+m.ID, color).Tooltip(locales.ChatContextSent()).Clicked() {
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
				ui.Text(c, locales.ChatUnanswered()).FontSize(kit.FsNote).TextColor(t.Warning)
				if a.pal.Ghost(c, locales.ChatRetry(), ui.Color{}).Disabled(a.busy != "").Clicked() {
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
	if a.turn.pending != "" && !a.saved() {
		ui.Column(c).FillWidth().AlignItems(ui.End).Gap(8).Children(func() {
			for n, q := range a.turn.quotes {
				ui.Column(c).MaxWidthPercent(82).Children(func() {
					a.quoteBlock(c, q, fmt.Sprintf("pending-%d", n), nil)
				})
			}
			a.userBubble(c, a.turn.pending)
		})
	}
	reasoning, text, steps, compacting := a.turn.live.read()
	found := workspace.Looks(steps, "")
	proposing, writing := len(a.turn.live.proposals()) > 0 || len(proposed(steps, "")) > 0, a.writingProps()
	ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Start).Children(func() {
		a.aiMark(c)
		ui.Column(c).Grow(1).Shrink(1).Gap(10).Children(func() {
			if reasoning != "" {
				a.thought(c, "live-reply", locales.ChatThought(), reasoning, text == "" && len(found) == 0 && !proposing)
			}
			if len(found) > 0 {
				a.looked(c, "live-look", found, text == "" && !proposing)
			}
			a.turnCards(c, steps)
			if text != "" {
				a.liveMarkdown(c, text, kit.ReplyText)
			}
			status := locales.ChatReplying()
			switch {
			case compacting:
				status = locales.ChatPacking()
			case writing:
				status = locales.ChatProposing()
			case text != "":
			case len(found) > 0:
				status = locales.ChatLookingMore()
			default:
				status = locales.ChatThinkingMore()
			}
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c)
				ui.Text(c, status).FontSize(kit.FsControl).TextColor(t.TextMuted).Grow(1).Shrink(1)
				if a.pal.Ghost(c, locales.ChatCancelRequest(), ui.Color{}).Clicked() && a.cancel != nil {
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
			return e.Role == workspace.RoleUser && e.Text == a.turn.pending
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
		label = locales.ChatThinking()
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
	label := locales.ChatLooking()
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
						if a.pal.PaneLink(c, l.Label, locales.ChatViewLore(), fmt.Sprintf("查看设定-%s-%d", key, n)) {
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
var partNames = map[string]func() string{
	workspace.PartRules:    locales.ChatPartRules,
	workspace.PartIndex:    locales.ChatPartCatalog,
	workspace.PartDigest:   locales.ChatPartDigest,
	workspace.PartOutline:  locales.ChatPartOutline,
	workspace.PartCards:    locales.ChatPartCards,
	workspace.PartOutlines: locales.ChatPartPrevOutlines,
	workspace.PartPrevious: locales.ChatPartPrevChapter,
	workspace.PartChapter:  locales.ChatPartThisChapter,
	workspace.PartRecap:    locales.ChatPartRecap,
	workspace.PartLooked:   locales.ChatPartLook,
	workspace.PartTalk:     locales.ChatPartBefore,
	workspace.PartProse:    locales.ChatPartProse,
	workspace.PartInput:    locales.ChatPartSaid,
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
					name := ""
					if label := partNames[p.Name]; label != nil {
						name = label()
					}
					if p.Name == workspace.PartSetting {
						name = locales.ChatPartLoreCount(len(setting))
					}
					parts = append(parts, name+" "+tokens.Round(p.Tokens))
				}
				ui.Text(c, locales.ChatSentAbout(tokens.Round(total))).FontSize(kit.FsNote).FontWeight(600)
				if n := workspace.Requests(a.events, m); n > 1 { // what it says is of the request that replied
					note(locales.ChatRoundRequests(n))
				}
				note(strings.Join(parts, " · "))
			}
			if u := m.Usage; u != nil {
				used := locales.ChatUsage(tokens.Round(u.Input), tokens.Round(u.Output))
				if u.CacheRead > 0 {
					used = locales.ChatUsageCached(tokens.Round(u.Input), tokens.Round(u.CacheRead), tokens.Round(u.Output))
				}
				note(used)
			}
			for _, item := range beside {
				row(item.Text, true)
			}
			head := locales.ChatAttachedAll(len(setting))
			switch {
			case index:
				head = locales.ChatCrowded()
			case len(m.Omitted) > 0: // a reply from before the setting went whole or as an index
				head = locales.ChatAttachedOmitted(len(setting), len(m.Omitted))
			case m.Sent != nil && m.Sent.AsOf != "": // a talk that keeps to a chapter has the setting as of it
				head = locales.ChatAttachedAt(len(setting), m.Sent.AsOf)
			}
			ui.Text(c, head).FontSize(kit.FsNote).FontWeight(600)
			if index {
				note(locales.ChatCatalogNote())
			}
			if m.Sent != nil && m.Sent.Ahead > 0 {
				note(locales.ChatMoreChanges(m.Sent.Ahead))
			}
			if len(setting) == 0 && len(m.Omitted) == 0 {
				note(locales.ChatLoreEmpty())
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
