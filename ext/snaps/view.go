package snaps

import (
	"writer/editor"
	"writer/ext/hostapi"
	"writer/workspace"
)

// View owns the extension's presentation state on the frame thread.
type View struct {
	// Book is frame-thread presentation state.
	Book hostapi.Book
	// Palette is frame-thread presentation state.
	Palette hostapi.Palette
	// Notices is frame-thread presentation state.
	Notices hostapi.Notices
	// Desktop is frame-thread presentation state.
	Desktop hostapi.Desktop
	// AI is frame-thread presentation state.
	AI hostapi.AI
	// Talk is frame-thread presentation state.
	Talk hostapi.Talk
	// SnapsOpen is frame-thread presentation state.
	SnapsOpen bool
	// Snaps is frame-thread presentation state.
	Snaps []workspace.Snapshot
	// Snap is frame-thread presentation state.
	Snap string // the snapshot chosen
	// SnapText is frame-thread presentation state.
	SnapText string
	// Compare is frame-thread presentation state.
	Compare bool
	// SnapShown is frame-thread presentation state.
	SnapShown snapView
}

func (a *View) editor() *editor.Editor { _, ed, _ := a.Book.Chapter(); return ed }
func (a *View) node() string           { id, _, _ := a.Desktop.Selected(); return id }
func (a *View) lost() bool             { _, b, _ := a.Desktop.Selected(); return b }
func (a *View) paused() bool {
	_, _, b := a.Desktop.Selected()
	return b || a.Desktop.ChapterExternal()
}
func (a *View) title() string { s, _ := a.Book.ChapterTitle(a.node()); return s }

// Reason returns the localized snapshot reason on the frame thread.
func (a *View) Reason(s string) string { return a.snapReason(s) }
