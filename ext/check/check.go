package check

import (
	"github.com/egoist/mygo/ui"
	"slices"
	"time"
	"writer/workspace"

	"writer/ext/check/checktext"
)

// When a chapter is checked against the setting, as the work's
// preferences keep it.
const (
	checkOnLeave  = ""       // as the author leaves a chapter changed
	checkOnDemand = "manual" // only when asked
	checkOnPause  = "always" // whenever the writing pauses
)

var checkWhens = []struct {
	When       string
	label, sub func() string
}{
	{checkOnLeave, checktext.StatusWhenAfter, checktext.StatusWhenAfterAbout},
	{checkOnDemand, checktext.StatusWhenManual, checktext.StatusWhenManualAbout},
	{checkOnPause, checktext.StatusWhenWriting, checktext.StatusWhenWritingAbout},
}

// checks is the chapters waiting to be checked against the setting, one
// at a time in the background, what the checks found, and the page that
// shows it.
type checks struct {
	// Queue holds pending and running chapter checks.
	Queue workspace.CheckQueue
	// Runner holds the cancellable request mailbox.
	Runner checkRunner

	// Of holds the store whose checks are cached.
	Of *workspace.Store // the work the queue is of
	// When holds the selected automatic check timing.
	When string

	// Items holds cached unresolved findings.
	Items cached[checkKey, []Item]
	// ItemsMeant holds cached findings including intentional ones.
	ItemsMeant cached[checkKey, []Item]
	// Listed holds cached findings grouped by new name.
	Listed cached[checkKey, []Item]

	// Tab holds the selected findings filter.
	Tab int
	// Shown holds the selected finding identity.
	Shown string // the finding the page shows
	// WhenOpen holds whether the timing popover is open.
	WhenOpen bool
	// AskOpen holds whether the book check confirmation is open.
	AskOpen bool
	// Due holds chapters the book check would read.
	Due []string // the chapters a check of the book would read

	// Prose holds the comparison text and its chapter.
	Prose, ProseOf string // the chapter the page quotes, and its ID
	// Live holds the cached editor prose.
	Live string // the open chapter's text, as of liveVer
	// LiveOf holds the chapter of the cached editor prose.
	LiveOf string
	// LiveVer holds the editor version of the cached prose.
	LiveVer int
}

// Item is a finding with where it is.
type Item struct {
	// Node holds the finding location and chapter title.
	Node, Title string
	// Here holds whether this finding belongs to the live chapter.
	Here bool // in the chapter open
	// F holds the stored finding.
	F workspace.Finding
	// Entry holds the referenced setting entry.
	Entry workspace.Entry
	// Also holds other occurrences grouped under this finding.
	Also []Item // a new name found in later chapters too, listed here once
}

// entryTitle names an entry the way the setting lists it: 人物 · 林晚.
func entryTitle(e workspace.Entry) string {
	if len(e.Names) > 0 {
		return e.Category + " · " + e.Names[0]
	}
	return e.Category
}

// loadChecks reads what the checks of the work found.
func (a *Panel) loadChecks() {
	a.warn(a.book().ReloadChecks())
	a.warn(a.book().ReloadTree())
}

// hasSetting tells whether there is a setting to check prose against.
func (a *Panel) hasSetting() bool {
	return slices.ContainsFunc(a.entries(), func(e workspace.Entry) bool { return e.Place == workspace.PlaceBody })
}

// leaveCheck is called as the author leaves the chapter shown: what the
// prose no longer says is no longer found, and a chapter changed is
// checked again, when that is when to; it is not changed again until it
// is. A chapter
// about to be shown, its text not read yet, is no chapter left; with the
// AI off, the checks are left as they were.
func (a *Panel) leaveCheck() {
	w := a.writing()
	k := &a.State
	if w.Node == "" || w.cur().ed == nil || w.cur().lost || a.store() == nil || !a.checksOn() {
		return
	}
	a.pruneStoredCheck(w.Node, w.cur().saved)
	changed := w.cur().changed
	a.editingHost.AcknowledgeChapterChange()
	if !changed || k.When != checkOnLeave || !a.hasSetting() {
		return
	}
	k.Queue.Language = a.store().Work().Language
	k.Queue.EnqueueStale(a.book().Checks(), w.Node, w.cur().saved, a.ai().MinCheck())
}

// pruneStoredCheck drops from chapter's check what its prose, text, no longer
// says and the author did not call meant.
func (a *Panel) pruneStoredCheck(chapter, text string) {
	had, ok := a.book().Checks()[chapter]
	if !ok {
		return
	}
	had, changed := had.Prune(text)
	if !changed {
		return
	}
	if err := a.book().SaveCheck(chapter, had); err != nil {
		a.warn(err)
		return
	}
}

// pruneStoredChecks does so for every chapter with something found, as of the
// disk.
func (a *Panel) pruneStoredChecks() {
	for _, id := range workspace.PrunableChapters(a.book().Checks()) {
		if ch, err := a.store().ReadChapter(id); err == nil {
			a.pruneStoredCheck(id, ch.Text)
		}
	}
}

// pollChecks takes in a check done, and starts the next.
func (a *Panel) pollChecks(c *ui.Context) {
	k := &a.State
	if !a.checksOn() {
		a.stopChecks()
		return
	}
	if k.Of != a.store() {
		k.Of = a.store()
		k.Shown, k.Prose, k.ProseOf, k.Tab = "", "", "", 0
		k.Queue.WorkChanged()
		a.loadChecks()
	}
	if d, ok := k.Runner.finished(); ok {
		k.Queue.Finished()
		if d.Store == a.store() {
			a.saveCheckResult(d)
		}
	}
	id, ok := k.Queue.Next()
	if !ok {
		return
	}
	pw, err := a.pen()
	if err != nil {
		k.Queue.CannotStart(id, err)
		return
	}
	k.Runner.start(pw, a.store(), id, c.Services())
}

// saveCheckResult saves what a check found, with what the author called meant
// while it ran.
func (a *Panel) saveCheckResult(d checkDone) {
	k := &a.State
	result, keep := k.Queue.Keep(d.Chapter, a.book().Checks()[d.Chapter], d.Check, d.Err)
	if !keep {
		return
	}
	if err := a.book().SaveCheck(d.Chapter, result); err != nil {
		k.Queue.Fail(d.Chapter, err)
		return
	}
}

// stopChecks drops the checks waiting and stops the one running, as the
// work closes or the AI is turned off.
func (a *Panel) stopChecks() {
	k := &a.State
	k.Runner.stop()
	k.Queue.Stop()
	k.Queue.Finished()
}

// checkWhilePausing checks the chapter shown once the writing rests, when
// that is when to: each state of the prose at most once.
func (a *Panel) checkWhilePausing() {
	w := a.writing()
	k := &a.State
	if k.When != checkOnPause || !a.checksOn() || w.cur().ed == nil || w.Node == "" || w.cur().lost || w.cur().ed.Doc.Version() != w.cur().savedVer || !a.hasSetting() {
		return
	}
	k.Queue.Language = a.store().Work().Language
	k.Queue.AfterPause(w.Node, w.cur().saved, a.book().Checks()[w.Node], a.ai().MinCheck(), w.cur().ed.LastEdit(), time.Now())
}
