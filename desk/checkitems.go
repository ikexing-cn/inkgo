package desk

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"slices"
	"strings"
	"writer/desk/book"
	"writer/editor"
	"writer/workspace"

	"writer/locales"
)

// checkKey is everything checkItems reads: the book, and the chapter open
// with its editor's text, when that text counts as live, and the language
// revision, since the margin notes it derives say words.
type checkKey struct {
	locale  uint64
	book    *book.Book
	version int
	ed      *editor.Editor
	doc     int
	node    string
	live    bool
}

func (a *App) checkKey() checkKey {
	w := &a.write
	doc := 0
	if w.cur().ed != nil {
		doc = w.cur().ed.Doc.Version()
	}
	return checkKey{a.localeRevision.Load(), a.book, a.book.Version(), w.cur().ed, doc, w.node, a.liveChapter()}
}

// doubtKey is the check items the marks are drawn from, and whether the AI
// is on.
type doubtKey struct {
	check checkKey
	ai    bool
}

// liveChapter says the open chapter is the one shown on the writing page.
func (a *App) liveChapter() bool {
	w := &a.write
	return w.cur().ed != nil && w.node != "" && a.page == pageWrite
}

// liveText is the open chapter's text as it is being written.
func (a *App) liveText() (string, bool) {
	w := &a.write
	k := &a.ck
	if !a.liveChapter() {
		return "", false
	}
	if k.liveOf != w.node || k.liveVer != w.cur().ed.Doc.Version() || k.live == "" {
		k.live, k.liveOf, k.liveVer = w.cur().ed.Doc.String(), w.node, w.cur().ed.Doc.Version()
	}
	return k.live, true
}

// checkItems is what the checks found and still holds, in the book's
// order: with meant, also what the author said is as meant. A new name the
// author left out of the setting once is left out in every chapter.
func (a *App) checkItems(meant bool) []checkItem {
	c := &a.ck.items
	if meant {
		c = &a.ck.itemsMeant
	}
	return c.get(a.checkKey(), func() []checkItem { return a.buildCheckItems(meant) })
}

func (a *App) buildCheckItems(meant bool) []checkItem {
	tree := a.book.Tree()
	live, open := a.liveText()
	left := map[string]bool{}
	for _, had := range a.book.Checks() {
		for _, f := range had.Findings {
			if f.Kind == workspace.FindNew && f.Meant {
				left[f.Name] = true
			}
		}
	}
	var out []checkItem
	o := tree.Order()
	tree.Walk(func(nd *workspace.Node, _ int) {
		if nd.Folder {
			return
		}
		here := open && nd.ID == a.write.node
		for _, f := range a.book.Checks()[nd.ID].Findings {
			if f.Meant && !meant || here && !strings.Contains(live, f.Quote) {
				continue
			}
			it := checkItem{node: nd.ID, title: nd.Title, here: here, f: f}
			switch f.Kind {
			case workspace.FindConflict:
				e, ok := (workspace.Setting{Entries: a.entries()}).Entry(f.Entry)
				e.Text, _, _ = e.Reading(o, nd.ID) // the chapter was checked against the entry as it reads there
				if !ok || f.Says != "" && !strings.Contains(e.Text, f.Says) {
					continue // the setting was changed
				}
				it.entry = e
			case workspace.FindName:
				for _, e := range a.entries() {
					if slices.Contains(e.Names, f.Name) {
						it.entry = e
					}
				}
				if it.entry.ID == "" {
					continue
				}
			case workspace.FindNew:
				if slices.ContainsFunc(a.entries(), func(e workspace.Entry) bool {
					return e.Place != workspace.PlaceDropped && slices.Contains(e.Names, f.Name)
				}) || left[f.Name] && !meant {
					continue // taken into the setting, or left out of it
				}
			}
			out = append(out, it)
		}
	})
	return out
}

// listedChecks is what the checks found as the author goes through it: a
// new name found in several chapters is one, where it first comes, since
// it is taken into the setting or left out once.
func (a *App) listedChecks() []checkItem {
	return a.ck.listed.get(a.checkKey(), func() []checkItem {
		var out []checkItem
		first := map[string]int{}
		for _, it := range a.checkItems(false) {
			if it.f.Kind == workspace.FindNew {
				if i, ok := first[it.f.Name]; ok {
					out[i].also = append(out[i].also, it)
					continue
				}
				first[it.f.Name] = len(out)
			}
			out = append(out, it)
		}
		return out
	})
}

// setMeant records that the author means what it found.
func (a *App) setMeant(it checkItem) {
	before := a.book.Checks()[it.node]
	had := before
	had.Findings = slices.Clone(before.Findings)
	for i := range had.Findings {
		if had.Findings[i].ID == it.f.ID {
			had.Findings[i].Meant = true
		}
	}
	if err := a.book.SaveCheckOver(it.node, had, before); err != nil {
		a.warn(err)
		return
	}
	st := a.Store
	what := locales.CheckMarkedNoChange()
	if it.f.Kind == workspace.FindNew {
		what = locales.CheckIgnored()
	}
	a.status.Did(what, func() error {
		if a.Store != st {
			return fmt.Errorf("%s", locales.StatusWorkSwitchedUndo())
		}
		return a.book.SaveCheckOver(it.node, before, had)
	})
}

// checkDue lists the chapters a check of the book would read: those long
// enough, never checked or changed since.
func (a *App) checkDue() []string {
	texts, err := a.Store.Texts() // the book in one pass
	if err != nil {
		a.warn(err)
		return nil
	}
	return workspace.DueChapters(a.book.Checks(), a.book.Tree(), texts, a.ai.MinCheck())
}

// openCheck shows the page of the checks, at finding id when given.
func (a *App) openCheck(id string) {
	a.write.pendOpen = false
	a.page = pageCheck
	if id != "" {
		a.ck.shown, a.ck.tab = id, 0
	}
}

// checkPending is the findings as the pending list has them.
func (a *App) checkPending() (pendGroup, bool) {
	g := pendGroup{id: "check", name: locales.PageCheck(), color: a.pal.Amber}
	if !a.checksOn() {
		return g, false
	}
	for _, it := range a.listedChecks() {
		finding := it.f.ID
		p := pendItem{key: "检查/" + it.node + "/" + it.f.Kind + "/" + it.f.Quote, text: it.f.Title, sub: it.title, here: it.here}
		for _, o := range it.also {
			if o.here && !p.here { // a name found here too is of this chapter
				p.sub, p.here, finding = o.title, true, o.f.ID
			}
		}
		p.open = func(_ ui.Services) { a.openCheck(finding) }
		if it.f.Kind == workspace.FindNew {
			p.text = locales.StatusNotInLore(it.f.Name)
		}
		if it.here {
			p.sub += locales.StatusHereTag()
		}
		if !a.write.ignored[p.key] {
			g.items = append(g.items, p)
		}
	}
	return g, len(g.items) > 0
}

// markDoubts puts the light dashed line under what the checks found in the
// chapter shown, while the AI is on.
func (a *App) markDoubts() {
	w := &a.write
	if w.cur().ed == nil {
		return
	}
	w.cur().ed.SetDoubtColor(a.pal.Amber)
	key := doubtKey{a.checkKey(), a.checksOn()}
	w.doubts.get(key, func() [][2]int {
		text, ok := a.liveText()
		if !ok {
			w.cur().ed.SetDoubts(nil)
			return nil
		}
		items := slices.DeleteFunc(slices.Clone(a.checkItems(false)), func(it checkItem) bool { return !it.here || !a.checksOn() })
		var ranges [][2]int
		for _, it := range items {
			if at := strings.Index(text, it.f.Quote); at >= 0 {
				s := len([]rune(text[:at]))
				ranges = append(ranges, [2]int{s, s + len([]rune(it.f.Quote))})
			}
		}
		w.cur().ed.SetDoubts(ranges)
		return ranges
	})
}

// checkState says in a line where the check of the chapter shown stands,
// whether there is something of it to look at, and what more there is to
// say of the line, if anything.
func (a *App) checkState() (line string, found int, can bool, tip string) {
	w := &a.write
	k := &a.ck
	for _, it := range a.checkItems(false) {
		if it.here {
			found++
		}
	}

	text, _ := a.liveText()
	had, ok := a.book.Checks()[w.node]
	switch {
	case k.queue.Running == w.node:
		return locales.StatusChecking(), 0, false, ""
	case slices.Contains(k.queue.Pending, w.node):
		return locales.StatusQueued(), 0, false, ""
	case k.queue.Failed[w.node] != nil:
		line := locales.StatusFailed(a.errText(k.queue.Failed[w.node]))
		return line, 0, true, line // what went wrong is long: in full on hover
	case !a.hasSetting():
		return locales.CheckNoSettings(), 0, false, ""
	case workspace.Count(text) < a.ai.MinCheck():
		return locales.CheckMinimum(a.ai.MinCheck()), 0, false, ""
	case !ok:
		return locales.StatusNever(), 0, true, ""
	case found > 0 && had.Stale(text):
		return locales.StatusDirtyIssues(found), found, true, ""
	case found > 0 && had.Read > 0:
		return locales.StatusPartIssues(had.Read, found), found, false, ""
	case found > 0:
		return locales.StatusIssues(found), found, false, ""
	case had.Stale(text):
		return locales.StatusEdited(), 0, true, ""
	case had.Read > 0:
		// The rest was not looked at: not to be told as nothing found.
		return locales.StatusPartOK(had.Read), 0, false, locales.StatusPartLong()
	}
	return locales.StatusChecked(), 0, false, ""
}
