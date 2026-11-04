package talk

import (
	"writer/ext/talk/talktext"
	"writer/locales"
	"writer/workspace"
)

// talkTitle is what a talk is called in the list, kept without words of
// the interface so a language change needs no rereading.
// TalkTitle runs on the frame thread.
func (a *Panel) TalkTitle(events []workspace.Event) workspace.ConversationTitle {
	return workspace.TalkTitle(events, a.nodeTitle)
}

// talkName is a known talk's title in the interface language.
// TalkName runs on the frame thread.
func (a *Panel) TalkName(id string) (string, bool) {
	title, ok := a.Titles[id]
	return TitleText(title), ok
}

// TitleText formats a stored title on the frame thread.
func TitleText(title workspace.ConversationTitle) string {
	switch title.Kind {
	case workspace.AIDraft:
		text := title.Text
		if text == "" {
			text = locales.AppChapterDeleted()
		}
		return talktext.SideDraftTitle(text)
	case workspace.AIRewrite:
		return talktext.SideRewriteTitle(title.Text)
	case workspace.TitleWords:
		return title.Text
	default:
		return talktext.SideNewTalk()
	}
}
