package workspace

import (
	"fmt"
	"slices"
	"strings"
)

// The prose lives under chapters/ as the chapter tree shows it: a folder is
// a directory, a document 标题.txt, and the outline of either 标题.outline.txt
// beside it. A document's marks are 标题.marks.json. Each directory's .order
// lists its nodes in order with their IDs and titles, which file names
// cannot hold; files that appear without an entry, as ones copied in by
// hand, become nodes of their own.
const (
	chaptersDir = "chapters"
	trashDir    = "trash"
	orderFile   = ".order"
	trashedFile = ".trashed.json"
	textExt     = ".txt"
	outlineExt  = ".outline.txt"
	marksExt    = ".marks.json"
)

// IsProsePath reports whether a slash-separated path under a work is live
// chapter prose, excluding outlines and files outside the chapter tree.
func IsProsePath(rel string) bool {
	return strings.HasPrefix(rel, chaptersDir+"/") && strings.HasSuffix(rel, textExt) && !strings.HasSuffix(rel, outlineExt)
}

// fileName turns title into a name its directory can hold beside the names
// taken: characters file systems refuse become their full-width forms, and
// a name clashing with another, letter case aside, is numbered.
func fileName(title string, taken func(string) bool) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(title) {
		switch {
		case r < 0x20 || r == 0x7f:
		case strings.ContainsRune(`<>:"/\|?*`, r):
			b.WriteRune(map[rune]rune{'<': '＜', '>': '＞', ':': '：', '"': '＂', '/': '／', '\\': '＼', '|': '｜', '?': '？', '*': '＊'}[r])
		default:
			b.WriteRune(r)
		}
	}
	name := strings.TrimRight(b.String(), ". ")
	if r := []rune(name); len(r) > 80 {
		name = strings.TrimRight(string(r[:80]), ". ")
	}
	if strings.HasPrefix(name, ".") {
		name = "．" + name[1:] // hidden files and .order are not nodes
	}
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".outline") || strings.HasSuffix(lower, ".marks") {
		name += "_"
	}
	stem := strings.ToUpper(name)
	if i := strings.IndexByte(stem, '.'); i >= 0 {
		stem = stem[:i]
	}
	switch stem {
	case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		name += "_"
	}
	if name == "" {
		name = "未命名"
	}
	out := name
	for n := 2; taken(out); n++ {
		out = fmt.Sprintf("%s (%d)", name, n)
	}
	return out
}

// takenIn returns whether a name clashes with a node of dir other than
// except, letter case aside, as on Windows and macOS.
func takenIn(dir *Node, except *Node) func(string) bool {
	return func(name string) bool {
		for _, n := range dir.Children {
			if n != except && strings.EqualFold(n.file, name) {
				return true
			}
		}
		return false
	}
}

// diskPath returns the path of parts under chapters/.
func (s *Store) diskPath(parts ...string) (string, error) {
	return s.path(append([]string{chaptersDir}, parts...)...)
}

// nodePath returns the path of n's file with ext; a folder's directory
// with ext "".
func (s *Store) nodePath(n *Node, ext string) (string, error) {
	return s.diskPath(append(slices.Clone(n.dir), n.file+ext)...)
}

// childDir is the directory, under chapters/, of what folder n holds.
func childDir(n *Node) []string {
	if n.Parent == nil {
		return nil
	}
	return append(slices.Clone(n.dir), n.file)
}

// companions are the files of n beside its own, which move with it. A
// .summary.json an older version wrote is not among them: it stays where
// it was.
func companions(n *Node) []string {
	if n.Folder {
		return []string{"", outlineExt}
	}
	return []string{textExt, outlineExt, marksExt, checkExt}
}
