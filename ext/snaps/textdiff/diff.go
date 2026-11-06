package textdiff

import (
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// Kinds of a piece of a diff.
const (
	Same    = iota
	Removed // in the old text only
	Added   // in the new text only
)

type Piece struct {
	Kind int
	Text string
}

// Compare compares two texts paragraph by paragraph, and the characters
// of a paragraph changed in place, so that a changed word shows as itself.
func Compare(old, new string) []Piece {
	dmp := diffmatchpatch.New() // Keep the default one-second timeout per diff.
	// Include the final (possibly empty) paragraph, just as strings.Split did.
	a, b, lines := dmp.DiffLinesToRunes(old+"\n", new+"\n")
	ops := dmp.DiffCharsToLines(dmp.DiffMainRunes(a, b, false), lines)
	var out []Piece
	add := func(kind int, text string) {
		if text == "" {
			return
		}
		if n := len(out) - 1; n >= 0 && out[n].Kind == kind {
			out[n].Text += text
			return
		}
		out = append(out, Piece{kind, text})
	}
	var olds, news []string
	flush := func() {
		// Paragraphs replaced one for one compare by character.
		for k := 0; k < max(len(olds), len(news)); k++ {
			switch {
			case k < len(olds) && k < len(news):
				diffs := dmp.DiffMainRunes([]rune(olds[k]), []rune(news[k]), false)
				for _, d := range dmp.DiffCleanupSemantic(diffs) {
					kind := Same
					switch d.Type {
					case diffmatchpatch.DiffDelete:
						kind = Removed
					case diffmatchpatch.DiffInsert:
						kind = Added
					}
					add(kind, d.Text)
				}
				add(Same, "\n")
			case k < len(olds):
				add(Removed, olds[k]+"\n")
			default:
				add(Added, news[k]+"\n")
			}
		}
		olds, news = nil, nil
	}
	for _, op := range ops {
		switch op.Type {
		case diffmatchpatch.DiffEqual:
			flush()
			add(Same, op.Text)
		case diffmatchpatch.DiffDelete:
			olds = append(olds, strings.Split(strings.TrimSuffix(op.Text, "\n"), "\n")...)
		case diffmatchpatch.DiffInsert:
			news = append(news, strings.Split(strings.TrimSuffix(op.Text, "\n"), "\n")...)
		}
	}
	flush()
	if n := len(out) - 1; n >= 0 {
		out[n].Text = strings.TrimSuffix(out[n].Text, "\n")
		if out[n].Text == "" {
			out = out[:n]
		}
	}
	return out
}

// Paragraphs gives the pieces of a text a paragraph at a time: a piece
// running over the end of a line is cut there, and an empty line has none.
func Paragraphs(pieces []Piece) [][]Piece {
	out := [][]Piece{nil}
	for _, p := range pieces {
		for i, line := range strings.Split(p.Text, "\n") {
			if i > 0 {
				out = append(out, nil)
			}
			if line != "" {
				out[len(out)-1] = append(out[len(out)-1], Piece{p.Kind, line})
			}
		}
	}
	return out
}
