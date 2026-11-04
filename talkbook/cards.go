package talkbook

import (
	"fmt"
	"strings"
	"writer/internal/textclip"
	"writer/workspace"
)

// cardChapter is the live chapter of a card, or none for an unplaced card.
func (b Book) cardChapter(id string) string {
	if n, ok := b.Tree.Node(id); ok && !n.Folder {
		return b.Name[id]
	}
	return ""
}

// CardLine is one card in story order, shared by tools and fixed context.
func CardLine(p workspace.Plan, b Book, i int) string {
	c := p.Cards[i]
	parts := []string{fmt.Sprintf("#%d", i+1)}
	if when := oneLine(c.When); when != "" {
		parts = append(parts, "时间："+when)
	}
	parts = append(parts, "发生："+oneLine(c.Title))
	chapter := "未放进章节"
	if name := b.cardChapter(c.Chapter); name != "" {
		chapter = "「" + oneLine(name) + "」"
	}
	parts = append(parts, chapter)
	for _, l := range p.Lines {
		if l.ID == c.Line {
			parts = append(parts, "情节线："+oneLine(l.Name))
			break
		}
	}
	if about := oneLine(c.About); about != "" {
		parts = append(parts, "说明："+textclip.Clip(about, 200))
	}
	return strings.Join(parts, "｜")
}

// ChapterCards keeps the full book's numbering and story order.
func ChapterCards(p workspace.Plan, b Book, chapter string) []string {
	var rows []string
	for i, c := range p.Cards {
		if c.Chapter == chapter {
			rows = append(rows, CardLine(p, b, i))
		}
	}
	return rows
}
