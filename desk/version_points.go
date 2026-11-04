package desk

import (
	"github.com/egoist/mygo/ui"
	"writer/ext"
	"writer/ext/hostapi"
)

func (a *App) writingKeys(fn func(hostapi.WritingKey)) {
	for _, i := range ext.Items(a.extensions, hostapi.WritingKeys) {
		_, _ = ext.Call(a.extensions, ext.HookOf(ext.CallPoint[hostapi.WritingKey, func(hostapi.WritingKey), struct{}]{Point: hostapi.WritingKeys}, i.ID), fn, func(f func(hostapi.WritingKey)) struct{} { f(i.Value); return struct{}{} })
	}
}
func (a *App) writingEscape() bool {
	done := false
	a.writingKeys(func(k hostapi.WritingKey) {
		if k.Escape != nil {
			done = k.Escape() || done
		}
	})
	return done
}
func (a *App) hideSelection() bool {
	hide := false
	a.writingKeys(func(k hostapi.WritingKey) {
		if k.HideSelection != nil {
			hide = k.HideSelection() || hide
		}
	})
	return hide
}
func (a *App) closeFind() {
	a.writingKeys(func(k hostapi.WritingKey) {
		if k.Close != nil {
			k.Close()
		}
	})
}
func (a *App) beforeProse(c *ui.Context) {
	for _, i := range ext.Items(a.extensions, hostapi.BeforeProse) {
		_, _ = ext.Call(a.extensions, ext.HookOf(ext.CallPoint[hostapi.DrawItem, *ui.Context, struct{}]{Point: hostapi.BeforeProse}, i.ID), c, drawItemCall(i.Value))
	}
}
