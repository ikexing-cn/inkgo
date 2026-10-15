package model

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestBuiltinLimits(t *testing.T) {
	if m := Builtin("deepseek-flash"); m != (Model{"deepseek-flash", 1_000_000, 384_000, 80}) || m.Input() != 616_000 {
		t.Fatalf("deepseek-flash = %+v", m)
	}
	if m := Builtin("qwen3"); m != (Model{"qwen3", 32_000, 4096, 80}) {
		t.Fatalf("a model the table does not know = %+v", m)
	}
	for id, b := range table {
		if err := validateModels([]Model{Builtin(id)}); err != nil || b.price.Currency == "" || (b.above > 0) != (b.long.Currency != "") {
			t.Errorf("%s: %v %+v", id, err, b)
		}
	}
	cfg := DefaultConfig()
	cfg.Providers[0].Models[0].Window = 64_000
	if got := cfg.Model("deepseek", "deepseek-flash").Window; got != 64_000 {
		t.Fatalf("configured window = %d", got)
	}
	if got := cfg.Model("kimi", "deepseek-flash").Window; got != 1_000_000 {
		t.Fatalf("a model the provider does not list = %d", got)
	}
}

// A request costs its fresh input, its cache reads and writes and its
// output, each at its price; past its threshold, at the dearer prices.
func TestCost(t *testing.T) {
	near := func(got, want float64) bool { return math.Abs(got-want) < 1e-9 }
	// 1M in, 600K of it read from the cache and 100K written to it; 10K out.
	u := Usage{Input: 1_000_000, CacheRead: 600_000, CacheWrite: 100_000, Output: 10_000}
	if c, cur, ok := Cost("deepseek-flash", u); !ok || cur != "$" || !near(c, 0.3*0.3+0.6*0.006+0.1*0.3+0.01*1.2) {
		t.Fatalf("deepseek-flash: %v %q %v", c, cur, ok)
	}
	if c, _, _ := Cost("claude-haiku-5-5", Usage{Input: 100_000, Output: 1000}); !near(c, 0.1*0.1+0.001*0.5) {
		t.Fatalf("haiku at its threshold: %v", c)
	}
	if c, _, _ := Cost("claude-haiku-5-5", Usage{Input: 100_001, Output: 1000}); !near(c, 0.100001*0.5+0.001*2.5) {
		t.Fatalf("haiku past its threshold: %v", c)
	}
	if _, _, ok := Cost("qwen3", u); ok {
		t.Fatal("a model the table does not know has a price")
	}
}

// The limits are written as an author would write them, and read in any
// of the ways an author may.
func TestModelYAML(t *testing.T) {
	data, err := yaml.Marshal([]Model{{"a", 128_000, 8192, 80}, {"b", 1_000_000, 4000, 90}})
	if err != nil {
		t.Fatal(err)
	}
	want := "- id: a\n  context: 128K\n  max_output: 8192\n  compact_at: 80\n- id: b\n  context: 1M\n  max_output: 4K\n  compact_at: 90\n"
	if string(data) != want {
		t.Fatalf("written as\n%s", data)
	}
	var got []Model
	in := "- kimi-k3\n- id: qwen3\n  context: 1.5m\n- id: x\n  context: 131072\n  max_output: 8 k\n  compact_at: 60\n"
	if err := yaml.Unmarshal([]byte(in), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != Builtin("kimi-k3") || got[1] != (Model{"qwen3", 1_500_000, 4096, 80}) || got[2] != (Model{"x", 131072, 8000, 60}) {
		t.Fatalf("read as %+v", got)
	}
	for in, msg := range map[string]string{
		"- id: x\n  context: 一百万\n": "x 的上下文上限和最大输出须为正整数，可写成 128K、1M",
		"- id: x\n  key: secret\n":  "模型只能写 id、context、max_output、compact_at 几项",
	} {
		var m []Model
		if err := yaml.Unmarshal([]byte(in), &m); err == nil || err.Error() != msg {
			t.Errorf("%q: %v", in, err)
		}
	}
}

func TestModelValidation(t *testing.T) {
	for msg, models := range map[string][]Model{
		"x 的最大输出须小于上下文上限":                {{"x", 8000, 8000, 80}},
		"x 的压缩阈值须在 50% 到 95% 之间":         {{"x", 8000, 4000, 96}},
		"x 的上下文上限和最大输出须为正整数，可写成 128K、1M": {{"x", 8000, 0, 80}},
		"模型 x 重复": {Builtin("x"), Builtin("x")},
		"模型 ID 不能为空，前后不能有空格": {Builtin(" x")},
	} {
		if err := validateModels(models); err == nil || err.Error() != msg {
			t.Errorf("%+v: %v", models, err)
		}
	}
	// Load says what is wrong with a model rather than that the YAML is.
	path := filepath.Join(t.TempDir(), "providers.yaml")
	cfg := "schema: 1\nproviders:\n  - id: local\n    name: Local\n    base_url: http://localhost:11434/v1\n    protocol: chat\n    models:\n      - id: qwen3\n        context: 8K\n        max_output: "
	for out, msg := range map[string]string{"8K\n": "qwen3 的最大输出须小于上下文上限", "很多\n": "qwen3 的上下文上限和最大输出须为正整数"} {
		if err := os.WriteFile(path, []byte(cfg+out), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil || !strings.Contains(err.Error(), msg) {
			t.Errorf("Load with max_output %q: %v", out, err)
		}
	}
}
