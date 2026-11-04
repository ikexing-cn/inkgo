package desk

import (
	"github.com/egoist/mygo/ui"
	"slices"
	"time"
	"writer/workspace"

	"writer/locales"
)

// When a chapter is checked against the setting, as the work's
// preferences keep it.
const (
	checkOnLeave  = ""       // as the author leaves a chapter changed
	checkOnDemand = "manual" // only when asked
	checkOnPause  = "always" // whenever the writing pauses
)

var checkWhens = []struct {
	when       string
	label, sub func() string
}{
	{checkOnLeave, locales.StatusWhenAfter, locales.StatusWhenAfterAbout},
	{checkOnDemand, locales.StatusWhenManual, locales.StatusWhenManualAbout},
	{checkOnPause, locales.StatusWhenWriting, locales.StatusWhenWritingAbout},
}

// checks is the chapters waiting to be checked against the setting, one
// at a time in the background, what the checks found, and the page that
// shows it.
type checks struct {
	queue  workspace.CheckQueue
	runner checkRunner

	of   *workspace.Store // the work the queue is of
	when string

	items      cached[checkKey, []checkItem]
	itemsMeant cached[checkKey, []checkItem]
	listed     cached[checkKey, []checkItem]

	tab      int
	shown    string // the finding the page shows
	whenOpen bool
	askOpen  bool
	due      []string // the chapters a check of the book would read

	prose, proseOf string // the chapter the page quotes, and its ID
	live           string // the open chapter's text, as of liveVer
	liveOf         string
	liveVer        int
}

// checkItem is a finding with where it is.
type checkItem struct {
	node, title string
	here        bool // in the chapter open
	f           workspace.Finding
	entry       workspace.Entry
	also        []checkItem // a new name found in later chapters too, listed here once
}

// entryTitle names an entry the way the setting lists it: 人物 · 林晚.
func entryTitle(e workspace.Entry) string {
	if len(e.Names) > 0 {
		return e.Category + " · " + e.Names[0]
	}
	return e.Category
}

// loadChecks reads what the checks of the work found.
func (a *App) loadChecks() {
	a.warn(a.book.ReloadChecks())
	a.warn(a.book.ReloadTree())
}

// hasSetting tells whether there is a setting to check prose against.
func (a *App) hasSetting() bool {
	return slices.ContainsFunc(a.entries(), func(e workspace.Entry) bool { return e.Place == workspace.PlaceBody })
}

// leaveCheck is called as the author leaves the chapter shown: what the
// prose no longer says is no longer found, and a chapter changed is
// checked again, when that is when to; it is not changed again until it
// is. A chapter
// about to be shown, its text not read yet, is no chapter left; with the
// AI off, the checks are left as they were.
func (a *App) leaveCheck() {
	w := &a.write
	k := &a.ck
	if w.node == "" || w.cur().ed == nil || w.cur().lost || a.Store == nil || !a.checksOn() {
		return
	}
	a.pruneStoredCheck(w.node, w.cur().saved)
	changed := w.cur().changed
	w.cur().changed = false
	if !changed || k.when != checkOnLeave || !a.hasSetting() {
		return
	}
	k.queue.Language = a.Store.Work().Language
	k.queue.EnqueueStale(a.book.Checks(), w.node, w.cur().saved, a.ai.MinCheck())
}

// pruneStoredCheck drops from chapter's check what its prose, text, no longer
// says and the author did not call meant.
func (a *App) pruneStoredCheck(chapter, text string) {
	had, ok := a.book.Checks()[chapter]
	if !ok {
		return
	}
	had, changed := had.Prune(text)
	if !changed {
		return
	}
	if err := a.book.SaveCheck(chapter, had); err != nil {
		a.warn(err)
		return
	}
}

// pruneStoredChecks does so for every chapter with something found, as of the
// disk.
func (a *App) pruneStoredChecks() {
	for _, id := range workspace.PrunableChapters(a.book.Checks()) {
		if ch, err := a.Store.ReadChapter(id); err == nil {
			a.pruneStoredCheck(id, ch.Text)
		}
	}
}

// pollChecks takes in a check done, and starts the next.
func (a *App) pollChecks(c *ui.Context) {
	k := &a.ck
	if !a.checksOn() {
		a.stopChecks()
		return
	}
	if k.of != a.Store {
		k.of = a.Store
		k.shown, k.prose, k.proseOf, k.tab = "", "", "", 0
		k.queue.WorkChanged()
		a.loadChecks()
	}
	if d, ok := k.runner.finished(); ok {
		k.queue.Finished()
		if d.store == a.Store {
			a.saveCheckResult(d)
		}
	}
	id, ok := k.queue.Next()
	if !ok {
		return
	}
	pw, err := a.ai.Pen(a.Store, a.material())
	if err != nil {
		k.queue.CannotStart(id, err)
		return
	}
	k.runner.start(pw, a.Store, id, c.Services())
}

// saveCheckResult saves what a check found, with what the author called meant
// while it ran.
func (a *App) saveCheckResult(d checkDone) {
	k := &a.ck
	result, keep := k.queue.Keep(d.chapter, a.book.Checks()[d.chapter], d.check, d.err)
	if !keep {
		return
	}
	if err := a.book.SaveCheck(d.chapter, result); err != nil {
		k.queue.Fail(d.chapter, err)
		return
	}
}

// stopChecks drops the checks waiting and stops the one running, as the
// work closes or the AI is turned off.
func (a *App) stopChecks() {
	k := &a.ck
	k.runner.stop()
	k.queue.Stop()
}

// checkWhilePausing checks the chapter shown once the writing rests, when
// that is when to: each state of the prose at most once.
func (a *App) checkWhilePausing() {
	w := &a.write
	k := &a.ck
	if k.when != checkOnPause || !a.checksOn() || w.cur().ed == nil || w.node == "" || w.cur().lost || w.cur().ed.Doc.Version() != w.cur().savedVer || !a.hasSetting() {
		return
	}
	k.queue.Language = a.Store.Work().Language
	k.queue.AfterPause(w.node, w.cur().saved, a.book.Checks()[w.node], a.ai.MinCheck(), w.cur().ed.LastEdit(), time.Now())
}
