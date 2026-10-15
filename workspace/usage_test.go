package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// What the work spends adds up by model, in the order first used, in the
// work's folder; a cost counts only in the model's currency.
func TestSpentAddsUp(t *testing.T) {
	s := testStore(t)
	if got, err := s.Spent(); err != nil || len(got) != 0 {
		t.Fatalf("a new work spent %+v %v", got, err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(s.Spend("deepseek/deepseek-flash", Usage{Input: 1000, CacheRead: 600, Output: 20, Reasoning: 5}, 0.001, "$"))
	must(s.Spend("kimi/kimi-k3", Usage{Input: 50, Output: 5}, 0, ""))
	must(s.Spend("deepseek/deepseek-flash", Usage{Input: 3000, CacheRead: 2000, CacheWrite: 100, Output: 80}, 0.002, "$"))
	must(s.Spend("deepseek/deepseek-flash", Usage{Input: 10}, 5, "¥"))
	got, err := s.Spent()
	must(err)
	want := []Spent{
		{Model: "deepseek/deepseek-flash", Requests: 3, Usage: Usage{Input: 4010, CacheRead: 2600, CacheWrite: 100, Output: 100, Reasoning: 5}, Cost: 0.003, Currency: "$"},
		{Model: "kimi/kimi-k3", Requests: 1, Usage: Usage{Input: 50, Output: 5}},
	}
	if len(got) != 2 || got[1] != want[1] || got[0].Usage != want[0].Usage || got[0].Requests != 3 || got[0].Currency != "$" || got[0].Cost < 0.0029 || got[0].Cost > 0.0031 {
		t.Fatalf("spent %+v", got)
	}
	if _, err := os.Stat(filepath.Join(s.Root(), "usage.json")); err != nil {
		t.Fatal(err)
	}
	if err := s.Spend(" ", Usage{Input: 1}, 0, ""); err == nil {
		t.Fatal("spent on no model")
	}
}

// A hand edit gone wrong is reported with the file and the model.
func TestSpentIsChecked(t *testing.T) {
	for _, c := range []struct{ file, want string }{
		{`{"models": [{"model": "", "requests": 1, "input": 1, "output": 1}]}`, "models[0]: model is empty"},
		{`{"models": [{"model": "a/b", "requests": 1, "input": 1, "output": 1}, {"model": "a/b", "requests": 1, "input": 1, "output": 1}]}`, `models[1]: model "a/b" is listed twice`},
		{`{"models": [{"model": "a/b", "requests": 1, "input": -1, "output": 1}]}`, "models[0]: counts cannot be negative"},
		{`{"models": [{"model": "a/b", "requests": 1, "input": 1, "output": 1, "cost": 1}]}`, "models[0]: cost needs its currency"},
		{`{"models": [{"model": "a/b", "tokens": 1}]}`, `unknown field "tokens"`},
	} {
		s := testStore(t)
		if err := os.WriteFile(filepath.Join(s.Root(), "usage.json"), []byte(c.file), 0644); err != nil {
			t.Fatal(err)
		}
		_, err := s.Spent()
		if err == nil || !strings.Contains(err.Error(), "usage.json") || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v", c.file, err)
		}
		if err := s.Spend("a/b", Usage{Input: 1}, 0, ""); err == nil {
			t.Errorf("%s: spent over it", c.file)
		}
	}
}
