package desk

import (
	"github.com/egoist/mygo/ui"
	"writer/workspace"
)

// aiOn tells whether the AI is on on this machine. Off, nothing of it
// shows, and nothing is sent.
func (a *App) aiOn() bool { return !a.shelf.NoAI }

// loadAI reads the services configured, as the app starts with the AI on
// or the AI comes back on.
func (a *App) loadAI() {
	if a.ai.ConfigFile() != "" {
		if err := a.ai.Reload(); err != nil {
			a.SetError(err)
		}
	}
	if ps := a.ai.Providers(); len(ps) > 0 && a.providerID == "" {
		a.chooseProvider(ps[0].ID)
	}
}

// aiView is the settings' AI: the switch for every work on this machine,
// and with the AI on, the models.
func (a *App) aiView(c *ui.Context) {
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		off := !a.aiOn() // the box shows the switch; it changes once asked
		if ui.Checkbox(c, &off, "关闭 AI").Changed() {
			a.bk.askAI = true
		}
		a.tickNote(c, "隐藏全部 AI 功能，不发送任何内容。")
	})
	if a.aiOn() {
		a.modelsView(c)
	}
}

// askAI asks, over the settings, before the AI is turned off or on. Esc
// and the shade put the question away, the settings staying.
func (a *App) askAI(c *ui.Context) {
	off := a.aiOn() // what is asked is to turn it off
	title, about, verb := "是否开启 AI？", "重新显示 AI 功能。检查会按设置把章节内容发给模型服务；未配置模型服务时不发送任何内容。", "开启 AI"
	if off {
		title, about, verb = "是否关闭 AI？", "隐藏全部 AI 功能，不再发送任何内容；正在进行的 AI 任务会停止，没有处理的 AI 提议会丢弃。讨论、检查结果和 AI 记录不会删除，重新开启后恢复。", "关闭 AI"
	}
	turn := false
	a.modal(c, &a.bk.askAI, func() {
		ui.Column(c).Width(dialogAsk).Padding(dialogPad).Gap(16).Children(func() {
			a.pal.dialogHead(c, title, about)
			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
				if a.pal.secondary(c, "取消").Clicked() {
					a.bk.askAI = false
				}
				if a.pal.primary(c, verb).Label("确定" + verb).Clicked() {
					turn = true
				}
			})
		})
	})
	if turn {
		a.bk.askAI = false
		a.turnAI(!off)
	}
}

// turnAI turns the AI on or off on this machine, and keeps that in the
// shelf. Off, what the AI is doing stops: what it proposed in the prose
// and was not taken is let go, a reply is let go as 取消请求 lets it go, the
// author's words kept, and the checks waiting are dropped; the pages and the parts of
// the AI give way to the prose and the setting, and the toasts go, most
// of them the AI's. Nothing it left is deleted: on again, it is all back,
// the talk beside the prose too when it was there, and the talk or check
// page it was turned off on while the prose still shows.
func (a *App) turnAI(on bool) {
	a.shelf.NoAI = !on
	a.saveShelf()
	if on {
		a.loadAI()
		if a.offAway && a.page == pageWrite {
			a.page = a.offFrom
		}
		a.offAway = false
	} else {
		if a.cancel != nil {
			a.cancel()
		}
		if a.Store != nil {
			a.dropProps(a.write.props, workspace.FateOff)
			a.stopChecks()
			if a.page == pageTalk || a.page == pageCheck {
				a.offFrom, a.offAway, a.page = a.page, true, pageWrite
			}
		}
		w := &a.write
		w.draftOpen, w.rewriting = false, false
		a.doc.route.closeWaiting() // what waits on the author there is the AI's
		a.status.clear()
	}
	if ed := a.write.ed; ed != nil {
		ed.SetNoAI(!on)
	}
}
