package desk

import (
	"github.com/egoist/mygo/ui"
	"writer/ext"
	"writer/ext/hostapi"
	"writer/locales"
)

func registerCheckSettings(r *ext.Registrar, a *App) {
	ext.Contribute(r, hostapi.SettingsPages, "check-reminders", hostapi.SettingsPage{Key: "reminders", Title: locales.SettingsTabRemind, Label: "设置-提醒", Order: 20})
}
func (a *App) reminderSettings(c *ui.Context) {
	for _, owner := range a.extViews.pageOwners["reminders"] {
		if owner == ext.HostOwner {
			continue
		}
		if a.extensions.Ledger(owner).State != ext.On {
			continue
		}
		for _, item := range a.extViews.sections {
			if item.Owner == owner && (!item.Value.Work || a.Store != nil) {
				_, _ = ext.Call(a.extensions, ext.HookOf(ext.CallPoint[hostapi.SettingsSection, *ui.Context, struct{}]{Point: hostapi.SettingsSections}, item.ID), c, settingsCall(item.Value))
			}
		}
	}
	if a.Store != nil && a.checksOn() {
		a.pal.Subhead(c, locales.SettingsCheckText(), "")
		ui.Column(c).FillWidth().Gap(2).Children(func() { a.whenRows(c) })
	}
}
