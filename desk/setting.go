package desk

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/workspace"
)

// docView is the state of the setting panel.
type docView struct {
	hidden   bool
	history  bool
	open     map[string]*bool // sections unfolded, by category or place
	editing  string           // the entry being edited, "new" for a new one
	text     string
	category string
}

// prefs are the desk's own per-work settings, kept in .writer, which is
// not part of the work.
type prefs struct {
	SettingHidden bool `json:"setting_hidden"`
}

func (a *App) prefsPath() string { return filepath.Join(a.Store.Root(), ".writer", "desk.json") }

func (a *App) loadPrefs() {
	var p prefs
	if b, err := os.ReadFile(a.prefsPath()); err == nil {
		_ = json.Unmarshal(b, &p) // a damaged file is as good as none
	}
	a.doc.hidden = p.SettingHidden
}

// setDocHidden folds the panel away, for the talk alone, or brings it back.
func (a *App) setDocHidden(hidden bool) {
	a.doc.hidden = hidden
	b, err := json.Marshal(prefs{SettingHidden: hidden})
	if err == nil {
		err = os.WriteFile(a.prefsPath(), b, 0644)
	}
	a.setError(err)
}

var placeNames = map[string]string{workspace.PlaceBody: "正文", workspace.PlaceOpen: "悬而未决", workspace.PlaceDropped: "已放弃"}

// revise records the author's change and shows the setting after it.
func (a *App) revise(ops ...workspace.Op) {
	_, err := a.Store.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: ops})
	if err == nil {
		err = a.refresh()
	}
	a.setError(err)
}

func (a *App) settingPanel(c *ui.Context) {
	body := 0
	for _, e := range a.set.Entries {
		if e.Place == workspace.PlaceBody {
			body++
		}
	}
	ui.Row(c).FillWidth().Height(34).Gap(4).AlignItems(ui.Center).Children(func() {
		title := "设定"
		if a.doc.history {
			title = "整理历史"
		}
		ui.Text(c, title).FontSize(15).FontWeight(600)
		if !a.doc.history && body > 0 {
			ui.Textf(c, "%d", body).FontSize(12.5).TextColor(a.pal.faint).Padding(0, 0, 0, 4)
		}
		ui.Spacer(c)
		a.bookDot(c)
		color := ui.Color{}
		if a.doc.history {
			color = a.pal.brand
		}
		if iconButton(c, icClock, "设定历史", color).Tooltip("整理历史").Clicked() {
			a.doc.history = !a.doc.history
		}
		if iconButton(c, icPanel, "收起设定", ui.Color{}).Tooltip("收起设定文档").Clicked() {
			a.setDocHidden(true)
		}
	})
	if a.doc.history {
		a.history(c)
	} else {
		a.document(c)
	}
}

// document shows the body by category, then what is open and what was
// dropped, folded.
func (a *App) document(c *ui.Context) {
	t := a.pal.theme
	ui.Scroll(c).Key("setting").Grow(1).FillWidth().Children(func() {
		ui.Column(c).FillWidth().Gap(14).Padding(2, 4, 12, 0).Children(func() {
			if len(a.set.Entries) == 0 && a.doc.editing == "" {
				ui.Text(c, "聊着聊着，你定下的设定会整理到这里。也可以直接新增一条。").FontSize(13).TextColor(t.TextMuted).LineHeight(1.7)
			}
			for _, cat := range a.Store.Work().Categories {
				a.docSection(c, workspace.PlaceBody, cat.Name, cat.Name, true)
			}
			a.docSection(c, workspace.PlaceOpen, "", "悬而未决", true)
			a.docSection(c, workspace.PlaceDropped, "", "已放弃", false)
			a.newEntry(c)
		})
	})
}

// docSection lists the entries in place, of category when it is set, under a
// heading that folds them.
func (a *App) docSection(c *ui.Context, place, category, title string, unfolded bool) {
	var entries []workspace.Entry
	for _, e := range a.set.Entries {
		if e.Place == place && (category == "" || e.Category == category) {
			entries = append(entries, e)
		}
	}
	if len(entries) == 0 {
		return
	}
	if a.doc.open == nil {
		a.doc.open = map[string]*bool{}
	}
	key := place + "/" + category
	open := a.doc.open[key]
	if open == nil {
		open = &unfolded
		a.doc.open[key] = open
	}
	ui.Collapsible(c, fmt.Sprintf("%s · %d", title, len(entries)), open, func() {
		ui.Column(c).FillWidth().Gap(2).Padding(4, 0, 0, 0).Children(func() {
			for _, e := range entries {
				a.entry(c, e)
			}
		})
	}).FillWidth()
}

// entry is one statement; its actions show on hover, and one the
// bookkeeper just changed is tinted for a moment.
func (a *App) entry(c *ui.Context, e workspace.Entry) {
	t := a.pal.theme
	box := ui.Column(c).Key(e.ID).FillWidth().Padding(6, 8, 6, 10).Radius(8).Gap(8)
	if at, ok := a.flash[e.ID]; ok {
		if left := flashFor - time.Since(at); left > 0 {
			box.Background(a.pal.brand.Alpha(0.16 * float32(left) / float32(flashFor)))
			c.AnimationFrame()
		} else {
			delete(a.flash, e.ID)
		}
	}
	hover := box.Hovered() || box.FocusWithin()
	if hover && a.doc.editing != e.ID {
		box.Background(t.SurfaceHover)
	}
	box.Children(func() {
		if a.doc.editing == e.ID {
			a.entryForm(c, e)
			return
		}
		ui.Row(c).FillWidth().Gap(4).AlignItems(ui.Start).Children(func() {
			text := ui.Text(c, e.Text).FontSize(13.5).LineHeight(1.65).Selectable().Grow(1).Shrink(1)
			if e.Place == workspace.PlaceDropped {
				text.TextColor(t.TextMuted)
			}
			acts := ui.Row(c).Gap(0).Shrink(0)
			if !hover {
				acts.Opacity(0)
			}
			acts.Children(func() {
				switch e.Place {
				case workspace.PlaceOpen:
					if iconButton(c, icCheck, "采纳-"+e.ID, ui.Color{}).Tooltip("放入正文").Clicked() {
						a.revise(workspace.Op{Op: workspace.OpMove, Entry: e.ID, Place: workspace.PlaceBody})
					}
				case workspace.PlaceDropped:
					if iconButton(c, icUndo, "移回-"+e.ID, ui.Color{}).Tooltip("移回正文").Clicked() {
						a.revise(workspace.Op{Op: workspace.OpMove, Entry: e.ID, Place: workspace.PlaceBody})
					}
				}
				if iconButton(c, icPencil, "编辑-"+e.ID, ui.Color{}).Tooltip("修改").Clicked() {
					a.doc.editing, a.doc.text, a.doc.category = e.ID, e.Text, e.Category
				}
				if e.Place != workspace.PlaceDropped && iconButton(c, icX, "放弃-"+e.ID, ui.Color{}).Tooltip("放弃").Clicked() {
					a.revise(workspace.Op{Op: workspace.OpMove, Entry: e.ID, Place: workspace.PlaceDropped})
				}
			})
		})
	})
}

// entryForm edits e's text and category, or writes a new entry for the
// body when e is empty.
func (a *App) entryForm(c *ui.Context, e workspace.Entry) {
	t := a.pal.theme
	ui.TextAreaBase(c, &a.doc.text).Label("设定内容").Placeholder("写下一条设定").FillWidth().Height(76).FontSize(13.5).LineHeight(1.65).
		Padding(6, 8).Radius(6).Background(a.pal.raised).Border(1, t.Border)
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Select(c, &a.doc.category, a.categoryNames()).Label("设定分类").Width(120)
		ui.Spacer(c)
		if ghost(c, "取消", ui.Color{}).Clicked() {
			a.doc.editing = ""
		}
		text := trim(a.doc.text)
		if a.pal.primary(c, "保存").Disabled(text == "").Clicked() {
			var ops []workspace.Op
			switch {
			case e.ID == "":
				ops = append(ops, workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: a.doc.category, Text: text})
			default:
				if text != e.Text {
					ops = append(ops, workspace.Op{Op: workspace.OpEdit, Entry: e.ID, Text: text})
				}
				if a.doc.category != e.Category {
					ops = append(ops, workspace.Op{Op: workspace.OpMove, Entry: e.ID, Category: a.doc.category})
				}
			}
			if len(ops) > 0 {
				a.revise(ops...)
			}
			if a.Error == "" {
				a.doc.editing = ""
			}
		}
	})
}

func (a *App) newEntry(c *ui.Context) {
	if a.doc.editing == "new" {
		ui.Column(c).FillWidth().Padding(6, 8, 6, 10).Radius(8).Gap(8).Background(a.pal.raised).Children(func() {
			a.entryForm(c, workspace.Entry{})
		})
		return
	}
	b := ui.ButtonBase(c).Row().Gap(6).AlignItems(ui.Center).Padding(5, 8).Radius(7).Label("新增设定")
	if b.Hovered() {
		b.Background(a.pal.theme.SurfaceHover)
	}
	b.Children(func() {
		glyph(c, icPlus, 14, a.pal.faint)
		ui.Text(c, "新增设定").FontSize(13).TextColor(a.pal.theme.TextMuted)
	})
	if b.Clicked() {
		a.doc.editing, a.doc.text, a.doc.category = "new", "", workspace.Other
	}
}

// history lists the revisions, newest first: what changed, who changed it,
// and for the bookkeeper's, how it reasoned. A batch it could not take in
// can be retried here.
func (a *App) history(c *ui.Context) {
	t := a.pal.theme
	ui.Scroll(c).Key("history").Grow(1).FillWidth().Children(func() {
		ui.Column(c).FillWidth().Gap(10).Padding(2, 4, 12, 0).Children(func() {
			if len(a.set.Revisions) == 0 {
				ui.Text(c, "还没有整理过。每轮对话之后，整理会在后台进行。").FontSize(13).TextColor(t.TextMuted)
			}
			for n := len(a.set.Revisions) - 1; n >= 0; n-- {
				r := a.set.Revisions[n]
				ui.Column(c).Key(r.ID).FillWidth().Padding(10, 12).Gap(6).Radius(10).Background(a.pal.raised).Border(1, a.pal.line).Children(func() {
					ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
						who := "你改的"
						if r.By == workspace.ByBookkeeper {
							who = "后台整理"
							if r.Redo != "" {
								who += "（重试）"
							}
						}
						ui.Text(c, who).FontSize(12.5).FontWeight(600)
						if m := modelLabel(r.Model, r.Effort); m != "" {
							ui.Text(c, m).FontSize(11.5).TextColor(a.pal.faint).SingleLine().Ellipsis("…").Shrink(1)
						}
						ui.Spacer(c)
						ui.Text(c, r.At.Local().Format("01-02 15:04")).FontSize(11.5).TextColor(a.pal.faint)
					})
					if r.Failed != "" {
						ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Start).Children(func() {
							glyph(c, icAlert, 13, t.Warning)
							ui.Text(c, "这几轮没整理成功："+r.Failed).FontSize(12.5).TextColor(t.Warning).Grow(1).Shrink(1)
						})
						if !a.set.Redone(r.ID) && ghost(c, "重试", t.Text).Label("重试整理-"+r.ID).Disabled(a.busy != "").Clicked() {
							a.redo(c, r.ID)
						}
						return
					}
					if len(r.Ops) == 0 {
						ui.Text(c, "没有需要改的").FontSize(12.5).TextColor(t.TextMuted)
					}
					for _, op := range r.Ops {
						ui.Text(c, a.describeOp(op)).FontSize(12.5).LineHeight(1.6).Selectable().FillWidth()
					}
					if len(r.Skipped) > 0 {
						ui.Textf(c, "%d 处你刚改过，没有覆盖，放进了悬而未决", len(r.Skipped)).FontSize(12).TextColor(t.TextMuted)
					}
					if r.Reasoning != "" {
						a.thought(c, r.ID, "整理时的思考", r.Reasoning, false)
					}
				})
			}
		})
	})
}

// describeOp says what op did, in the words of the panel.
func (a *App) describeOp(op workspace.Op) string {
	e, _ := a.set.Entry(op.Entry)
	switch op.Op {
	case workspace.OpAdd:
		return fmt.Sprintf("新增到%s · %s：%s", placeNames[op.Place], op.Category, op.Text)
	case workspace.OpEdit:
		return "改写为：" + op.Text
	}
	text := clip(e.Text, 24)
	if op.Place != "" {
		return fmt.Sprintf("移到%s：%s", placeNames[op.Place], text)
	}
	return fmt.Sprintf("改归%s：%s", op.Category, text)
}

// redo retries a batch the bookkeeper could not take in.
func (a *App) redo(c *ui.Context, failed string) {
	book, err := a.Book(a.Store)
	if err != nil {
		a.setError(err)
		return
	}
	a.run(c, "正在重新整理…", func(ctx context.Context) (func(), error) {
		return nil, book.Redo(ctx, failed)
	})
}

// bookFailed reports whether the bookkeeper's latest batch failed and has
// not been retried; a later batch that went through clears it.
func (a *App) bookFailed() bool {
	for n := len(a.set.Revisions) - 1; n >= 0; n-- {
		if r := a.set.Revisions[n]; r.By == workspace.ByBookkeeper {
			return r.Failed != "" && !a.set.Redone(r.ID)
		}
	}
	return false
}

// bookDot is the one sign of the background bookkeeper: a dot that breathes
// while it works and turns to a warning when its last batch failed. It
// opens the history.
func (a *App) bookDot(c *ui.Context) {
	busy := a.worker != nil && a.worker.Busy()
	failed := !busy && a.bookFailed()
	if !busy && !failed {
		return
	}
	label, color := "正在整理设定", a.pal.brand
	if failed {
		label, color = "上次整理没成功，点开看看", a.pal.theme.Warning
	}
	b := ui.ButtonBase(c).Label(label).Tooltip(label).Padding(7).Radius(6)
	if b.Hovered() {
		b.Background(a.pal.theme.SurfaceHover)
	}
	b.Children(func() {
		d := dot(c, color, 7)
		if busy {
			phase := float64(time.Now().UnixMilli()%breath) / float64(breath)
			d.Opacity(float32(0.3 + 0.7*(0.5+0.5*math.Cos(2*math.Pi*phase))))
			c.After(breathStep)
		}
	})
	if b.Clicked() {
		a.setDocHidden(false)
		a.doc.history = true
	}
}

// The dot breathes once in breath, redrawn every breathStep: slow enough
// not to pull the eye from the talk, and to cost next to nothing.
const (
	breath     = 1800
	breathStep = 90 * time.Millisecond
)
