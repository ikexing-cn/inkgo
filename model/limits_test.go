package model

import (
	"testing"
)

func TestBuiltinLimits(t *testing.T) {
	if m := Builtin("deepseek-flash"); m != (Model{"deepseek-flash", 1_048_576, 393_216, 80}) || m.Input() != 655_360 {
		t.Fatalf("deepseek-flash = %+v", m)
	}
	if m := Builtin("qwen3"); m != (Model{"qwen3", 32_000, 4096, 80}) {
		t.Fatalf("a model the table does not know = %+v", m)
	}
	for _, p := range embeddedDirectory {
		for id := range p.Models {
			if err := validateModels([]Model{Builtin(id)}); err != nil {
				t.Errorf("%s: %v", id, err)
			}
		}
	}
	cfg := DefaultConfig()
	cfg.Providers[0].Models[0].Window = 64_000
	if got := cfg.Model("deepseek", "deepseek-flash").Window; got != 64_000 {
		t.Fatalf("configured window = %d", got)
	}
	if got := cfg.Model("kimi", "deepseek-flash").Window; got != 1_048_576 {
		t.Fatalf("a model the provider does not list = %d", got)
	}
}
