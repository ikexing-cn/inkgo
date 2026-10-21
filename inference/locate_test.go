package inference

import (
	"strings"
	"testing"
)

func TestLocateForgivesCopyMarks(t *testing.T) {
	text := "**一种是它吃声音。** 那东西一动，周围的“雨声”会先被吸走。"
	for quote, want := range map[string]string{
		"它吃声音": "它吃声音",
		"一种是它吃声音。那东西一动": "一种是它吃声音。** 那东西一动",
		"周围的雨声会先被吸走":    "周围的“雨声”会先被吸走",
	} {
		if got, ok := locate(text, quote); !ok || got != want || !strings.Contains(text, got) {
			t.Errorf("locate(%q) = %q %v, want %q", quote, got, ok, want)
		}
	}
	for _, quote := range []string{"", "  ", "它不吃声音", "**"} {
		if _, ok := locate(text, quote); ok {
			t.Errorf("locate(%q) found", quote)
		}
	}
}
