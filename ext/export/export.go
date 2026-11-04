package export

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/ext"
	"writer/ext/export/format"
	"writer/ext/hostapi"
	"writer/internal/atomicfile"
	"writer/locales"
	"writer/workspace"
)

// Dialog is the export dialog: the work as it is exported, read
// when the dialog opens; the range chosen, with the first and last
// chapters of 指定范围 by the names the drop-downs show; the pen name; and
// whether the paragraphs of a text are indented.
type Dialog struct {
	book       *ext.Ref[hostapi.Book]
	palette    *ext.Ref[hostapi.Palette]
	notices    *ext.Ref[hostapi.Notices]
	jobs       *ext.Ref[hostapi.Jobs]
	dialogs    *ext.Ref[hostapi.Dialogs]
	setting    *ext.Pref[Choices]
	choices    *Choices
	generation uint64
	planned    planKey
	Open       bool
	pieces     []workspace.Piece
	selection  format.Selection
	pen        string
	indent     bool
	plan       format.Plan
}

// preferences is how the export dialog was last left on this machine; the
// first time, the prose alone, as Word.
func (d *Dialog) preferences() *Choices {
	if d.choices == nil {
		v := d.setting.Get()
		d.choices = &v
	}
	return d.choices
}

func (d *Dialog) saveChoices() { d.setting.Set(*d.preferences()) }

type planKey struct {
	book    *book.Book
	version int
	chapter string
	locale  string
}

func (d *Dialog) planKey() planKey {
	b := d.book.Get().Content()
	id, _, _ := d.book.Get().Chapter()
	return planKey{b, b.Version(), id, i18n.Locale()}
}

// Show asks what to export, starting from the whole work, the
// pen name the work had last, and 指定范围 from the first chapter to
// the third.
func (d *Dialog) Show() {
	d.book.Get().Flush()
	store := d.book.Get().Store()
	pieces, err := store.Pieces()
	if err != nil {
		d.notices.Get().Notices().Warn(err)
		return
	}
	fresh := *d
	fresh.Open, fresh.pieces, fresh.selection, fresh.pen, fresh.indent = true, pieces, format.Selection{Scope: format.ScopeWork}, store.Work().Pen, true
	fresh.choices = nil
	fresh.generation++
	if fresh.pen == "" {
		fresh.pen = fresh.preferences().Pen
	}
	if names, _ := format.ChapterNames(pieces); len(names) > 0 {
		fresh.selection.From, fresh.selection.To = names[0], names[min(2, len(names)-1)]
	}
	*d = fresh
	d.Replan()
}

// Replan keeps a plan for the choices and the current chapter, using the
// prose and outlines read when the dialog opened.
func (d *Dialog) Replan() {
	if !d.Open {
		return
	}
	set := d.preferences()
	id, _, _ := d.book.Get().Chapter()
	d.planned = d.planKey()
	d.plan = format.BuildPlan(d.pieces, d.book.Get().Content().Setting().Entries, id, set.Options, d.selection)
	d.selection.Scope = d.plan.Scope
}

// View is the export dialog: of the prose, the outline and the
// setting, what is ticked, each its own file, as Word or as plain text;
// the prose and the outline of the range chosen.
func (d *Dialog) View(c *ui.Context) {
	if !d.Open {
		return
	}
	pal := d.palette.Get().Colors()
	t := pal.Theme
	set := d.preferences()
	if d.planned != d.planKey() {
		d.Replan()
	}
	pal.Modal(c, &d.Open, func() {
		plan := &d.plan
		ui.Column(c).Width(kit.DialogSmall).Padding(kit.DialogPad).Gap(16).Children(func() {
			pal.DialogHead(c, locales.ExportTitle(), aboutLabel(plan.About))
			kit.Labeled(c, locales.ExportContent(), func() {
				ui.Row(c).Gap(24).MinHeight(kit.HControl).AlignItems(ui.Center).Children(func() {
					for _, o := range []struct {
						on   *bool
						what string
					}{{&set.Prose, format.ProseContent}, {&set.Outline, format.OutlineContent}, {&set.Setting, format.SettingContent}} {
						if ui.Checkbox(c, o.on, contentLabel(o.what)).Changed() {
							d.saveChoices()
							d.Replan()
						}
					}
				})
			})
			kit.Labeled(c, locales.ExportFormat(), func() {
				if kit.Dots(c, &set.Text, locales.ExportFormatAria(), []bool{false, true}, []string{"Word", locales.ExportPlain()}) {
					d.saveChoices()
					d.Replan()
				}
			})
			if set.Prose || set.Outline {
				kit.Labeled(c, locales.ExportRange(), func() {
					if kit.Dots(c, &d.selection.Scope, locales.ExportRangeAria(), plan.Scopes, scopeLabels(plan.Scopes)) {
						d.Replan()
					}
				})
				if d.selection.Scope == format.ScopeSpan {
					d.span(c)
				}
			}
			if set.Prose && !set.Text {
				kit.Labeled(c, locales.ExportPen(), func() {
					ui.TextInput(c, &d.pen).Label(locales.ExportPen()).Placeholder(locales.ExportPenHint()).FillWidth()
				})
			}
			if set.Prose && set.Text {
				ui.Checkbox(c, &d.indent, locales.ExportIndent())
			}
			for _, what := range plan.Ticked {
				if why := plan.Skip[what]; why != "" {
					ui.Text(c, skipLabel(what, why)).FontSize(kit.FsNote).TextColor(t.TextMuted)
				}
			}
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				ui.Spacer(c)
				if pal.Secondary(c, locales.CommonCancel()).Clicked() {
					d.Open = false
				}
				if pal.Primary(c, locales.ExportTitle()).Disabled(len(plan.Going) == 0).Clicked() {
					d.export(c, d.plan)
				}
			})
		})
	})
}

// span is 指定范围: from one chapter to another, the second only
// from those after the first.
func (d *Dialog) span(c *ui.Context) {
	pal := d.palette.Get().Colors()
	names := d.plan.Names
	at := func(name string) int { return max(0, slices.Index(names, name)) }
	ui.Row(c).FillWidth().Gap(12).Children(func() {
		kit.Labeled(c, locales.ExportFrom(), func() {
			if pal.Select(c, &d.selection.From, names).Label(locales.ExportFrom()).FillWidth().MinWidth(0).Changed() {
				if at(d.selection.To) < at(d.selection.From) {
					d.selection.To = d.selection.From
				}
				d.Replan()
			}
		})
		kit.Labeled(c, locales.ExportTo(), func() {
			if pal.Select(c, &d.selection.To, names[at(d.selection.From):]).Label(locales.ExportTo()).FillWidth().MinWidth(0).Changed() {
				d.Replan()
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
func (d *Dialog) export(c *ui.Context, plan format.Plan) {
	generation := d.generation
	status := d.notices.Get().Notices()
	content := d.book.Get().Content()
	set := d.preferences()
	store := d.book.Get().Store()
	work := store.Work()
	suffix := ".docx"
	if set.Text {
		suffix = ".txt"
	}
	pen := strings.TrimSpace(d.pen)
	type file struct{ name, data string }
	var files []file
	for _, what := range plan.Going {
		var doc format.Doc
		parts := []string{work.Name}
		switch what {
		case format.ProseContent:
			doc = format.Prose(format.Cover{Title: work.Name, Pen: pen, Words: kit.Words(plan.Words)}, plan.Pieces, plan.Whole)
			parts = append(parts, plan.Part)
		case format.OutlineContent:
			doc = format.Outline(work.Name, plan.Pieces)
			parts = append(parts, plan.Part, "大纲")
		case format.SettingContent:
			doc = format.Setting(work.Name, work.Categories, content.Setting().Entries, func(entry workspace.Entry) []string { return laterLines(content, entry) })
			parts = append(parts, "设定")
		}
		data := doc.Text(what == format.ProseContent && d.indent)
		if !set.Text {
			b, err := doc.Docx()
			if err != nil {
				status.Warn(err)
				return
			}
			data = string(b)
		}
		name := strings.Join(slices.DeleteFunc(parts, func(s string) bool { return s == "" }), "-")
		files = append(files, file{fileName.Replace(name) + suffix, data})
	}
	write := func(dir string) {
		if d.generation != generation || !d.Open {
			return
		}
		var path string
		for _, f := range files {
			path = filepath.Join(dir, f.name)
			if err := atomicfile.Write(path, []byte(f.data), 0644); err != nil {
				status.Warn(err)
				return
			}
		}
		d.Open = false
		if set.Prose && !set.Text && pen != work.Pen {
			work.Pen = pen
			current := d.book.Get().Store()
			status.Warn(current.SaveWork(work))
		}
		if set.Prose && !set.Text && pen != set.Pen {
			set.Pen = pen
			d.saveChoices()
		}
		status.Did(locales.ExportedOne(path))
		if len(files) > 1 {
			status.Did(locales.ExportedMany(len(files), dir))
		}
	}
	save, _ := d.dialogs.Get().Dialogs()
	if len(files) == 1 && save != nil {
		sv := c.Services()
		d.jobs.Get().StartJob(sv, func() func() {
			path, err := save(locales.ExportTitle(), files[0].name)
			return func() {
				if d.generation != generation || !d.Open {
					return
				}
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
	if len(files) > 1 && d.dialogs.Get().PickInto(c, locales.ExportInto(), write) {
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

// Plan is the export plan held for the current choices and chapter.
// The returned slices and maps are read-only.
func (d *Dialog) Plan() format.Plan { return d.plan }

// Selection is the chosen scope and the chapter names at its ends.
func (d *Dialog) Selection() format.Selection { return d.selection }

// Presentation stays separate from the export package's stored values and file names.
func contentLabel(what string) string {
	switch what {
	case format.ProseContent:
		return locales.ExportBody()
	case format.OutlineContent:
		return locales.PagePlan()
	case format.SettingContent:
		return locales.ExportSettings()
	}
	return what
}

func scopeLabels(scopes []string) []string {
	labels := make([]string, len(scopes))
	for i, scope := range scopes {
		switch scope {
		case format.ScopeWork:
			labels[i] = locales.ExportRangeBook()
		case format.ScopeVolume:
			labels[i] = locales.ExportRangeVolume()
		case format.ScopeChapter:
			labels[i] = locales.ExportRangeChapter()
		case format.ScopeSpan:
			labels[i] = locales.ExportRangeCustom()
		}
	}
	return labels
}

func aboutLabel(about string) string {
	switch about {
	case format.AboutMany:
		return locales.ExportAboutMany()
	case format.AboutLore:
		return locales.ExportAboutSettings()
	case format.AboutOutline:
		return locales.ExportAboutOutline()
	case format.AboutText:
		return locales.ExportAboutText()
	default:
		return locales.ExportAboutWord()
	}
}

// skipLabel says why what is not exported, from the plan's Skip code.
func skipLabel(what, why string) string {
	switch why {
	case format.SkipNoChapters:
		if what == format.ProseContent {
			return locales.ExportSkipBody()
		}
		return locales.ExportSkipOutlineNone()
	case format.SkipNoOutlines:
		return locales.ExportSkipOutlineEmpty()
	case format.SkipNoLore:
		return locales.ExportSkipSettings()
	}
	return ""
}
