package workspace

import (
	"strings"
	"unicode/utf8"
)

// Placement spans runes [A, B). Keep leaves those words in place; Apart
// displays Text below them. The host supplies the proposal's display ID.
type Placement struct {
	A, B  int
	Keep  bool
	Apart bool
	Text  string
}

// Place is where p goes in text, as the editor shows it: in place of the
// words it replaces, after those it follows, as paragraphs after the last,
// or below the whole it replaces. It is nowhere once those words are gone,
// or, for the whole, once the text changed.
// The returned quote offset keeps near when no quoted words were found.
func (p Proposal) Place(text, base string, near int) (Placement, int, bool) {
	n := utf8.RuneCountInString(text)
	at := Placement{Text: p.Text}
	switch p.At {
	case ProposeEnd:
		at.A, at.B, at.Keep = n, n, true
		if n > 0 && !strings.HasSuffix(text, "\n") {
			at.Text = "\n" + p.Text
		}
		return at, near, true
	case ProposeAll:
		at.B, at.Apart = n, n > 0
		return at, near, text == base
	}
	from, ok := findQuote(p.Quote, text, near)
	if !ok {
		return at, near, false
	}
	at.A, at.B = from, from+utf8.RuneCountInString(p.Quote)
	if p.At == ProposeAfter {
		at.A, at.Keep = at.B, true
	}
	return at, from, true
}

// findQuote is where the words quoted are in text, in runes: of several, the
// nearest to where they were last found.
func findQuote(quote, text string, near int) (int, bool) {
	if quote == "" {
		return 0, false
	}
	best, found := 0, false
	for from := 0; ; {
		i := strings.Index(text[from:], quote)
		if i < 0 {
			break
		}
		at := utf8.RuneCountInString(text[:from+i])
		if !found || abs(at-near) < abs(best-near) {
			best, found = at, true
		}
		from += i + len(quote)
	}
	return best, found
}
