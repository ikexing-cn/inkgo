package workspace

import (
	"strings"

	"writer/internal/textclip"
)

// TalkTitle names a conversation by its first user event. chapter resolves
// the current chapter title; a missing chapter keeps the deleted label.
func TalkTitle(events []Event, chapter func(string) string) string {
	for _, e := range events {
		if e.Role != RoleUser {
			continue
		}
		title := chapter(e.Chapter)
		if title == "" {
			title = "已删除的章"
		}
		switch {
		case e.Origin == AIDraft:
			return "起草：" + title
		case e.Origin == AIRewrite && len(e.Quotes) > 0:
			return "改写：" + titleClip(e.Quotes[0].Text, 8)
		}
		return titleClip(e.Text, 14)
	}
	return "新对话"
}

func titleClip(s string, n int) string {
	s, _, _ = strings.Cut(s, "\n")
	return textclip.Clip(s, n)
}
