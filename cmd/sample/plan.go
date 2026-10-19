package main

import (
	"fmt"
	"strings"
	"writer/workspace"
)

// The outline follows each book's events; the early memory belongs to a
// later chapter, so story order and writing order visibly differ.
func (b book) writePlan(s *workspace.Store, chapters []string) error {
	names := []string{"查灯", "母亲的下落", "周祥"}
	memory, early, loose, unknown := "母亲在停电那晚失踪", "十九年前·秋", "井下的灯为什么还亮着", "何主任为什么锁着档案柜"
	if b.name == "青崖问剑" {
		names = []string{"问剑", "父亲的往事", "苏挽"}
		memory, early, loose, unknown = "父亲被逐出山门", "十六年前", "玄铁令的另一半在哪里", "守冢人为什么认得他"
	}
	var lines []workspace.PlotLine
	for _, name := range names {
		l, err := s.SaveLine(workspace.PlotLine{Name: name})
		if err != nil {
			return err
		}
		lines = append(lines, l)
	}
	after := ""
	add := func(c workspace.Card) error {
		card, err := s.SaveCard(c, after)
		if err == nil {
			after = card.ID
		}
		return err
	}
	if len(chapters) > 7 {
		if err := add(workspace.Card{Title: memory, When: early, Chapter: chapters[7], Line: lines[1].ID, About: "这一段在后面的章节里回忆，故事里发生得更早。"}); err != nil {
			return err
		}
	}
	i := 0
	for _, n := range b.nodes {
		if n.volume {
			continue
		}
		if i >= len(chapters) {
			break
		}
		when := fmt.Sprintf("第%d天", i+1)
		title := n.outline
		if at := strings.IndexAny(title, "。；"); at >= 0 {
			title = title[:at]
		}
		if title == "" {
			title = n.title
		}
		if err := add(workspace.Card{Title: title, When: when, About: n.outline, Chapter: chapters[i], Line: lines[i%len(lines)].ID}); err != nil {
			return err
		}
		if i == 2 || i == 6 || i == 12 {
			extra := "人物没有说出口的话"
			if i == 6 {
				extra = "线索指向更早的那一夜"
			}
			if i == 12 {
				extra = "旧物的来历终于有了眉目"
			}
			if err := add(workspace.Card{Title: extra, When: when, Chapter: chapters[i], Line: lines[(i+1)%len(lines)].ID}); err != nil {
				return err
			}
		}
		i++
	}
	if err := add(workspace.Card{Title: loose, When: "之后", Line: lines[1].ID}); err != nil {
		return err
	}
	if err := add(workspace.Card{Title: unknown, About: "还没决定写进哪一章。"}); err != nil {
		return err
	}
	return add(workspace.Card{Title: "尾声中的一封信", Line: lines[2].ID})
}
