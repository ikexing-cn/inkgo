package editor

import (
	"testing"
	"writer/shelf"
)

const paginatedLongText = "雨水顺着屋檐往下淌，巷子尽头那盏灯晃了两下，终于灭了。她数着自己的脚步，一步，两步，三步，数到第十七步的时候，听见身后有人喊她的名字，声音很轻，像是怕惊动了什么。她没有回头，只是把手电攥得更紧，指节都发了白。"

func TestPaginatedParagraphCache(t *testing.T) {
	s := &PaginatedText{}
	prefs := shelf.Phone{Size: 20, Spacing: "medium", Mode: "flip"}
	text := paginatedLongText + "\n" + paginatedLongText
	s.Layout(text, "第一章", prefs, 353, 766)
	old := s.Paras[0].Shape
	text += "雨"
	s.Layout(text, "第一章", prefs, 353, 766)
	if s.Paras[0].Shape != old || s.Paras[1].Shape == old {
		t.Fatal("unchanged paragraph not reused, or changed paragraph reused")
	}
	screens := map[string][2]float32{"narrow": {360, 780}, "standard": {393, 852}, "wide": {430, 932}}
	for _, key := range []string{"narrow", "wide", "standard"} {
		s.Layout(text, "第一章", prefs, screens[key][0]-40, screens[key][1]-86)
		if s.TextWidth != screens[key][0]-40 {
			t.Fatal("wrong screen width")
		}
	}
	if s.Paras[0].Shape == old {
		t.Fatal("layout with another width was reused after eviction")
	}
	for _, p := range s.Paras {
		for i, line := range p.Shape.Lines {
			if p.Shape.X(i, line.End) > s.TextWidth+1 {
				t.Fatal("line does not fit the phone")
			}
		}
	}
}
