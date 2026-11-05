package hostapi

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/editor"
	"writer/ext"
)

// WritingKey observes Escape and selection on the frame thread through ext.Call.
// Close is also called by host navigation; it must clear editor highlights.
type WritingKey struct {
	// Escape consumes Escape before the host clears the prose selection.
	Escape func() bool
	// HideSelection suppresses the prose selection toolbar while navigating matches.
	HideSelection func() bool
	// Close closes the bar on host navigation.
	Close func()
}

// WritingKeys collects WritingKeys callbacks on the frame thread.
const WritingKeys ext.PointKey[WritingKey] = "host.writing-keys"

// BeforeProse draws the find band above prose on the frame thread through ext.Call.
// This is Go-only; frame objects must not escape.
const BeforeProse ext.PointKey[DrawItem] = "host.before-prose"

// Names describes completion candidates and display-only setting links.
// Open runs on the frame thread through the owning contribution's guard and
// stays valid for that activation of the provider.
type Names struct {
	// Revision changes whenever the candidates or links change. Returned slices
	// are borrowed and must not be mutated after being handed to the host.
	Revision uint64
	Entities []editor.Entity
	Links    []editor.NameLink
	Open     func(string)
}

// NameProvider returns current candidates on the frame thread through ext.Call.
type NameProvider func(struct{}) Names

// NameHighlights supplies names independently of persistent prose marks.
const NameHighlights ext.PointKey[NameProvider] = "host.name-highlights"

// ProseRequest is the live chapter input to the editor mark point.
type ProseRequest struct{ Chapter, Text string }

// ProseMarkProvider gives the editor display-only marks on the frame thread.
// Revision is a cheap version of all source inputs other than the chapter and
// document, which the host tracks. The host calls Marks only on a change.
type ProseMarkProvider struct {
	Revision func() uint64
	Marks    func(chapter, text string) []editor.ProseMark
}

// ProseMarks supplies persistent decorations without exposing their source API.
const ProseMarks ext.PointKey[ProseMarkProvider] = "host.prose-marks"

// ProseUI exposes frame-thread geometry and shared writing controls.
// Draw callbacks borrow the current frame and must not retain it.
type ProseUI interface {
	// ChapterText reads a live open editor, or falls back to storage; frame thread.
	ChapterText(string) string
	// EnsureTab keeps a background chapter tab available; frame thread.
	EnsureTab(string)
	// Band draws a full width band above prose in the current frame.
	Band(*ui.Context, float32, float32, func())
	// Refresh reloads derived book and extension views on the frame thread.
	Refresh() error

	// Bounds returns the prose viewport in window coordinates.
	Bounds() ui.Rect
	// PaneVisible reports whether the named pane is being displayed.
	PaneVisible(string) bool
	// PaneOpened reports an explicit pane opening in this frame.
	PaneOpened() bool
	// PaneHidden reports whether the author folded the right pane.
	PaneHidden() bool
	// CloseSelectionTools closes competing selection actions.
	CloseSelectionTools()
	// PaneFind draws a shared pane search header in the current frame.
	PaneFind(*ui.Context, *string, string, string, func(*kit.Menu)) bool
}

// ProseUIService resolves the writing surface on the frame thread.
const ProseUIService ext.ServiceKey[ProseUI] = "host.prose-ui"

// ProseOverlays draws within the editor viewport on the frame thread.
const ProseOverlays ext.PointKey[DrawItem] = "host.prose-overlays"

// ToolHints draws writing hints before the pending button, on the frame thread.
const ToolHints ext.PointKey[DrawItem] = "host.tool-hints"

// Prediction supplies ghost text for a chapter. Predictor runs on a worker;
// Note and OnDemand are queried on the frame thread.
type Prediction struct {
	Predictor func(string) editor.Predictor
	Note      func() string
	OnDemand  func() bool
}

// Predictions collects active continuation providers.
const Predictions ext.PointKey[Prediction] = "host.predictions"

// LeaveQuestions asks frame-thread extensions about undecided author work.
const LeaveQuestions ext.PointKey[func(func()) bool] = "host.leave-questions"

// SelectionButton contributes a compact action above selected prose; frame thread.
type SelectionButton struct {
	Icon       *ui.SVG
	Label, Tip func() string
	Available  func() bool
	Run        func()
}

// SelectionButtons supplies actions before the host's capture action.
const SelectionButtons ext.PointKey[SelectionButton] = "host.selection-buttons"

// BeforeEditor draws bands after the chapter title and outline, on the frame thread.
const BeforeEditor ext.PointKey[DrawItem] = "host.before-editor"
