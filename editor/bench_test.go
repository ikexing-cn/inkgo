package editor

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func chapter(paraRunes, paras int) string {
	base := "雨下了整整一夜，林晚站在檐下，看着院子里那口老井，沈砚从屋里走出来，青石镇的灯一盏一盏地亮了。“你还记得那把钥匙吗？”她没有回答。"
	var b strings.Builder
	for i := 0; i < paras; i++ {
		b.WriteString("　　")
		n := 0
		for n < paraRunes {
			b.WriteString(base)
			n += len([]rune(base))
		}
		if i < paras-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func benchEditor(text string, names bool) *Editor {
	e := New(NewDoc(text), Options{Prose: true})
	e.width, e.height = 900, 700
	if names {
		var nl []NameLink
		for _, n := range []string{"林晚", "沈砚", "青石镇", "老井", "钥匙", "檐下", "屋里", "灯", "雨", "夜"} {
			nl = append(nl, NameLink{n, n})
		}
		e.SetNameLinks(nl, func(string) {})
	}
	e.ensureLayout()
	return e
}

// ~12k chars, 100 paragraphs of ~120 chars
func BenchmarkLayoutAfterKeystroke(b *testing.B) {
	e := benchEditor(chapter(120, 100), false)
	e.MoveTo(e.Doc.Len(), false)
	b.Logf("doc runes %d lines %d", e.Doc.Len(), len(e.lay.lines))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Insert("字")
		e.ensureLayout()
		e.Replace(e.Caret()-1, e.Caret(), "")
		e.ensureLayout()
	}
}

func BenchmarkLayoutAfterCaretMove(b *testing.B) {
	e := benchEditor(chapter(120, 100), false)
	n := e.Doc.Len()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.MoveTo(n/2+i%2, false)
		e.ensureLayout()
	}
}

func BenchmarkLayoutNoChange(b *testing.B) {
	e := benchEditor(chapter(120, 100), false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.ensureLayout()
	}
}

func BenchmarkLayoutHugeParagraphKeystroke(b *testing.B) {
	e := benchEditor(chapter(12000, 1), false)
	e.MoveTo(e.Doc.Len(), false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Insert("字")
		e.ensureLayout()
		e.Replace(e.Caret()-1, e.Caret(), "")
		e.ensureLayout()
	}
}

func BenchmarkNameSpansAfterKeystroke(b *testing.B) {
	e := benchEditor(chapter(120, 100), true)
	e.MoveTo(e.Doc.Len(), false)
	b.Logf("spans %d", len(e.nameRanges()))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Insert("字")
		e.ensureLayout()
		e.nameRanges()
		e.Replace(e.Caret()-1, e.Caret(), "")
		e.ensureLayout()
		e.nameRanges()
	}
}

func BenchmarkNameAt(b *testing.B) {
	e := benchEditor(chapter(120, 100), true)
	b.Logf("spans %d lines %d", len(e.nameRanges()), len(e.lay.lines))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.nameAt(300, 300)
	}
}

func BenchmarkNameRangesFilterOnly(b *testing.B) {
	e := benchEditor(chapter(120, 100), true)
	e.nameRanges()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.nameRanges()
	}
}

func frameBench(b *testing.B, text string) {
	e := New(NewDoc(text), Options{Prose: true})
	e.MoveTo(e.Doc.Len()/2, false)
	tt := ui.NewTester(func(c *ui.Context) { e.View(c).Fill() }, 900, 700)
	tt.Frame()
	tt.Frame()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tt.Frame()
	}
}

func BenchmarkFrameShortParas(b *testing.B) { frameBench(b, chapter(120, 100)) }
func BenchmarkFrameLongPara(b *testing.B)   { frameBench(b, chapter(12000, 1)) }
func BenchmarkFrameMidPara(b *testing.B)    { frameBench(b, chapter(1500, 8)) }

func BenchmarkFindAll(b *testing.B) {
	e := benchEditor(chapter(120, 100), false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Doc.Find("林晚")
	}
}

// painter calls: ranges for many found matches
func BenchmarkRangesManyFound(b *testing.B) {
	e := benchEditor(chapter(120, 100), false)
	found := e.Doc.Find("的")
	l := &e.lay
	b.Logf("found %d lines %d", len(found), len(l.lines))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, f := range found {
			l.ranges(l.toDisp(f[0], true), l.toDisp(f[1], false), func(x0, x1, y float32) {})
		}
	}
}

func sparse(paras int) string {
	base := "雨下了整整一夜，院子里那口古旧的水井泛着光，檐角的雨线一根接一根地落下来，把整条巷子都洗得发亮，远处传来几声犬吠，又很快被风声吞没，她站了很久。"
	var b strings.Builder
	for i := 0; i < paras; i++ {
		b.WriteString("　　")
		b.WriteString(base)
		b.WriteString("林晚想起沈砚说过的话，")
		b.WriteString(base[:60])
		if i < paras-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func sparseEditor() *Editor {
	e := New(NewDoc(sparse(70)), Options{Prose: true})
	e.width, e.height = 900, 700
	var nl []NameLink
	for _, n := range []string{"林晚", "沈砚", "青石镇", "老井", "周祥", "何主任", "晚晚", "钥匙", "旧城", "雨城"} {
		nl = append(nl, NameLink{n, n})
	}
	e.SetNameLinks(nl, func(string) {})
	e.ensureLayout()
	return e
}

func BenchmarkSparseNameSpansAfterKeystroke(b *testing.B) {
	e := sparseEditor()
	e.MoveTo(e.Doc.Len(), false)
	b.Logf("runes %d spans %d lines %d", e.Doc.Len(), len(e.nameRanges()), len(e.lay.lines))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Insert("字")
		e.ensureLayout()
		e.nameRanges()
		e.Replace(e.Caret()-1, e.Caret(), "")
		e.ensureLayout()
		e.nameRanges()
	}
}

func BenchmarkSparseNameAt(b *testing.B) {
	e := sparseEditor()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.nameAt(300, 300)
	}
}

func BenchmarkSparseKeystrokeNoNames(b *testing.B) {
	e := sparseEditor()
	e.SetNameLinks(nil, nil)
	e.MoveTo(e.Doc.Len(), false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Insert("字")
		e.ensureLayout()
		e.nameRanges()
		e.Replace(e.Caret()-1, e.Caret(), "")
		e.ensureLayout()
		e.nameRanges()
	}
}

func BenchmarkLayoutAfterEnter(b *testing.B) {
	e := benchEditor(chapter(120, 100), false)
	at := e.Doc.Len()/2 - 10
	e.MoveTo(at, false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Insert("\n")
		e.ensureLayout()
		e.Replace(at, at+1, "")
		e.ensureLayout()
	}
}
