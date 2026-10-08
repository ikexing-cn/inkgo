package desk

import (
	"regexp"
	"strconv"
	"strings"

	"writer/workspace"
)

// numbered is a title that starts 第N章, 第N卷 and the like, N in Chinese or
// in digits.
var numbered = regexp.MustCompile(`^第([0-9]+|[零〇一二两三四五六七八九十百千]+)([章卷回节部篇集])`)

// nextTitle is what follows the title before in its numbering — 第四章 名单
// gives 第五章 — or "" when the title has none.
func nextTitle(before string) string {
	m := numbered.FindStringSubmatch(strings.TrimSpace(before))
	if m == nil {
		return ""
	}
	if n, err := strconv.Atoi(m[1]); err == nil {
		return "第" + strconv.Itoa(n+1) + m[2]
	}
	n, ok := fromHan(m[1])
	if !ok {
		return ""
	}
	return "第" + toHan(n+1) + m[2]
}

// titleAfter suggests a title for the new node id from the nearest node of
// its kind before it in the tree's order.
func titleAfter(tree workspace.Tree, id string) string {
	me, ok := tree.Node(id)
	if !ok {
		return ""
	}
	prev := ""
	done := false
	tree.Walk(func(n *workspace.Node, _ int) {
		if done {
			return
		}
		if n.ID == id {
			done = true
			return
		}
		if n.Folder == me.Folder {
			prev = n.Title
		}
	})
	return nextTitle(prev)
}

var hanDigits = map[rune]int{'零': 0, '〇': 0, '一': 1, '二': 2, '两': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9}
var hanUnits = map[rune]int{'十': 10, '百': 100, '千': 1000}

// fromHan reads a Chinese number below ten thousand.
func fromHan(s string) (int, bool) {
	total, digit := 0, -1
	for _, r := range s {
		if d, ok := hanDigits[r]; ok {
			if digit >= 0 && digit != 0 {
				return 0, false // two digits in a row: 一二
			}
			digit = d
			continue
		}
		u := hanUnits[r]
		if digit < 0 {
			if u != 10 || total != 0 {
				return 0, false
			}
			digit = 1 // 十二
		}
		total += digit * u
		digit = -1
	}
	if digit > 0 {
		total += digit
	}
	return total, total > 0
}

// toHan writes n below ten thousand as a chapter would: 十二, 一百零五.
func toHan(n int) string {
	if n <= 0 || n >= 10000 {
		return strconv.Itoa(n)
	}
	digits := []string{"零", "一", "二", "三", "四", "五", "六", "七", "八", "九"}
	units := []string{"千", "百", "十", ""}
	parts := []int{n / 1000, n / 100 % 10, n / 10 % 10, n % 10}
	var b strings.Builder
	zero := false
	for i, d := range parts {
		if d == 0 {
			zero = b.Len() > 0
			continue
		}
		if zero {
			b.WriteString("零")
			zero = false
		}
		if !(d == 1 && i == 2 && b.Len() == 0) { // 十二, not 一十二
			b.WriteString(digits[d])
		}
		b.WriteString(units[i])
	}
	return b.String()
}
