package editor

import (
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
	if ms := e.Doc.Marks(); len(ms) != 1 || ms[0].Kind != MarkAIAccepted {
		t.Fatalf("marks %+v", ms)
	}
}

// A draft streams in as the editor stays read-only, and undoes as one
// step with what it replaced.
func TestDraftUndoesAsOne(t *testing.T) {
	e := New(NewDoc("旧的开头。"), Options{Prose: true})
	tt := newTester(e)
	e.BeginDraft(0, e.Doc.Len())
	for _, s := range []string{"雨下", "了一夜。\n", "她醒了。"} {
		e.DraftAppend(s)
		tt.Frame()
	}
	tt.Type("不该进去")
	if !e.ReadOnly() || e.Doc.String() != "雨下了一夜。\n她醒了。" {
		t.Fatalf("while drafting %q", e.Doc.String())
	}
	e.EndDraft("deepseek/deepseek-flash")
	ms := e.Doc.Marks()
	if e.ReadOnly() || len(ms) != 1 || ms[0].Kind != MarkAIDraft || ms[0].Start != 0 || ms[0].End != e.Doc.Len() || ms[0].Note != "deepseek/deepseek-flash" {
		t.Fatalf("after %+v", ms)
	}
	e.Undo()
	if e.Doc.String() != "旧的开头。" || len(e.Doc.Marks()) != 0 {
		t.Fatalf("undo %q %+v", e.Doc.String(), e.Doc.Marks())
	}
	e.Redo()
	if e.Doc.String() != "雨下了一夜。\n她醒了。" || len(e.Doc.Marks()) != 1 {
		t.Fatalf("redo %q", e.Doc.String())
	}
}

// Words the author changes leave the draft; what is around stays marked.
func TestDraftMarkLetsGoOfEditedWords(t *testing.T) {
	d := NewDoc("雨下了一夜，她醒了。")
	d.AddMark(MarkAIDraft, 0, 10, "")
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

// A rewrite shows beside its words until accepted, as one step.
func TestProposalAcceptedAsOneStep(t *testing.T) {
	e := New(NewDoc("她走到井边。井里有光。"), Options{Prose: true})
	tt := newTester(e)
	e.Propose(0, 6, "她慢慢走到井边，\n\n停住了。")
	tt.Frame()
	if _, _, text, ok := e.Proposal(); !ok || text != "她慢慢走到井边，\n停住了。" {
		t.Fatalf("proposal %q", text)
	}
	if e.Doc.String() != "她走到井边。井里有光。" {
		t.Fatal("a proposal changed the text")
	}
	if !e.AcceptProposal("m") {
		t.Fatal("not accepted")
	}
	if e.Doc.String() != "她慢慢走到井边，\n停住了。井里有光。" {
		t.Fatalf("accepted %q", e.Doc.String())
	}
	ms := e.Doc.Marks()
	if len(ms) != 1 || ms[0].Kind != MarkAIAccepted || e.Doc.Slice(ms[0].Start, ms[0].End) != "她慢慢走到井边，\n停住了。" {
		t.Fatalf("marks %+v", ms)
	}
	e.Undo()
	if e.Doc.String() != "她走到井边。井里有光。" || len(e.Doc.Marks()) != 0 {
		t.Fatalf("undo %q", e.Doc.String())
	}
	e.Propose(0, 2, "他")
	e.Insert("x") // the text changing drops what was proposed
	if _, _, _, ok := e.Proposal(); ok {
		t.Fatal("a proposal outlived its text")
	}
}

// With the AI off, nothing asks the predictor and a prediction shown goes;
// what an AI wrote is not underlined nor a mark at the caret, yet its mark
// stays, for when the AI is back.
func TestNoAI(t *testing.T) {
	const text = "她推开门。窗外下雨"
	e := New(NewDoc(text), Options{Predictor: DemoPredictor{}, PredictOnDemand: true})
	e.Doc.AddMark(MarkAIDraft, 0, 5, "deepseek/deepseek-flash")
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
