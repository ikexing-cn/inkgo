package desk

import (
	"context"
	"github.com/egoist/mygo/ui"
	"net/url"
	"writer/ext/ai/aiapi"
	"writer/ext/hostapi"
	"writer/locales"
	"writer/workspace"
)

func (a *App) destination(sel workspace.Selection) string {
	for _, p := range a.ai.Providers() {
		if p.ID == sel.Provider {
			u, err := url.Parse(p.BaseURL)
			if err != nil {
				return locales.SettingsInvalidAddress()
			}
			kind := locales.SettingsRemote()
			if isLocal(p.BaseURL) {
				kind = locales.SettingsLocal()
			}
			return kind + " " + u.Host + " / " + sel.Model
		}
	}
	return locales.ChatNeedModel()
}

// writingSwitches are the writing page's calls to the models, kept as
// they are ticked.
func (a *App) writingSwitches(c *ui.Context) {
	busy := a.held()
	a.pal.Subhead(c, locales.SettingsWriting(), "")
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		if ui.Checkbox(c, &a.write.autoPredict, locales.SettingsAutoContinue()).Disabled(busy).Changed() {
			for _, ed := range a.write.editors() {
				ed.SetPredictOnDemand(!a.write.autoPredict)
			}
			a.savePrefs() // kept as ticked, not with the models above
		}
		a.pal.TickNote(c, locales.SettingsAutoNote())
	})
}

// Choices runs on the frame thread.
func (h extensionHost) Choices() *hostapi.ModelChoices { return &h.a.ModelChoices }

// Held runs on the frame thread.
func (h extensionHost) Held() bool { return h.a.held() }

// Busy runs on the frame thread.
func (h extensionHost) Busy() bool { return h.a.busy != "" }

// ModelsChanged runs on the frame thread.
func (h extensionHost) ModelsChanged() { h.a.modelsChanged() }

// ErrorText runs on the frame thread.
func (h extensionHost) ErrorText(err error) string { return h.a.errText(err) }

// Run runs on the frame thread.
func (h extensionHost) Run(c *ui.Context, name string, fn func(context.Context) (func(), error)) {
	h.a.run(c, name, fn)
}

// WritingSettings runs on the frame thread.
func (h extensionHost) WritingSettings(c *ui.Context) { h.a.writingSwitches(c) }

func isLocal(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	h := u.Hostname()
	return h == "localhost" || h == "127.0.0.1" || h == "::1"
}

func modelIDs(p aiapi.Provider) []string {
	ids := make([]string, len(p.Models))
	for i, m := range p.Models {
		ids[i] = m.ID
	}
	return ids
}
