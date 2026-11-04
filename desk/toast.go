package desk

import (
	"errors"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/locales"
)

// noticeFor is how long a confirmation stays. The time stops while the
// pointer rests on it.
var noticeFor = 3 * time.Second

// toastsShown is how many show at once, the rest fold into one line that opens them.
const toastsShown = 2

// toasts float the error, the request running, what was done and what to
// do first at the bottom right, over every page and dialog: none of them pushes
// the page down.
// The fields stay the state; a toast shows each while it is set, and
// closing the toast clears it.
func (a *App) toasts(c *ui.Context) {
	busy := a.busy
	if a.replying() && a.talkShown() || !a.aiOn() {
		busy = "" // the turn shows its own progress in its talk; all that runs is the AI's
	}
	errText := a.status.ErrorText()
	errKey := errText
	if n := a.status.Repeated(); n > 1 {
		errKey += "×" + strconv.Itoa(n)
	}
	a.syncToast(c, "busy", busy, busy, -1, func() {})
	a.syncToast(c, "error", errText, errKey, -1, a.status.CloseError)
	notice := a.status.NoticeText()
	serial := a.status.NoticeSerial()
	a.syncToast(c, "notice", notice, strconv.Itoa(serial), noticeFor, func() {
		if a.status.NoticeSerial() == serial {
			a.status.CloseNotice(notice)
		}
	})
	hint := a.status.HintText()
	a.syncToast(c, "hint", hint, hint, noticeFor, func() {
		a.status.CloseHint(hint)
	})
	ui.ToastViewportBase(c, func(v ui.Element, list []ui.Toast) {
		v.Justify(ui.End).AlignItems(ui.End).Gap(8).Padding(0, 12, kit.HRow+8, 0)
		if len(list) <= toastsShown {
			a.toastsOpen = false
		}
		start := 0
		if len(list) > toastsShown && !a.toastsOpen {
			start = len(list) - toastsShown
			a.pal.MoreToasts(c, start, &a.toastsOpen)
		}
		for i := start; i < len(list); i++ { // the newest at the bottom
			a.pal.ToastCard(c, list[i], &a.status, noticeFor, func() { a.toastActions(c, list[i]) })
		}
	})
}

// syncToast shows text as the toast id while it is set, again when key
// changes, and closes the toast once text is cleared. done runs when the
// toast closes by itself or by the author.
func (a *App) syncToast(c *ui.Context, id, text, key string, life time.Duration, done func()) {
	if a.toastShown == nil {
		a.toastShown = map[string]string{}
	}
	switch {
	case text == "" && a.toastShown[id] != "":
		delete(a.toastShown, id)
		c.CloseToast(id)
	case text != "" && a.toastShown[id] != key:
		a.toastShown[id] = key
		c.CloseToast(id)
		title, detail := kit.SplitToast(text)
		c.AddToast(ui.Toast{ID: id, Type: id, Title: title, Description: detail, Timeout: life, OnClose: func() {
			if a.toastShown[id] == key {
				delete(a.toastShown, id)
				done()
			}
		}})
	}
}

// toastActions are what can be done about it, the first the likely one.
func (a *App) toastActions(c *ui.Context, ts ui.Toast) {
	var acts []kit.ToastAction
	switch ts.Type {
	case "notice":
		if a.status.CanUndo() {
			acts = append(acts, kit.ToastAction{Label: locales.CmdUndo(), Name: "撤销提示", Fn: a.status.Undo})
		}
	case "busy":
		acts = append(acts, kit.ToastAction{Label: locales.CommonCancel(), Fn: func() {
			if a.cancel != nil {
				a.cancel()
			}
		}, Name: locales.ChatCancelRequest()})
	case "error":
		if again := a.status.RetryAction(); again != nil {
			acts = append(acts, kit.ToastAction{Label: locales.CommonRetry(), Fn: func() { a.SetError(nil); again() }})
		}
		var setup setupFailure
		if errors.As(a.status.Err(), &setup) {
			// What it says is done by going there: it goes, as in the prototype.
			acts = append(acts, kit.ToastAction{Label: locales.CommonOpenSettings(), Fn: func() { a.SetError(nil); a.openPrefs("AI") }})
		}
		text := a.status.ErrorText()
		acts = append(acts, kit.ToastAction{Label: locales.AppCopyDetails(), Fn: func() { c.WriteClipboard(text) }})
	}
	a.pal.ToastActions(c, acts)
}
