package editor

import (
	"strings"
	"testing"
)

// Each 50-rune paragraph has three names and two nonoverlapping hits.
func proseCostEditor() *Editor {
	text := strings.Repeat("Mara rain Mara rain Mara "+strings.Repeat("x", 24)+"\n", 100)
	e := benchEditor(text, false)
	e.SetNameLinks([]NameLink{{Name: "Mara", Entry: "mara"}}, nil)
	var hits [][2]int
	for i := 0; i < 100; i++ {
		hits = append(hits, [2]int{i*50 + 5, i*50 + 9}, [2]int{i*50 + 15, i*50 + 19})
	}
	e.SetFound(hits, 0)
	e.SetProseMarks([]ProseMark{{Kind: MarkAI, Start: 30, End: 34}})
	return e
}

func BenchmarkProseNamesAndFind(b *testing.B) {
	for _, changing := range []bool{false, true} {
		name := "steady"
		if changing {
			name = "selection-change"
		}
		b.Run(name, func(b *testing.B) {
			e := proseCostEditor()
			e.rebuildNames()
			if e.Doc.Len() != 5000 || len(e.nameSpans) != 300 || len(e.found) != 200 {
				b.Fatal(e.Doc.Len(), len(e.nameSpans), len(e.found))
			}
			frame := func() {
				e.proseLayers()
				e.decorations()
				e.visibleNames(0, e.lay.height, func(nameBox) {})
			}
			frame()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if changing {
					e.Select(i%2, i%2+1)
				}
				frame()
			}
		})
	}
}
