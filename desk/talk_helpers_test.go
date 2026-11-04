package desk

import (
	"github.com/egoist/mygo/ui"
	"time"
	"writer/desk/kit"
	talkmod "writer/ext/talk"
	"writer/ext/talk/talktext"
	"writer/workspace"
)

func talkDay(id string, now time.Time) string {
	at, err := time.ParseInLocation("20060102-1504", id[:min(len(id), 13)], time.Local)
	if err != nil {
		return ""
	}
	return kit.DayOf(at, now)
}
func talkTitleText(title workspace.ConversationTitle) string { return talkmod.TitleText(title) }

var cmdSend = &command{label: talktext.HelpSend, key: ui.KeyEnter, mods: modCmd, page: pageTalk, ai: true, by: byBox}
