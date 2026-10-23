package desk

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/export"
	"writer/shelf"
	"writer/workspace"
)

// exportView is the export dialog: the work as it is exported, read
// when the dialog opens; the range chosen, with the first and last
// chapters of 指定范围 by the names the drop-downs show; the pen name; and
// whether the paragraphs of a text are indented.
type exportView struct {
	host      host
	open      bool
	pieces    []workspace.Piece
	selection export.Selection
	pen       string
	indent    bool
	plan      export.Plan
}

// preferences is how the export dialog was last left on this machine; the
// first time, the prose alone, as Word.
func (e *exportView) preferences() *shelf.Export {
	if e.host.shelfPrefs().Export == nil {
		e.host.shelfPrefs().Export = &shelf.Export{Options: export.Options{Prose: true}}
	}
	return e.host.shelfPrefs().Export
}

// show asks what to export, starting from the whole work, the
// pen name the work had last, and 指定范围 from the first chapter to
// the third.
func (e *exportView) show() {
	e.host.flush()
	store, _ := e.host.workspace()
	pieces, err := store.Pieces()
	if err != nil {
		e.host.notices().Warn(err)
		return
	}
	fresh := exportView{host: e.host, open: true, pieces: pieces, selection: export.Selection{Scope: export.ScopeWork}, pen: store.Work().Pen, indent: true}
	if fresh.pen == "" {
		fresh.pen = e.preferences().Pen
	}
	if names, _ := export.ChapterNames(pieces); len(names) > 0 {
		fresh.selection.From, fresh.selection.To = names[0], names[min(2, len(names)-1)]
	}
	*e = fresh
	e.replan()
}

// replan keeps a plan for the choices and the current chapter, using the
// prose and outlines read when the dialog opened.
func (e *exportView) replan() {
	if !e.open {
		return
	}
	set := e.preferences()
	id, _, _ := e.host.chapter()
	e.plan = export.BuildPlan(e.pieces, e.host.content().Setting().Entries, id, set.Options, e.selection)
	e.selection.Scope = e.plan.Scope
}

// view is the export dialog: of the prose, the outline and the
// setting, what is ticked, each its own file, as Word or as plain text;
// the prose and the outline of the range chosen.
func (e *exportView) view(c *ui.Context) {
	pal := e.host.colors()
	t := pal.Theme
	set := e.preferences()
	id, _, _ := e.host.chapter()
	if e.plan.Current != id {
		e.replan()
	}
	pal.Modal(c, &e.open, func() {
		plan := &e.plan
		ui.Column(c).Width(kit.DialogSmall).Padding(kit.DialogPad).Gap(16).Children(func() {
			pal.DialogHead(c, "导出", plan.About)
			kit.Labeled(c, "内容", func() {
				ui.Row(c).Gap(24).MinHeight(kit.HControl).AlignItems(ui.Center).Children(func() {
					for _, o := range []struct {
						on   *bool
						what string
					}{{&set.Prose, export.ProseContent}, {&set.Outline, export.OutlineContent}, {&set.Setting, export.SettingContent}} {
						if ui.Checkbox(c, o.on, o.what).Changed() {
							e.host.saveShelf()
							e.replan()
						}
					}
				})
			})
			kit.Labeled(c, "格式", func() {
				if kit.Dots(c, &set.Text, "导出格式", []bool{false, true}, []string{"Word", "纯文本"}) {
					e.host.saveShelf()
					e.replan()
				}
			})
			if set.Prose || set.Outline {
				kit.Labeled(c, "范围", func() {
					if kit.Dots(c, &e.selection.Scope, "导出范围", plan.Scopes, plan.Scopes) {
						e.replan()
					}
				})
				if e.selection.Scope == export.ScopeSpan {
					e.span(c)
				}
			}
			if set.Prose && !set.Text {
				kit.Labeled(c, "笔名", func() {
					ui.TextInput(c, &e.pen).Label("笔名").Placeholder("选填，写在首页").FillWidth()
				})
			}
			if set.Prose && set.Text {
				ui.Checkbox(c, &e.indent, "段首加两个全角空格")
			}
			for _, what := range plan.Ticked {
				if note := plan.Skip[what]; note != "" {
					ui.Text(c, note).FontSize(kit.FsNote).TextColor(t.TextMuted)
				}
			}
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				ui.Spacer(c)
				if pal.Secondary(c, "取消").Clicked() {
					e.open = false
				}
				if pal.Primary(c, "导出").Disabled(len(plan.Going) == 0).Clicked() {
					e.export(c, e.plan)
				}
			})
		})
	})
}

// span is 指定范围: from one chapter to another, the second only
// from those after the first.
func (e *exportView) span(c *ui.Context) {
	names := e.plan.Names
	at := func(name string) int { return max(0, slices.Index(names, name)) }
	ui.Row(c).FillWidth().Gap(12).Children(func() {
		kit.Labeled(c, "从", func() {
			if ui.Select(c, &e.selection.From, names).Label("从").FillWidth().MinWidth(0).Changed() {
				if at(e.selection.To) < at(e.selection.From) {
					e.selection.To = e.selection.From
				}
				e.replan()
			}
		})
		kit.Labeled(c, "到", func() {
			if ui.Select(c, &e.selection.To, names[at(e.selection.From):]).Label("到").FillWidth().MinWidth(0).Changed() {
				e.replan()
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
func (e *exportView) export(c *ui.Context, plan export.Plan) {
	status := e.host.notices()
	content := e.host.content()
	set := e.preferences()
	store, _ := e.host.workspace()
	work := store.Work()
	ext := ".docx"
	if set.Text {
		ext = ".txt"
	}
	pen := strings.TrimSpace(e.pen)
	type file struct{ name, data string }
	var files []file
	for _, what := range plan.Going {
		var doc export.Doc
		parts := []string{work.Name}
		switch what {
		case export.ProseContent:
			doc = export.Prose(export.Cover{Title: work.Name, Pen: pen, Words: kit.Words(plan.Words)}, plan.Pieces, plan.Whole)
			parts = append(parts, plan.Part)
		case export.OutlineContent:
			doc = export.Outline(work.Name, plan.Pieces)
			parts = append(parts, plan.Part, "大纲")
		case export.SettingContent:
			doc = export.Setting(work.Name, work.Categories, content.Setting().Entries, func(entry workspace.Entry) []string { return laterLines(content, entry) })
			parts = append(parts, "设定")
		}
		data := doc.Text(what == export.ProseContent && e.indent)
		if !set.Text {
			b, err := doc.Docx()
			if err != nil {
				status.Warn(err)
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
				status.Warn(err)
				return
			}
		}
		e.open = false
		if set.Prose && !set.Text && pen != work.Pen {
			work.Pen = pen
			current, _ := e.host.workspace()
			status.Warn(current.SaveWork(work))
		}
		if set.Prose && !set.Text && pen != set.Pen {
			set.Pen = pen
			e.host.saveShelf()
		}
		status.Did("已导出到 " + path)
		if len(files) > 1 {
			status.Did(fmt.Sprintf("已导出 %d 个文件到 %s", len(files), dir))
		}
	}
	save, _ := e.host.dialogs()
	if len(files) == 1 && save != nil {
		sv := c.Services()
		e.host.startJob(sv, func() func() {
			path, err := save("导出", files[0].name)
			return func() {
				if err != nil {
					status.Warn(err)
				} else if path != "" {
					files[0].name = filepath.Base(path)
					write(filepath.Dir(path))
				}
			}
		})
		return
	}
	if len(files) > 1 && e.host.pickInto(c, "导出到", write) {
		return
	}
	dir := filepath.Join(store.Root(), "导出")
	if err := os.MkdirAll(dir, 0755); err != nil {
		status.Warn(err)
		return
	}
	write(dir)
}

// laterLines are how e reads from chapters on, as the setting exports
// them: each change that holds, from where it starts.
func laterLines(b *book.Book, e workspace.Entry) []string {
	var out []string
	o := b.Tree().Order()
	for _, g := range e.Holding(o) {
		out = append(out, kit.FromLabel(b.Tree(), g.From)+"："+g.Text)
	}
	return out
}
