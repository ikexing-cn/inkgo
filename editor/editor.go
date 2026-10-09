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
	// PredictOnDemand asks the predictor only when the author asks, with
	// Cmd+J, never as typing pauses.
	PredictOnDemand bool
	// GhostNote, if set, names what wrote a prediction the author accepts,
	// as the note of its mark.
	GhostNote func() string
	Font      ui.Font
	// Prose indents each paragraph's first line as it shows it, keeping
	// indents out of the text: Tab inserts none, and a paste loses its own.
	Prose bool
	// Placeholder shows while the text is empty.
	Placeholder string
	// Menu adds items to the end of the context menu.
	Menu func(m *ui.Menu)
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
	asked             bool // the author asked for a prediction
	cancel            context.CancelFunc
	results           chan prediction
	lastEdit          time.Time

	// Entity completion.
	comp      []Entity
	compTyped int
	compSel   int
	compOpen  bool

	// A rewrite shown beside its words, and a draft being written.
	prop proposal
	dr   draft

	// Ranges found, painted under the text; the current one stronger.
	found   [][2]int
	current int

	// View state.
	c             *ui.Context
	lay           layout
	layKey        layoutKey
	shapes        map[string]shaped
	width, height float32
	scrollY       float32
	reveal        bool
	// typewriter keeps the line written on mid-view; centered is the
	// version of the text it was last centered for.
	typewriter bool
	centered   int
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
	// joined undoes and redoes the step with the one before it.
	joined bool
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
