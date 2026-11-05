package check

import (
	"fmt"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
	"slices"
	"strings"
	"writer/desk/book"
	"writer/editor"
	"writer/ext/hostapi"
	"writer/workspace"

	"writer/ext/check/checktext"
	"writer/locales"
)

// checkKey is everything checkItems reads: the book, and the chapter open
// with its editor's text, when that text counts as live, and the language
// revision, since the margin notes it derives say words.
type checkKey struct {
	locale  string
	book    *book.Book
	version int
	ed      *editor.Editor
	doc     int
	Node    string
	Live    bool
}

func (a *Panel) checkKey() checkKey {
	w := a.writing()
	doc := 0
	if w.cur().ed != nil {
		doc = w.cur().ed.Doc.Version()
	}
	return checkKey{i18n.Locale(), a.book(), a.book().Version(), w.cur().ed, doc, w.Node, a.liveChapter()}
}

// liveChapter says the open chapter is the one shown on the writing page.
func (a *Panel) liveChapter() bool {
	w := a.writing()
	return w.cur().ed != nil && w.Node != "" && a.desktop.Writing()
}

// liveText is the open chapter's text as it is being written.
func (a *Panel) liveText() (string, bool) {
	w := a.writing()
	k := &a.State
	if !a.liveChapter() {
		return "", false
	}
	if k.LiveOf != w.Node || k.LiveVer != w.cur().ed.Doc.Version() || k.Live == "" {
		k.Live, k.LiveOf, k.LiveVer = w.cur().ed.Doc.String(), w.Node, w.cur().ed.Doc.Version()
	}
	return k.Live, true
}

// checkItems is what the checks found and still holds, in the book's
// order: with meant, also what the author said is as meant. A new name the
// author left out of the setting once is left out in every chapter.
func (a *Panel) checkItems(meant bool) []Item {
	c := &a.State.Items
	if meant {
		c = &a.State.ItemsMeant
	}
	return c.get(a.checkKey(), func() []Item { return a.buildCheckItems(meant) })
}

func (a *Panel) buildCheckItems(meant bool) []Item {
	tree := a.book().Tree()
	live, open := a.liveText()
	left := map[string]bool{}
	for _, had := range a.book().Checks() {
		for _, F := range had.Findings {
			if F.Kind == workspace.FindNew && F.Meant {
				left[F.Name] = true
			}
		}
	}
	var out []Item
	o := tree.Order()
	tree.Walk(func(nd *workspace.Node, _ int) {
		if nd.Folder {
			return
		}
		here := open && nd.ID == a.node()
		for _, F := range a.book().Checks()[nd.ID].Findings {
			if F.Meant && !meant || here && !strings.Contains(live, F.Quote) {
				continue
			}
			it := Item{Node: nd.ID, Title: nd.Title, Here: here, F: F}
			switch F.Kind {
			case workspace.FindConflict:
				e, ok := (workspace.Setting{Entries: a.entries()}).Entry(F.Entry)
				e.Text, _, _ = e.Reading(o, nd.ID) // the chapter was checked against the entry as it reads there
				if !ok || F.Says != "" && !strings.Contains(e.Text, F.Says) {
					continue // the setting was changed
				}
				it.Entry = e
			case workspace.FindName:
				for _, e := range a.entries() {
					if slices.Contains(e.Names, F.Name) {
						it.Entry = e
					}
				}
				if it.Entry.ID == "" {
					continue
				}
			case workspace.FindNew:
				if slices.ContainsFunc(a.entries(), func(e workspace.Entry) bool {
					return e.Place != workspace.PlaceDropped && slices.Contains(e.Names, F.Name)
				}) || left[F.Name] && !meant {
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
func (a *Panel) listedChecks() []Item {
	return a.State.Listed.get(a.checkKey(), func() []Item {
		var out []Item
		first := map[string]int{}
		for _, it := range a.checkItems(false) {
			if it.F.Kind == workspace.FindNew {
				if i, ok := first[it.F.Name]; ok {
					out[i].Also = append(out[i].Also, it)
					continue
				}
				first[it.F.Name] = len(out)
			}
			out = append(out, it)
		}
		return out
	})
}

// setMeant records that the author means what it found.
func (a *Panel) setMeant(it Item) {
	before := a.book().Checks()[it.Node]
	had := before
	had.Findings = slices.Clone(before.Findings)
	for i := range had.Findings {
		if had.Findings[i].ID == it.F.ID {
			had.Findings[i].Meant = true
		}
	}
	if err := a.book().SaveCheckOver(it.Node, had, before); err != nil {
		a.warn(err)
		return
	}
	st := a.store()
	what := checktext.CheckMarkedNoChange()
	if it.F.Kind == workspace.FindNew {
		what = locales.CheckIgnored()
	}
	a.status().Did(what, func() error {
		if a.store() != st {
			return fmt.Errorf("%s", locales.StatusWorkSwitchedUndo())
		}
		return a.book().SaveCheckOver(it.Node, before, had)
	})
}

// checkDue lists the chapters a check of the book would read: those long
// enough, never checked or changed since.
func (a *Panel) checkDue() []string {
	texts, err := a.store().Texts() // the book in one pass
	if err != nil {
		a.warn(err)
		return nil
	}
	return workspace.DueChapters(a.book().Checks(), a.book().Tree(), texts, a.ai().MinCheck(), a.store().Work().Language)
}

// openCheck shows the page of the checks, at finding id when given.
func (a *Panel) openCheck(id string) {
	a.desktop.ShowPage("check.check")
	if id != "" {
		a.State.Shown, a.State.Tab = id, 0
	}
}

// checkPending is the findings as the pending list has them.
func (a *Panel) checkPending() (hostapi.PendingGroup, bool) {
	g := hostapi.PendingGroup{ID: "check", Name: locales.PageCheck(), Color: a.pal().Amber}
	if !a.checksOn() {
		return g, false
	}
	for _, it := range a.listedChecks() {
		finding := it.F.ID
		p := hostapi.PendingItem{Key: "检查/" + it.Node + "/" + it.F.Kind + "/" + it.F.Quote, Text: it.F.Title, Sub: it.Title, Here: it.Here}
		for _, o := range it.Also {
			if o.Here && !p.Here { // a name found here too is of this chapter
				p.Sub, p.Here, finding = o.Title, true, o.F.ID
			}
		}
		p.Open = func(_ ui.Services) { a.openCheck(finding) }
		if it.F.Kind == workspace.FindNew {
			p.Text = checktext.StatusNotInLore(it.F.Name)
		}
		if it.Here {
			p.Sub += locales.StatusHereTag()
		}
		g.Items = append(g.Items, p)
	}
	return g, len(g.Items) > 0
}

// checkState says in a line where the check of the chapter shown stands,
// whether there is something of it to look at, and what more there is to
// say of the line, if anything.
func (a *Panel) checkState() (line string, found int, can bool, tip string) {
	w := a.writing()
	k := &a.State
	for _, it := range a.checkItems(false) {
		if it.Here {
			found++
		}
	}

	text, _ := a.liveText()
	had, ok := a.book().Checks()[w.Node]
	switch {
	case k.Queue.Running == w.Node:
		return checktext.StatusChecking(), 0, false, ""
	case slices.Contains(k.Queue.Pending, w.Node):
		return checktext.StatusQueued(), 0, false, ""
	case k.Queue.Failed[w.Node] != nil:
		line := checktext.StatusFailed(a.status().FormatError(k.Queue.Failed[w.Node]))
		return line, 0, true, line // what went wrong is long: in full on hover
	case !a.hasSetting():
		return checktext.CheckNoSettings(), 0, false, ""
	case a.store().Work().Language.Count(text) < a.ai().MinCheck():
		return checktext.CheckMinimum(a.ai().MinCheck(), a.countUnit()), 0, false, ""
	case !ok:
		return checktext.StatusNever(), 0, true, ""
	case found > 0 && had.Stale(text):
		return checktext.StatusDirtyIssues(found), found, true, ""
	case found > 0 && had.Read > 0:
		return checktext.StatusPartIssues(had.Read, found, a.countUnit()), found, false, ""
	case found > 0:
		return checktext.StatusIssues(found), found, false, ""
	case had.Stale(text):
		return checktext.StatusEdited(), 0, true, ""
	case had.Read > 0:
		// The rest was not looked at: not to be told as nothing found.
		return checktext.StatusPartOK(had.Read, a.countUnit()), 0, false, checktext.StatusPartLong()
	}
	return checktext.StatusChecked(), 0, false, ""
}
