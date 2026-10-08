package desk

import "strings"

// Kinds of a piece of a diff.
const (
	same = iota
	gone // in the old text only
	came // in the new text only
)

type piece struct {
	kind int
	text string
}

// maxCells bounds the table of a character diff inside a paragraph; a
// pair larger than that shows as replaced whole.
const maxCells = 4_000_000

// diffText compares two texts paragraph by paragraph, and the characters
// of a paragraph changed in place, so that a changed word shows as itself.
func diffText(old, new string) []piece {
	a, b := strings.Split(old, "\n"), strings.Split(new, "\n")
	var out []piece
	add := func(kind int, text string) {
		if text == "" {
			return
		}
		if n := len(out) - 1; n >= 0 && out[n].kind == kind {
			out[n].text += text
			return
		}
		out = append(out, piece{kind, text})
	}
	ops := lcs(len(a), len(b), func(i, j int) bool { return a[i] == b[j] })
	var olds, news []string
	flush := func() {
		// Paragraphs replaced one for one compare by character.
		for k := 0; k < max(len(olds), len(news)); k++ {
			switch {
			case k < len(olds) && k < len(news):
				for _, p := range diffRunes(olds[k], news[k]) {
					add(p.kind, p.text)
				}
				add(same, "\n")
			case k < len(olds):
				add(gone, olds[k]+"\n")
			default:
				add(came, news[k]+"\n")
			}
		}
		olds, news = nil, nil
	}
	for _, op := range ops {
		switch op.kind {
		case same:
			flush()
			add(same, a[op.i]+"\n")
		case gone:
			olds = append(olds, a[op.i])
		case came:
			news = append(news, b[op.j])
		}
	}
	flush()
	if n := len(out) - 1; n >= 0 {
		out[n].text = strings.TrimSuffix(out[n].text, "\n")
		if out[n].text == "" {
			out = out[:n]
		}
	}
	return out
}

func diffRunes(old, new string) []piece {
	a, b := []rune(old), []rune(new)
	if len(a)*len(b) > maxCells {
		return []piece{{gone, old}, {came, new}}
	}
	var out []piece
	for _, op := range lcs(len(a), len(b), func(i, j int) bool { return a[i] == b[j] }) {
		var r rune
		if op.kind == came {
			r = b[op.j]
		} else {
			r = a[op.i]
		}
		if n := len(out) - 1; n >= 0 && out[n].kind == op.kind {
			out[n].text += string(r)
		} else {
			out = append(out, piece{op.kind, string(r)})
		}
	}
	return out
}

type step struct{ kind, i, j int }

// lcs returns the edit from n items to m items keeping the longest common
// subsequence, gone before came where both apply.
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
		out = append(out, step{same, k, k})
	}
	i, j := 0, 0
	for i < rows || j < cols {
		switch {
		case i < rows && j < cols && eq(pre+i, pre+j):
			out = append(out, step{same, pre + i, pre + j})
			i++
			j++
		case j == cols || i < rows && t[i+1][j] >= t[i][j+1]:
			out = append(out, step{gone, pre + i, 0})
			i++
		default:
			out = append(out, step{came, 0, pre + j})
			j++
		}
	}
	for k := 0; k < suf; k++ {
		out = append(out, step{same, n - suf + k, m - suf + k})
	}
	return out
}
