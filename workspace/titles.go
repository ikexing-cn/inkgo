package workspace

import (
	"strings"

	"writer/internal/textclip"
)

// SessionTitle names a conversation by its first words.
func SessionTitle(events []Event, chapter func(string) string) string {
	for _, e := range events {
		if e.Role == RoleUser {
			title := chapter(e.Chapter)
			if title == "" {
				title = "已删除的章"
			}
			return TalkTitle(e, title)
		}
	}
	return "新对话"
}

// TalkTitle names a talk by its first words e, said at the chapter titled
// chapter: one opened to draft or rewrite it by what it does.
func TalkTitle(e Event, chapter string) string {
	switch {
	case e.Origin == AIDraft:
		return "起草：" + chapter
	case e.Origin == AIRewrite && len(e.Quotes) > 0:
		return "改写：" + titleClip(e.Quotes[0].Text, 8)
	}
	return titleClip(e.Text, 14)
}

func titleClip(s string, n int) string {
	s, _, _ = strings.Cut(s, "\n")
	return textclip.Clip(s, n)
}
