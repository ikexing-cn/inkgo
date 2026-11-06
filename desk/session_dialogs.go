package desk

import (
	"github.com/egoist/mygo/ui"
	"writer/ext"
)

type dialogsHost struct{ extensionHost }

func (h dialogsHost) BindSession(s *ext.Session) any { return sessionDialogs{h.extensionHost, s} }

type sessionDialogs struct {
	extensionHost
	session *ext.Session
}

func (d sessionDialogs) PickInto(c *ui.Context, title string, picked func(string)) bool {
	pick := d.a.PickFolder
	if pick == nil || !d.session.Active() {
		return false
	}
	// Native pickers have no cancellation API. The host owns the platform
	// operation; only the result callback belongs to the extension.
	d.a.jobs.start(c.Services(), func() func() {
		path, err := pick(title)
		return func() {
			if !d.session.Active() {
				return
			}
			if err != nil {
				d.a.SetError(err)
			} else if path != "" {
				d.guard(func() { picked(path) })
			}
		}
	})
	return true
}

func (d sessionDialogs) guard(fn func()) {
	if d.session == nil {
		fn()
		return
	}
	(&sessionJobs{a: d.a, s: d.session}).guard(fn)
}
func (d sessionDialogs) SaveInto(c *ui.Context, title, name string, picked func(string, error)) bool {
	save := d.a.SaveFile
	if save == nil || (d.session != nil && !d.session.Active()) {
		return false
	}
	d.a.jobs.start(c.Services(), func() func() {
		path, err := save(title, name)
		return func() { d.guard(func() { picked(path, err) }) }
	})
	return true
}
