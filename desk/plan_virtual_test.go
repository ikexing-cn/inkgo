package desk

import (
	"fmt"
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

func planVisibleCard(t *testing.T, a *App, tt *ui.Tester, id string) {
	t.Helper()
	r, ok := tt.Find("卡片-" + id)
	v := a.plan.viewport
	left := float32(planChapterWidth)
	if a.plan.mode == 1 {
		left = planLaneWidth
	}
	if !ok || r.W <= 0 || r.H <= 0 || r.X < v.X+left || r.X+r.W > v.X+v.W+1 || r.Y < v.Y || r.Y+r.H > v.Y+v.H+1 {
		t.Fatalf("card not visible: %s %+v viewport %+v", id, r, v)
	}
}

func TestOutlineVirtualScrolling(t *testing.T) {
	a, tt, sv := outlineScale(t, 120)
	p := &a.plan
	last := outline(a).Cards[len(outline(a).Cards)-1]
	if _, ok := tt.Find("大纲章节-" + last.Chapter); ok {
		t.Fatal("offscreen chapter was built")
	}
	maxY := p.scroll[0].MaxY
	p.scroll[0].Y = maxY / 2
	sv.Invalidate()
	tt.Frame()
	tt.Frame()
	if p.scroll[0].MaxY != maxY {
		t.Fatal("placeholder changed scroll height", maxY, p.scroll[0].MaxY)
	}
	if _, ok := tt.Find("大纲章节-" + outline(a).Cards[0].Chapter); ok {
		t.Fatal("scrolled-away chapter was built")
	}
	// Search the whole book, including rows which have never been built.
	planType(t, tt, "搜索大纲", "事件0240")
	hasText(t, tt, "找到 1 张卡片")
	if _, ok := tt.Find("大纲章节-" + last.Chapter); !ok {
		t.Fatal("search missed a virtual chapter")
	}
	click(t, tt, "清除搜索")
	a.plan.seePlanCard(last.ID)
	sv.Invalidate()
	tt.Frame()
	tt.Frame()
	planVisibleCard(t, a, tt, last.ID)
	// A new card in a distant row must be built before ScrollIntoView runs.
	click(t, tt, fmt.Sprintf("添加卡片-%s|%s|-2", last.Chapter, last.Line))
	tt.Type("新记在最后一章的事")
	click(t, tt, "保存卡片")
	added, _ := outline(a).Card(p.seen)
	if added.Title != "新记在最后一章的事" {
		t.Fatal(added)
	}
	planVisibleCard(t, a, tt, added.ID)
	// The prose jump also has to create a distant time column first.
	showPage(a, pageWrite)
	a.openNode(last.Chapter)
	sv.Invalidate()
	tt.Frame()
	tt.Frame()
	when := storyWhen(a.plan.cardsOf(last.Chapter))
	click(t, tt, "故事中 "+when)
	if p.mode != 1 {
		t.Fatal("story jump did not select timeline")
	}
	first := a.plan.cardsOf(last.Chapter)[0]
	planVisibleCard(t, a, tt, first.ID)
	if _, ok := tt.Find("时间列-0"); ok {
		t.Fatal("offscreen time header was built")
	}
	a.plan.seePlanCard(last.ID)
	sv.Invalidate()
	tt.Frame()
	tt.Frame()
	planVisibleCard(t, a, tt, last.ID)
	col := -1
	for i, c := range p.index.columns {
		for _, e := range c.Cards {
			if e.ID == last.ID {
				col = i
			}
		}
	}
	head, ok := tt.Find(fmt.Sprintf("时间列-%d", col))
	if !ok {
		t.Fatal("visible column has no header")
	}
	cell, ok := tt.Find(fmt.Sprintf("大纲格-|%s|%d", last.Line, col))
	if !ok || math.Abs(float64(head.X-cell.X-12)) > 1 {
		t.Fatal("header and lane columns diverged", head, cell)
	}
	side, _ := tt.Find("大纲泳道-" + last.Line)
	yExtent := p.scroll[1].MaxY
	p.scroll[1].X = 0
	sv.Invalidate()
	tt.Frame()
	tt.Frame()
	side2, _ := tt.Find("大纲泳道-" + last.Line)
	if side.X != side2.X || yExtent != p.scroll[1].MaxY {
		t.Fatal("horizontal scroll moved sidebar or resized lanes")
	}
	if _, ok := tt.Find(fmt.Sprintf("时间列-%d", col)); ok {
		t.Fatal("scrolled-away time header was built")
	}
	// The bottom border belongs to the whole final row.
	row, ok := tt.Find("新增情节线整行")
	if !ok || row.W < p.viewport.W {
		t.Fatal("new-line row does not span timeline", row)
	}
}

func TestOutlineVirtualDrag(t *testing.T) {
	for mode, name := range []string{"chapters", "timeline"} {
		t.Run(name, func(t *testing.T) {
			a, tt, sv := outlineScale(t, 120)
			p := &a.plan
			p.mode = mode
			sv.Invalidate()
			tt.Frame()
			tt.Frame()
			source := outline(a).Cards[0]
			r, ok := tt.Find("卡片-" + source.ID)
			if !ok {
				t.Fatal("no drag source")
			}
			tt.Press(r.X+r.W/2, r.Y+r.H/2)
			tt.Move(r.X+r.W/2+10, r.Y+r.H/2+10)
			tt.Frame()
			if p.dragging != source.ID {
				t.Fatal("drag did not start")
			}
			var target, wantChapter, wantWhen string
			if mode == 0 {
				wantChapter = outline(a).Cards[119].Chapter
				tt.Scroll(p.viewport.X+p.viewport.W/2, p.viewport.Y+p.viewport.H/2, 0, p.scroll[0].MaxY)
				target = fmt.Sprintf("大纲格-%s|%s|-2", wantChapter, source.Line)
			} else {
				col := len(p.index.columns) - 1
				wantWhen = p.index.columns[col].When
				tt.Scroll(p.viewport.X+p.viewport.W/2, p.viewport.Y+p.viewport.H/2, p.scroll[1].MaxX, 0)
				target = fmt.Sprintf("大纲格-|%s|%d", source.Line, col)
			}
			sv.Invalidate()
			tt.Frame()
			tt.Frame()
			if _, ok := tt.Find("卡片-" + source.ID); !ok {
				t.Fatal("drag source vanished outside virtual range")
			}
			dst, ok := tt.Find(target)
			if !ok {
				t.Fatal("virtual target not built", target)
			}
			x, y := dst.X+dst.W/2, dst.Y+dst.H-12
			tt.Move(x, y)
			tt.Frame()
			tt.Release(x, y)
			tt.Frame()
			tt.Frame()
			got, _ := planStored(t, a).Card(source.ID)
			if mode == 0 && got.Chapter != wantChapter || mode == 1 && got.When != wantWhen {
				t.Fatal("drop into virtual target failed", got, target)
			}
		})
	}
}
