package workspace

import (
	"regexp"
	"strconv"
	"strings"

	"writer/internal/textclip"
)

// Mention is a piece of a digest: words, or an entry it names.
type Mention struct {
	Text       string
	Entry      *Entry
	Start, End int // byte offsets in the input text
}

// entryNumber is how the talk's model names an entry: by its number.
var entryNumber = regexp.MustCompile(`\[(\d+)\]`)

// Mentions splits text at the entries it names by number, each piece of
// them showing the entry by its name, or its first words when it names
// nothing. A number no entry has stays as it is.
func (s Setting) Mentions(text string) []Mention {
	var out []Mention
	at := 0
	for _, m := range entryNumber.FindAllStringSubmatchIndex(text, -1) {
		n, _ := strconv.Atoi(text[m[2]:m[3]])
		if n < 1 || n > len(s.Entries) {
			continue
		}
		if m[0] > at {
			out = append(out, Mention{Text: text[at:m[0]], Start: at, End: m[0]})
		}
		e := s.Entries[n-1]
		words := strings.TrimSpace(e.Text)
		first, _, _ := strings.Cut(words[:strings.IndexAny(words+"。", "。！？；")], "\n")
		name := textclip.Clip(first, 16)
		if len(e.Names) > 0 {
			name = e.Names[0]
		}
		out = append(out, Mention{Text: name, Entry: &e, Start: m[0], End: m[1]})
		at = m[1]
	}
	if at < len(text) {
		out = append(out, Mention{Text: text[at:], Start: at, End: len(text)})
	}
	return out
}
