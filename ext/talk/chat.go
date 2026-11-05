package talk

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/ext/talk/talktext"
	"writer/workspace"
)

// column is the reading width of the conversation.
const column = 720

// Discussion runs on the frame thread.
func (a *Panel) Discussion(c *ui.Context) {
	pad := a.TalkPadding(c)
	ui.Row(c).FillWidth().Grow(1).AlignItems(ui.Stretch).Children(func() {
		chat := ui.Column(c).Grow(1).Shrink(1).FillHeight()
		a.LastChat = chat.Bounds()
		chat.Children(func() {
			a.ChatTop(c)
			a.ChatList(c, false)
			ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(0, pad, 20, pad).Children(func() {
				a.Composer(c)
			})
		})
		a.native.SettingColumn(c)
	})
}

// TalkPadding runs on the frame thread.
func (a *Panel) TalkPadding(c *ui.Context) float32 {
	win, _ := c.Size()
	if win < 1180 {
		return 24
	}
	return 32
}

// chatList builds only the messages in view: laying out a long session
// whole each frame made streaming and scrolling stutter. Where the talk was
// compacted last, its digest lies under the last turn it covers.
// ChatList runs on the frame thread.
func (a *Panel) ChatList(c *ui.Context, side bool) {
	rows := a.Rows[:0]
	if !a.HasMessages() && !a.Replying() {
		rows = append(rows, -1)
	}
	digest, through := a.Compacted()
	for n, e := range a.Events {
		if said(e) {
			rows = append(rows, n)
		}
		if n == through {
			rows = append(rows, digest)
		}
	}
	if a.Replying() {
		rows = append(rows, -2)
	}
	a.Rows = rows
	if !side && len(rows) == 1 && rows[0] == -1 {
		// Nothing said yet: the page opens in the middle of its room.
		ui.Column(c).Grow(1).FillWidth().Padding(24, a.TalkPadding(c)).Children(func() { a.EmptyChat(c) })
		return
	}
	a.Chat.FollowEnd = true
	a.Chat.Key = func(i int) any {
		if rows[i] < 0 {
			return rows[i]
		}
		return a.Events[rows[i]].ID
	}
	list := ui.List(c, &a.Chat, len(rows), func(i int) {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(column).Children(func() {
				switch r := rows[i]; r {
				case -1:
					a.EmptyChat(c)
				case -2:
					a.Thinking(c)
				default:
					if a.Events[r].Role == workspace.RoleDigest {
						a.Digest(c, a.Events[r], through)
					} else {
						a.Message(c, a.Events[r])
					}
				}
			})
		})
	})
	if side {
		list.Grow(1).FillWidth().Gap(20).Padding(8, 2, 12, 2)
	} else {
		list.Grow(1).FillWidth().Gap(24).Padding(32, a.TalkPadding(c), 24, a.TalkPadding(c))
	}
}

// said tells the author's words and the replies from what a reply looked
// up on its way, which folds into the reply.
func said(e workspace.Event) bool {
	return e.Role == workspace.RoleUser || e.Role == workspace.RoleAssistant
}

// compacted is where the latest digest of the talk is among its events,
// and the event it runs through; -1 for both when there is none.
// Compacted runs on the frame thread.
func (a *Panel) Compacted() (int, int) {
	for n := len(a.Events) - 1; n >= 0; n-- {
		if d := a.Events[n]; d.Role == workspace.RoleDigest {
			for m, e := range a.Events {
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
// Digest runs on the frame thread.
func (a *Panel) Digest(c *ui.Context, d workspace.Event, through int) {
	t := a.palette().Theme
	turns := 0
	for _, e := range a.Events[:through+1] {
		if e.Role == workspace.RoleAssistant {
			turns++
		}
	}
	key := "digest-" + d.ID
	open := a.Thoughts[key]
	if open == nil {
		open = new(bool)
		a.Thoughts[key] = open
	}
	ui.Column(c.Key(d.ID)).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
		label := talktext.ChatDigestRounds(turns)
		head := ui.ButtonBase(c).Row().Gap(4).AlignItems(ui.Center).Label(label)
		fg, mark := a.palette().Faint, kit.IcChevron
		if head.Hovered() {
			fg = a.palette().Ink
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
		ui.Column(c).FillWidth().Padding(12, 14).Gap(8).Radius(kit.RCard).Background(a.palette().Side).Border(1, a.palette().Line).Children(func() {
			for _, s := range workspace.DigestSections(d.Text) {
				ui.Column(c).FillWidth().Children(func() {
					if s[0] != "" {
						ui.Text(c, s[0]).FontSize(kit.FsNote).FontWeight(600).LineHeight(kit.LhBody).FillWidth()
					}
					a.Markdown(c, s[1], kit.DigestText)
				})
			}
			ui.Text(c, talktext.ChatDigestNote()).FontSize(kit.FsNote).LineHeight(kit.LhBody).TextColor(t.TextMuted).FillWidth()
		})
	})
}

// spoken is how many events of the talk are its words and replies.
// Spoken runs on the frame thread.
func (a *Panel) Spoken() int {
	n := 0
	for _, e := range a.Events {
		if said(e) {
			n++
		}
	}
	return n
}

// HasMessages runs on the frame thread.
func (a *Panel) HasMessages() bool { return a.Spoken() > 0 }

// LastMessage runs on the frame thread.
func (a *Panel) LastMessage() string {
	for n := len(a.Events) - 1; n >= 0; n-- {
		if said(a.Events[n]) {
			return a.Events[n].ID
		}
	}
	return ""
}

// ChatTop runs on the frame thread.
func (a *Panel) ChatTop(c *ui.Context) {
	t := a.palette().Theme
	words, _ := a.TalkName(a.Session)
	ui.Row(c).FillWidth().Height(kit.TitleHeight).Padding(0, 24).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Text(c, talktext.TalkAll()).FontSize(kit.FsControl).TextColor(t.TextMuted)
		kit.Glyph(c, kit.IcChevron, kit.IconSm, a.palette().Faint)
		ui.Text(c, words).FontSize(kit.FsBody).FontWeight(600).SingleLine().Ellipsis("…").Shrink(1)
		ui.Spacer(c)
		if n := a.Spoken(); n > 0 {
			ui.Text(c, talktext.TalkCount(n)).FontSize(kit.FsNote).TextColor(a.palette().Faint)
		}
	})
	a.palette().Hairline(c)
}

// openUnit is what the writing page has open: a chapter, or a folder,
// which the author thinks of as a volume.
// OpenUnit runs on the frame thread.
func (a *Panel) OpenUnit() string {
	if n, ok := a.content().Tree().Node(a.selected()); ok && n.Folder {
		return "volume"
	}
	return "chapter"
}

// SwitchSession runs on the frame thread.
func (a *Panel) SwitchSession(id string) {
	if id == "" || id == a.Session {
		return
	}
	a.Session = id
	a.Source, a.SourceOpen, a.ContextReply = nil, false, ""
	a.SetError(a.Refresh())
	a.Chat = ui.ListState{} // a list's place is its session's
}

// EmptyChat runs on the frame thread.
func (a *Panel) EmptyChat(c *ui.Context) {
	t := a.palette().Theme
	title, about := talktext.TalkStartTitle(), talktext.TalkStartAbout()
	starts := []string{talktext.TalkStart1(), talktext.TalkStart2(), talktext.TalkStart3()}
	if len(a.content().Setting().Entries) > 0 { // a work begun goes on from what it has
		title, about = talktext.TalkContinueTitle(), talktext.TalkContinueAbout()
		starts = []string{talktext.TalkNext1(), talktext.TalkNext2(), talktext.TalkNext3()}
	}
	if a.desktop.Writing() { // beside the prose the talk is about the page in front of the author
		pace := talktext.ChatAskChapterPace()
		if n, ok := a.content().Tree().Node(a.selected()); ok && n.Folder {
			pace = talktext.ChatAskVolumePace()
		}
		starts = []string{pace, talktext.ChatAskNext(), talktext.ChatAskMotives()}
		// In the column they are rows, as its lists are.
		ui.Column(c).FillWidth().Margin(0, -10).Children(func() {
			a.palette().PaneGroup(c, talktext.ChatExamples())
			for _, s := range starts {
				if a.palette().PaneRow(c, s, s, "", "", "", false).Clicked() {
					a.Input, a.FocusTalk = s, true
				}
			}
		})
		return
	}
	a.palette().Blank(c, kit.IcSparkle, title, about, func() {
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
					a.Input, a.FocusTalk = s, true
				}
			}
		})
	})
}

// Answered runs on the frame thread.
func (a *Panel) Answered(userID string) bool {
	for _, e := range a.Events {
		if e.Role == workspace.RoleAssistant && e.ReplyTo == userID {
			return true
		}
	}
	return false
}
