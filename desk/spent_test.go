package desk

import (
	"testing"

	"writer/workspace"
)

// The AI's settings show what the work spent on each model, read as they
// open: the tokens, the share read from the cache and the cost estimated,
// or that a model has no price. There is nothing to clear it with.
func TestTheWorkShowsWhatItSpent(t *testing.T) {
	a, tt, _ := fixture(t)
	a.openPrefs("AI")
	tt.Frame()
	if !tt.HasText("本作品用量") || !tt.HasText("按内置价格估算，以服务商账单为准。") || !tt.HasText("尚未使用 AI。") {
		t.Fatal("nothing spent yet", tt.Texts())
	}
	for _, s := range []struct {
		model string
		u     workspace.Usage
		cost  float64
		cur   string
	}{
		{"anthropic/claude-opus-5-5", workspace.Usage{Input: 1_682_400, CacheRead: 1_211_300, Output: 59_200}, 4.444, "$"},
		{"deepseek/deepseek-flash", workspace.Usage{Input: 142_000, CacheRead: 85_200, Output: 4100}, 0.021, "$"},
		{"local/qwen3", workspace.Usage{Input: 900, Output: 30}, 0, ""},
	} {
		if err := a.Store.Spend(s.model, s.u, s.cost, s.cur); err != nil {
			t.Fatal(err)
		}
	}
	tt.Frame()
	if !tt.HasText("尚未使用 AI。") {
		t.Fatal("read again each frame")
	}
	a.bk.prefs = false
	a.openPrefs("AI")
	tt.Frame()
	for _, s := range []string{
		"claude-opus-5-5", "输入 1.7M，缓存命中 72% · 输出 59.2K", "约 $4.44",
		"deepseek-flash", "输入 142K，缓存命中 60% · 输出 4.1K", "约 $0.02",
		"qwen3", "输入 900，缓存命中 0% · 输出 30", "无价格",
	} {
		if !tt.HasText(s) {
			t.Fatal(s, tt.Texts())
		}
	}
	if tt.HasText("尚未使用 AI。") || tt.HasText("清零") {
		t.Fatal(tt.Texts())
	}
}
