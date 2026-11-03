package phone

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"writer/editor"
)

const phoneLongText = "雨水顺着屋檐往下淌，巷子尽头那盏灯晃了两下，终于灭了。她数着自己的脚步，一步，两步，三步，数到第十七步的时候，听见身后有人喊她的名字，声音很轻，像是怕惊动了什么。她没有回头，只是把手电攥得更紧，指节都发了白。"

// The same walk as the prototype, through actual pointer and keyboard
// input, with assertions against the measured lines and painted ranges.
func TestPreviewLayoutAndInput(t *testing.T) {
	h, s, tt, id := previewFixture(t, "整条街的灯是同时灭的。")
	ed := h.ed
	if !tt.HasText("第 1 屏，共 1 屏") || s.prefs.NoFrame {
		t.Fatal("preview did not open", tt.Texts())
	}
	ed.Select(ed.Doc.Len(), ed.Doc.Len())
	ed.Focus()
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Type(phoneLongText + phoneLongText)
	tt.Key(0, ui.KeyEnter)
	tt.Type(phoneLongText + phoneLongText)
	tt.Frame()
	if s.Pages <= 1 || s.at != s.Pages-1 || s.caret.H == 0 || !tt.HasText("2 段超过 5 行，看下一处") {
		t.Fatalf("pages %d at %d caret %+v: %v", s.Pages, s.at, s.caret, tt.Texts())
	}
	pages := s.Pages
	click(t, tt, "下一屏 ›")
	if s.at != pages-1 || h.id != id {
		t.Fatal("next at the end left the chapter")
	}
	click(t, tt, "‹ 上一屏")
	if s.at != pages-2 {
		t.Fatal("previous did not flip")
	}
	click(t, tt, "下一屏 ›")
	if s.at != pages-1 {
		t.Fatal("next did not flip")
	}

	// A selection uses exactly the source rune offsets; moving it to a
	// previous page makes the preview follow once.
	start := s.Paras[2].Start + 10
	ed.Select(start, start+6)
	ed.Focus()
	tt.Frame()
	if len(s.pick) == 0 || s.caret.H != 0 {
		t.Fatal("selection not painted, or caret also painted", s.pick, s.caret)
	}
	ed.Select(ed.Doc.Len(), ed.Doc.Len())
	ed.Focus()
	tt.Frame()
	click(t, tt, "‹ 上一屏")
	manual := s.at
	ed.Insert("雨")
	tt.Frame()
	if s.at != manual {
		t.Fatal("typing on the same caret page pulled back a manually flipped preview")
	}
	ed.Select(0, 0)
	ed.Focus()
	tt.Frame()
	if s.at != 0 {
		t.Fatal("caret crossing did not follow")
	}
	click(t, tt, "‹ 上一屏")
	if s.at != 0 {
		t.Fatal("previous before the start flipped")
	}
	for s.at < s.Pages-1 {
		click(t, tt, "下一屏 ›")
	}
	// Click the center of a specific glyph on the last visible body line.
	var last editor.PageLine
	for _, l := range s.Lines {
		if l.Para >= 0 && l.Page == s.at {
			last = l
		}
	}
	para := s.Paras[last.Para]
	ln := para.Shape.Lines[last.Line]
	off := min(ln.Start+2, ln.End-1)
	x := (para.Shape.X(last.Line, off) + para.Shape.X(last.Line, off+1)) / 2
	y := last.Y + para.Shape.LineHeight/2
	px, py := s.stage.X+s.ox+(s.body.X+x)*s.scale, s.stage.Y+s.oy+(s.body.Y+y)*s.scale
	tt.Press(px, py)
	tt.Release(px, py)
	tt.Frame()
	tt.Frame()
	if ed.Caret() != para.Start+off+1 {
		t.Fatalf("clicked rune %d, caret is %d", para.Start+off, ed.Caret())
	}
	if box, ok := ed.RangeBox(ed.Caret(), ed.Caret()); !ok || box.Y < 0 || box.Y > ed.Height() {
		t.Fatal("clicked word not revealed", box, ok)
	}

	click(t, tt, "Aa")
	click(t, tt, "A+")
	click(t, tt, "夜间")
	if s.prefs.Size != 21 || s.prefs.Theme != "night" {
		t.Fatal("typesetting did not change", s.prefs)
	}
	click(t, tt, "恢复默认")
	if s.prefs != defaultPhone() {
		t.Fatal("defaults not restored", s.prefs)
	}
	click(t, tt, "Aa")
	click(t, tt, "滚动")
	if s.prefs.Mode != "scroll" || !tt.HasText("2 段超过 5 行") || tt.HasText("2 段超过 5 行，看下一处") {
		t.Fatal("scrolling mode", tt.Texts())
	}
	r := h.prose
	tt.Scroll(r.X+r.W/2, r.Y+r.H/2, 0, 100000)
	tt.Frame()
	if s.scrollY <= 0 || math.Abs(float64(s.scrollY-(s.Height-s.PageHeight))) > 1 {
		t.Fatalf("bottom %f of %f", s.scrollY, s.Height-s.PageHeight)
	}
	tt.Scroll(r.X+r.W/2, r.Y+r.H/2, 0, -100000)
	tt.Frame()
	if s.scrollY != 0 {
		t.Fatalf("top %f", s.scrollY)
	}
	// The reader can also scroll the read-only phone itself; moving the
	// source again takes over with the same paragraph anchor algorithm.
	phoneRect, _ := tt.Find("手机预览正文")
	tt.Scroll(phoneRect.X+phoneRect.W/2, phoneRect.Y+phoneRect.H/2, 0, 120)
	tt.Frame()
	if s.scrollY <= 0 {
		t.Fatal("phone itself did not scroll")
	}
	tt.Scroll(r.X+r.W/2, r.Y+r.H/2, 0, 40)
	tt.Frame()
	f, n, into := ed.ScrollAnchor()
	pp := s.Paras[n]
	wantY := min(max(pp.RollY+into*pp.Shape.Height-f*s.PageHeight, 0), max(s.Height-s.PageHeight, 0))
	if f >= 1 {
		wantY = max(s.Height-s.PageHeight, 0)
	}
	if f <= 0 {
		wantY = 0
	}
	if math.Abs(float64(s.scrollY-wantY)) > 1 {
		t.Fatal("interior scroll anchor", s.scrollY, wantY)
	}
	click(t, tt, "Aa")
	click(t, tt, "A+")
	click(t, tt, "恢复默认")
	if s.prefs.Mode != "scroll" || s.prefs.Size != 20 {
		t.Fatal("reset changed reading mode")
	}
	click(t, tt, "Aa")
	click(t, tt, "翻页")
	click(t, tt, "下一处长段")
	firstLong := s.at
	click(t, tt, "下一处长段")
	if len(s.LongPages) > 1 && s.at == firstLong {
		t.Fatal("next long did not advance")
	}
	for range len(s.LongPages) {
		click(t, tt, "下一处长段")
	}
	if h.id != id {
		t.Fatal("long paragraph navigation changed chapter")
	}

	// Opening another chapter starts at page one, even with a retained
	// caret at its end. The empty/folder state has a single line.
	other, err := h.store.NewChapter("", "另一章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.store.WriteChapter(other, strings.Repeat(phoneLongText+"\n", 8), nil); err != nil {
		t.Fatal(err)
	}
	h.openChapter(t, other)
	tt.Frame()
	if s.at != 0 {
		t.Fatal("new chapter did not start at first screen")
	}
	h.openChapter(t, id)
	tt.Frame()
	if s.at != 0 {
		t.Fatal("returning chapter restored a page position")
	}
	vol, err := h.store.NewChapter("", "第一卷", true)
	if err != nil {
		t.Fatal(err)
	}
	h.openChapter(t, vol)
	tt.Frame()
	if !tt.HasText("打开一章即可预览") {
		t.Fatal("folder has no empty state", tt.Texts())
	}
}

func TestPhonePreferencesDefaultUnknownChoices(t *testing.T) {
	h, p, tt, _ := previewFixture(t, "灯灭了。")
	for _, tc := range []struct {
		name  string
		phone *Typesetting
	}{
		{"untouched", nil},
		{"missing choices", &Typesetting{}},
		{"unknown choices", &Typesetting{Mode: "future", Screen: "future", Size: 25, Spacing: "future", Theme: "future"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h.prefs.Phone = tc.phone
			if p.Preferences() != defaultPhone() {
				t.Fatalf("read defaults: %+v", p.Preferences())
			}
		})
	}
	// Normalization also reaches the UI, and only invalid fields default.
	h.prefs.Phone = &Typesetting{Mode: "scroll", Screen: "future", Size: phoneSizeMin, Spacing: "loose", Theme: "eye", NoFrame: true, NoLong: true}
	tt.Frame()
	want := Typesetting{Mode: "scroll", Screen: "standard", Size: phoneSizeMin, Spacing: "loose", Theme: "eye", NoFrame: true, NoLong: true}
	if p.prefs != want {
		t.Fatal("valid preferences lost while defaulting", p.prefs)
	}
}

func TestNewPreviewStartsAtFirstScreen(t *testing.T) {
	_, p, _, _ := previewFixture(t, "灯灭了。")
	if p.at != 0 {
		t.Fatal("reopened preview did not start at first screen", p.at)
	}
}

// Typing in a chapter above ten thousand characters retains every
// unchanged paragraph's measured glyphs and line breaks. Include the whole
// editing frame, rather than timing a cache lookup in isolation.
func TestPreviewTypingKeepsParagraphShapes(t *testing.T) {
	var body strings.Builder
	for i := 0; i < 120; i++ {
		fmt.Fprintf(&body, "第%d夜，%s\n", i, phoneLongText)
	}
	h, p, tt, _ := previewFixture(t, body.String())
	if h.ed.Doc.Len() <= 10000 {
		t.Fatal("fixture must exceed ten thousand characters")
	}
	tt.Frame()
	ed := h.ed
	ed.Select(ed.Doc.Len(), ed.Doc.Len())
	ed.Focus()
	tt.Frame()
	old := p.Paras[0].Shape
	for range 10 {
		tt.Type("雨")
	}
	if old != p.Paras[0].Shape {
		t.Fatal("typing reshaped an unchanged paragraph")
	}
}
