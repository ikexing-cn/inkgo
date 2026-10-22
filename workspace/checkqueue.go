package workspace

import (
	"context"
	"errors"
	"slices"
	"time"
)

// CheckQueue is the pending checks for one open work. The caller owns
// execution, persistence and cancellation, and holds the stored results:
// each method that weighs them is given them, by chapter.
type CheckQueue struct {
	// Pending is the chapters waiting in submission order.
	Pending []string
	// Running is the chapter awaiting a result, including after cancellation.
	Running string
	// Failed holds the latest execution or persistence error by chapter.
	Failed map[string]string
	// Tried holds the prose hash last submitted after a writing pause.
	Tried map[string]string
}

// Keep returns a replacement result with the author's decisions in had,
// the result stored, preserved. Cancelled and failed results return false;
// failures are recorded for display.
func (c *CheckQueue) Keep(chapter string, had, result Check, err error) (Check, bool) {
	if errors.Is(err, context.Canceled) {
		return Check{}, false
	}
	if err != nil {
		c.Fail(chapter, err)
		return Check{}, false
	}
	delete(c.Failed, chapter)
	return result.KeepMeant(had), true
}

// EnqueueStale submits changed or unchecked prose once it is long enough.
func (c *CheckQueue) EnqueueStale(all map[string]Check, chapter, text string, minimum int) {
	if Count(text) < minimum {
		return
	}
	if had, ok := all[chapter]; !ok || had.Stale(text) {
		c.Enqueue(chapter)
	}
}

// Enqueue asks for chapter's check unless it is waiting or running.
func (c *CheckQueue) Enqueue(chapter string) {
	if c.Running == chapter || slices.Contains(c.Pending, chapter) {
		return
	}
	c.Pending = append(c.Pending, chapter)
	delete(c.Failed, chapter)
}

// PrunableChapters lists the chapters of all with findings the author has
// not called meant. Read only those chapters before pruning all stored
// results.
func PrunableChapters(all map[string]Check) []string {
	var out []string
	for id, had := range all {
		if !slices.ContainsFunc(had.Findings, func(f Finding) bool { return !f.Meant }) {
			continue
		}
		out = append(out, id)
	}
	return out
}

// Next starts the oldest waiting chapter, or reports that nothing can start.
func (c *CheckQueue) Next() (string, bool) {
	if c.Running != "" || len(c.Pending) == 0 {
		return "", false
	}
	c.Running, c.Pending = c.Pending[0], c.Pending[1:]
	return c.Running, true
}

// WorkChanged forgets failures and pause attempts from the previous work.
func (c *CheckQueue) WorkChanged() {
	clear(c.Failed)
	clear(c.Tried)
}

// Finished releases the running chapter when any work's result arrives.
func (c *CheckQueue) Finished() {
	c.Running = ""
}

// CannotStart records a setup failure and abandons the waiting chapters.
func (c *CheckQueue) CannotStart(chapter string, err error) {
	c.Fail(chapter, err)
	c.Running, c.Pending = "", nil
}

// Stop drops waiting chapters; a running request stays until its result arrives.
func (c *CheckQueue) Stop() {
	c.Pending = nil
}

// Fail records an execution or persistence error without changing saved results.
func (c *CheckQueue) Fail(chapter string, err error) {
	if c.Failed == nil {
		c.Failed = map[string]string{}
	}
	c.Failed[chapter] = err.Error()
}

// DueChapters lists long-enough chapters never checked, by all, or changed
// since, in tree order.
func DueChapters(all map[string]Check, tree Tree, texts map[string]string, minimum int) []string {
	var due []string
	tree.Walk(func(n *Node, _ int) {
		text, ok := texts[n.ID]
		if n.Folder || !ok || Count(text) < minimum {
			return
		}
		if had, ok := all[n.ID]; !ok || had.Stale(text) {
			due = append(due, n.ID)
		}
	})
	return due
}

// checkPause is how long the writing rests before a chapter is checked as
// it is written.
const checkPause = 8 * time.Second

// AfterPause submits saved prose after eight seconds without edits, at most
// once per prose hash, including failed attempts, unless had, the result
// stored, is of it. The caller checks its mode, whether the editor is saved,
// and whether a setting is available.
func (c *CheckQueue) AfterPause(chapter, text string, had Check, minimum int, edited, now time.Time) {
	if now.Sub(edited) < checkPause {
		return
	}
	hash := ProseHash(text)
	if c.Tried[chapter] == hash || had.Of == hash || Count(text) < minimum {
		return
	}
	if c.Tried == nil {
		c.Tried = map[string]string{}
	}
	c.Tried[chapter] = hash
	c.Enqueue(chapter)
}
