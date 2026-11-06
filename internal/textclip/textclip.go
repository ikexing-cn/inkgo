// Package textclip clips text by grapheme clusters, optionally marking omitted text.
package textclip

import "writer/internal/grapheme"

func First(s string, n int) string { end, _ := grapheme.Prefix(s, n); return s[:end] }
func Last(s string, n int) string  { start, _ := grapheme.Suffix(s, n); return s[start:] }
func Clip(s string, n int) string {
	end, more := grapheme.Prefix(s, n)
	if more {
		return s[:end] + "…"
	}
	return s
}
func Tail(s string, n int) string {
	start, more := grapheme.Suffix(s, n)
	if more {
		return "…" + s[start:]
	}
	return s
}
