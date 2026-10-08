package editor

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// Options configure an Editor.
type Options struct {
	// ReadOnly allows selection and copying without changing the document.
	ReadOnly bool
	// Entities are the confirmed names that complete as they are typed.
	Entities []Entity
	// Predictor predicts the text after the caret; nil turns predictions
	// off. PredictDelay is how long typing pauses before it is asked.
	Predictor    Predictor
	PredictDelay time.Duration
	Font         ui.Font
}

// Editor edits a Doc in a window of native UI. Its methods run on the main
// thread, as the view does.
type Editor struct {
	Doc  *Doc
	opts Options

	caret, anchor int
	goalX         float32
	hasGoal       bool
	undo, redo    []undoStep

	// The input method's composition, shown at the caret, not in the text.
	preedit      string
	preeditCaret int

	// An AI prediction shown after the caret until Tab accepts it.
	ghost             string
	ghostAt, ghostVer int
	predictVer        int
	predictAt         int
	predicting        bool
	cancel            context.CancelFunc
	results           chan prediction
	lastEdit          time.Time

	// Entity completion.
	comp      []Entity
	compTyped int
	compSel   int
	compOpen  bool

	// View state.
	c             *ui.Context
	lay           layout
	layKey        layoutKey
	shapes        map[string]shaped
	width, height float32
	scrollY       float32
	reveal        bool
	dragging      bool
	wantFocus     bool
	focused       bool
	popRect       ui.Rect
}

type prediction struct {
	text    string
	err     error
	ver, at int
}

type undoStep struct {
	at                        int
	removed, inserted         []rune
	caretBefore, anchorBefore int
	caretAfter                int
	marksBefore, marksAfter   []Mark
	typing                    bool
	when                      time.Time
}

// New returns an editor of doc.
func New(doc *Doc, opts Options) *Editor {
	if opts.Font.Size == 0 {
		opts.Font.Size = 17
	}
	if opts.Font.Family == "" {
		opts.Font.Family = "Microsoft YaHei UI, PingFang SC, Noto Sans CJK SC"
	}
	return &Editor{
		Doc: doc, opts: opts, predictVer: -1,
		results: make(chan prediction, 4), shapes: map[string]shaped{},
		lastEdit: time.Now(), wantFocus: true,
	}
}

// Caret returns the caret's offset, and Selection the selected range.
func (e *Editor) Caret() int { return e.caret }
func (e *Editor) Selection() (int, int) {
	return min(e.caret, e.anchor), max(e.caret, e.anchor)
}

// Preedit is the input method's composition; Ghost the prediction shown.
func (e *Editor) Preedit() string { return e.preedit }
func (e *Editor) Ghost() string {
	if e.ghostValid() {
		return e.ghost
	}
	return ""
}
func (e *Editor) Predicting() bool { return e.predicting }

// Completions returns the names offered at the caret, if any.
func (e *Editor) Completions() []Entity {
	if !e.compOpen {
		return nil
	}
	return slices.Clone(e.comp)
}

// SetText replaces the whole text, forgetting the undo history.
func (e *Editor) SetText(s string) {
	e.Doc = NewDoc(s)
	e.caret, e.anchor, e.scrollY = 0, 0, 0
	e.undo, e.redo, e.ghost, e.compOpen = nil, nil, "", false
	e.shapes = map[string]shaped{}
	e.lay = layout{}
}

func (e *Editor) ghostValid() bool {
	return e.ghost != "" && e.ghostAt == e.caret && e.anchor == e.caret &&
		e.ghostVer == e.Doc.version && e.preedit == ""
}

func singleLine(s string) string { return strings.ReplaceAll(normalize(s), "\n", "") }
