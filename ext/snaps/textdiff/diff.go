package textdiff

import "strings"

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

// maxCells bounds the table of a character diff inside a paragraph; a
// pair larger than that shows as replaced whole.
const maxCells = 4_000_000

// Compare compares two texts paragraph by paragraph, and the characters
// of a paragraph changed in place, so that a changed word shows as itself.
func Compare(old, new string) []Piece {
	a, b := strings.Split(old, "\n"), strings.Split(new, "\n")
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
	ops := lcs(len(a), len(b), func(i, j int) bool { return a[i] == b[j] })
	var olds, news []string
	flush := func() {
		// Paragraphs replaced one for one compare by character.
		for k := 0; k < max(len(olds), len(news)); k++ {
			switch {
			case k < len(olds) && k < len(news):
				for _, p := range diffRunes(olds[k], news[k]) {
					add(p.Kind, p.Text)
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
		switch op.kind {
		case Same:
			flush()
			add(Same, a[op.i]+"\n")
		case Removed:
			olds = append(olds, a[op.i])
		case Added:
			news = append(news, b[op.j])
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

func diffRunes(old, new string) []Piece {
	a, b := []rune(old), []rune(new)
	if len(a)*len(b) > maxCells {
		return []Piece{{Removed, old}, {Added, new}}
	}
	var out []Piece
	for _, op := range lcs(len(a), len(b), func(i, j int) bool { return a[i] == b[j] }) {
		var r rune
		if op.kind == Added {
			r = b[op.j]
		} else {
			r = a[op.i]
		}
		if n := len(out) - 1; n >= 0 && out[n].Kind == op.kind {
			out[n].Text += string(r)
		} else {
			out = append(out, Piece{op.kind, string(r)})
		}
	}
	return out
}

type step struct{ kind, i, j int }

// lcs returns the edit from n items to m items keeping the longest common
// subsequence, Removed before Added where both apply.
func lcs(n, m int, eq func(i, j int) bool) []step {
	// Shared ends cost nothing and keep the table small.
	pre := 0
	for pre < n && pre < m && eq(pre, pre) {
		pre++
	}
	suf := 0
	for suf < n-pre && suf < m-pre && eq(n-1-suf, m-1-suf) {
		suf++
	}
	rows, cols := n-pre-suf, m-pre-suf
	t := make([][]int32, rows+1)
	for i := range t {
		t[i] = make([]int32, cols+1)
	}
	for i := rows - 1; i >= 0; i-- {
		for j := cols - 1; j >= 0; j-- {
			if eq(pre+i, pre+j) {
				t[i][j] = t[i+1][j+1] + 1
			} else {
				t[i][j] = max(t[i+1][j], t[i][j+1])
			}
		}
	}
	var out []step
	for k := 0; k < pre; k++ {
		out = append(out, step{Same, k, k})
	}
	i, j := 0, 0
	for i < rows || j < cols {
		switch {
		case i < rows && j < cols && eq(pre+i, pre+j):
			out = append(out, step{Same, pre + i, pre + j})
			i++
			j++
		case j == cols || i < rows && t[i+1][j] >= t[i][j+1]:
			out = append(out, step{Removed, pre + i, 0})
			i++
		default:
			out = append(out, step{Added, 0, pre + j})
			j++
		}
	}
	for k := 0; k < suf; k++ {
		out = append(out, step{Same, n - suf + k, m - suf + k})
	}
	return out
}
