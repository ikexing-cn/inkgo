package talk

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"strings"
	"unicode/utf8"
	"writer/desk/kit"
	"writer/editor"
	"writer/ext/ai/aiapi"
	"writer/ext/talk/talktext"
	"writer/workspace"

	"writer/locales"
)

// composer is one rounded field: the text, then the model it goes to and a
// round send button. It grows with the text up to a few lines.
// Composer runs on the frame thread.
func (a *Panel) Composer(c *ui.Context) {
	t := a.palette().Theme
	work := a.Store.Work()
	ready := work.Discussion.Provider != "" && work.Discussion.Model != ""
	side := a.desktop.Writing()
	// The chip names the model and effort the next turn goes to, and
	// opens a menu to change either; its tooltip says where the words go.
	chip := func() {
		chip := ui.ButtonBase(c).Row().Gap(6).AlignItems(ui.Center).Padding(4, 8).Radius(kit.RControl).Label(talktext.ChatCurrentModel()).Tooltip(a.destination(work.Discussion)).Shrink(1).Disabled(a.held())
		if chip.Hovered() || a.desktop.MenuOpen("chat-model") {
			chip.Background(t.SurfaceHover)
		}
		chip.Children(func() {
			if ready {
				color := t.Success
				if !isLocal(a.ProviderURL(work.Discussion.Provider)) {
					color = a.palette().Ink
				}
				kit.Dot(c, color, 6)
				ui.Text(c, work.Discussion.Model).FontSize(kit.FsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Shrink(1)
				ui.Text(c, a.backend().EffortLabel(work.Discussion.Effort)).FontSize(kit.FsNote).TextColor(a.palette().Faint).SingleLine()
			} else {
				kit.Dot(c, t.Warning, 6)
				ui.Text(c, talktext.ChatPickModel()).FontSize(kit.FsNote).TextColor(t.Warning).SingleLine()
			}
			kit.Glyph(c, kit.IcChevronUp, kit.IconXs, a.palette().Faint)
		})
		a.desktop.MenuState().DropMenu(c, a.palette(), chip, "chat-model", func(panel ui.Element) {
			panel.TransformOrigin(1, 1)
			panel.AttachTo(chip, ui.AnchorTopRight, ui.AnchorBottomRight).Margin(0, 0, 4, 0).Width(220)
		}, func(m *kit.Menu) { a.ModelMenu(m, work) })
	}
	send := func() {
		can := !a.jobs.Busy() && kit.Trim(a.Input) != ""
		send := ui.ButtonBase(c).Size(kit.HControl, kit.HControl).Radius(kit.RRound).Center().Label(talktext.HelpSend()).Tooltip(talktext.TalkSend(sendKeys())).Disabled(!can).Shrink(0)
		bg := a.palette().Ink
		if !can {
			bg = t.SurfacePressed
		} else if send.Hovered() {
			bg = t.AccentHover
		}
		kit.Squeeze(send)
		send.Background(bg).Children(func() {
			fg := a.palette().OnInk
			if !can {
				fg = a.palette().Faint
			}
			kit.Glyph(c, kit.IcSend, kit.IconMd, fg)
		})
		if !a.jobs.Busy() && (send.Clicked() || c.Shortcut(ui.Cmd, ui.KeyEnter)) {
			a.Send(c)
		}
	}
	field := func(hint string, per, least int) ui.Element {
		lines := strings.Count(a.Input, "\n") + 1 + len([]rune(a.Input))/per
		h := float32(min(max(lines, least), 7)) * 25
		in := ui.TextAreaBase(c, &a.Input).Label(talktext.TalkInputLabel()).Placeholder(hint).Height(h).FontSize(kit.FsRead).LineHeight(kit.LhLoose).Disabled(a.held())
		if a.FocusTalk { // after the words offered, to go on with them
			n := utf8.RuneCountInString(a.Input)
			in.Focus().SetTextSelection(n, n)
			a.FocusTalk = false
		}
		return in
	}
	quotes := func() {
		for n, q := range a.Quotes {
			a.QuoteBlock(c, q, fmt.Sprintf("new-%d", n), func() { a.Quotes = append(a.Quotes[:n:n], a.Quotes[n+1:]...) })
		}
	}
	if side {
		ui.Column(c).FillWidth().Gap(8).Children(func() {
			box := ui.Column(c).FillWidth().Padding(10, 10, 10, 14).Gap(6).Radius(kit.RDialog).Background(a.palette().Canvas).Border(1, t.Border)
			if box.FocusWithin() {
				box.BorderColor(t.TextMuted.Alpha(0.55))
			}
			box.Children(func() {
				quotes()
				ui.Row(c).FillWidth().Gap(10).AlignItems(ui.End).Children(func() {
					// Enter is a new line here as on the page: the key that
					// sends is said, there being no room to show it beside.
					hint := talktext.ChatTypeMessage()
					if !a.HasMessages() {
						hint = talktext.ChatTypeTopic()
					}
					field(talktext.ChatSendHint(hint, sendKeys()), 16, 1).Grow(1).Shrink(1)
					send()
				})
			})
			ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
				// A talk opened to draft or rewrite keeps to its chapter: there
				// is nothing to choose.
				if kept := a.KeptTo(); kept != "" {
					ui.Text(c, talktext.ChatFollows(a.nodeTitle(kept))).FontSize(kit.FsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Shrink(1).
						Tooltip(talktext.ChatFixedTip())
				} else {
					label, tip := talktext.ChatAttachChapter(), talktext.ChatAttachChapterTip(a.Store.Work().Language.Unit())
					if a.OpenUnit() == "volume" {
						label, tip = talktext.ChatAttachVolume(), talktext.ChatAttachVolumeTip()
					}
					if ui.Checkbox(c, &a.Attach, label).Tooltip(tip).Changed() {
						a.savePrefs()
					}
				}
				ui.Spacer(c)
				chip()
			})
		})
		return
	}
	box := ui.Column(c).FillWidth().MaxWidth(column).Padding(12, 12, 10, 16).Gap(6).Radius(kit.RDialog).Background(a.palette().Raised).Border(1, t.Border)
	a.palette().Raise.On(box)
	if box.FocusWithin() {
		box.BorderColor(t.TextMuted.Alpha(0.55))
	}
	box.Children(func() {
		quotes()
		field(talktext.TalkInputHint(), 46, 2).FillWidth()
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			chip()
			ui.Spacer(c)
			ui.Text(c, sendKeys()).FontSize(kit.FsSmall).TextColor(a.palette().Faint)
			send()
		})
	})
}

// ProviderURL runs on the frame thread.
func (a *Panel) ProviderURL(id string) string {
	for _, p := range a.backend().Providers() {
		if p.ID == id {
			return p.BaseURL
		}
	}
	return ""
}

// keptTo is the chapter the talk shown keeps to, when it was opened to
// draft or rewrite it; its first words may be on their way still.
// KeptTo runs on the frame thread.
func (a *Panel) KeptTo() string {
	if a.Turn.Active && a.Turn.Session == a.Session {
		return a.Turn.Chapter
	}
	return originChapter(a.Events)
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

// OpenSource runs on the frame thread.
func (a *Panel) OpenSource(m workspace.Event, start, end int) {
	if !a.hasEntries() {
		return
	}
	a.Source = editor.New(editor.NewDoc(m.Text), editor.Options{ReadOnly: true, LineHeight: kit.LhRead,
		OwnMenu: func(x, y float32) { *a.desktop.MenuState() = kit.MenuState{Owner: "source-text", X: x, Y: y} }})
	if end > start {
		a.Source.MoveTo(start, false)
		a.Source.MoveTo(end, true)
	}
	a.SourceMessage = m.ID
	a.SourceCategory = a.Store.Work().Language.OtherCategory()
	a.SourceOpen = true
}

// Extraction runs on the frame thread.
func (a *Panel) Extraction(c *ui.Context) {
	if !a.hasEntries() {
		a.SourceOpen = false
		a.Source = nil
		return
	}
	t := a.palette().Theme
	a.palette().Modal(c, &a.SourceOpen, func() {
		ui.Column(c).Width(kit.DialogSource).Padding(kit.DialogPad).Gap(16).Children(func() {
			a.palette().DialogHead(c, talktext.ChatPull(), talktext.DialogSourceAbout())
			a.Source.View(c).Height(220).FillWidth().Border(1, t.Border).Radius(kit.RCard)
			start, end := a.Source.Selection()
			a.desktop.MenuState().MenuAt(c, a.palette(), "source-text", func(m *kit.Menu) {
				m.Item(locales.CommonCopy()).Hint(keyHint("C")).Disabled(start == end).Do(func() { a.Source.Copy() })
				m.Item(locales.CommonSelectAll()).Hint(keyHint("A")).Do(func() { a.Source.SelectAll() })
			})
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				a.palette().Select(c, &a.SourceCategory, a.CategoryNames()).Label(talktext.DialogSourceCat()).Width(140)
				if start == end {
					ui.Text(c, talktext.DialogNotPicked()).FontSize(kit.FsNote).TextColor(a.palette().Faint)
				} else {
					ui.Text(c, talktext.DialogPicked(a.Store.Work().Language.Count(a.Source.SelectedText()), a.Store.Work().Language.Unit())).FontSize(kit.FsNote).TextColor(a.palette().Ink)
				}
				ui.Spacer(c)
				if a.palette().Ghost(c, talktext.DialogSourceCollapse(), ui.Color{}).Clicked() {
					a.SourceOpen = false
				}
				if a.palette().Primary(c, locales.EditorAddToLore()).Disabled(a.held() || start == end).Clicked() {
					entry, err := Extract(a.Store, a.Session, a.SourceMessage, start, end, a.SourceCategory)
					if err == nil {
						a.SourceOpen = false
						a.notices().Did(locales.DialogSettingAdded())
						err = a.bookHost.CategoriesChanged()
						a.bookHost.Flash(entry.ID)
					}
					a.SetError(err)
				}
			})
		})
	})
	if !a.SourceOpen {
		a.Source = nil
	}
}

// CategoryNames runs on the frame thread.
func (a *Panel) CategoryNames() []string {
	var names []string
	for _, cat := range a.Store.Work().Categories {
		names = append(names, cat.Name)
	}
	return names
}

// modelMenu changes the discussion model and its effort without leaving the
// conversation. Models come from each provider's catalog; one outside it
// is set in the model settings.
// ModelMenu runs on the frame thread.
func (a *Panel) ModelMenu(m *kit.Menu, work workspace.Work) {
	var with []aiapi.Provider
	for _, p := range a.backend().Providers() {
		if len(p.Models) > 0 {
			with = append(with, p)
		}
	}
	for _, p := range with {
		// One service's models need no name over them.
		head := talktext.ChatModel()
		if len(with) > 1 {
			head = p.Name
		}
		m.Head(head)
		for _, id := range modelIDs(p) {
			on := work.Discussion.Provider == p.ID && work.Discussion.Model == id
			m.Item(id).On(on).Do(func() { a.SetTalk(func(s *workspace.Selection) { s.Provider, s.Model = p.ID, id }) })
		}
	}
	m.Item(talktext.ChatAiConfig()).Do(func() { a.desktop.OpenSettings("ai.settings") })
	m.Separator()
	m.Head(talktext.ChatEffort())
	for _, e := range a.backend().Efforts() {
		m.Item(a.backend().EffortLabel(e)).On(work.Discussion.Effort == e).Do(func() {
			a.SetTalk(func(s *workspace.Selection) { s.Effort = e })
		})
	}
}

// setTalk changes the work's discussion selection and saves it.
// SetTalk runs on the frame thread.
func (a *Panel) SetTalk(change func(*workspace.Selection)) {
	w := a.Store.Work()
	change(&w.Discussion)
	err := a.Store.SaveWork(w)
	if err == nil {
		a.selectionChanged(w.Discussion)
	}
	a.SetError(err)
}
