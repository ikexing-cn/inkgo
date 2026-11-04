package desk

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	talkmod "writer/ext/talk"
	"writer/ext/talk/talkapi"

	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/workspace"
)

func TestManyMarginNotesReuseResolvedQuotes(t *testing.T) {
	var body strings.Builder
	var findings []workspace.Finding
	for i := range 120 {
		quote := fmt.Sprintf("雨夜第%d盏路灯", i)
		fmt.Fprintln(&body, quote+strings.Repeat("她沿着亮着的灯往旧城走，雨一直没有停。", 6))
		if i%2 == 0 {
			findings = append(findings, workspace.Finding{ID: quote, Kind: workspace.FindNew, Name: quote, Quote: quote, Title: quote, Category: "物品", Text: "街道上的灯"})
		}
	}
	a, tt, node := writingFixture(t, body.String())
	if shownEditor(a).Doc.Len() <= 10000 {
		t.Fatal("fixture must exceed ten thousand characters")
	}
	if err := a.book.SaveCheck(node, workspace.Check{Of: workspace.ProseHash(body.String()), At: time.Now(), Findings: findings}); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(a.marginNotes()) != 60 {
		t.Fatal("missing margin notes")
	}
	before := cacheBuilds(a)
	for range 10 {
		tt.Frame()
	}
	if cacheBuilds(a) != before || tt.FramePending() {
		t.Fatal("unchanged frames searched long prose or rebuilt annotations")
	}
	prose := a.LastProse
	tt.Scroll(prose.X+prose.W/2, prose.Y+prose.H/2, 0, 1200)
	marginPositions(t, a, tt)
	if cacheBuilds(a) != before {
		t.Fatal("scrolling searched long prose again")
	}
}

// Typing in a chapter above ten thousand characters retains every
// unchanged paragraph's measured glyphs and line breaks. Include the whole
// editing frame, rather than timing a cache lookup in isolation.
func TestPhonePreviewTypingCost(t *testing.T) {
	var body strings.Builder
	for i := 0; i < 120; i++ {
		fmt.Fprintf(&body, "第%d夜，%s\n", i, phoneLongText)
	}
	a, tt, _ := writingFixture(t, body.String())
	// The general writing fixture saves immediately for disk assertions;
	// typing here uses the product's paused saving instead.
	a.book.Timing.SaveAfter = book.SaveAfter
	if shownEditor(a).Doc.Len() <= 10000 {
		t.Fatal("fixture must exceed ten thousand characters")
	}
	extensionHost{a}.Show("phone.preview")
	tt.Frame()
	ed := shownEditor(a)
	ed.Select(ed.Doc.Len(), ed.Doc.Len())
	ed.Focus()
	tt.Frame()
	var samples []time.Duration
	for range 10 {
		start := time.Now()
		tt.Type("雨")
		samples = append(samples, time.Since(start))
	}
	slices.Sort(samples)
	median := (samples[4] + samples[5]) / 2
	t.Logf("%d runes, median typing frame %v", ed.Doc.Len(), median)
	if median > 25*time.Millisecond {
		t.Fatalf("phone typing median %v exceeds 25 ms", median)
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
	s, err := workspace.Create(filepath.Join(root, "work"), "测试作品", "网文", "zh-Hans")
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
	tt := stillTester(func(c *ui.Context) { sv = c.Services(); a.View(c) }, 1266, 780)
	tt.Frame()
	if err = tt.Click("讨论"); err != nil {
		b.Fatal(err)
	}
	tt.Frame()
	a.busy, talkOf(a).Turn.Active, talkOf(a).Turn.Session = "AI 正在思考", true, talkOf(a).Session
	talkOf(a).Turn.Live = talkmod.Live{}
	reply := []rune(longReply(99) + longReply(100))
	tt.Frame()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		at := i % (len(reply) / 2)
		if at == 0 {
			// Start halfway, as a long reply is most of the time.
			talkOf(a).Turn.Live = talkmod.Live{}
			if reasoning {
				talkOf(a).Turn.Live.Add(talkapi.Progress{Reasoning: string(reply[:len(reply)/2])})
			} else {
				talkOf(a).Turn.Live.Add(talkapi.Progress{Reasoning: strings.Repeat("先想想雨的来历。", 40), Text: string(reply[:len(reply)/2])})
			}
		}
		if reasoning {
			talkOf(a).Turn.Live.Add(talkapi.Progress{Reasoning: string(reply[at : at+1])})
		} else {
			talkOf(a).Turn.Live.Add(talkapi.Progress{Text: string(reply[len(reply)/2+at : len(reply)/2+at+1])})
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
