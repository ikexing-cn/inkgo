package snaps

import (
	"slices"

	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/ext/snaps/snapstext"
	"writer/ext/snaps/textdiff"
	"writer/locales"
	"writer/workspace"
)

// snapReasons say why a snapshot was taken; with the AI off, snapReason
// tells one taken before the AI changed the prose as one before a change.
var snapReasons = map[string]func() string{
	workspace.SnapOpen: snapstext.SnapKindOpening, workspace.SnapInterval: snapstext.SnapKindWriting, workspace.SnapRestore: snapstext.SnapKindRestore,
	workspace.SnapExternal: snapstext.SnapKindOutside, workspace.SnapAI: snapstext.SnapKindAI, workspace.SnapReplace: snapstext.SnapKindSwap,
}

func (a *View) snapReason(reason string) string {
	if reason == workspace.SnapAI && !a.AI.Enabled("ai") {
		return snapstext.SnapKindEdit()
	}
	if label := snapReasons[reason]; label != nil {
		return label()
	}
	return ""
}

func (a *View) openSnapshots() {
	w := a
	a.Notices.Notices().Warn(a.Editing.SaveText())
	snaps, err := a.Book.Store().Snapshots(a.node())
	if err != nil {
		a.Notices.Notices().Warn(err)
		return
	}
	w.Snaps, w.SnapsOpen, w.Compare = snaps, true, true
	w.Snap, w.SnapText = "", ""
	if len(snaps) > 0 {
		a.pickSnapshot(snaps[0].Name)
	}
}

func (a *View) pickSnapshot(name string) {
	w := a
	text, err := a.Book.Store().SnapshotText(a.node(), name)
	if err != nil {
		a.Notices.Notices().Warn(err)
		return
	}
	w.Snap, w.SnapText = name, text
}

// snapView is the snapshot chosen as its dialog shows it: whether it is
// the text as it stands, and its paragraphs, with what differs marked when
// the two are compared.
type snapView struct {
	key   [5]any // the snapshot, comparing or not, the editor's text, the two colours
	Same  bool
	Paras [][]ui.Span
	Long  bool         // more than snapWhole: only the paragraphs in view are built
	List  ui.ListState // where a Long one is scrolled to
}

// snapWhole is the most a snapshot shows with every paragraph built, in
// bytes of text and in paragraphs: so its text selects from end to end. A
// longer one builds the paragraphs in view, and selects among those: built
// whole, a frame of it took 47 ms at sixty thousand characters, and at
// three hundred thousand the toolkit no longer kept the paragraphs' lines.
const snapWhole, snapWholeParas = 90_000, 1000

// snapView is made as the snapshot, the comparing or the text beside it
// changes, not in each frame: comparing two chapters takes Long. A
// paragraph is an element of its own, for the toolkit keeps the lines of
// a text only up to a size, and lays a longer one out again in every frame.
func (a *View) snapView() *snapView {
	t := a.Palette.Colors().Theme
	w := a
	v := &w.SnapShown
	key := [5]any{w.Snap, w.Compare, a.editor(), a.editor().Doc.Version(), [2]ui.Color{t.Danger, t.Success}}
	if v.key == key {
		return v
	}
	now := a.editor().Doc.String()
	*v = snapView{key: key, Same: w.Snap != "" && w.SnapText == now}
	if w.Snap == "" {
		return v
	}
	pieces := []textdiff.Piece{{Kind: textdiff.Same, Text: w.SnapText}}
	if w.Compare {
		pieces = textdiff.Compare(w.SnapText, now)
	}
	size := 0
	for _, p := range pieces {
		size += len(p.Text)
	}
	paras := textdiff.Paragraphs(pieces)
	v.Long = size > snapWhole || len(paras) > snapWholeParas
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
		v.Paras = append(v.Paras, spans)
	}
	return v
}

func (a *View) snapshots(c *ui.Context) {
	t := a.Palette.Colors().Theme
	w := a
	a.Palette.Colors().Modal(c, &w.SnapsOpen, func() {
		shown := a.snapView()
		same := shown.Same // nothing to restore
		ui.Column(c).Width(kit.DialogWide).Height(kit.DialogTall).Padding(kit.DialogPad).Gap(16).Children(func() {
			ai := ""
			if a.AI.Enabled("ai") {
				ai = snapstext.SnapAboutAI()
			}
			a.Palette.Colors().DialogHead(c, snapstext.SnapTitle(a.title()), snapstext.SnapAbout(ai))
			ui.Row(c).FillWidth().Grow(1).Gap(16).AlignItems(ui.Stretch).Children(func() {
				ui.Scroll(c.Key("snaps")).Width(200).FillHeight().Shrink(0).Children(func() {
					ui.Column(c).FillWidth().Gap(2).Children(func() {
						if len(w.Snaps) == 0 {
							ui.Text(c, snapstext.SnapNone()).FontSize(kit.FsControl).TextColor(a.Palette.Colors().Faint).Padding(8)
						}
						for _, sn := range w.Snaps {
							on := sn.Name == w.Snap
							at := kit.When(sn.At, time.Now())
							b := ui.ButtonBase(c.Key("s-"+sn.Name)).Column().FillWidth().AlignItems(ui.Start).Gap(2).Padding(8, 10).Radius(kit.RControl).Label("快照-" + at)
							if on {
								b.Background(t.SurfacePressed)
							} else if b.Hovered() {
								b.Background(t.SurfaceHover)
							}
							b.Children(func() {
								ui.Text(c, at).FontSize(kit.FsControl)
								ui.Text(c, snapstext.SnapRowInfo(a.snapReason(sn.Reason), sn.Count, a.Book.Store().Work().Language.Unit())).FontSize(kit.FsNote).TextColor(t.TextMuted)
							})
							if b.Clicked() {
								a.pickSnapshot(sn.Name)
							}
						}
					})
				})
				ui.Column(c).Grow(1).Shrink(1).FillHeight().Gap(10).Children(func() {
					ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
						ui.Checkbox(c, &w.Compare, snapstext.SnapCompare())
						if same {
							ui.Text(c, snapstext.SnapSame()).FontSize(kit.FsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
						} else if w.Compare {
							ui.Text(c, snapstext.SnapDiffHint()).FontSize(kit.FsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
						}
					})
					if i := slices.IndexFunc(w.Snaps, func(s workspace.Snapshot) bool { return s.Name == w.Snap }); i >= 0 && a.AI.Enabled("ai") {
						switch sn := w.Snaps[i]; {
						case len(sn.Talks) > 0:
							a.talkRecord(c, sn.Sources)
						case sn.AI != nil:
							a.aiRecord(c, sn.AI)
						}
					}
					// The paragraphs select as one text, and copy a line each.
					para := func(i int) {
						switch spans := shown.Paras[i]; {
						case len(spans) == 0: // an empty line keeps its height
							ui.Text(c, "").FontSize(kit.FsRead).LineHeight(kit.LhRead).FillWidth()
						case len(spans) == 1 && spans[0] == (ui.Span{Text: spans[0].Text}):
							ui.Text(c, spans[0].Text).FontSize(kit.FsRead).LineHeight(kit.LhRead).FillWidth()
						default:
							ui.RichText(c, spans...).FontSize(kit.FsRead).LineHeight(kit.LhRead).FillWidth()
						}
					}
					if shown.Long {
						ui.Box(c).FillWidth().Grow(1).Background(a.Palette.Colors().Canvas).Border(1, a.Palette.Colors().Line).Radius(kit.RControl).Selectable().Children(func() {
							ui.List(c, &shown.List, len(shown.Paras), para).Fill().Padding(12, 16)
						})
					} else {
						ui.Scroll(c.Key("Snap-text")).FillWidth().Grow(1).Background(a.Palette.Colors().Canvas).Border(1, a.Palette.Colors().Line).Radius(kit.RControl).Children(func() {
							ui.Column(c).FillWidth().Padding(12, 16).Selectable().Children(func() {
								for i := range shown.Paras {
									para(i)
								}
							})
						})
					}
				})
			})
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Spacer(c)
				if a.Palette.Colors().Secondary(c, locales.CommonClose()).Clicked() {
					w.SnapsOpen = false
				}
				if a.Palette.Colors().Primary(c, snapstext.SnapRestore()).Disabled(w.Snap == "" || same || a.lost() || a.paused()).Clicked() {
					a.restoreSnapshot()
				}
			})
		})
	})
}

// restoreSnapshot replaces the text with the snapshot chosen, keeping the
// text replaced as a snapshot of its own.
func (a *View) restoreSnapshot() {
	w := a
	i := slices.IndexFunc(w.Snaps, func(s workspace.Snapshot) bool { return s.Name == w.Snap })
	if i < 0 {
		return
	}
	if err := a.Editing.RestoreSnapshot(w.Snaps[i], w.SnapText); err != nil {
		a.Notices.Notices().Warn(err)
		return
	}
	w.SnapsOpen = false
	a.Notices.Notices().Did(snapstext.SnapRestored(kit.When(w.Snaps[i].At, time.Now())))
}
