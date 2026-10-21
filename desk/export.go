package desk

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
	"writer/export"
	"writer/shelf"
	"writer/workspace"
)

// exportState is the export dialog: the work as it is exported, read
// when the dialog opens; the range chosen, with the first and last
// chapters of 指定范围 by the names the drop-downs show; the pen name; and
// whether the paragraphs of a text are indented.
type exportState struct {
	open      bool
	pieces    []workspace.Piece
	selection export.Selection
	pen       string
	indent    bool
	plan      export.Plan
}

// exportPrefs is how the export dialog was last left on this machine; the
// first time, the prose alone, as Word.
func (a *App) exportPrefs() *shelf.Export {
	if a.shelf.Export == nil {
		a.shelf.Export = &shelf.Export{Options: export.Options{Prose: true}}
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
	x := exportState{open: true, pieces: pieces, selection: export.Selection{Scope: export.ScopeWork}, pen: a.Store.Work().Pen, indent: true}
	if x.pen == "" {
		x.pen = a.exportPrefs().Pen
	}
	if names, _ := export.ChapterNames(pieces); len(names) > 0 {
		x.selection.From, x.selection.To = names[0], names[min(2, len(names)-1)]
	}
	a.write.exporting = x
	a.replanExport()
}

// replanExport keeps a plan for the choices and the current chapter, using the
// prose and outlines read when the dialog opened.
func (a *App) replanExport() {
	x := &a.write.exporting
	if !x.open {
		return
	}
	set := a.exportPrefs()
	x.plan = export.BuildPlan(x.pieces, a.set.Entries, a.exportChapter(), set.Options, x.selection)
	x.selection.Scope = x.plan.Scope
}

func (a *App) exportChapter() string {
	if a.page == pageWrite {
		return a.write.node
	}
	return ""
}

// exportView is the export dialog: of the prose, the outline and the
// setting, what is ticked, each its own file, as Word or as plain text;
// the prose and the outline of the range chosen.
func (a *App) exportView(c *ui.Context) {
	t := a.pal.theme
	x, set := &a.write.exporting, a.exportPrefs()
	if x.plan.Current != a.exportChapter() {
		a.replanExport()
	}
	a.modal(c, &x.open, func() {
		plan := &x.plan
		ui.Column(c).Width(dialogSmall).Padding(dialogPad).Gap(16).Children(func() {
			a.pal.dialogHead(c, "导出", plan.About)
			labeled(c, "内容", func() {
				ui.Row(c).Gap(24).MinHeight(hControl).AlignItems(ui.Center).Children(func() {
					for _, o := range []struct {
						on   *bool
						what string
					}{{&set.Prose, export.ProseContent}, {&set.Outline, export.OutlineContent}, {&set.Setting, export.SettingContent}} {
						if ui.Checkbox(c, o.on, o.what).Changed() {
							a.saveShelf()
							a.replanExport()
						}
					}
				})
			})
			labeled(c, "格式", func() {
				if dots(c, &set.Text, "导出格式", []bool{false, true}, []string{"Word", "纯文本"}) {
					a.saveShelf()
					a.replanExport()
				}
			})
			if set.Prose || set.Outline {
				labeled(c, "范围", func() {
					if dots(c, &x.selection.Scope, "导出范围", plan.Scopes, plan.Scopes) {
						a.replanExport()
					}
				})
				if x.selection.Scope == export.ScopeSpan {
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
			for _, what := range plan.Ticked {
				if note := plan.Skip[what]; note != "" {
					ui.Text(c, note).FontSize(fsNote).TextColor(t.TextMuted)
				}
			}
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				ui.Spacer(c)
				if a.pal.secondary(c, "取消").Clicked() {
					x.open = false
				}
				if a.pal.primary(c, "导出").Disabled(len(plan.Going) == 0).Clicked() {
					a.export(c, x.plan)
				}
			})
		})
	})
}

// exportSpan is 指定范围: from one chapter to another, the second only
// from those after the first.
func (a *App) exportSpan(c *ui.Context) {
	x := &a.write.exporting
	names := x.plan.Names
	at := func(name string) int { return max(0, slices.Index(names, name)) }
	ui.Row(c).FillWidth().Gap(12).Children(func() {
		labeled(c, "从", func() {
			if ui.Select(c, &x.selection.From, names).Label("从").FillWidth().MinWidth(0).Changed() {
				if at(x.selection.To) < at(x.selection.From) {
					x.selection.To = x.selection.From
				}
				a.replanExport()
			}
		})
		labeled(c, "到", func() {
			if ui.Select(c, &x.selection.To, names[at(x.selection.From):]).Label("到").FillWidth().MinWidth(0).Changed() {
				a.replanExport()
			}
		})
	})
}

// fileName is a name the systems take for a file: what they do not is a
// space, as is the full-width colon between a chapter's number and title.
var fileName = strings.NewReplacer("：", " ", `\`, " ", "/", " ", ":", " ", "*", " ", "?", " ", `"`, " ", "<", " ", ">", " ", "|", " ")

// export writes what plan exports where the author picks: a file for one
// thing, a folder for several, or the work's 导出 folder when there is no
// dialog to ask with.
func (a *App) export(c *ui.Context, plan export.Plan) {
	x, set := &a.write.exporting, a.exportPrefs()
	work := a.Store.Work()
	ext := ".docx"
	if set.Text {
		ext = ".txt"
	}
	pen := strings.TrimSpace(x.pen)
	type file struct{ name, data string }
	var files []file
	for _, what := range plan.Going {
		var doc export.Doc
		parts := []string{work.Name}
		switch what {
		case export.ProseContent:
			doc = export.Prose(export.Cover{Title: work.Name, Pen: pen, Words: words(plan.Words)}, plan.Pieces, plan.Whole)
			parts = append(parts, plan.Part)
		case export.OutlineContent:
			doc = export.Outline(work.Name, plan.Pieces)
			parts = append(parts, plan.Part, "大纲")
		case export.SettingContent:
			doc = export.Setting(work.Name, work.Categories, a.set.Entries, a.laterLines)
			parts = append(parts, "设定")
		}
		data := doc.Text(what == export.ProseContent && x.indent)
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
