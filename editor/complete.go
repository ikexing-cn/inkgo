package editor

import (
	"strings"
	"unicode"
)

// Entity is a confirmed name of the canon: a character or a place. Only
// entities complete; completion never invents a name.
type Entity struct {
	Name string
	Kind string // 人物, 地名…
	// Of is the name Name is another name for, as 林晚 for 晚晚.
	Of string
}

// maxCandidates bounds the completions offered at once.
const maxCandidates = 6

// maxNameRunes bounds how far back completion looks for a typed prefix.
const maxNameRunes = 8

// candidates returns the entities whose names start with the longest run
// of letters typed before the caret, and how many runes of the name are
// typed already. A name typed in full proposes nothing.
func candidates(before []rune, entities []Entity) ([]Entity, int) {
	run := 0
	for run < len(before) && run < maxNameRunes && unicode.IsLetter(before[len(before)-1-run]) {
		run++
	}
	for k := run; k >= 1; k-- {
		prefix := string(before[len(before)-k:])
		var out []Entity
		for _, e := range entities {
			if strings.HasPrefix(e.Name, prefix) && len([]rune(e.Name)) > k {
				out = append(out, e)
			}
		}
		if len(out) > 0 {
			return out[:min(len(out), maxCandidates)], k
		}
	}
	return nil, 0
}
