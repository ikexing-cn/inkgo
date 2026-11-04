package desk

import (
	"writer/locales"
	"writer/workspace"
)

// talkTitle is what a talk is called in the list, kept without words of
// the interface so a language change needs no rereading.
func (a *App) talkTitle(events []workspace.Event) workspace.ConversationTitle {
	return workspace.TalkTitle(events, a.nodeTitle)
}

// talkName is a known talk's title in the interface language.
func (a *App) talkName(id string) (string, bool) {
	title, ok := a.titles[id]
	return talkTitleText(title), ok
}

func talkTitleText(title workspace.ConversationTitle) string {
	switch title.Kind {
	case workspace.AIDraft:
		text := title.Text
		if text == "" {
			text = locales.AppChapterDeleted()
		}
		return locales.SideDraftTitle(text)
	case workspace.AIRewrite:
		return locales.SideRewriteTitle(title.Text)
	case workspace.TitleWords:
		return title.Text
	default:
		return locales.SideNewTalk()
	}
}
