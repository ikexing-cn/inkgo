package workspace

import (
	"strings"
	"testing"
	"unicode"
)

// proseText is novel-style Chinese: dialogue quotes, commas, ellipses, em dashes, ！？ and ： .
var proseText = strings.Repeat("“你还是走吧。”她轻声说，声音像被雨打湿了一样。\n他沉默了很久——久到窗外的灯一盏一盏地灭了……“我不走。”\n‘你总是这样。’她叹了口气，把伞递了过去：“拿着，别淋湿了。”\n“真的？！”他抬起头，眼里有光。\n", 150)

func BenchmarkCountChapter(b *testing.B) {
	for _, tc := range []struct {
		name, text string
		lang       Language
	}{{"Chinese", strings.Repeat("雨下了一整夜，林晚站在屋檐下望着远处的灯光。\n", 400), Chinese}, {"ChineseProse", proseText, Chinese}, {"Mixed", strings.Repeat("雨下了一整夜，林晚站在屋檐下望着远处的灯光。\n雨下了一整夜，林晚站在屋檐下望着远处的灯光。\n她笑了👍🏽，转身走进雨里。\n", 150), Chinese}, {"English", strings.Repeat("The rain fell all night, and she waited by the window.\n", 200), English}} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				tc.lang.Count(tc.text)
			}
		})
	}
}

// The rune-based implementation before the grapheme migration, retained only
// here as the performance reference for ordinary chapter text.
func countRunesBefore(l Language, s string) int {
	n := 0
	if l != English {
		for _, r := range s {
			if !unicode.IsSpace(r) {
				n++
			}
		}
		return n
	}
	r := []rune(s)
	inside := false
	for i, ch := range r {
		if unicode.In(ch, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
			n++
			inside = false
			continue
		}
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) {
			if !inside {
				n++
			}
			inside = true
			continue
		}
		if (ch == '\'' || ch == '-') && inside && i+1 < len(r) && (unicode.IsLetter(r[i+1]) || unicode.IsDigit(r[i+1])) {
			continue
		}
		inside = false
	}
	return n
}

func BenchmarkCountChapterRuneReference(b *testing.B) {
	for _, tc := range []struct {
		name, text string
		lang       Language
	}{{"Chinese", strings.Repeat("雨下了一整夜，林晚站在屋檐下望着远处的灯光。\n", 400), Chinese}, {"ChineseProse", proseText, Chinese}, {"Mixed", strings.Repeat("雨下了一整夜，林晚站在屋檐下望着远处的灯光。\n雨下了一整夜，林晚站在屋檐下望着远处的灯光。\n她笑了👍🏽，转身走进雨里。\n", 150), Chinese}, {"English", strings.Repeat("The rain fell all night, and she waited by the window.\n", 200), English}} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				countRunesBefore(tc.lang, tc.text)
			}
		})
	}
}
