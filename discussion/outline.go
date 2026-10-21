package discussion

import (
	"fmt"
	"strings"

	"writer/internal/textclip"
	"writer/prompt"
	"writer/workspace"
)

// cardChapter is the live chapter of a card, or none for an unplaced card.
func (b book) cardChapter(id string) string {
	if n, ok := b.tree.Node(id); ok && !n.Folder {
		return b.name[id]
	}
	return ""
}

// cardLine is one card in story order, shared by tools and fixed context.
func cardLine(p workspace.Plan, b book, i int) string {
	c := p.Cards[i]
	parts := []string{fmt.Sprintf("#%d", i+1)}
	if when := prompt.OneLine(c.When); when != "" {
		parts = append(parts, "时间："+when)
	}
	parts = append(parts, "发生："+prompt.OneLine(c.Title))
	chapter := "未放进章节"
	if name := b.cardChapter(c.Chapter); name != "" {
		chapter = "「" + prompt.OneLine(name) + "」"
	}
	parts = append(parts, chapter)
	for _, l := range p.Lines {
		if l.ID == c.Line {
			parts = append(parts, "情节线："+prompt.OneLine(l.Name))
			break
		}
	}
	if about := prompt.OneLine(c.About); about != "" {
		parts = append(parts, "说明："+textclip.Clip(about, 200))
	}
	return strings.Join(parts, "｜")
}

// chapterCards keeps the full book's numbering and story order.
func chapterCards(p workspace.Plan, b book, chapter string) []string {
	var rows []string
	for i, c := range p.Cards {
		if c.Chapter == chapter {
			rows = append(rows, cardLine(p, b, i))
		}
	}
	return rows
}

type outlineGroup struct {
	head, none string
	rows       []string
}

func (t *toolbox) readOutline(in input) outcome {
	p, err := t.store.Plan()
	if err != nil {
		return failed("大纲文件读不出来，暂时不能查阅。")
	}
	b, err := t.book()
	if err != nil {
		return failed("读不出章节，暂时不能查阅大纲。")
	}
	var head string
	var groups []outlineGroup
	switch {
	case in.Line != "":
		var names []string
		var matches []workspace.PlotLine
		for _, l := range p.Lines {
			names = append(names, l.Name)
			if l.Name == in.Line {
				matches = append(matches, l)
			}
		}
		if len(matches) == 0 {
			if len(names) == 0 {
				return failed("没有叫「%s」的情节线；大纲还没有情节线。", in.Line)
			}
			return failed("没有叫「%s」的情节线；现有的情节线：%s。", in.Line, quoted(names))
		}
		g := outlineGroup{head: "情节线「" + in.Line + "」", none: "这条线没有卡片。"}
		for i, c := range p.Cards {
			for _, l := range matches {
				if c.Line == l.ID {
					g.rows = append(g.rows, cardLine(p, b, i))
					break
				}
			}
		}
		groups = append(groups, g)
	case in.Chapter != "":
		n, missing := b.find(in.Chapter)
		if n == nil {
			return failed("%s", missing)
		}
		ids := []string{n.ID}
		if n.Folder {
			for _, ch := range b.docs {
				if ch != n && ch.Within(n) {
					ids = append(ids, ch.ID)
				}
			}
		}
		outlines, err := t.store.OutlinesOf(ids...)
		if err != nil {
			return failed("读不出「%s」的纲。", b.name[n.ID])
		}
		outline := func(n *workspace.Node) string {
			kind := "章纲"
			if n.Folder {
				kind = "卷纲"
			}
			text := strings.TrimSpace(outlines[n.ID])
			if text == "" {
				text = "（没有" + kind + "）"
			}
			return "「" + b.name[n.ID] + "」" + kind + "：\n" + text
		}
		text := outline(n)
		if !n.Folder {
			groups = append(groups, outlineGroup{text, "这一章没有卡片。", chapterCards(p, b, n.ID)})
			break
		}
		head = text
		count := 0
		for _, ch := range b.docs {
			if ch == n || !ch.Within(n) {
				continue
			}
			text := outline(ch)
			rows := chapterCards(p, b, ch.ID)
			count += len(rows)
			if len(rows) > 0 || in.From <= 0 {
				groups = append(groups, outlineGroup{text, "这一章没有卡片。", rows})
			}
		}
		if count == 0 {
			head += "\n这一卷没有卡片。"
		}
	case in.Query != "":
		g := outlineGroup{none: "没有含「" + in.Query + "」这个词的卡片。"}
		for i, c := range p.Cards {
			if strings.Contains(c.Title, in.Query) || strings.Contains(c.About, in.Query) || strings.Contains(c.When, in.Query) {
				g.rows = append(g.rows, cardLine(p, b, i))
			}
		}
		groups = append(groups, g)
	default:
		counts := map[string]int{}
		unplaced := 0
		g := outlineGroup{none: "大纲是空的。"}
		for i, c := range p.Cards {
			counts[c.Line]++
			if b.cardChapter(c.Chapter) == "" {
				unplaced++
			}
			g.rows = append(g.rows, cardLine(p, b, i))
		}
		lines := []string{fmt.Sprintf("大纲共 %d 张卡片，按故事里发生的先后排列。", len(p.Cards))}
		for _, l := range p.Lines {
			lines = append(lines, fmt.Sprintf("情节线「%s」：%d 张。", l.Name, counts[l.ID]))
		}
		lines = append(lines, fmt.Sprintf("没有情节线：%d 张。未放进章节：%d 张。", counts[""], unplaced))
		head = strings.Join(lines, "\n")
		groups = append(groups, g)
	}
	return outcome{text: outlinePage(head, groups, in.From)}
}

// outlinePage shares one page limit across a volume's chapters.
func outlinePage(head string, groups []outlineGroup, from int) string {
	total := 0
	for _, g := range groups {
		total += len(g.rows)
	}
	from = min(max(from, 0), total)
	end := min(from+maxCards, total)
	var lines []string
	if head != "" {
		lines = append(lines, head)
	}
	at := 0
	for _, g := range groups {
		start, stop := max(from-at, 0), min(end-at, len(g.rows))
		at += len(g.rows)
		if len(g.rows) > 0 && start >= stop {
			continue
		}
		if g.head != "" {
			lines = append(lines, g.head)
		}
		if len(g.rows) == 0 {
			lines = append(lines, g.none)
		} else {
			lines = append(lines, g.rows[start:stop]...)
		}
	}
	if total > 0 && from == total {
		lines = append(lines, "后面没有卡片了。")
	} else if end < total || from > 0 {
		more := fmt.Sprintf("这里列的是第 %d–%d 张，共 %d 张。", from+1, end, total)
		if end < total {
			more += fmt.Sprintf("要看后面的，from 填 %d 再查一次。", end)
		}
		lines = append(lines, more)
	}
	return strings.Join(lines, "\n")
}
