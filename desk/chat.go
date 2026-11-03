package desk

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/locales"
	"writer/workspace"
)

// column is the reading width of the conversation.
const column = 720

func (a *App) discussion(c *ui.Context) {
	pad := a.talkPadding(c)
	ui.Row(c).FillWidth().Grow(1).AlignItems(ui.Stretch).Children(func() {
		chat := ui.Column(c).Grow(1).Shrink(1).FillHeight()
		a.LastChat = chat.Bounds()
		chat.Children(func() {
			a.chatTop(c)
			a.chatList(c, false)
			ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(0, pad, 20, pad).Children(func() {
				a.composer(c)
			})
		})
		a.settingColumn(c)
	})
}

func (a *App) talkPadding(c *ui.Context) float32 {
	win, _ := c.Size()
	if win < narrow {
		return 24
	}
	return 32
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
		ui.Column(c).Grow(1).FillWidth().Padding(24, a.talkPadding(c)).Children(func() { a.emptyChat(c) })
		return
	}
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
						a.message(c, a.events[r])
					}
				}
			})
		})
	})
	if side {
		list.Grow(1).FillWidth().Gap(20).Padding(8, 2, 12, 2)
	} else {
		list.Grow(1).FillWidth().Gap(24).Padding(32, a.talkPadding(c), 24, a.talkPadding(c))
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
		label := locales.ChatDigestRounds(turns)
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
			ui.Text(c, locales.ChatDigestNote()).FontSize(kit.FsNote).LineHeight(kit.LhBody).TextColor(t.TextMuted).FillWidth()
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
		ui.Text(c, locales.TalkAll()).FontSize(kit.FsControl).TextColor(t.TextMuted)
		kit.Glyph(c, kit.IcChevron, kit.IconSm, a.pal.Faint)
		ui.Text(c, words).FontSize(kit.FsBody).FontWeight(600).SingleLine().Ellipsis("…").Shrink(1)
		ui.Spacer(c)
		if n := a.spoken(); n > 0 {
			ui.Text(c, locales.TalkCount(n)).FontSize(kit.FsNote).TextColor(a.pal.Faint)
		}
	})
	a.pal.Hairline(c)
}

// openUnit is what the writing page has open: a chapter, or a folder,
// which the author thinks of as a volume.
func (a *App) openUnit() string {
	if n, ok := a.book.Tree().Node(a.write.node); ok && n.Folder {
		return "volume"
	}
	return "chapter"
}

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
	title, about := locales.TalkStartTitle(), locales.TalkStartAbout()
	starts := []string{locales.TalkStart1(), locales.TalkStart2(), locales.TalkStart3()}
	if len(a.book.Setting().Entries) > 0 { // a work begun goes on from what it has
		title, about = locales.TalkContinueTitle(), locales.TalkContinueAbout()
		starts = []string{locales.TalkNext1(), locales.TalkNext2(), locales.TalkNext3()}
	}
	if a.page == pageWrite { // beside the prose the talk is about the page in front of the author
		pace := locales.ChatAskChapterPace()
		if n, ok := a.book.Tree().Node(a.write.node); ok && n.Folder {
			pace = locales.ChatAskVolumePace()
		}
		starts = []string{pace, locales.ChatAskNext(), locales.ChatAskMotives()}
		// In the column they are rows, as its lists are.
		ui.Column(c).FillWidth().Margin(0, -10).Children(func() {
			a.pal.PaneGroup(c, locales.ChatExamples())
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
				kit.PressRow(c, b)
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
