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

// An entry's number is its place in the setting, from 1: entry n is
// Entries[n-1]. Entries are never removed, so a number stays the entry's.

// EntryNumbers maps entry IDs to their numbers in the setting, not in a
// filtered view.
func (s Setting) EntryNumbers() map[string]int {
	at := map[string]int{}
	for n, e := range s.Entries {
		at[e.ID] = n + 1
	}
	return at
}

// EntryByNumber is the entry numbered n, if there is one.
func (s Setting) EntryByNumber(n int) (Entry, bool) {
	if n < 1 || n > len(s.Entries) {
		return Entry{}, false
	}
	return s.Entries[n-1], true
}

// Mentions splits text at the entries it names by number, each piece of
// them showing the entry by its name, or its first words when it names
// nothing. A number no entry has stays as it is.
func (s Setting) Mentions(text string) []Mention {
	var out []Mention
	at := 0
	for _, m := range entryNumber.FindAllStringSubmatchIndex(text, -1) {
		n, _ := strconv.Atoi(text[m[2]:m[3]])
		e, ok := s.EntryByNumber(n)
		if !ok {
			continue
		}
		if m[0] > at {
			out = append(out, Mention{Text: text[at:m[0]], Start: at, End: m[0]})
		}
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
