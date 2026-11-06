package ai

import (
	"context"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/ext/ai/aitext"
)

func modelFailure(kind string) string {
	switch kind {
	case "connect":
		return aitext.SettingsModelsConnect()
	case "http":
		return aitext.SettingsModelsHTTP()
	case "data":
		return aitext.SettingsModelsData()
	default:
		return aitext.SettingsModelsCache()
	}
}

// modelData is the 模型资料 group: where the limits come from, the switch
// for the daily background sync, the last result, and a button to sync now.
// A sync runs as a plain job, apart from the AI's requests, so neither waits
// on the other.
func (a *View) modelData(c *ui.Context) {
	if a.updates != nil {
		sv := c.Services()
		jobs := a.jobs.Get()
		a.updates.OnChange(func() { jobs.Post(sv, func() {}) })
	}
	a.pal.Subhead(c, aitext.SettingsModelData(), aitext.SettingsModelDataNote())

	if ui.Checkbox(c, &a.modelsAuto, aitext.SettingsModelsAuto()).Label("模型资料-后台自动同步").Changed() {
		a.autoModels.Set(a.modelsAuto)
		if a.updates != nil {
			a.updates.SetEnabled(a.modelsAuto)
		}
	}
	a.pal.TickNote(c, aitext.SettingsModelsAutoNote())
	ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Shrink(1).Children(func() {
			text, color := aitext.SettingsModelsNever(), a.pal.Theme.Text
			switch s := a.modelStatus(); {
			case a.modelsSyncing:
				text = aitext.SettingsModelsSyncing()
			case s.Failure != "":
				text, color = aitext.SettingsModelsFailed(modelFailure(s.Failure)), a.pal.Theme.Danger
			case !s.Success.IsZero():
				text = aitext.SettingsModelsLast(kit.When(s.Success, time.Now()))
			}
			ui.Text(c, text).FontSize(kit.FsControl).TextColor(color)
			ui.Text(c, aitext.SettingsModelsSyncNote()).FontSize(kit.FsNote).TextColor(a.pal.Theme.TextMuted)
		})
		if a.pal.Secondary(c, aitext.SettingsModelsSyncNow()).Disabled(a.updates == nil || a.modelsSyncing).Clicked() {
			updates := a.updates
			a.modelsSyncing = true
			a.jobs.Get().StartJob(c.Services(), func(ctx context.Context) func() {
				err := updates.Sync(ctx)
				return func() {
					a.modelsSyncing = false
					if err == nil {
						a.status.Did(aitext.SettingsModelsSynced())
					}
				}
			})
		}
	})
}

func (a *View) modelStatus() ModelDataState {
	if a.updates == nil {
		return ModelDataState{}
	}
	return a.updates.Status()
}
