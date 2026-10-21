package discussion

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"writer/internal/strictjson"
)

// Locate finds quote in text verbatim or, failing that, ignoring spaces
// and the marks a model drops or adds copying words, and returns the
// text's own words for it.
func Locate(text, quote string) (string, bool) { return locate(text, quote) }

// Decode reads exactly one strict JSON value from a model's answer,
// allowing a ```json fence.
func Decode(s string, v any) error { return decode(s, v) }

// decode reads exactly one strict JSON value, allowing a ```json fence.
func decode(s string, v any) error {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json\n") && strings.HasSuffix(s, "```") {
		s = strings.TrimSuffix(strings.TrimPrefix(s, "```json\n"), "```")
	}
	if err := strictjson.Decode([]byte(s), v); err != nil {
		if errors.Is(err, strictjson.ErrTrailing) {
			return errors.New("返回了多个 JSON 值或尾随内容")
		}
		return fmt.Errorf("JSON 不符合约定：%w", err)
	}
	return nil
}

// copyMarks are what a model drops or adds copying words: spaces, Markdown
// marks and quotation marks.
const copyMarks = "*_`#>\"'“”‘’「」"

// locate finds quote in text verbatim or, failing that, ignoring copyMarks,
// and returns the text's own words for it.
func locate(text, quote string) (string, bool) {
	skip := func(r rune) bool { return unicode.IsSpace(r) || strings.ContainsRune(copyMarks, r) }
	q := []rune(strings.Map(func(r rune) rune {
		if skip(r) {
			return -1
		}
		return r
	}, quote))
	if len(q) == 0 {
		return "", false // no words to find
	}
	if strings.Contains(text, quote) {
		return quote, true
	}
	var kept []rune
	var at []int // the byte offset in text of each rune kept
	for i, r := range text {
		if !skip(r) {
			kept, at = append(kept, r), append(at, i)
		}
	}
	for i := 0; i+len(q) <= len(kept); i++ {
		if slices.Equal(kept[i:i+len(q)], q) {
			last := i + len(q) - 1
			return text[at[i] : at[last]+utf8.RuneLen(kept[last])], true
		}
	}
	return "", false
}
