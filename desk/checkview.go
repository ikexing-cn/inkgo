package desk

import (
	"github.com/egoist/mygo/ui"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
	"writer/desk/kit"
	"writer/ext/lore/loreapi"
	"writer/internal/textclip"
	"writer/workspace"

	"writer/locales"
)

// whenRows are the three times to check, one chosen.
func (a *App) whenRows(c *ui.Context) {
	t := a.pal.Theme
	k := &a.ck
	for _, o := range checkWhens {
		on := o.when == k.when
		ring := a.pal.Faint
		if on {
			ring = a.pal.Ink
		}
		b := ui.ButtonBase(c).Row().FillWidth().Gap(10).Padding(6, 8).Radius(kit.RControl).AlignItems(ui.Start).Justify(ui.Start).Label(o.label())
		if b.Hovered() {
			b.Background(t.SurfaceHover)
		}
		b.Children(func() {
			ui.Box(c).Size(14, 14).Margin(4, 0, 0, 0).Radius(kit.RRound).Border(1.5, ring).Center().Shrink(0).Children(func() {
				if on {
					kit.Dot(c, a.pal.Ink, 6)
				}
			})
			ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
				ui.Text(c, o.label()).FontSize(kit.FsBody)
				ui.Text(c, o.sub()).FontSize(kit.FsNote).TextColor(t.TextMuted)
			})
		})
		if b.Clicked() && !on {
			k.when = o.when
			a.savePrefs()
		}
	}
}

// checkKinds are the page's tabs.
var checkKinds = []struct {
	kind  string
	label func() string
}{
	{"", locales.CheckKindAll}, {workspace.FindConflict, locales.CheckKindMismatch}, {workspace.FindName, locales.CheckKindName}, {workspace.FindNew, locales.CheckKindNew},
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
func (a *App) checkPage(c *ui.Context) {
	k := &a.ck
	all := a.listedChecks()
	var shown []checkItem
	for _, it := range all {
		if kind := checkKinds[k.tab].kind; kind == "" || kind == it.f.Kind {
			shown = append(shown, it)
		}
	}
	at := slices.IndexFunc(shown, func(it checkItem) bool {
		return it.f.ID == k.shown || slices.ContainsFunc(it.also, func(o checkItem) bool { return o.f.ID == k.shown })
	})
	if at < 0 {
		at = min(0, len(shown)-1)
	}
	if at >= 0 {
		k.shown = shown[at].f.ID
	}
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Width(360).Shrink(0).FillHeight().Children(func() { a.checkList(c, all, shown) })
		ui.Box(c).Width(1).FillHeight().Background(a.pal.Line)
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
func (a *App) checkList(c *ui.Context, all, shown []checkItem) {
	t := a.pal.Theme
	k := &a.ck
	ui.Column(c).FillWidth().Padding(14, 20, 10, 20).Gap(10).Shrink(0).Children(func() {
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			a.checkProgress(c)
		})
		names := make([]string, len(checkKinds))
		for i, kind := range checkKinds {
			n := 0
			for _, it := range all {
				if kind.kind == "" || kind.kind == it.f.Kind {
					n++
				}
			}
			names[i] = locales.CheckKindCount(kind.label(), n)
		}
		ui.Row(c).Children(func() { a.pal.Segments(c, &k.tab, locales.CheckKindAria(), names) })
	})
	ui.Scroll(c.Key("checks")).FillWidth().Grow(1).Shrink(1).Children(func() {
		ui.Column(c).FillWidth().Padding(0, 14, 12, 14).Gap(2).Children(func() {
			for _, it := range shown {
				sub := it.title
				switch it.f.Kind {
				case workspace.FindConflict:
					sub = locales.CheckMismatchAbout(sub, entryTitle(it.entry))
				case workspace.FindName:
					sub = locales.CheckNameAbout(sub, it.f.Name)
				case workspace.FindNew:
					at := sub
					sub = locales.CheckFirstSeen(it.f.Category, at)
					if len(it.also) > 0 {
						sub = locales.CheckFirstSeenMore(it.f.Category, at, len(it.also)+1)
					}
				}
				b := ui.ButtonBase(c.Key("ck-"+it.f.ID)).Row().FillWidth().AlignItems(ui.Center).Gap(10).Padding(8, 10).Radius(kit.RControl).Label("检查-" + it.f.Title)
				if it.f.ID == k.shown {
					b.Background(t.SurfacePressed)
				} else if b.Hovered() {
					b.Background(t.SurfaceHover)
				}
				kit.PressRow(c, b)
				b.Children(func() {
					ui.Column(c).Grow(1).Shrink(1).Basis(0).Children(func() {
						ui.Text(c, it.f.Title).FontSize(kit.FsBody).LineHeight(kit.LhBody).SingleLine().Ellipsis("…").FillWidth()
						ui.Text(c, sub).FontSize(kit.FsNote).LineHeight(kit.LhBody).TextColor(t.TextMuted).SingleLine().Ellipsis("…").FillWidth()
					})
					ui.Text(c, kindLabel(it.f.Kind)).FontSize(kit.FsNote).TextColor(t.TextMuted).SingleLine().Shrink(0)
				})
				if b.Clicked() {
					k.shown = it.f.ID
				}
			}
		})
	})
}

// checkProgress says how far the checks got, and checks the book again.
func (a *App) checkProgress(c *ui.Context) {
	t := a.pal.Theme
	k := &a.ck
	if k.queue.Running != "" {
		left := len(k.queue.Pending)
		_, title := a.checkAt(k.queue.Running)
		line := locales.CheckRunning(title)
		if left > 0 {
			line = locales.CheckRunningMore(title, left)
		}
		ui.Text(c, line).FontSize(kit.FsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
		if a.pal.Ghost(c, locales.CheckStop(), ui.Color{}).Clicked() {
			k.queue.Stop()
			k.runner.stop()
		}
		return
	}
	last, lastTitle, done, failed := 0, "", 0, 0
	for id := range a.book.Checks() {
		no, title := a.checkAt(id)
		if no > last {
			last, lastTitle = no, title
		}
		if _, bad := k.queue.Failed[id]; no > 0 && !bad {
			done++
		}
	}
	for id := range k.queue.Failed {
		if no, _ := a.checkAt(id); no > 0 {
			failed++
		}
	}
	switch {
	case failed > 0:
		// 已检查至 would read as all before it checked.
		ui.Text(c, locales.CheckProgress(done, failed)).FontSize(kit.FsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Shrink(1)
	case last > 0:
		ui.Text(c, locales.CheckCheckedUpTo(lastTitle)).FontSize(kit.FsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Shrink(1)
	}
	label := locales.CheckRecheck()
	if last == 0 {
		label = locales.CheckAllBook()
	}
	b := a.pal.Ghost(c, label, ui.Color{}).Label(label)
	if b.Clicked() {
		k.due = a.checkDue()
		switch {
		case !a.hasSetting():
			a.status.Hint(locales.CheckNoSettings())
		case len(k.due) == 0:
			a.status.Hint(locales.CheckAllDone())
		default:
			k.askOpen = true
		}
	}
	kit.Popover(c, b, &k.askOpen, 1, 0, func(panel ui.Element) {
		panel.AttachTo(b, ui.AnchorBottomRight, ui.AnchorTopRight).Margin(6, 0, 0, 0).Width(280).Radius(kit.RCard).Padding(14).Gap(12).
			Background(a.pal.Canvas).Border(1, t.Border)
		a.pal.Pop.On(panel)
		ui.Text(c, locales.CheckOwedTitle(len(k.due))).FontSize(kit.FsBody).FontWeight(600)
		ui.Text(c, locales.CheckOwedNote(len(k.due))).FontSize(kit.FsNote).TextColor(t.TextMuted)
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			ui.Spacer(c)
			if a.pal.Secondary(c, locales.CommonCancel()).Clicked() {
				k.askOpen = false
			}
			if a.pal.Primary(c, locales.CheckStart()).Clicked() {
				k.askOpen = false
				for _, id := range k.due {
					a.ck.queue.Enqueue(id)
				}
			}
		})
	})
}

// checkAt is which chapter of the book node is, as the checks number
// them, and its title; a chapter no longer in the book is 0, 已删除的章.
func (a *App) checkAt(node string) (no int, title string) {
	n := 0
	title = locales.AppChapterDeleted()
	a.book.Tree().Walk(func(nd *workspace.Node, _ int) {
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
func (a *App) checkEmpty(c *ui.Context, all int) {
	k := &a.ck
	pic, words, how := kit.IcDone, locales.CheckNoneFound(), locales.CheckAutoNote()
	switch {
	case all > 0:
		words, how = locales.CheckNoneInKind(), ""
	case !a.hasSetting():
		words, how = locales.CheckNoSettingsTitle(), locales.CheckNoSettingsAbout()
	case len(a.book.Checks()) == 0 && k.queue.Running == "":
		words, how = locales.CheckNotYet(), locales.CheckNotYetAbout(locales.CheckAllBook())
	case k.queue.Running != "":
		words, how = locales.CheckRunningTitle(), locales.CheckRunningAbout()
	case k.when == checkOnDemand:
		how = locales.CheckManualNote(locales.CheckModeManual(), locales.CheckRecheck())
	}
	// One that failed is not told as one that found nothing; the first in
	// the book is named, so it is the same one every frame.
	if lost := a.firstFailed(); lost != "" && k.queue.Running == "" {
		_, title := a.checkAt(lost)
		pic, words, how = kit.IcAlert, locales.CheckIncomplete(), locales.CheckFailedAt(title, a.errText(k.queue.Failed[lost]))
	}
	a.pal.Blank(c, pic, words, how, nil)
}

// firstFailed is the first chapter in the book whose check failed, or "".
func (a *App) firstFailed() string {
	first := ""
	a.book.Tree().Walk(func(nd *workspace.Node, _ int) {
		if _, ok := a.ck.queue.Failed[nd.ID]; ok && first == "" && !nd.Folder {
			first = nd.ID
		}
	})
	return first
}

// quoted is text in spans with its part, quote, as the design marks what
// is at odds; long text is cut around it.
func (a *App) quoted(text, quote string) []ui.Span {
	const around = 120
	i := strings.Index(text, quote)
	if quote == "" || i < 0 {
		return []ui.Span{{Text: text}}
	}
	before, after := text[:i], text[i+len(quote):]
	return []ui.Span{{Text: textclip.Tail(before, around)}, {Text: quote, Color: a.pal.Theme.Text, Background: a.pal.Theme.SurfacePressed}, {Text: textclip.Clip(after, around)}}
}

// checkCard is one side of the comparison: what it is, a link to it, and
// its words.
func (a *App) checkCard(c *ui.Context, head, link string, spans []ui.Span) (clicked bool) {
	t := a.pal.Theme
	ui.Column(c).Grow(1).Shrink(1).Basis(0).Gap(6).Children(func() {
		// As tall without the link as with it, so cards side by side line up.
		ui.Row(c).FillWidth().MinHeight(22).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, head).FontSize(kit.FsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
			if link != "" {
				clicked = a.pal.Ghost(c, link, ui.Color{}).Clicked()
			}
		})
		ui.RichText(c, spans...).Label("检查对照-"+head).FontSize(kit.FsRead).LineHeight(kit.LhRead).TextColor(t.TextMuted).FillWidth().
			Padding(12, 14).Radius(kit.RCard).Background(a.pal.Side).Border(1, a.pal.Line).Selectable().Grow(1)
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
func (a *App) checkDetail(c *ui.Context, shown []checkItem, at int) {
	t := a.pal.Theme
	k := &a.ck
	it := shown[at]
	if k.proseOf != it.node {
		k.prose, k.proseOf = "", it.node
		if ch, err := a.Store.ReadChapter(it.node); err == nil {
			k.prose = ch.Text
		}
	}
	next := func() { k.shown = shown[(at+1)%len(shown)].f.ID }
	toProse := func() { a.findInChapter(it.node, it.f.Quote) }
	toEntry := func() {
		if !a.entryNavigationAvailable() {
			return
		}
		a.findInChapter(it.node, it.f.Quote)
		a.entryNavigation(func(n loreapi.EntryNavigation) { n.Reveal(it.entry.ID) })
		a.savePrefs()
	}
	ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Text(c, it.f.Title).FontSize(kit.FsDialog).FontWeight(700).Grow(1).Shrink(1)
		ui.Textf(c, "%d / %d", at+1, len(shown)).FontSize(kit.FsNote).TextColor(t.TextMuted).SingleLine().Shrink(0)
		if a.pal.Ghost(c, locales.CheckNext(), ui.Color{}).Disabled(len(shown) < 2).Clicked() {
			next()
		}
	})
	ui.Scroll(c.Key("check-" + it.f.ID)).FillWidth().Grow(1).Shrink(1).Children(func() {
		ui.Column(c).FillWidth().Gap(16).Children(func() {
			para, no := paragraphOf(k.prose, it.f.Quote)
			head := locales.CheckTextChapter(it.title)
			if no > 0 {
				head = locales.CheckTextPara(it.title, no)
			}
			ui.Row(c).FillWidth().Gap(16).AlignItems(ui.Stretch).Children(func() {
				if a.checkCard(c, head, locales.CheckSeeText(), a.quoted(para, it.f.Quote)) {
					toProse()
				}
				switch it.f.Kind {
				case workspace.FindConflict:
					if a.checkCard(c, locales.CheckLoreHeading(entryTitle(it.entry)), a.entryLinkLabel(), a.quoted(it.entry.Text, it.f.Says)) {
						toEntry()
					}
				case workspace.FindName:
					if a.checkCard(c, locales.CheckLoreHeading(entryTitle(it.entry)), a.entryLinkLabel(), a.quoted(it.entry.Text, it.f.Name)) {
						toEntry()
					}
				case workspace.FindNew:
					text := it.f.Name
					if it.f.Text != "" {
						text += "：" + it.f.Text
					}
					a.checkCard(c, locales.CheckCanAdd(it.f.Category), "", a.quoted(text, it.f.Name))
				}
			})
			head, alike := locales.CheckAlsoNew(), slices.Clone(it.also)
			if it.f.Kind == workspace.FindConflict {
				head = locales.CheckAlsoSimilar()
				for _, o := range a.checkItems(true) {
					if o.f.Kind == workspace.FindConflict && o.f.Entry == it.f.Entry && o.f.ID != it.f.ID && overlap(it.entry.Text, it.f.Says, o.f.Says) {
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
							ui.Text(c, o.title).FontSize(kit.FsControl).TextColor(t.TextMuted).SingleLine().Shrink(0)
							ui.Text(c, o.f.Quote).FontSize(kit.FsControl).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
							if o.f.Kind == workspace.FindConflict {
								said := locales.CheckAlsoOpen()
								if o.f.Meant {
									said = locales.CheckMarkedNoChange()
								}
								ui.Text(c, said).FontSize(kit.FsNote).TextColor(t.TextMuted).SingleLine().Shrink(0)
							}
						})
					}
				})
			})
		})
	})
	a.pal.Hairline(c)
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
		if a.pal.Ghost(c, locales.CheckLater(), ui.Color{}).Disabled(len(shown) < 2).Clicked() {
			next()
		}
		ui.Spacer(c)
		acts := a.checkActions(it)
		for i, act := range acts {
			var button ui.Element
			if i == len(acts)-1 {
				button = a.pal.Primary(c, act.label)
			} else {
				button = a.pal.Secondary(c, act.label)
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
func (a *App) takeFound(it checkItem) {
	text := it.f.Text
	if text == "" {
		text = it.f.Name
	}
	r, err := a.book.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{
		Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: it.f.Category, Text: text, Names: []string{it.f.Name},
		Sources: []workspace.Source{{Chapter: it.node, Quote: it.f.Quote}},
	}}})
	if err == nil {
		err = a.refreshFromSetting()
	}
	if err != nil {
		a.warn(err)
		return
	}
	a.flash[r.Ops[0].Entry] = time.Now()
	a.status.Did(locales.CheckAddedSetting())
}

func (a *App) entryLinkLabel() string {
	if !a.entryNavigationAvailable() {
		return ""
	}
	return locales.CheckSeeSetting()
}
