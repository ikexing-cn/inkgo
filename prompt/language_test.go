package prompt

import (
	"strings"
	"testing"
)

func TestEnglishBookTokenEstimatesUseFourCharactersPerToken(t *testing.T) {
	text := strings.Repeat("a", 1000)
	if Estimate(text, "en") != 266 || Estimate(text, "zh-Hans") != 350 {
		t.Fatal(Estimate(text, "en"), Estimate(text, "zh-Hans"))
	}
	if Estimate("中", "en") != Estimate("中", "zh-Hans") {
		t.Fatal("mixed-script margin changed")
	}
	b := NewBudget(2000, "en")
	b.Must("text", text)
	if b.Left() != 2000-512-266 {
		t.Fatal(b.Left())
	}
	cut := ClipTokens(text, 60, "en")
	if Estimate(cut, "en") > 60 {
		t.Fatal(Estimate(cut, "en"))
	}
}
