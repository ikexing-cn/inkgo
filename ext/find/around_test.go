package find

import "testing"

func TestAroundKeepsALittleOnEachSide(t *testing.T) {
	if got := around("一二三四五林晚六七八九十", "林晚", 2); got != "…四五林晚六七…" {
		t.Fatal(got)
	}
	if got := around("林晚\n\n走了", "林晚", 5); got != "林晚 走了" {
		t.Fatal(got)
	}
}
func TestAroundMeasure(t *testing.T) {
	if got := around("甲乙ab\n关键\ncd丙丁", "关键", 3); got != "…ab 关键 cd…" {
		t.Fatalf("around = %q", got)
	}
	if got := around("甲乙", "无", 3); got != "" {
		t.Fatalf("missing match = %q", got)
	}
}
