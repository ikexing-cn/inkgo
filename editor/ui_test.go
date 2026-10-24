package editor

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

type testPredictor struct{}

func (testPredictor) Predict(ctx context.Context, _ string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return "她没有回头。", nil
}

func newTester(e *Editor) *ui.Tester {
	tt := ui.NewTester(func(c *ui.Context) { e.View(c).Fill() }, 600, 400)
	tt.Frame()
	tt.Frame() // the first paint learns the size
	return tt
}

func TestTypeComposeAndKeys(t *testing.T) {
	e := New(NewDoc(""), Options{})
	tt := newTester(e)
	if _, ok := tt.TextCaret(); !ok {
		t.Fatal("editor does not take text from input methods")
	}
	tt.Compose("ni", 2)
	if e.Preedit() != "ni" || e.Doc.Len() != 0 {
		t.Fatalf("composition must not enter the text: preedit %q doc %q", e.Preedit(), e.Doc.String())
	}
	tt.Type("你")
	if e.Preedit() != "" || e.Doc.String() != "你" {
		t.Fatalf("commit: preedit %q doc %q", e.Preedit(), e.Doc.String())
	}
	tt.Type("好")
	tt.Key(0, ui.KeyEnter)
	tt.Type("世界")
	tt.Key(0, ui.KeyBackspace)
	if got := e.Doc.String(); got != "你好\n世" {
		t.Fatalf("doc = %q", got)
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	if got := e.Doc.String(); got != "你好\n世界" {
		t.Fatalf("after undo = %q", got)
	}
	tt.Key(0, ui.KeyUp)
	if e.Caret() > 2 {
		t.Fatalf("up should go to the first line, caret %d", e.Caret())
	}
}

func TestReadOnlySourceSelection(t *testing.T) {
	e := New(NewDoc("雨夜🌙，林晚怕黑。"), Options{ReadOnly: true, Predictor: testPredictor{}})
	tt := newTester(e)
	tt.Key(ui.Cmd, ui.KeyA)
	if e.SelectedText() != e.Doc.String() {
		t.Fatal("read-only selection failed")
	}
	tt.Command("copy")
	if tt.Clipboard() != e.Doc.String() {
		t.Fatal("read-only copy failed")
	}
	tt.Type("改写")
	tt.Compose("ni", 2)
	tt.Key(0, ui.KeyBackspace)
	tt.Key(0, ui.KeyDelete)
	tt.Key(0, ui.KeyEnter)
	tt.Command("cut")
	tt.Command("paste")
	tt.Key(ui.Cmd, ui.KeyZ)
	if e.Doc.String() != "雨夜🌙，林晚怕黑。" || e.Preedit() != "" || e.Ghost() != "" {
		t.Fatalf("read-only source changed: %q", e.Doc.String())
	}
	tt.Key(0, ui.KeyHome)
	tt.Key(ui.Shift, ui.KeyRight)
	if e.SelectedText() != "雨" {
		t.Fatalf("read-only navigation selection: %q", e.SelectedText())
	}
}

func TestLongTextWraps(t *testing.T) {
	s := ""
	for range 20 {
		s += "雨下了整整一夜，院子里的老井泛着光。"
	}
	e := New(NewDoc(s), Options{})
	newTester(e)
	if n := len(e.lay.lines); n < 5 {
		t.Fatalf("expected wrapping, got %d lines", n)
	}
	for _, ln := range e.lay.lines[1:] {
		if noLineStart(e.lay.disp[ln.start]) {
			t.Fatalf("line starts with %q", e.lay.disp[ln.start])
		}
	}
}

func TestGhostAcceptedWithTab(t *testing.T) {
	e := New(NewDoc("她推开门"), Options{Predictor: testPredictor{}, PredictDelay: 0})
	e.MoveTo(e.Doc.Len(), false)
	tt := newTester(e)
	deadline := time.Now().Add(2 * time.Second)
	for e.Ghost() == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	g := e.Ghost()
	if g == "" {
		t.Fatal("no prediction shown")
	}
	if e.Doc.String() != "她推开门" {
		t.Fatal("a prediction must not change the text before Tab")
	}
	tt.Key(0, ui.KeyTab)
	if e.Doc.String() != "她推开门"+g {
		t.Fatalf("after Tab: %q", e.Doc.String())
	}
	ms := e.Doc.Marks()
	if len(ms) != 1 || ms[0].Kind != MarkAI || e.Doc.Slice(ms[0].Start, ms[0].End) != g {
		t.Fatalf("accepted text not marked: %+v", ms)
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	if e.Doc.String() != "她推开门" || len(e.Doc.Marks()) != 0 {
		t.Fatalf("undo accept: %q %+v", e.Doc.String(), e.Doc.Marks())
	}
}

func TestEntityCompletion(t *testing.T) {
	e := New(NewDoc(""), Options{Entities: []Entity{{Name: "沈砚", Kind: "人物"}}})
	tt := newTester(e)
	// An input method composes, commits, then ends the composition.
	tt.Compose("shen", 4)
	tt.Type("沈")
	tt.Compose("", 0)
	if len(e.Completions()) != 1 {
		t.Fatalf("completions = %v", e.Completions())
	}
	tt.Key(0, ui.KeyEnter)
	if e.Doc.String() != "沈砚" {
		t.Fatalf("doc = %q", e.Doc.String())
	}
	// Composing again hides them.
	tt.Type("沈")
	tt.Compose("y", 1)
	if len(e.Completions()) != 0 {
		t.Fatal("offered while composing")
	}
}

// Another name completes too, showing whose it is; at most a few are
// offered.
func TestCompleteAnotherName(t *testing.T) {
	ents := []Entity{{Name: "林晚", Kind: "人物"}, {Name: "晚晚", Kind: "人物", Of: "林晚"}}
	for _, n := range []string{"林一", "林二", "林三", "林四", "林五", "林六"} {
		ents = append(ents, Entity{Name: n})
	}
	e := New(NewDoc(""), Options{Entities: ents})
	tt := newTester(e)
	tt.Type("晚")
	if c := e.Completions(); len(c) != 1 || c[0].Of != "林晚" {
		t.Fatalf("completions = %v", c)
	}
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Type("林")
	if e.Doc.String() != "晚晚林" || len(e.Completions()) != maxCandidates {
		t.Fatalf("%q, %d offered", e.Doc.String(), len(e.Completions()))
	}
}

// While completions are offered, no prediction comes by itself; one asked
// for closes them.
func TestCompletionHoldsThePrediction(t *testing.T) {
	e := New(NewDoc(""), Options{Entities: []Entity{{Name: "沈砚"}}, Predictor: testPredictor{}, PredictDelay: 0})
	tt := newTester(e)
	tt.Type("沈")
	time.Sleep(30 * time.Millisecond)
	tt.Frame()
	if e.Predicting() || e.Ghost() != "" {
		t.Fatal("predicted over the completions")
	}
	tt.Key(ui.Cmd, ui.KeyJ)
	if len(e.Completions()) != 0 {
		t.Fatal("asking kept the completions")
	}
	deadline := time.Now().Add(2 * time.Second)
	for e.Ghost() == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	if e.Ghost() == "" {
		t.Fatal("no prediction when asked")
	}
}

func TestClickMovesCaret(t *testing.T) {
	e := New(NewDoc("一二三四五六七八九十"), Options{})
	tt := newTester(e)
	tt.ClickAt(e.lay.left+1, padTop+5)
	if e.Caret() != 0 {
		t.Fatalf("click at the start: caret %d", e.Caret())
	}
	ln := &e.lay.lines[0]
	tt.ClickAt(e.lay.left+e.lay.x(ln, 5), padTop+5)
	if e.Caret() != 5 {
		t.Fatalf("click at rune 5: caret %d", e.Caret())
	}
}

// Cmd+Backspace and Cmd+Delete delete as far as Cmd+Left and Cmd+Right
// go, and at a paragraph's edge only the line break.
func TestDeleteAWord(t *testing.T) {
	e := New(NewDoc("林晚推门，雨停了\n老周"), Options{})
	tt := newTester(e)
	e.MoveTo(8, false) // after 雨停了
	tt.Key(ui.Cmd, ui.KeyBackspace)
	if got := e.Doc.String(); got != "林晚推门，\n老周" {
		t.Fatalf("back: %q", got)
	}
	tt.Key(ui.Cmd, ui.KeyDelete)
	if got := e.Doc.String(); got != "林晚推门，老周" {
		t.Fatalf("the break: %q", got)
	}
	e.MoveTo(0, false)
	tt.Key(ui.Cmd, ui.KeyDelete)
	if got := e.Doc.String(); got != "，老周" {
		t.Fatalf("ahead: %q", got)
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	if got := e.Doc.String(); got != "林晚推门，老周" {
		t.Fatalf("undo: %q", got)
	}
}

// In typewriter mode typing keeps the caret's line mid-view, even on the
// last line, while clicks and arrows leave the view where it is.
func TestTypewriterScrolling(t *testing.T) {
	text := strings.Repeat("一行字。\n", 60)
	e := New(NewDoc(text), Options{})
	tt := newTester(e)
	e.SetTypewriter(true)
	e.MoveTo(e.Doc.Len(), false)
	tt.Type("末")
	tt.Frame()
	mid := func() float32 {
		r, _ := tt.TextCaret()
		return r.Y + r.H/2
	}
	if y := mid(); y < 180 || y > 220 {
		t.Fatalf("the last line at %v, not mid-view in 400", y)
	}
	before := e.scrollY
	e.MoveTo(e.Doc.Len()-10, false) // two lines up, still in view
	tt.Frame()
	if e.scrollY != before {
		t.Fatalf("an arrow moved the view: %v to %v", before, e.scrollY)
	}
	e.SetTypewriter(false)
	tt.Frame()
	if e.scrollY > e.lay.height-e.height {
		t.Fatal("scrolled past the end when off")
	}
}

// The scroll bar's thumb drags the text and its track pages it, with an
// arrow over the bar and the caret left where it was.
func TestScrollBarDrags(t *testing.T) {
	e := New(NewDoc(strings.Repeat("一行字。\n", 200)), Options{})
	tt := newTester(e)
	x := e.width - 4
	tt.Move(x, 10)
	if tt.Cursor() != ui.CursorDefault || !e.barHot {
		t.Fatalf("cursor %v over the bar", tt.Cursor())
	}
	_, h := e.thumb()
	tt.Press(x, h/2)
	tt.Move(x, h/2+100)
	tt.Release(x, h/2+100)
	if want := 100 * e.maxScroll() / (e.height - h); e.scrollY < want-1 || e.scrollY > want+1 {
		t.Fatalf("the thumb dragged 100 down scrolled %v, not %v", e.scrollY, want)
	}
	if e.Caret() != 0 || e.barDrag {
		t.Fatalf("caret %d, still held %v", e.Caret(), e.barDrag)
	}
	at := e.scrollY
	tt.ClickAt(x, e.height-2)
	if e.scrollY < at+e.height*0.9-1 {
		t.Fatalf("a click below the thumb scrolled from %v to %v", at, e.scrollY)
	}
}

// A new size reshapes the text: the glyphs grow, not only the lines.
func TestFontSizeReshapes(t *testing.T) {
	e := New(NewDoc("一二三"), Options{Font: ui.Font{Size: 16}})
	tt := newTester(e)
	tt.Frame()
	before := e.lay.lines[0].sh.glyphs[0].Advance
	e.SetFontSize(24)
	tt.Frame()
	if after := e.lay.lines[0].sh.glyphs[0].Advance; after <= before {
		t.Fatalf("a glyph %v wide at 16, %v at 24", before, after)
	}
}

// Ctrl+wheel zooms instead of scrolling: a notch a step, a touchpad a
// step every zoomStep.
func TestCtrlWheelZooms(t *testing.T) {
	var steps []int
	e := New(NewDoc(strings.Repeat("一行字。\n", 60)), Options{Zoom: func(d int) { steps = append(steps, d) }})
	tt := newTester(e)
	tt.Frame()
	e.input(ui.InputEvent{Kind: ui.InputScroll, DY: -120, Mods: ui.Cmd})
	e.input(ui.InputEvent{Kind: ui.InputScroll, DY: 50, Mods: ui.Cmd})
	for range 5 {
		e.input(ui.InputEvent{Kind: ui.InputScroll, DY: -10, Mods: ui.Cmd, Precise: true})
	}
	if !slices.Equal(steps, []int{1, -1, 1}) || e.scrollY != 0 {
		t.Fatalf("steps %v, scrolled %v", steps, e.scrollY)
	}
	e.input(ui.InputEvent{Kind: ui.InputScroll, DY: 50})
	if e.scrollY == 0 || len(steps) != 3 {
		t.Fatal("a plain wheel did not scroll")
	}
}

func TestSelectionTop(t *testing.T) {
	e := New(NewDoc("整条街的灯是同时灭的。\n林晚站在巷口。"), Options{})
	tt := newTester(e)
	if _, ok := e.SelectionTop(); ok {
		t.Fatal("no selection, yet a place for it")
	}
	start := strings.Index(e.Doc.String(), "\n") + 1
	e.MoveTo(len([]rune(e.Doc.String()[:start]))+2, false)
	e.MoveTo(e.Doc.Len(), true)
	tt.Frame()
	r, ok := e.SelectionTop()
	if !ok || r.H <= 0 {
		t.Fatal("selection in view has no place")
	}
	e.MoveTo(0, false)
	e.MoveTo(2, true)
	tt.Frame()
	r0, _ := e.SelectionTop()
	if r.Y <= r0.Y || r.X <= r0.X {
		t.Fatalf("second line's selection at %+v is not right of and below the first's %+v", r, r0)
	}
}
