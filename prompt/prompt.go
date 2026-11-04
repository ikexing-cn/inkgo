// Package prompt lays out what a request sends within the room its model
// gives it, and records what each part took. The talk and the writing
// share it, so a model reads the setting the same way whatever it is
// asked, and every request is measured against its model's limits.
package prompt

import (
	"crypto/sha256"
	"encoding/hex"
	"unicode/utf8"

	"writer/workspace"
)

// Estimate approximates tokens without a provider tokenizer. Chinese uses
// three ASCII characters per token, English four; non-ASCII characters
// retain the Chinese margin of 1.5 tokens each. Every part adds 16 tokens.
// The language is required; the result is never an exact count.
func Estimate(s string, language workspace.Language) int {
	ascii, other := 0, 0
	for _, r := range s {
		if r < utf8.RuneSelf {
			ascii++
		} else {
			other++
		}
	}
	return estimate(ascii, other, language)
}

// CutMark ends what was cut to fit.
const CutMark = "……（后面太长，略去）"

// ClipTokens cuts s from its end to at most tokens, as estimated. If even
// the marker does not fit, it still returns the marker, as the talk expects.
func ClipTokens(s string, tokens int, language workspace.Language) string {
	r := []rune(s)
	ascii, other, cut := 0, 0, 0
	// CutMark contains only non-ASCII runes.
	mark := utf8.RuneCountInString(CutMark)
	for i, ch := range r {
		if ch < utf8.RuneSelf {
			ascii++
		} else {
			other++
		}
		if estimate(ascii, other+mark, language) <= tokens {
			cut = i + 1
		}
	}
	if estimate(ascii, other, language) <= tokens {
		return s
	}
	return string(r[:cut]) + CutMark
}

// slack is what headings and the overhead of each message may take
// besides the parts.
const slack = 512

// Budget is the room of one request, as estimated, and what each of its
// parts took of it.
type Budget struct {
	language workspace.Language
	left     int
	taken    map[string]int
}

// NewBudget is a budget of room tokens, a model's input.
func NewBudget(room int, language workspace.Language) *Budget {
	return &Budget{language: language, left: room - slack, taken: map[string]int{}}
}

// Left is the room not yet taken; below 0 once what must go is too much.
func (b *Budget) Left() int { return b.left }

// Take charges text to part if it fits, and tells whether it did.
func (b *Budget) Take(part, text string) bool {
	n := Estimate(text, b.language)
	if n > b.left {
		return false
	}
	b.left -= n
	b.taken[part] += n
	return true
}

// Must charges text to part, fitting or not, and tells whether the
// request still fits.
func (b *Budget) Must(part, text string) bool {
	n := Estimate(text, b.language)
	b.left -= n
	b.taken[part] += n
	return b.left >= 0
}

// Return gives back what part took.
func (b *Budget) Return(part string) {
	b.left += b.taken[part]
	delete(b.taken, part)
}

// Size is what a request of the parts s records takes, its headings and
// the overhead of its messages included.
func Size(s *workspace.Sent) int {
	n := slack
	for _, p := range s.Parts {
		n += p.Tokens
	}
	return n
}

// Sent records the parts taken, in their order, with the rules the
// request had and the revision of set it read.
func (b *Budget) Sent(rules string, set workspace.Setting) *workspace.Sent {
	sum := sha256.Sum256([]byte(rules))
	out := &workspace.Sent{Rules: hex.EncodeToString(sum[:4]), Setting: len(set.Revisions)}
	for _, name := range workspace.Parts {
		if n := b.taken[name]; n > 0 {
			out.Parts = append(out.Parts, workspace.Part{Name: name, Tokens: n})
		}
	}
	return out
}

// estimate retains the non-ASCII margin for mixed text and Chinese rules.
func estimate(ascii, other int, language workspace.Language) int {
	// OpenAI's tokenizer rule of thumb is about four English characters per token.
	// https://help.openai.com/en/articles/4936856-what-are-tokens-and-how-to-count-them
	ratio := language.ASCIICharsPerToken()
	return (ascii+ratio-1)/ratio + (other*3+1)/2 + 16
}

// Clip cuts text using this budget's book language and token estimate.
func (b *Budget) Clip(text string, tokens int) string { return ClipTokens(text, tokens, b.language) }
