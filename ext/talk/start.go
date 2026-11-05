package talk

import (
	"errors"
	"github.com/egoist/mygo/ui"
	"writer/ext/ai/aiapi"
	"writer/ext/talk/talktext"
	"writer/locales"
	"writer/workspace"
)

// startTalk opens a new talk that keeps to the chapter shown, to draft or
// rewrite it, with the author's first words; it shows beside the prose.
// It reports whether the talk began: there must be a model to talk with,
// and no other reply in flight.
// StartTalk runs on the frame thread.
func (a *Panel) StartTalk(c *ui.Context, words workspace.Event) bool {
	if work := a.Store.Work(); work.Discussion.Provider == "" || work.Discussion.Model == "" {
		a.SetError(&aiapi.SetupError{Err: errors.New(locales.ChatNeedModel())})
		return false
	}
	what := map[string]string{workspace.AIDraft: locales.SettingsFeatureDraft(), workspace.AIRewrite: locales.SettingsFeatureRewrite()}[words.Origin]
	if a.jobs.Busy() {
		a.notices().Hint(talktext.DraftBusyOther(what))
		return false
	}
	a.bookHost.Flush() // the talk reads the chapter from the disk
	id, err := a.BlankSession()
	if err != nil {
		a.warn(err)
		return false
	}
	a.SwitchSession(id)
	words.Chapter, words.Role = a.selected(), workspace.RoleUser
	a.Titles[id], a.Blank[id] = a.TalkTitle([]workspace.Event{words}), false
	a.pane.Show("talk")
	a.Talks = false
	a.desktop.SetFocus(false)
	a.savePrefs()
	a.Say(c, id, words, nil)
	return true
}
