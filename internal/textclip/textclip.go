// Package textclip clips text by runes, optionally marking omitted text.
package textclip

import "unicode/utf8"

func First(s string, n int) string {
	r := []rune(s)
	return string(r[:min(n, len(r))])
}

func Last(s string, n int) string {
	r := []rune(s)
	return string(r[max(0, len(r)-n):])
}

// Clip keeps n runes and adds an ellipsis only when text was omitted.
func Clip(s string, n int) string {
	if utf8.RuneCountInString(s) > n {
		return First(s, n) + "…"
	}
	return s
}

// Tail keeps n final runes, prefixing an ellipsis when text was omitted.
func Tail(s string, n int) string {
	if utf8.RuneCountInString(s) > n {
		return "…" + Last(s, n)
	}
	return s
}
