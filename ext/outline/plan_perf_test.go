package outline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/workspace"
)

// outlineScale uses the documented disk formats to build a large fixture once.
func outlineScale(t *testing.T, chapters int) (*testHost, *ui.Tester, ui.Services) {
	t.Helper()
	a := newTestHost(t)
	var order []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		File  string `json:"file"`
	}
	var ids []string
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < chapters; i++ {
		id, title := fmt.Sprintf("20261019-0001-%04x", i), fmt.Sprintf("第%04d章", i+1)
		ids = append(ids, id)
		order = append(order, struct {
			ID    string `json:"id"`
			Title string `json:"title"`
			File  string `json:"file"`
		}{id, title, title})
		check(os.WriteFile(filepath.Join(a.Store.Root(), "chapters", title+".txt"), []byte("故事正文。"), 0600))
	}
	data, err := json.Marshal(order)
	check(err)
	check(os.WriteFile(filepath.Join(a.Store.Root(), "chapters", ".order"), data, 0600))
	p := workspace.Plan{Schema: workspace.Schema}
	for i := 0; i < 4; i++ {
		p.Lines = append(p.Lines, workspace.PlotLine{ID: fmt.Sprintf("20261019-0002-%04x", i), Name: fmt.Sprintf("情节线%d", i+1)})
	}
	for i := 0; i < chapters*2; i++ {
		p.Cards = append(p.Cards, workspace.Card{ID: fmt.Sprintf("20261019-0003-%04x", i), Title: fmt.Sprintf("事件%04d：灯罩里留下的名字", i+1), When: fmt.Sprintf("第%d天", i/4), Chapter: ids[i%chapters], Line: p.Lines[i%4].ID})
	}
	data, err = json.Marshal(p)
	check(err)
	check(os.WriteFile(filepath.Join(a.Store.Root(), "ext", "outline", "outline.json"), data, 0600))
	var sv ui.Services
	tt := ui.NewTester(func(c *ui.Context) { sv = c.Services(); a.draw(c) }, 1266, 780)
	tt.Frame()
	if outlineErr(a) != nil {
		t.Fatal(outlineErr(a))
	}
	return a, tt, sv
}

// A median of ten complete refreshes catches regressions without a single
// scheduling pause deciding the result. Initial disk loading is outside it.
func TestOutlineFrameCost(t *testing.T) {
	for _, chapters := range []int{300, 1000} {
		t.Run(fmt.Sprintf("%d", chapters), func(t *testing.T) {
			a, tt, sv := outlineScale(t, chapters)
			for mode, name := range []string{"chapters", "timeline"} {
				t.Run(name, func(t *testing.T) {
					a.plan.mode = mode
					sv.Invalidate()
					tt.Frame() // warm layout and glyph caches
					var samples []time.Duration
					for i := 0; i < 10; i++ {
						// Include hover changes, as a pointer moves over the cards.
						tt.Move(a.plan.viewport.X+planChapterWidth+40+float32(i), a.plan.viewport.Y+40)
						sv.Invalidate()
						start := time.Now()
						tt.Advance(time.Second / 60)
						samples = append(samples, time.Since(start))
					}
					slices.Sort(samples)
					median := (samples[4] + samples[5]) / 2
					t.Logf("%d chapters, %d cards, 4 lines, median of 10: %v", chapters, chapters*2, median)
					if median > 25*time.Millisecond {
						t.Fatalf("outline frame median %v exceeds 25 ms", median)
					}
					if len(a.plan.index.chapters) != chapters || len(a.plan.index.cards) != chapters || len(a.plan.index.columns) != chapters/2 {
						t.Fatal("incomplete outline index")
					}
				})
			}
		})
	}
}
