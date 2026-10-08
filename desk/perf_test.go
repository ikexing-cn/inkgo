package desk

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"writer/model"
	"writer/workspace"
)

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
	a := NewWithConfig(filepath.Join(root, "providers.yaml"))
	if err = a.Open(filepath.Join(root, "work")); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { a.Close() })
	tt := ui.NewTester(a.View, 1266, 780)
	tt.Frame()
	a.busy, a.talking = "AI 正在思考", true
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
				a.live.add(model.Delta{Reasoning: string(reply[:len(reply)/2])})
			} else {
				a.live.add(model.Delta{Reasoning: strings.Repeat("先想想雨的来历。", 40), Text: string(reply[:len(reply)/2])})
			}
		}
		if reasoning {
			a.live.add(model.Delta{Reasoning: string(reply[at : at+1])})
		} else {
			a.live.add(model.Delta{Text: string(reply[len(reply)/2+at : len(reply)/2+at+1])})
		}
		tt.Frame()
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
