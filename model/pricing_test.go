package model

import (
	"math"
	"testing"
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
