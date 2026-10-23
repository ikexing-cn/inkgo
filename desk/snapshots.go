package desk

import (
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/internal/textdiff"
	"writer/workspace"
)

// snapReasons say why a snapshot was taken; with the AI off, snapReason
// tells one taken before the AI changed the prose as one before a change.
var snapReasons = map[string]string{
	workspace.SnapOpen: "打开时", workspace.SnapInterval: "写作中", workspace.SnapRestore: "恢复前",
	workspace.SnapExternal: "外部改动", workspace.SnapAI: "AI 改动前", workspace.SnapReplace: "全书替换前",
}

func (a *App) snapReason(reason string) string {
	if reason == workspace.SnapAI && !a.aiOn() {
		return "改动前"
	}
	return snapReasons[reason]
}

func (a *App) openSnapshots() {
	w := &a.write
	a.warn(a.saveText())
	snaps, err := a.Store.Snapshots(w.node)
	if err != nil {
		a.warn(err)
		return
	}
	w.snaps, w.snapsOpen, w.compare = snaps, true, true
	w.snap, w.snapText = "", ""
	if len(snaps) > 0 {
		a.pickSnapshot(snaps[0].Name)
	}
}

func (a *App) pickSnapshot(name string) {
	w := &a.write
	text, err := a.Store.SnapshotText(w.node, name)
	if err != nil {
		a.warn(err)
		return
	}
	w.snap, w.snapText = name, text
}

// snapView is the snapshot chosen as its dialog shows it: whether it is
// the text as it stands, and its paragraphs, with what differs marked when
// the two are compared.
type snapView struct {
	key   [5]any // the snapshot, comparing or not, the editor's text, the two colours
	same  bool
	paras [][]ui.Span
	long  bool         // more than snapWhole: only the paragraphs in view are built
	list  ui.ListState // where a long one is scrolled to
}

// snapWhole is the most a snapshot shows with every paragraph built, in
// bytes of text and in paragraphs: so its text selects from end to end. A
// longer one builds the paragraphs in view, and selects among those: built
// whole, a frame of it took 47 ms at sixty thousand characters, and at
// three hundred thousand the toolkit no longer kept the paragraphs' lines.
const snapWhole, snapWholeParas = 90_000, 1000

// snapView is made as the snapshot, the comparing or the text beside it
// changes, not in each frame: comparing two chapters takes long. A
// paragraph is an element of its own, for the toolkit keeps the lines of
// a text only up to a size, and lays a longer one out again in every frame.
func (a *App) snapView() *snapView {
	t := a.pal.theme
	w := &a.write
	v := &w.snapShown
	key := [5]any{w.snap, w.compare, w.cur().ed, w.cur().ed.Doc.Version(), [2]ui.Color{t.Danger, t.Success}}
	if v.key == key {
		return v
	}
	now := w.cur().ed.Doc.String()
	*v = snapView{key: key, same: w.snap != "" && w.snapText == now}
	if w.snap == "" {
		return v
	}
	pieces := []textdiff.Piece{{Kind: textdiff.Same, Text: w.snapText}}
	if w.compare {
		pieces = textdiff.Compare(w.snapText, now)
	}
	size := 0
	for _, p := range pieces {
		size += len(p.Text)
	}
	paras := textdiff.Paragraphs(pieces)
	v.long = size > snapWhole || len(paras) > snapWholeParas
	for _, para := range paras {
		spans := make([]ui.Span, len(para))
		for i, p := range para {
			switch p.Kind {
			case textdiff.Removed:
				spans[i] = ui.Span{Text: p.Text, Color: t.Danger, Strikethrough: true, Background: t.Danger.Alpha(0.12)}
			case textdiff.Added:
				spans[i] = ui.Span{Text: p.Text, Color: t.Success, Background: t.Success.Alpha(0.12)}
			default:
				spans[i] = ui.Span{Text: p.Text}
			}
		}
		v.paras = append(v.paras, spans)
	}
	return v
}

func (a *App) snapshots(c *ui.Context) {
	t := a.pal.theme
	w := &a.write
	a.modal(c, &w.snapsOpen, func() {
		shown := a.snapView()
		same := shown.same // nothing to restore
		ui.Column(c).Width(dialogWide).Height(dialogTall).Padding(dialogPad).Gap(16).Children(func() {
			kept := "打开章节时、连续写作每十分钟、恢复前、采纳 AI 提议前、全书替换前都会保存一份。最近七天的全部保留，更早的每天保留最后一份。"
			if !a.aiOn() {
				kept = strings.Replace(kept, "采纳 AI 提议前、", "", 1)
			}
			a.pal.dialogHead(c, "快照 · "+a.shownTitle(), kept)
			ui.Row(c).FillWidth().Grow(1).Gap(16).AlignItems(ui.Stretch).Children(func() {
				ui.Scroll(c.Key("snaps")).Width(200).FillHeight().Shrink(0).Children(func() {
					ui.Column(c).FillWidth().Gap(2).Children(func() {
						if len(w.snaps) == 0 {
							ui.Text(c, "本章暂无快照").FontSize(fsControl).TextColor(a.pal.faint).Padding(8)
						}
						for _, sn := range w.snaps {
							on := sn.Name == w.snap
							at := when(sn.At, time.Now())
							b := ui.ButtonBase(c.Key("s-"+sn.Name)).Column().FillWidth().AlignItems(ui.Start).Gap(2).Padding(8, 10).Radius(rControl).Label("快照-" + at)
							if on {
								b.Background(t.SurfacePressed)
							} else if b.Hovered() {
								b.Background(t.SurfaceHover)
							}
							b.Children(func() {
								ui.Text(c, at).FontSize(fsControl)
								ui.Textf(c, "%s · %d 字", a.snapReason(sn.Reason), sn.Count).FontSize(fsNote).TextColor(t.TextMuted)
							})
							if b.Clicked() {
								a.pickSnapshot(sn.Name)
							}
						}
					})
				})
				ui.Column(c).Grow(1).Shrink(1).FillHeight().Gap(10).Children(func() {
					ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
						ui.Checkbox(c, &w.compare, "与当前对比")
						if same {
							ui.Text(c, "该快照与当前正文相同").FontSize(fsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
						} else if w.compare {
							ui.Text(c, "红色删除线为快照中有而当前没有的内容，绿色为当前新增的内容").FontSize(fsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
						}
					})
					if i := slices.IndexFunc(w.snaps, func(s workspace.Snapshot) bool { return s.Name == w.snap }); i >= 0 && a.aiOn() {
						switch sn := w.snaps[i]; {
						case len(sn.Talks) > 0:
							a.talkRecord(c, sn.Talks)
						case sn.AI != nil:
							a.aiRecord(c, sn.AI)
						}
					}
					// The paragraphs select as one text, and copy a line each.
					para := func(i int) {
						switch spans := shown.paras[i]; {
						case len(spans) == 0: // an empty line keeps its height
							ui.Text(c, "").FontSize(fsRead).LineHeight(lhRead).FillWidth()
						case len(spans) == 1 && spans[0] == (ui.Span{Text: spans[0].Text}):
							ui.Text(c, spans[0].Text).FontSize(fsRead).LineHeight(lhRead).FillWidth()
						default:
							ui.RichText(c, spans...).FontSize(fsRead).LineHeight(lhRead).FillWidth()
						}
					}
					if shown.long {
						ui.Box(c).FillWidth().Grow(1).Background(a.pal.canvas).Border(1, a.pal.line).Radius(rControl).Selectable().Children(func() {
							ui.List(c, &shown.list, len(shown.paras), para).Fill().Padding(12, 16)
						})
					} else {
						ui.Scroll(c.Key("snap-text")).FillWidth().Grow(1).Background(a.pal.canvas).Border(1, a.pal.line).Radius(rControl).Children(func() {
							ui.Column(c).FillWidth().Padding(12, 16).Selectable().Children(func() {
								for i := range shown.paras {
									para(i)
								}
							})
						})
					}
				})
			})
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Spacer(c)
				if a.pal.secondary(c, "关闭").Clicked() {
					w.snapsOpen = false
				}
				if a.pal.primary(c, "恢复这个版本").Disabled(w.snap == "" || same || w.cur().lost || w.cur().external != nil).Clicked() {
					a.restoreSnapshot()
				}
			})
		})
	})
}

// restoreSnapshot replaces the text with the snapshot chosen, keeping the
// text replaced as a snapshot of its own.
func (a *App) restoreSnapshot() {
	w := &a.write
	i := slices.IndexFunc(w.snaps, func(s workspace.Snapshot) bool { return s.Name == w.snap })
	if i < 0 {
		return
	}
	if err := a.saveText(); err != nil {
		a.warn(err)
		return
	}
	if _, err := a.Store.TakeSnapshot(w.node, workspace.SnapRestore, w.cur().ed.Doc.String(), storedMarks(w.cur().ed.Doc.Marks())); err != nil {
		a.warn(err)
		return
	}
	w.cur().ed = a.chapterEditor(w.snapText, w.snaps[i].Marks)
	w.cur().savedVer = -1
	if err := a.saveText(); err != nil {
		a.warn(err)
		return
	}
	w.snapsOpen = false
	a.status.did("已恢复到 " + when(w.snaps[i].At, time.Now()) + " 的版本，恢复前的内容已存为快照")
}
