// Package talkbook names book documents for talk tools.
package talkbook

import (
	"slices"
	"strings"
	"unicode"
	"writer/workspace"
)

// Book is the documents of the work in reading order, each named for the
// model: by its title, or with its volume's before it when two share one.
type Book struct {
	Tree   workspace.Tree
	Docs   []*workspace.Node
	Name   map[string]string
	names  []string // each once, in reading order
	byName map[string]*workspace.Node
}

// Of names the nodes of a tree for the model.
func Of(tree workspace.Tree) Book {
	b := Book{Tree: tree, Name: map[string]string{}, byName: map[string]*workspace.Node{}}
	count := map[string]int{}
	tree.Walk(func(n *workspace.Node, _ int) {
		count[n.Title]++
		if !n.Folder {
			b.Docs = append(b.Docs, n)
		}
	})
	tree.Walk(func(n *workspace.Node, _ int) {
		name := n.Title
		if count[name] > 1 && n.Parent != nil && n.Parent.Title != "" {
			name = n.Parent.Title + "/" + name
		}
		b.Name[n.ID] = name
		if _, taken := b.byName[name]; !taken {
			b.byName[name] = n
			b.names = append(b.names, name)
		}
	})
	return b
}

// Find is the document the model names, taking its name, in 「」 or 《》
// or not, or else the one name that is it but for spaces and marks, or
// that begins with it, or that has it. Finding none, or more than one, it
// says so for the model, with the names it may mean.
func (b Book) Find(name string) (*workspace.Node, string) {
	name = strings.Trim(strings.TrimSpace(name), "「」《》")
	if n, ok := b.byName[name]; ok {
		return n, ""
	}
	key := loose(name)
	var same, begins, has []string
	for _, s := range b.names {
		switch k := loose(s); {
		case key == "":
		case k == key:
			same = append(same, s)
		case strings.HasPrefix(k, key):
			begins = append(begins, s)
		case strings.Contains(k, key):
			has = append(has, s)
		}
	}
	for _, found := range [][]string{same, begins, has} {
		switch {
		case len(found) == 1:
			return b.byName[found[0]], ""
		case len(found) > 1:
			return nil, "「" + name + "」对得上好几章：" + Quoted(found[:min(len(found), 5)]) + "；写全章名。"
		}
	}
	if near := b.near(key); len(near) > 0 {
		return nil, "没有叫「" + name + "」的章；相近的有" + Quoted(near) + "。"
	}
	return nil, "没有叫「" + name + "」的章；章名要和搜全书的结果里写的一样。"
}

// Named is the name of the document the model names, or what it said
// where that finds none.
func (b Book) Named(name string) string {
	if n, _ := b.Find(name); n != nil {
		return b.Name[n.ID]
	}
	return strings.TrimSpace(name)
}

// near is up to five names that share a run of two characters or more
// with key, the longest run first.
func (b Book) near(key string) []string {
	type close struct {
		Name string
		run  int
	}
	var found []close
	for _, s := range b.names {
		if run := common([]rune(loose(s)), []rune(key)); run >= 2 {
			found = append(found, close{s, run})
		}
	}
	slices.SortStableFunc(found, func(x, y close) int { return y.run - x.run })
	var out []string
	for _, c := range found[:min(len(found), 5)] {
		out = append(out, c.Name)
	}
	return out
}

// loose is s without its spaces and marks, as a model may give a name in
// other words.
func loose(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			return -1
		}
		return unicode.ToLower(r)
	}, s)
}

// common is how long the longest run is that a and b share.
func common(a, b []rune) int {
	best, prev := 0, make([]int, len(b)+1)
	for _, x := range a {
		cur := make([]int, len(b)+1)
		for k, y := range b {
			if x == y {
				cur[k+1] = prev[k] + 1
				best = max(best, cur[k+1])
			}
		}
		prev = cur
	}
	return best
}

// Quoted wraps each name in 「」.
func Quoted(names []string) string {
	var b strings.Builder
	for _, n := range names {
		b.WriteString("「" + strings.TrimSpace(n) + "」")
	}
	return b.String()
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }
