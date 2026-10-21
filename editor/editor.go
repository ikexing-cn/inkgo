package editor

import (
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

const defaultFontFamily = "Microsoft YaHei UI, PingFang SC, Noto Sans CJK SC"

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
	// NoAI shows nothing of the AI: the predictor is never asked, and what
	// an AI wrote is neither underlined nor a mark at the caret.
	NoAI bool
	// GhostNote, if set, names what wrote a prediction the author accepts,
	// as the note of its mark.
	GhostNote func() string
	Font      ui.Font
	// LineHeight is the height of a line as a multiple of the font's size;
	// zero takes 1.8.
	LineHeight float32
	// Prose indents each paragraph's first line as it shows it, keeping
	// indents out of the text: Tab inserts none, and a paste loses its own.
	Prose bool
	// Placeholder shows while the text is empty.
	Placeholder string
	// OwnMenu, when set, leaves the context menu to the host, which draws
	// its own: it is told where in the window one was asked for, the
	// pointer or, for the menu key, under the caret.
	OwnMenu func(x, y float32)
	// Zoom, when set, takes Ctrl+wheel (Cmd on macOS): d is 1 to make the
	// text larger, -1 smaller.
	Zoom func(d int)
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

	predict  predictor
	lastEdit time.Time

	// Entity completion.
	comp      []Entity
	compTyped int
	compSel   int
	compOpen  bool

	// What an AI proposes, shown in the text, with room after each; the
	// version tells the layout they changed. follow keeps the end of one
	// in view as it grows, and show brings the start of one into view.
	props        []Proposal
	gap          float32
	propVer      int
	follow, show string

	// Ranges found, painted under the text; the current one stronger.
	found                 [][2]int
	doubts                [][2]int
	current               int
	names                 []NameLink
	namesVer              int
	nameSpans             []nameRange
	nameBoxes             []nameBox
	nameLayout, layoutVer int
	openName              func(string)

	// View state.
	sv            ui.Services // the window's clipboard and redraw, also outside a frame
	lay           layout
	layKey        layoutKey
	shapes        []paragraphShape
	width, height float32
	scrollY       float32
	reveal        bool
	// typewriter keeps the line written on mid-view; centered is the
	// version of the text it was last centered for.
	typewriter bool
	centered   int
	// zoomed is how far a touchpad has scrolled toward the next Zoom step.
	zoomed    float32
	dragging  bool
	wantFocus bool
	menuKey   bool // the menu key was pressed: the host is told in View
	focused   bool
	popRect   ui.Rect
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
		opts.Font.Family = defaultFontFamily
	}
	return &Editor{
		Doc: doc, opts: opts, predict: predictor{predictVer: -1, results: make(chan prediction, 4)},
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
	if e.predict.valid(e) {
		return e.predict.ghost
	}
	return ""
}
func (e *Editor) Predicting() bool { return e.predict.predicting }

// Completions returns the names offered at the caret, if any.
func (e *Editor) Completions() []Entity {
	if !e.compOpen {
		return nil
	}
	return slices.Clone(e.comp)
}

func singleLine(s string) string { return strings.ReplaceAll(normalize(s), "\n", "") }
