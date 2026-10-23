package kit

import (
	"fmt"
	"strconv"
	"strings"

	"writer/internal/textclip"
	"writer/workspace"
)

func Clip(s string, n int) string {
	s, _, _ = strings.Cut(s, "\n")
	return textclip.Clip(s, n)
}

// fromLabel says where a change of the setting starts, as the panel does:
// 第87章起 for a chapter numbered, its title for one that is not and for a
// volume, and 所在章节已删除 for a node no longer in the tree.
func FromLabel(tree workspace.Tree, id string) string {
	n, ok := tree.Node(id)
	if !ok {
		return "所在章节已删除"
	}
	if m := workspace.TitleNumber(strings.TrimSpace(n.Title)); m != "" && !n.Folder {
		return m + "起"
	}
	return n.Title + "起"
}

// words says how many characters, the way the design writes them: grouped
// by thousands, and in 万 from ten thousand on.
func Words(n int) string {
	if n >= 10000 {
		return strings.TrimSuffix(fmt.Sprintf("%.1f", float64(n)/10000), ".0") + " 万字"
	}
	return Grouped(n) + " 字"
}

// grouped writes n with a comma between each three digits.
func Grouped(n int) string {
	if n < 0 {
		return "-" + Grouped(-n)
	}
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// signed is grouped with its sign.
func Signed(n int) string {
	if n < 0 {
		return Grouped(n)
	}
	return "+" + Grouped(n)
}

// count is how many a menu's row leads to, at its right, when any.
func Count(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}

// nearFind is text from a little before find in it, so that a line cut
// short still shows what was found.
func NearFind(text, find string) string {
	text = strings.Join(strings.Fields(text), " ")
	i := strings.Index(text, find)
	if find == "" || i < 0 {
		return text
	}
	before := []rune(text[:i])
	if len(before) <= 8 {
		return text
	}
	return "…" + textclip.Last(text[:i], 6) + text[i:]
}

func Trim(s string) string { return strings.TrimSpace(s) }

// heading is a chapter's title over its prose: the name after 第几章, which
// the breadcrumb above already says.
func Heading(title string) string {
	if _, name, ok := strings.Cut(title, "："); ok && strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return title
}
