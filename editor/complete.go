package editor

import (
	"context"
	"strings"
	"time"
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

// DemoPredictor stands in for a model: it waits Delay, then returns one of
// a few fixed continuations, so the inline prediction can be tried
// without one.
type DemoPredictor struct{ Delay time.Duration }

var (
	demoSentences = []string{
		"远处传来一声犬吠，又很快被风声吞没。",
		"她没有回头，只是把手里的东西攥得更紧了些。",
		"灯芯噼啪一响，屋里的影子跟着晃了晃。",
	}
	demoClauses = []string{
		"，像是早就料到会有这么一天。",
		"，却始终没有说出口。",
		"，声音低得几乎听不见。",
	}
)

func (d DemoPredictor) Predict(ctx context.Context, before string) (string, error) {
	select {
	case <-time.After(d.Delay):
	case <-ctx.Done():
		return "", ctx.Err()
	}
	rs := []rune(before)
	if len(rs) == 0 {
		return demoSentences[0], nil
	}
	i := len(rs) % 3
	if strings.ContainsRune("。！？…”」", rs[len(rs)-1]) {
		return demoSentences[i], nil
	}
	return demoClauses[i], nil
}
