package editor

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// On demand, a prediction comes only when the author asks, and anywhere.
func TestPredictionOnDemand(t *testing.T) {
	e := New(NewDoc("她推开门。窗外下雨"), Options{Predictor: DemoPredictor{}, PredictOnDemand: true})
	e.MoveTo(5, false)
	tt := newTester(e)
	time.Sleep(30 * time.Millisecond)
	tt.Frame()
	if e.Ghost() != "" || e.Predicting() {
		t.Fatal("a prediction came unasked")
	}
	tt.Key(ui.Cmd, ui.KeyJ)
	deadline := time.Now().Add(2 * time.Second)
	for e.Ghost() == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	if e.Ghost() == "" {
		t.Fatal("no prediction when asked")
	}
	tt.Key(0, ui.KeyTab)
	if ms := e.Doc.Marks(); len(ms) != 1 || ms[0].Kind != MarkAI {
		t.Fatalf("marks %+v", ms)
	}
}

// What an AI proposes shows in the text, which stays as it was: in
// place of the words struck, after the words it follows, or as
// paragraphs of its own; the author goes on typing elsewhere. Taken, all
// go in as one step, marked as the AI's, with the caret after the last.
func TestProposalsShowAndGoInAsOneStep(t *testing.T) {
	e := New(NewDoc("她走到井边。井里有光。\n雨停了。"), Options{Prose: true})
	tt := newTester(e)
	ps := []Proposal{
		{ID: "swap", A: 0, B: 6, Text: "她慢慢走到井边，\n\n停住了。"},
		{ID: "end", A: 16, B: 16, Keep: true, Text: "\n天亮了。"},
	}
	e.SetProposals(ps, 80)
	tt.Frame()
	if e.Doc.String() != "她走到井边。井里有光。\n雨停了。" {
		t.Fatal("a proposal changed the text")
	}
	if got := string(e.lay.disp); got != "她走到井边。她慢慢走到井边，\n停住了。\uFFFC井里有光。\n雨停了。\n天亮了。\uFFFC" {
		t.Fatalf("shown %q", got)
	}
	var struck, green []string
	for i, k := range e.lay.kinds {
		switch k {
		case runeStruck:
			struck = append(struck, string(e.lay.disp[i]))
		case runeNew:
			green = append(green, string(e.lay.disp[i]))
		}
	}
	if strings.Join(struck, "") != "她走到井边。" || strings.Join(green, "") != "她慢慢走到井边，\n停住了。\n天亮了。" {
		t.Fatalf("struck %q, green %q", struck, green)
	}
	// The room after each is as wide as asked, after its last word.
	gap, ok := e.ProposalGap("swap")
	if !ok || gap.W != 80 {
		t.Fatalf("room %+v %v", gap, ok)
	}
	ln := &e.lay.lines[e.lay.lineOf(e.lay.gaps["swap"])]
	if x := e.lay.x(ln, e.lay.gaps["swap"]+1) - e.lay.x(ln, e.lay.gaps["swap"]); x != 80 {
		t.Fatalf("the room is %v wide", x)
	}
	// A click on the words proposed is a click at the place they show.
	if d := e.lay.toDoc(e.lay.gaps["swap"] - 2); d != 6 {
		t.Fatalf("maps to %d", d)
	}
	if d := e.lay.toDisp(7, false); string(e.lay.disp[d]) != "里" {
		t.Fatalf("the text after shows at %d", d)
	}
	e.MoveTo(e.Doc.Len(), false)
	e.Insert("后来")
	if e.Doc.String() != "她走到井边。井里有光。\n雨停了。后来" {
		t.Fatalf("typed %q", e.Doc.String())
	}
	ps[1].A, ps[1].B = e.Doc.Len(), e.Doc.Len()
	e.Take(ps, "deepseek/deepseek-flash")
	if e.Doc.String() != "她慢慢走到井边，\n停住了。井里有光。\n雨停了。后来\n天亮了。" {
		t.Fatalf("taken %q", e.Doc.String())
	}
	ms := e.Doc.Marks()
	if len(ms) != 2 || ms[0].Kind != MarkAI || e.Doc.Slice(ms[0].Start, ms[0].End) != "她慢慢走到井边，\n停住了。" ||
		e.Doc.Slice(ms[1].Start, ms[1].End) != "天亮了。" || ms[1].Note != "deepseek/deepseek-flash" || e.Caret() != e.Doc.Len() {
		t.Fatalf("marks %+v, caret %d", ms, e.Caret())
	}
	e.Undo()
	// Undone, the caret is where the first went in, though it was at the
	// end before; redone, after the last.
	if e.Doc.String() != "她走到井边。井里有光。\n雨停了。后来" || len(e.Doc.Marks()) != 0 || e.Caret() != 0 {
		t.Fatalf("undo %q %+v, caret %d", e.Doc.String(), e.Doc.Marks(), e.Caret())
	}
	e.Redo()
	if e.Doc.String() != "她慢慢走到井边，\n停住了。井里有光。\n雨停了。后来\n天亮了。" || len(e.Doc.Marks()) != 2 || e.Caret() != e.Doc.Len() {
		t.Fatalf("redo %q %+v, caret %d", e.Doc.String(), e.Doc.Marks(), e.Caret())
	}
}

// A proposal for the whole chapter shows below the words it would
// replace, and taken it takes their place.
func TestAProposalOfAllShowsApart(t *testing.T) {
	e := New(NewDoc("旧的开头。"), Options{Prose: true})
	tt := newTester(e)
	e.SetProposals([]Proposal{{ID: "all", A: 0, B: 5, Apart: true, Text: "雨下了一夜。\n她醒了。"}}, 60)
	tt.Frame()
	if got := string(e.lay.disp); got != "旧的开头。\n雨下了一夜。\n她醒了。\uFFFC" || e.lay.lines[1].y == e.lay.lines[0].y {
		t.Fatalf("shown %q", got)
	}
	e.Take(e.Proposals(), "m")
	if ms := e.Doc.Marks(); e.Doc.String() != "雨下了一夜。\n她醒了。" || len(ms) != 1 || ms[0].Start != 0 || ms[0].End != e.Doc.Len() {
		t.Fatalf("taken %q %+v", e.Doc.String(), ms)
	}
	e.Undo()
	if e.Doc.String() != "旧的开头。" {
		t.Fatalf("undo %q", e.Doc.String())
	}
}

// A proposal growing at the end of the text keeps its end in view while
// that was in view; brought into view, its start shows.
func TestAProposalComesIntoView(t *testing.T) {
	e := New(NewDoc(strings.Repeat("雨下了一夜。\n", 40)), Options{Prose: true})
	tt := newTester(e)
	n := e.Doc.Len()
	e.SetProposals([]Proposal{{ID: "d", A: n, B: n, Keep: true, Text: "她"}}, 60)
	tt.Frame()
	if _, ok := e.ProposalGap("d"); ok {
		t.Fatal("the end of a long text is in view")
	}
	e.RevealProposal("d")
	tt.Frame()
	if _, ok := e.ProposalGap("d"); !ok {
		t.Fatal("not brought into view")
	}
	e.SetProposals([]Proposal{{ID: "d", A: n, B: n, Keep: true, Text: "她" + strings.Repeat("醒了。\n", 12)}}, 60)
	tt.Frame()
	if _, ok := e.ProposalGap("d"); !ok {
		t.Fatal("the end went out of view as it grew")
	}
	// Asked to show it in the frame the caret would come into view, as a
	// tab comes back, it shows.
	e.SetProposals([]Proposal{{ID: "d", A: n, B: n, Keep: true, Text: "她"}}, 60)
	e.MoveTo(0, false)
	e.RevealProposal("d")
	tt.Frame()
	if _, ok := e.ProposalGap("d"); !ok {
		t.Fatal("the caret took the view from the proposal asked for")
	}
}

// Words the author changes leave what an AI wrote; what is around stays
// marked.
func TestTheAIMarkLetsGoOfEditedWords(t *testing.T) {
	d := NewDoc("雨下了一夜，她醒了。")
	d.Add(Mark{Kind: MarkAI, Start: 0, End: 10, Note: ""})
	d.replace(3, 5, []rune("整整一夜")) // "一夜" becomes "整整一夜"
	ms := d.Marks()
	if len(ms) != 2 || d.Slice(ms[0].Start, ms[0].End) != "雨下了" || d.Slice(ms[1].Start, ms[1].End) != "，她醒了。" {
		t.Fatalf("split %+v %q", ms, d.String())
	}
	d.replace(0, 3, []rune("大雨"))
	if ms = d.Marks(); len(ms) != 1 || d.Slice(ms[0].Start, ms[0].End) != "，她醒了。" {
		t.Fatalf("rewritten whole %+v", ms)
	}
	d.replace(d.Len(), d.Len(), []rune("后来"))
	if ms = d.Marks(); len(ms) != 1 || d.Slice(ms[0].Start, ms[0].End) != "，她醒了。" {
		t.Fatalf("typing after %+v", ms)
	}
}

// With the AI off, nothing asks the predictor and a prediction shown goes;
// what an AI wrote is not underlined nor a mark at the caret, yet its mark
// stays, for when the AI is back.
func TestNoAI(t *testing.T) {
	const text = "她推开门。窗外下雨"
	e := New(NewDoc(text), Options{Predictor: DemoPredictor{}, PredictOnDemand: true})
	e.Doc.Add(Mark{Kind: MarkAI, Start: 0, End: 5, Note: "deepseek/deepseek-flash"})
	e.MoveTo(len([]rune(text)), false)
	tt := newTester(e)
	tt.Key(ui.Cmd, ui.KeyJ)
	deadline := time.Now().Add(2 * time.Second)
	for e.Ghost() == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	if e.Ghost() == "" {
		t.Fatal("no prediction when asked")
	}
	on := pixels(tt)
	e.SetNoAI(true)
	tt.Frame()
	if e.Ghost() != "" {
		t.Fatal("the prediction stayed with the AI off")
	}
	tt.Key(ui.Cmd, ui.KeyJ)
	time.Sleep(50 * time.Millisecond)
	tt.Frame()
	if e.Ghost() != "" || e.Predicting() {
		t.Fatal("Cmd+J asked for a prediction with the AI off")
	}
	plain := New(NewDoc(text), Options{NoAI: true})
	plain.MoveTo(len([]rune(text)), false)
	if off := pixels(tt); off == on || off != pixels(newTester(plain)) {
		t.Fatal("with the AI off, the draft is still underlined")
	}
	e.MoveTo(2, false)
	if e.Marked() {
		t.Fatal("the draft is a mark at the caret with the AI off")
	}
	e.Unmark()
	if len(e.Doc.Marks()) != 1 {
		t.Fatalf("the draft's mark went with the AI off: %+v", e.Doc.Marks())
	}
	e.SetNoAI(false)
	if !e.Marked() {
		t.Fatal("the draft's mark did not come back with the AI")
	}
}

// pixels is what tt shows now.
func pixels(tt *ui.Tester) string {
	tt.Frame()
	img := tt.Image()
	b := img.Bounds()
	out := make([]byte, 0, b.Dx()*b.Dy()*4)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			out = append(out, byte(r>>8), byte(g>>8), byte(bl>>8), byte(a>>8))
		}
	}
	return string(out)
}

func TestPredictionSurvivesMarkChanges(t *testing.T) {
	e := New(NewDoc("雨"), Options{})
	e.MoveTo(1, false)
	e.predict.ghost, e.predict.ghostAt, e.predict.ghostVer = "停了。", 1, e.Doc.textVer
	e.Doc.Add(Mark{Kind: MarkForeshadow, Start: 0, End: 1})
	if e.Ghost() != "停了。" {
		t.Fatal("mark edit invalidated ghost")
	}
	e.Insert("又下")
	if e.Ghost() != "" {
		t.Fatal("text edit left ghost valid")
	}
}
