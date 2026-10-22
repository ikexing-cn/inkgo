package desk

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/aiport"
	"writer/workspace"
)

// outlineScale uses the documented disk formats to build a large fixture once.
func outlineScale(t *testing.T, chapters int) (*App, *ui.Tester, ui.Services) {
	t.Helper()
	a, _, _ := fixture(t)
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
	check(os.WriteFile(filepath.Join(a.Store.Root(), "outline.json"), data, 0600))
	a.page = pagePlan
	var sv ui.Services
	tt := ui.NewTester(func(c *ui.Context) { sv = c.Services(); a.View(c) }, 1266, 780)
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

// Typing in a chapter above ten thousand characters retains every
// unchanged paragraph's measured glyphs and line breaks. Include the whole
// editing frame, rather than timing a cache lookup in isolation.
func TestPhonePreviewTypingCost(t *testing.T) {
	pause := saveAfter
	var body strings.Builder
	for i := 0; i < 120; i++ {
		fmt.Fprintf(&body, "第%d夜，%s\n", i, phoneLongText)
	}
	a, tt, _ := writingFixture(t, body.String())
	// The general writing fixture saves immediately for disk assertions;
	// typing here uses the product's paused saving instead.
	saveAfter = pause
	if a.write.ed.Doc.Len() <= 10000 {
		t.Fatal("fixture must exceed ten thousand characters")
	}
	a.openPhone()
	tt.Frame()
	ed := a.write.ed
	ed.Select(ed.Doc.Len(), ed.Doc.Len())
	ed.Focus()
	tt.Frame()
	old := a.phone.Paras[0].Shape
	var samples []time.Duration
	for range 10 {
		start := time.Now()
		tt.Type("雨")
		samples = append(samples, time.Since(start))
	}
	slices.Sort(samples)
	median := (samples[4] + samples[5]) / 2
	t.Logf("%d runes, %d paragraphs, median typing frame %v", ed.Doc.Len(), len(a.phone.Paras), median)
	if median > 25*time.Millisecond {
		t.Fatalf("phone typing median %v exceeds 25 ms", median)
	}
	if old != a.phone.Paras[0].Shape {
		t.Fatal("typing reshaped an unchanged paragraph")
	}
}

// longReply is what a talkative model writes in one turn.
func longReply(n int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## 第 %d 个方向\n\n", n)
	for p := 0; p < 4; p++ {
		b.WriteString("雨从不停，城市里的人把晴天当成传说。**主角**记得没有雨的天空，这让她在每一次对话里都显得格格不入，")
		b.WriteString("而她自己并不知道这份记忆从何而来。也许是转生的残留，也许只是一场梦。\n\n")
	}
	b.WriteString("- 公会把雨当作计时单位\n- 晴天被写进禁书\n- 主角的职业和光有关\n\n")
	b.WriteString("1. 先定雨的来历\n2. 再定主角的记忆\n")
	return b.String()
}

// streamingBench opens a session with turns earlier replies and streams a
// new reply into it, one delta a frame.
func streamingBench(b *testing.B, turns int, reasoning bool) {
	root := b.TempDir()
	s, err := workspace.Create(filepath.Join(root, "work"), "测试作品", "网文")
	if err != nil {
		b.Fatal(err)
	}
	session, err := s.NewSession()
	if err != nil {
		b.Fatal(err)
	}
	for n := 0; n < turns; n++ {
		u, err := s.Append(session, workspace.Event{Role: workspace.RoleUser, Text: "继续说说这个城市"})
		if err != nil {
			b.Fatal(err)
		}
		if _, err = s.Append(session, workspace.Event{Role: workspace.RoleAssistant, ReplyTo: u.ID, Text: longReply(n)}); err != nil {
			b.Fatal(err)
		}
	}
	s.Close()
	a := newApp(filepath.Join(root, "providers.yaml"))
	if err = a.Open(filepath.Join(root, "work")); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { a.Close() })
	var sv ui.Services
	tt := ui.NewTester(func(c *ui.Context) { sv = c.Services(); a.View(c) }, 1266, 780)
	tt.Frame()
	if err = tt.Click("讨论"); err != nil {
		b.Fatal(err)
	}
	tt.Frame()
	a.busy, a.talking, a.talkSession = "AI 正在思考", true, a.session
	a.live.reset()
	reply := []rune(longReply(99) + longReply(100))
	tt.Frame()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		at := i % (len(reply) / 2)
		if at == 0 {
			// Start halfway, as a long reply is most of the time.
			a.live.reset()
			if reasoning {
				a.live.add(aiport.Progress{Reasoning: string(reply[:len(reply)/2])})
			} else {
				a.live.add(aiport.Progress{Reasoning: strings.Repeat("先想想雨的来历。", 40), Text: string(reply[:len(reply)/2])})
			}
		}
		if reasoning {
			a.live.add(aiport.Progress{Reasoning: string(reply[at : at+1])})
		} else {
			a.live.add(aiport.Progress{Text: string(reply[len(reply)/2+at : len(reply)/2+at+1])})
		}
		// One refresh, as a window draws: what fades in asks for the next
		// frame, which a Frame on a clock standing still would run at once,
		// over and over.
		sv.Invalidate()
		tt.Advance(time.Second / 60)
	}
	b.StopTimer()
	if !tt.HasText("正在回复…") && !tt.HasText("正在思考…") {
		b.Fatal("the reply did not stream", tt.Texts())
	}
}

func BenchmarkStreamingFrame(b *testing.B) {
	for _, turns := range []int{0, 20, 60} {
		b.Run(fmt.Sprintf("turns=%d", turns), func(b *testing.B) { streamingBench(b, turns, false) })
	}
	b.Run("reasoning", func(b *testing.B) { streamingBench(b, 20, true) })
}
