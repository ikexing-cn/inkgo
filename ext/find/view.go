package find

import (
	"github.com/egoist/mygo/ui"
	"writer/editor"
	"writer/ext/hostapi"
)

// View owns the extension's presentation state on the frame thread.
type View struct {
	// Book is frame-thread presentation state.
	Book    hostapi.Book
	Editing hostapi.Editing
	// Palette is frame-thread presentation state.
	Palette hostapi.Palette
	// Notices is frame-thread presentation state.
	Notices hostapi.Notices
	// Desktop is frame-thread presentation state.
	Desktop hostapi.Navigation
	Native  hostapi.NativeUI
	Paths   hostapi.MachinePaths
	// Finding is frame-thread presentation state.
	Finding bool
	// FindAgain is frame-thread presentation state.
	FindAgain bool // the caret goes to the find box, its words selected
	// Query is frame-thread presentation state.
	Query, With string
	// Current is frame-thread presentation state.
	Current int
	// Wide is frame-thread presentation state.
	Wide bool // Finding in the whole book
	// Sure is frame-thread presentation state.
	Sure bool // asking before replacing through the book
	// Texts is frame-thread presentation state.
	Texts map[string]string // the chapters' text while Finding in the book
	// Hits is frame-thread presentation state.
	Hits []bookHit
	// HitsFor is frame-thread presentation state.
	hitRows  []bookRow
	hitTotal int
	hitList  ui.ListState
	HitsFor  [3]any // the Query, chapter and edit the Hits were found for

}

func (a *View) editor() *editor.Editor { _, ed, _ := a.Editing.Chapter(); return ed }
func (a *View) node() string           { id, _, _ := a.Desktop.Selected(); return id }
func (a *View) lost() bool             { _, b, _ := a.Desktop.Selected(); return b }
func (a *View) paused() bool {
	_, _, b := a.Desktop.Selected()
	return b || a.Editing.ChapterExternal()
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
func tail(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return "…" + string(r[len(r)-n:])
	}
	return s
}
