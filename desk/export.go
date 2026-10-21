package desk

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"writer/export"
	"writer/shelf"
	"writer/workspace"
)

// The ranges of the export dialog.
const (
	scopeWork    = "全书"
	scopeVolume  = "本卷"
	scopeChapter = "本章"
	scopeSpan    = "指定范围"
)

// What the export dialog exports, each its own file.
const (
	exportProse   = "正文"
	exportOutline = "大纲"
	exportSetting = "设定"
)

// exportState is the export dialog: the work as it is exported, read
// when the dialog opens; the range chosen, with the first and last
// chapters of 指定范围 by the names the drop-downs show; the pen name; and
// whether the paragraphs of a text are indented.
type exportState struct {
	open     bool
	pieces   []workspace.Piece
	scope    string
	from, to string
	pen      string
	indent   bool
}

// exportPrefs is how the export dialog was last left on this machine; the
// first time, the prose alone, as Word.
func (a *App) exportPrefs() *shelf.Export {
	if a.shelf.Export == nil {
		a.shelf.Export = &shelf.Export{Prose: true}
	}
	return a.shelf.Export
}

// openExport asks what to export, starting from the whole work, the
// pen name the work had last, and 指定范围 from the first chapter to
// the third.
func (a *App) openExport() {
	a.flush()
	pieces, err := a.Store.Pieces()
	if err != nil {
		a.warn(err)
		return
	}
	x := exportState{open: true, pieces: pieces, scope: scopeWork, pen: a.Store.Work().Pen, indent: true}
	if x.pen == "" {
		x.pen = a.exportPrefs().Pen
	}
	if names, _ := chapterNames(pieces); len(names) > 0 {
		x.from, x.to = names[0], names[min(2, len(names)-1)]
	}
	a.write.exporting = x
}

// chapterNames are the chapters of pieces by the names the drop-downs of
// 指定范围 show, a number after a name some chapter before already has;
// ids are the chapters by those names.
func chapterNames(pieces []workspace.Piece) (names []string, ids map[string]string) {
	ids = map[string]string{}
	for _, p := range pieces {
		if p.Folder {
			continue
		}
		name := p.Title
		for i := 2; ids[name] != ""; i++ {
			name = p.Title + " (" + strconv.Itoa(i) + ")"
		}
		names, ids[name] = append(names, name), p.ID
	}
	return names, ids
}

// exportPlan is what the dialog would export as it stands.
type exportPlan struct {
	scopes []string          // the ranges the dialog offers
	pieces []workspace.Piece // of the prose and the outline, in the range
	whole  bool              // the range is the whole work
	part   string            // the range in the names of the files
	skip   map[string]string // why what is ticked cannot be exported
	going  []string          // what is ticked and can be, in the dialog's order
	about  string            // the dialog's description
	words  int               // of the prose in the range
}

func (a *App) exportPlan() exportPlan {
	x, set := &a.write.exporting, a.exportPrefs()
	var plan exportPlan
	// The chapter open on the page of prose, and the volume it is in, or
	// the volume itself when that is what is open.
	cur, vol := -1, -1
	if a.page == pageWrite {
		for i, p := range x.pieces {
			if p.ID == a.write.node {
				cur = i
			}
		}
	}
	if cur >= 0 && x.pieces[cur].Folder {
		cur, vol = -1, cur
	} else if cur >= 0 {
		for i := cur - 1; i >= 0 && x.pieces[cur].Depth > 0; i-- {
			if x.pieces[i].Depth < x.pieces[cur].Depth {
				vol = i
				break
			}
		}
	}
	names, ids := chapterNames(x.pieces)
	plan.scopes = []string{scopeWork}
	if vol >= 0 {
		plan.scopes = append(plan.scopes, scopeVolume)
	}
	if cur >= 0 {
		plan.scopes = append(plan.scopes, scopeChapter)
	}
	if len(names) > 1 {
		plan.scopes = append(plan.scopes, scopeSpan)
	}
	scope := x.scope
	if !slices.Contains(plan.scopes, scope) {
		scope = scopeWork
	}
	chapters := map[string]bool{}
	switch scope {
	case scopeWork:
		chapters, plan.whole = nil, true
	case scopeVolume:
		v := x.pieces[vol]
		for _, p := range x.pieces[vol+1:] {
			if p.Depth <= v.Depth {
				break
			}
			chapters[p.ID] = true
		}
		plan.part = v.Title
	case scopeChapter:
		chapters[x.pieces[cur].ID] = true
		plan.part = x.pieces[cur].Title
	case scopeSpan:
		in := false
		for _, name := range names {
			in = in || name == x.from
			if in {
				chapters[ids[name]] = true
			}
			if name == x.to {
				break
			}
		}
		plan.part = titleOf(x.pieces, ids[x.from])
		if x.to != x.from {
			plan.part += "至" + titleOf(x.pieces, ids[x.to])
		}
	}
	plan.pieces = export.Within(x.pieces, chapters)
	chaptered, outlined := false, false
	for _, p := range plan.pieces {
		chaptered = chaptered || !p.Folder
		outlined = outlined || p.Outline != ""
		plan.words += workspace.Count(p.Text)
	}
	plan.skip = map[string]string{}
	if !chaptered {
		plan.skip[exportProse] = "还没有章节，不导出正文。"
		plan.skip[exportOutline] = "还没有章节，不导出大纲。"
	} else if !outlined {
		plan.skip[exportOutline] = "所选范围内还没有写章纲，不导出大纲。"
	}
	if !export.Settled(a.set.Entries) {
		plan.skip[exportSetting] = "还没有已定的设定，不导出设定。"
	}
	ticked := a.exportTicked()
	for _, what := range ticked {
		if plan.skip[what] == "" {
			plan.going = append(plan.going, what)
		}
	}
	switch {
	case len(ticked) > 1:
		plan.about = "勾选的几项各存为一个文件，放在同一文件夹。"
	case set.Setting:
		plan.about = "按分类列出已定的设定，不含待确认的。"
	case set.Outline:
		plan.about = "按章节顺序列出各卷、各章的纲。"
	case set.Text:
		plan.about = "合并为一个文件，各卷和各章前加上标题。"
	default:
		plan.about = "首页写书名、笔名和字数，每章另起一页。"
	}
	return plan
}

// titleOf is the title of the piece id.
func titleOf(pieces []workspace.Piece, id string) string {
	for _, p := range pieces {
		if p.ID == id {
			return p.Title
		}
	}
	return ""
}

// exportTicked is what is ticked, in the dialog's order.
func (a *App) exportTicked() []string {
	set := a.exportPrefs()
	var out []string
	for _, o := range []struct {
		on   bool
		what string
	}{{set.Prose, exportProse}, {set.Outline, exportOutline}, {set.Setting, exportSetting}} {
		if o.on {
			out = append(out, o.what)
		}
	}
	return out
}

// exportView is the export dialog: of the prose, the outline and the
// setting, what is ticked, each its own file, as Word or as plain text;
// the prose and the outline of the range chosen.
func (a *App) exportView(c *ui.Context) {
	t := a.pal.theme
	x, set := &a.write.exporting, a.exportPrefs()
	a.modal(c, &x.open, func() {
		plan := a.exportPlan()
		ui.Column(c).Width(dialogSmall).Padding(dialogPad).Gap(16).Children(func() {
			a.pal.dialogHead(c, "导出", plan.about)
			labeled(c, "内容", func() {
				ui.Row(c).Gap(24).MinHeight(hControl).AlignItems(ui.Center).Children(func() {
					for _, o := range []struct {
						on   *bool
						what string
					}{{&set.Prose, exportProse}, {&set.Outline, exportOutline}, {&set.Setting, exportSetting}} {
						if ui.Checkbox(c, o.on, o.what).Changed() {
							a.saveShelf()
						}
					}
				})
			})
			labeled(c, "格式", func() {
				if dots(c, &set.Text, "导出格式", []bool{false, true}, []string{"Word", "纯文本"}) {
					a.saveShelf()
				}
			})
			if set.Prose || set.Outline {
				if !slices.Contains(plan.scopes, x.scope) {
					x.scope = scopeWork
				}
				labeled(c, "范围", func() {
					dots(c, &x.scope, "导出范围", plan.scopes, plan.scopes)
				})
				if x.scope == scopeSpan {
					a.exportSpan(c)
				}
			}
			if set.Prose && !set.Text {
				labeled(c, "笔名", func() {
					ui.TextInput(c, &x.pen).Label("笔名").Placeholder("选填，写在首页").FillWidth()
				})
			}
			if set.Prose && set.Text {
				ui.Checkbox(c, &x.indent, "段首加两个全角空格")
			}
			for _, what := range a.exportTicked() {
				if note := plan.skip[what]; note != "" {
					ui.Text(c, note).FontSize(fsNote).TextColor(t.TextMuted)
				}
			}
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				ui.Spacer(c)
				if a.pal.secondary(c, "取消").Clicked() {
					x.open = false
				}
				if a.pal.primary(c, "导出").Disabled(len(plan.going) == 0).Clicked() {
					a.export(c, plan)
				}
			})
		})
	})
}

// exportSpan is 指定范围: from one chapter to another, the second only
// from those after the first.
func (a *App) exportSpan(c *ui.Context) {
	x := &a.write.exporting
	names, _ := chapterNames(x.pieces)
	at := func(name string) int { return max(0, slices.Index(names, name)) }
	ui.Row(c).FillWidth().Gap(12).Children(func() {
		labeled(c, "从", func() {
			if ui.Select(c, &x.from, names).Label("从").FillWidth().MinWidth(0).Changed() && at(x.to) < at(x.from) {
				x.to = x.from
			}
		})
		labeled(c, "到", func() {
			ui.Select(c, &x.to, names[at(x.from):]).Label("到").FillWidth().MinWidth(0)
		})
	})
}

// fileName is a name the systems take for a file: what they do not is a
// space, as is the full-width colon between a chapter's number and title.
var fileName = strings.NewReplacer("：", " ", `\`, " ", "/", " ", ":", " ", "*", " ", "?", " ", `"`, " ", "<", " ", ">", " ", "|", " ")

// export writes what plan exports where the author picks: a file for one
// thing, a folder for several, or the work's 导出 folder when there is no
// dialog to ask with.
func (a *App) export(c *ui.Context, plan exportPlan) {
	x, set := &a.write.exporting, a.exportPrefs()
	work := a.Store.Work()
	ext := ".docx"
	if set.Text {
		ext = ".txt"
	}
	pen := strings.TrimSpace(x.pen)
	type file struct{ name, data string }
	var files []file
	for _, what := range plan.going {
		var doc export.Doc
		parts := []string{work.Name}
		switch what {
		case exportProse:
			doc = export.Prose(export.Cover{Title: work.Name, Pen: pen, Words: words(plan.words)}, plan.pieces, plan.whole)
			parts = append(parts, plan.part)
		case exportOutline:
			doc = export.Outline(work.Name, plan.pieces)
			parts = append(parts, plan.part, "大纲")
		case exportSetting:
			doc = export.Setting(work.Name, work.Categories, a.set.Entries, a.laterLines)
			parts = append(parts, "设定")
		}
		data := doc.Text(what == exportProse && x.indent)
		if !set.Text {
			b, err := doc.Docx()
			if err != nil {
				a.warn(err)
				return
			}
			data = string(b)
		}
		name := strings.Join(slices.DeleteFunc(parts, func(s string) bool { return s == "" }), "-")
		files = append(files, file{fileName.Replace(name) + ext, data})
	}
	write := func(dir string) {
		var path string
		for _, f := range files {
			path = filepath.Join(dir, f.name)
			if err := os.WriteFile(path, []byte(f.data), 0644); err != nil {
				a.warn(err)
				return
			}
		}
		x.open = false
		if set.Prose && !set.Text && pen != work.Pen {
			work.Pen = pen
			a.warn(a.Store.SaveWork(work))
		}
		if set.Prose && !set.Text && pen != set.Pen {
			set.Pen = pen
			a.saveShelf()
		}
		a.notice = "已导出到 " + path
		if len(files) > 1 {
			a.notice = fmt.Sprintf("已导出 %d 个文件到 %s", len(files), dir)
		}
	}
	if len(files) == 1 && a.SaveFile != nil {
		save, sv := a.SaveFile, c.Services()
		go func() {
			path, err := save("导出", files[0].name)
			a.picks <- func() {
				if err != nil {
					a.warn(err)
				} else if path != "" {
					files[0].name = filepath.Base(path)
					write(filepath.Dir(path))
				}
			}
			sv.Invalidate()
		}()
		return
	}
	if len(files) > 1 && a.PickFolder != nil {
		a.pickInto(c, "导出到", write)
		return
	}
	dir := filepath.Join(a.Store.Root(), "导出")
	if err := os.MkdirAll(dir, 0755); err != nil {
		a.warn(err)
		return
	}
	write(dir)
}
