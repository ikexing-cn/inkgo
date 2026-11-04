package trash

import (
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
	// TrashOpen is frame-thread presentation state.
	TrashOpen bool
	// Trash is frame-thread presentation state.
	Trash []workspace.Trashed
	// Purge is frame-thread presentation state.
	Purge string // the trashed node whose Purge awaits confirmation

}
