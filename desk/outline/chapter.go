package outline

import (
	"github.com/egoist/mygo/ui"
	"strings"
	"time"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/locales"
)

// Draft returns the live chapter draft, including unsaved edits, on the frame thread.
func (p *Page) Draft(id string) *book.OutlineDraft {
	if p.drafts == nil {
		p.drafts = map[string]*book.OutlineDraft{}
	}
	if d := p.drafts[id]; d != nil {
		return d
	}
	text := ""
	if b := p.Services.Book.Content(); b != nil {
		text = b.Outlines()[id]
	}
	d := &book.OutlineDraft{Node: id, Text: text, Saved: text, Seen: text}
	p.drafts[id] = d
	return d
}

// SetExpanded changes the writing header's disclosure state on the frame thread.
func (p *Page) SetExpanded(open bool) { p.chapterExpanded = open }

// SaveChapter flushes the writing draft through the host save lifecycle.
func (p *Page) SaveChapter() error {
	id, lost, _ := p.Services.Desktop.Selected()
	if lost {
		return nil
	}
	if id == "" {
		return nil
	}
	if _, ok := p.Services.Book.Content().Tree().Node(id); !ok {
		return nil
	}
	return p.Draft(id).Save(p.Services.Book.Content())
}

// WatchChapter saves the writing draft after its normal debounce, before page drawing.
func (p *Page) WatchChapter(c *ui.Context) {
	id, lost, paused := p.Services.Desktop.Selected()
	if lost || paused || !p.Services.Desktop.Writing() {
		return
	}
	if id == "" {
		return
	}
	d := p.Draft(id)
	d.Change(time.Now())
	if wait, due := d.Waits(p.Services.Book.Content().Timing.SaveAfter, time.Now()); due {
		if wait > 0 {
			c.After(wait)
		} else {
			p.Services.Notices.Notices().Warn(p.SaveChapter())
		}
	}
}

// ChapterHead draws the chapter outline and story time below the writing title.
func (p *Page) ChapterHead(c *ui.Context) {
	id, _, _ := p.Services.Book.Chapter()
	n, ok := p.Services.Book.Content().Tree().Node(id)
	if ok && n.Folder {
		p.chapterBox(c, locales.EditorVolumeOutline(), 160)
		return
	}
	label := locales.EditorOutline()
	if strings.TrimSpace(p.Draft(id).Text) == "" {
		label = locales.EditorOutlineBlank()
	}
	var parts ui.CollapsibleParts
	ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() { parts = p.disclosure(c, label, &p.chapterExpanded); p.ChapterStory(c) })
	parts.Panel(func() {
		ui.Box(c).FillWidth().Padding(2, 0, 4, 0).Children(func() { p.chapterBox(c, locales.EditorOutline(), 96) })
	})
}

func (p *Page) chapterBox(c *ui.Context, label string, height float32) {
	id, _, _ := p.Services.Book.Chapter()
	_, exists := p.Services.Book.Content().Tree().Node(id)
	ui.TextAreaBase(c, &p.Draft(id).Text).Label(label).Placeholder(locales.EditorOutlineHint()).FillWidth().Height(height).
		FontSize(kit.FsBody).LineHeight(kit.LhLoose).Padding(10, 12).Radius(kit.RControl).Background(p.Services.Palette.Colors().Side).Disabled(!exists)
}

func (p *Page) disclosure(c *ui.Context, label string, open *bool) ui.CollapsibleParts {
	t := p.Services.Palette.Colors().Theme
	parts := ui.CollapsibleBase(c, open)
	color := t.TextMuted
	if parts.Trigger.Hovered() || *open {
		color = t.Text
	}
	arrow := " ▾"
	if *open {
		arrow = " ▴"
	}
	parts.Trigger.Label(label).Padding(2, 0).Children(func() {
		ui.Text(c, label+arrow).FontSize(kit.FsNote).TextColor(color).SingleLine()
	})
	return parts
}

// ChapterOutline returns the live draft for model context, including unsaved edits.
func (p *Page) ChapterOutline(id string) string { return p.Draft(id).Text }

// OpenChapter resets the writing draft to the freshly read tab content.
func (p *Page) OpenChapter(id, text string) {
	d := p.Draft(id)
	d.Text, d.Saved, d.Seen = text, text, text
}
