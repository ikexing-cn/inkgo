package workspace

import (
	"strings"

	"writer/internal/textclip"
)

// TalkTitle names a conversation by its first user event. chapter resolves
// the current chapter title; a missing chapter keeps the deleted label.
func TalkTitle(events []Event, chapter func(string) string) ConversationTitle {
	for _, e := range events {
		if e.Role != RoleUser {
			continue
		}
		title := chapter(e.Chapter)
		switch {
		case e.Origin == AIDraft:
			return ConversationTitle{Kind: AIDraft, Text: title}
		case e.Origin == AIRewrite && len(e.Quotes) > 0:
			return ConversationTitle{Kind: AIRewrite, Text: titleClip(e.Quotes[0].Text, 8)}
		}
		return ConversationTitle{Kind: TitleWords, Text: titleClip(e.Text, 14)}
	}
	return ConversationTitle{}
}

func titleClip(s string, n int) string {
	s, _, _ = strings.Cut(s, "\n")
	return textclip.Clip(s, n)
}

// ConversationTitle holds display data without interface-language words.
// Kind is AIDraft, AIRewrite, TitleWords, or TitleNew.
type ConversationTitle struct{ Kind, Text string }

// Kinds of ConversationTitle besides AIDraft and AIRewrite.
const (
	TitleNew   = ""      // no words from the author yet
	TitleWords = "words" // the author's first words, clipped
)
