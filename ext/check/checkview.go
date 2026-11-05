package check

import (
	"github.com/egoist/mygo/ui"
	"slices"
	"strings"
	"unicode/utf8"
	"writer/desk/kit"
	"writer/ext/lore/loreapi"
	"writer/internal/textclip"
	"writer/workspace"

	"writer/ext/check/checktext"
	"writer/locales"
)

// whenRows are the three times to check, one chosen.
func (a *Panel) whenRows(c *ui.Context) {
	t := a.pal().Theme
	k := &a.State
	for _, o := range checkWhens {
		on := o.When == k.When
		ring := a.pal().Faint
		if on {
			ring = a.pal().Ink
		}
		b := ui.ButtonBase(c).Row().FillWidth().Gap(10).Padding(6, 8).Radius(kit.RControl).AlignItems(ui.Start).Justify(ui.Start).Label(o.label())
		if b.Hovered() {
			b.Background(t.SurfaceHover)
		}
		b.Children(func() {
			ui.Box(c).Size(14, 14).Margin(4, 0, 0, 0).Radius(kit.RRound).Border(1.5, ring).Center().Shrink(0).Children(func() {
				if on {
					kit.Dot(c, a.pal().Ink, 6)
				}
			})
			ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
				ui.Text(c, o.label()).FontSize(kit.FsBody)
				ui.Text(c, o.sub()).FontSize(kit.FsNote).TextColor(t.TextMuted)
			})
		})
		if b.Clicked() && !on {
			k.When = o.When
			a.editingHost.SavePreferences()
		}
	}
}

// checkKinds are the page's tabs.
var checkKinds = []struct {
	kind  string
	label func() string
}{
	{"", checktext.CheckKindAll}, {workspace.FindConflict, checktext.CheckKindMismatch}, {workspace.FindName, checktext.CheckKindName}, {workspace.FindNew, checktext.CheckKindNew},
}

func kindLabel(kind string) string {
	for _, k := range checkKinds {
		if k.kind == kind {
			return k.label()
		}
	}
	return ""
}

// checkPage is the page 检查: what the checks found on the left, the one
// chosen on the right, the prose beside the setting.
func (a *Panel) checkPage(c *ui.Context) {
	k := &a.State
	all := a.listedChecks()
	var shown []Item
	for _, it := range all {
		if kind := checkKinds[k.Tab].kind; kind == "" || kind == it.F.Kind {
			shown = append(shown, it)
		}
	}
	at := slices.IndexFunc(shown, func(it Item) bool {
		return it.F.ID == k.Shown || slices.ContainsFunc(it.Also, func(o Item) bool { return o.F.ID == k.Shown })
	})
	if at < 0 {
		at = min(0, len(shown)-1)
	}
	if at >= 0 {
		k.Shown = shown[at].F.ID
	}
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Width(360).Shrink(0).FillHeight().Children(func() { a.checkList(c, all, shown) })
		ui.Box(c).Width(1).FillHeight().Background(a.pal().Line)
		ui.Column(c).Grow(1).Shrink(1).FillHeight().Padding(32, 32, 20, 32).Gap(20).Children(func() {
			if at < 0 {
				a.checkEmpty(c, len(all))
				return
			}
			a.checkDetail(c, shown, at)
		})
	})
}

// checkList is the page's left: how far the checks got, the kinds, and
// the findings.
func (a *Panel) checkList(c *ui.Context, all, shown []Item) {
	t := a.pal().Theme
	k := &a.State
	ui.Column(c).FillWidth().Padding(14, 20, 10, 20).Gap(10).Shrink(0).Children(func() {
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			a.checkProgress(c)
		})
		names := make([]string, len(checkKinds))
		for i, kind := range checkKinds {
			n := 0
			for _, it := range all {
				if kind.kind == "" || kind.kind == it.F.Kind {
					n++
				}
			}
			names[i] = checktext.CheckKindCount(kind.label(), n)
		}
		ui.Row(c).Children(func() { a.pal().Segments(c, &k.Tab, checktext.CheckKindAria(), names) })
	})
	ui.Scroll(c.Key("checks")).FillWidth().Grow(1).Shrink(1).Children(func() {
		ui.Column(c).FillWidth().Padding(0, 14, 12, 14).Gap(2).Children(func() {
			for _, it := range shown {
				sub := it.Title
				switch it.F.Kind {
				case workspace.FindConflict:
					sub = checktext.CheckMismatchAbout(sub, entryTitle(it.Entry))
				case workspace.FindName:
					sub = checktext.CheckNameAbout(sub, it.F.Name)
				case workspace.FindNew:
					at := sub
					sub = checktext.CheckFirstSeen(it.F.Category, at)
					if len(it.Also) > 0 {
						sub = checktext.CheckFirstSeenMore(it.F.Category, at, len(it.Also)+1)
					}
				}
				b := ui.ButtonBase(c.Key("ck-"+it.F.ID)).Row().FillWidth().AlignItems(ui.Center).Gap(10).Padding(8, 10).Radius(kit.RControl).Label("检查-" + it.F.Title)
				if it.F.ID == k.Shown {
					b.Background(t.SurfacePressed)
				} else if b.Hovered() {
					b.Background(t.SurfaceHover)
				}
				kit.PressRow(c, b)
				b.Children(func() {
					ui.Column(c).Grow(1).Shrink(1).Basis(0).Children(func() {
						ui.Text(c, it.F.Title).FontSize(kit.FsBody).LineHeight(kit.LhBody).SingleLine().Ellipsis("…").FillWidth()
						ui.Text(c, sub).FontSize(kit.FsNote).LineHeight(kit.LhBody).TextColor(t.TextMuted).SingleLine().Ellipsis("…").FillWidth()
					})
					ui.Text(c, kindLabel(it.F.Kind)).FontSize(kit.FsNote).TextColor(t.TextMuted).SingleLine().Shrink(0)
				})
				if b.Clicked() {
					k.Shown = it.F.ID
				}
			}
		})
	})
}

// checkProgress says how far the checks got, and checks the book again.
func (a *Panel) checkProgress(c *ui.Context) {
	t := a.pal().Theme
	k := &a.State
	if k.Queue.Running != "" {
		left := len(k.Queue.Pending)
		_, title := a.checkAt(k.Queue.Running)
		line := checktext.CheckRunning(title)
		if left > 0 {
			line = checktext.CheckRunningMore(title, left)
		}
		ui.Text(c, line).FontSize(kit.FsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
		if a.pal().Ghost(c, checktext.CheckStop(), ui.Color{}).Clicked() {
			k.Queue.Stop()
			k.Runner.stop()
		}
		return
	}
	last, lastTitle, done, failed := 0, "", 0, 0
	for id := range a.book().Checks() {
		no, title := a.checkAt(id)
		if no > last {
			last, lastTitle = no, title
		}
		if _, bad := k.Queue.Failed[id]; no > 0 && !bad {
			done++
		}
	}
	for id := range k.Queue.Failed {
		if no, _ := a.checkAt(id); no > 0 {
			failed++
		}
	}
	switch {
	case failed > 0:
		// 已检查至 would read as all before it checked.
		ui.Text(c, checktext.CheckProgress(done, failed)).FontSize(kit.FsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Shrink(1)
	case last > 0:
		ui.Text(c, checktext.CheckCheckedUpTo(lastTitle)).FontSize(kit.FsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Shrink(1)
	}
	label := checktext.CheckRecheck()
	if last == 0 {
		label = checktext.CheckAllBook()
	}
	b := a.pal().Ghost(c, label, ui.Color{}).Label(label)
	if b.Clicked() {
		k.Due = a.checkDue()
		switch {
		case !a.hasSetting():
			a.status().Hint(checktext.CheckNoSettings())
		case len(k.Due) == 0:
			a.status().Hint(checktext.CheckAllDone())
		default:
			k.AskOpen = true
		}
	}
	kit.Popover(c, b, &k.AskOpen, 1, 0, func(panel ui.Element) {
		panel.AttachTo(b, ui.AnchorBottomRight, ui.AnchorTopRight).Margin(6, 0, 0, 0).Width(280).Radius(kit.RCard).Padding(14).Gap(12).
			Background(a.pal().Canvas).Border(1, t.Border)
		a.pal().Pop.On(panel)
		ui.Text(c, checktext.CheckOwedTitle(len(k.Due))).FontSize(kit.FsBody).FontWeight(600)
		ui.Text(c, checktext.CheckOwedNote(len(k.Due))).FontSize(kit.FsNote).TextColor(t.TextMuted)
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			ui.Spacer(c)
			if a.pal().Secondary(c, locales.CommonCancel()).Clicked() {
				k.AskOpen = false
			}
			if a.pal().Primary(c, checktext.CheckStart()).Clicked() {
				k.AskOpen = false
				for _, id := range k.Due {
					a.State.Queue.Enqueue(id)
				}
			}
		})
	})
}

// checkAt is which chapter of the book node is, as the checks number
// them, and its title; a chapter no longer in the book is 0, 已删除的章.
func (a *Panel) checkAt(node string) (no int, title string) {
	n := 0
	title = locales.AppChapterDeleted()
	a.book().Tree().Walk(func(nd *workspace.Node, _ int) {
		if !nd.Folder {
			n++
			if nd.ID == node {
				no, title = n, nd.Title
			}
		}
	})
	return no, title
}

// checkEmpty is the page's right with nothing to show.
func (a *Panel) checkEmpty(c *ui.Context, all int) {
	k := &a.State
	pic, words, how := kit.IcDone, checktext.CheckNoneFound(), checktext.CheckAutoNote()
	switch {
	case all > 0:
		words, how = checktext.CheckNoneInKind(), ""
	case !a.hasSetting():
		words, how = checktext.CheckNoSettingsTitle(), checktext.CheckNoSettingsAbout()
	case len(a.book().Checks()) == 0 && k.Queue.Running == "":
		words, how = checktext.CheckNotYet(), checktext.CheckNotYetAbout(checktext.CheckAllBook())
	case k.Queue.Running != "":
		words, how = checktext.CheckRunningTitle(), checktext.CheckRunningAbout()
	case k.When == checkOnDemand:
		how = checktext.CheckManualNote(checktext.CheckModeManual(), checktext.CheckRecheck())
	}
	// One that failed is not told as one that found nothing; the first in
	// the book is named, so it is the same one every frame.
	if lost := a.firstFailed(); lost != "" && k.Queue.Running == "" {
		_, title := a.checkAt(lost)
		pic, words, how = kit.IcAlert, checktext.CheckIncomplete(), checktext.CheckFailedAt(title, a.status().FormatError(k.Queue.Failed[lost]))
	}
	a.pal().Blank(c, pic, words, how, nil)
}

// firstFailed is the first chapter in the book whose check failed, or "".
func (a *Panel) firstFailed() string {
	first := ""
	a.book().Tree().Walk(func(nd *workspace.Node, _ int) {
		if _, ok := a.State.Queue.Failed[nd.ID]; ok && first == "" && !nd.Folder {
			first = nd.ID
		}
	})
	return first
}

// quoted is text in spans with its part, quote, as the design marks what
// is at odds; long text is cut around it.
func (a *Panel) quoted(text, quote string) []ui.Span {
	const around = 120
	i := strings.Index(text, quote)
	if quote == "" || i < 0 {
		return []ui.Span{{Text: text}}
	}
	before, after := text[:i], text[i+len(quote):]
	return []ui.Span{{Text: textclip.Tail(before, around)}, {Text: quote, Color: a.pal().Theme.Text, Background: a.pal().Theme.SurfacePressed}, {Text: textclip.Clip(after, around)}}
}

// checkCard is one side of the comparison: what it is, a link to it, and
// its words.
func (a *Panel) checkCard(c *ui.Context, head, link string, spans []ui.Span) (clicked bool) {
	t := a.pal().Theme
	ui.Column(c).Grow(1).Shrink(1).Basis(0).Gap(6).Children(func() {
		// As tall without the link as with it, so cards side by side line up.
		ui.Row(c).FillWidth().MinHeight(22).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, head).FontSize(kit.FsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
			if link != "" {
				clicked = a.pal().Ghost(c, link, ui.Color{}).Clicked()
			}
		})
		ui.RichText(c, spans...).Label("检查对照-"+head).FontSize(kit.FsRead).LineHeight(kit.LhRead).TextColor(t.TextMuted).FillWidth().
			Padding(12, 14).Radius(kit.RCard).Background(a.pal().Side).Border(1, a.pal().Line).Selectable().Grow(1)
	})
	return clicked
}

// overlap tells whether two parts of text are in one sentence of it:
// findings on one entry are alike where they hold to the same of it, not
// where one is of her fear of the dark and the other of her left hand. A
// part not given, or not found, may be any.
func overlap(text, x, y string) bool {
	i, j := strings.Index(text, x), strings.Index(text, y)
	if x == "" || y == "" || i < 0 || j < 0 {
		return true
	}
	s1, e1 := sentences(text, i, i+len(x))
	s2, e2 := sentences(text, j, j+len(y))
	return s1 < e2 && s2 < e1
}

// sentences widens text[from:to] to the sentences it is in.
func sentences(text string, from, to int) (int, int) {
	const ends = "。！？；\n"
	if k := strings.LastIndexAny(text[:from], ends); k >= 0 {
		_, n := utf8.DecodeRuneInString(text[k:])
		from = k + n
	} else {
		from = 0
	}
	if k := strings.IndexAny(text[to:], ends); k >= 0 {
		to += k
	} else {
		to = len(text)
	}
	return from, to
}

// paragraphOf is the paragraph of text that has quote, and which it is.
func paragraphOf(text, quote string) (string, int) {
	n := 0
	for _, p := range strings.Split(text, "\n") {
		if strings.TrimSpace(p) == "" {
			continue
		}
		n++
		if strings.Contains(p, quote) {
			return strings.TrimSpace(p), n
		}
	}
	return quote, 0
}

// checkDetail is the finding chosen: the prose beside the setting, what
// was found alike before, and what to do about it.
func (a *Panel) checkDetail(c *ui.Context, shown []Item, at int) {
	t := a.pal().Theme
	k := &a.State
	it := shown[at]
	if k.ProseOf != it.Node {
		k.Prose, k.ProseOf = "", it.Node
		if ch, err := a.store().ReadChapter(it.Node); err == nil {
			k.Prose = ch.Text
		}
	}
	next := func() { k.Shown = shown[(at+1)%len(shown)].F.ID }
	toProse := func() { a.search.FindInChapter(it.Node, it.F.Quote) }
	toEntry := func() {
		if !a.entryNavigationAvailable() {
			return
		}
		a.search.FindInChapter(it.Node, it.F.Quote)
		a.entryNavigation(func(n loreapi.EntryNavigation) { n.Reveal(it.Entry.ID) })
		a.editingHost.SavePreferences()
	}
	ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Text(c, it.F.Title).FontSize(kit.FsDialog).FontWeight(700).Grow(1).Shrink(1)
		ui.Textf(c, "%d / %d", at+1, len(shown)).FontSize(kit.FsNote).TextColor(t.TextMuted).SingleLine().Shrink(0)
		if a.pal().Ghost(c, checktext.CheckNext(), ui.Color{}).Disabled(len(shown) < 2).Clicked() {
			next()
		}
	})
	ui.Scroll(c.Key("check-" + it.F.ID)).FillWidth().Grow(1).Shrink(1).Children(func() {
		ui.Column(c).FillWidth().Gap(16).Children(func() {
			para, no := paragraphOf(k.Prose, it.F.Quote)
			head := checktext.CheckTextChapter(it.Title)
			if no > 0 {
				head = checktext.CheckTextPara(it.Title, no)
			}
			ui.Row(c).FillWidth().Gap(16).AlignItems(ui.Stretch).Children(func() {
				if a.checkCard(c, head, locales.CheckSeeText(), a.quoted(para, it.F.Quote)) {
					toProse()
				}
				switch it.F.Kind {
				case workspace.FindConflict:
					if a.checkCard(c, checktext.CheckLoreHeading(entryTitle(it.Entry)), a.entryLinkLabel(), a.quoted(it.Entry.Text, it.F.Says)) {
						toEntry()
					}
				case workspace.FindName:
					if a.checkCard(c, checktext.CheckLoreHeading(entryTitle(it.Entry)), a.entryLinkLabel(), a.quoted(it.Entry.Text, it.F.Name)) {
						toEntry()
					}
				case workspace.FindNew:
					text := it.F.Name
					if it.F.Text != "" {
						text += "：" + it.F.Text
					}
					a.checkCard(c, checktext.CheckCanAdd(it.F.Category), "", a.quoted(text, it.F.Name))
				}
			})
			head, alike := checktext.CheckAlsoNew(), slices.Clone(it.Also)
			if it.F.Kind == workspace.FindConflict {
				head = checktext.CheckAlsoSimilar()
				for _, o := range a.checkItems(true) {
					if o.F.Kind == workspace.FindConflict && o.F.Entry == it.F.Entry && o.F.ID != it.F.ID && overlap(it.Entry.Text, it.F.Says, o.F.Says) {
						alike = append(alike, o)
					}
				}
			}
			if len(alike) == 0 {
				return
			}
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				ui.Text(c, head).FontSize(kit.FsNote).TextColor(t.TextMuted)
				ui.Column(c).FillWidth().Gap(8).Children(func() {
					for _, o := range alike {
						ui.Row(c).FillWidth().Gap(12).Padding(8, 12).Radius(kit.RControl).Background(t.SurfaceHover).AlignItems(ui.Center).Children(func() {
							ui.Text(c, o.Title).FontSize(kit.FsControl).TextColor(t.TextMuted).SingleLine().Shrink(0)
							ui.Text(c, o.F.Quote).FontSize(kit.FsControl).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
							if o.F.Kind == workspace.FindConflict {
								said := checktext.CheckAlsoOpen()
								if o.F.Meant {
									said = checktext.CheckMarkedNoChange()
								}
								ui.Text(c, said).FontSize(kit.FsNote).TextColor(t.TextMuted).SingleLine().Shrink(0)
							}
						})
					}
				})
			})
		})
	})
	a.pal().Hairline(c)
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
		if a.pal().Ghost(c, checktext.CheckLater(), ui.Color{}).Disabled(len(shown) < 2).Clicked() {
			next()
		}
		ui.Spacer(c)
		acts := a.checkActions(it)
		for i, act := range acts {
			var button ui.Element
			if i == len(acts)-1 {
				button = a.pal().Primary(c, act.label)
			} else {
				button = a.pal().Secondary(c, act.label)
			}
			if button.Clicked() {
				if i == 0 && len(shown) > 1 {
					next()
				}
				act.run()
			}
		}
	})
}

// takeFound adds what a check found new to the setting, as the author's.
func (a *Panel) takeFound(it Item) {
	text := it.F.Text
	if text == "" {
		text = it.F.Name
	}
	r, err := a.revisions.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{
		Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: it.F.Category, Text: text, Names: []string{it.F.Name},
		Sources: []workspace.Source{{Chapter: it.Node, Quote: it.F.Quote}},
	}}})
	if err == nil {
		err = a.revisions.Refresh()
	}
	if err != nil {
		a.warn(err)
		return
	}
	a.editingHost.Flash(r.Ops[0].Entry)
	a.status().Did(checktext.CheckAddedSetting())
}

func (a *Panel) entryLinkLabel() string {
	if !a.entryNavigationAvailable() {
		return ""
	}
	return checktext.CheckSeeSetting()
}
