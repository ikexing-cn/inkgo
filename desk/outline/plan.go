package outline

import (
	"errors"
	"github.com/egoist/mygo/ui"
	"io"
	"slices"
	"strings"
	"time"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/workspace"

	"writer/locales"
)

const planFlash = 1600 * time.Millisecond

// Page holds the outline table, timeline, drafts, and card editing state.
type Page struct {
	Host                Host
	loaded              bool
	loadErr             error
	checked             time.Time // when the outline was last read
	checksPaused        bool
	index               planIndex
	geometry            *planGeometry
	dragging            string
	mode                int
	query               string
	Pane                bool
	outlineDraft        book.OutlineDraft
	outlineHeld         bool
	lineID, lineName    string
	lineHeld, lineFocus bool
	cardOpen            bool
	card                workspace.Card
	after, lastTitle    string
	added               int
	timeOpen            bool
	column              int
	when                string
	seen                string
	seenAt              time.Time
	reveal              bool
	scroll              [2]ui.ScrollState
	viewport            ui.Rect
}

type dragCard string

// WindowActivity applies the host's activity before a frame. Returned
// also covers a blur/focus pair that happened without an intervening frame.
func (p *Page) WindowActivity(active, returned bool) {
	p.checksPaused = !active
	if returned {
		p.checked = time.Time{}
	}
}

// Rebuilt once at the start of each outline frame (and after a mutation).
// Each query below then touches only the cards in its chapter or time column.
type planIndex struct {
	chapters []*workspace.Node
	live     map[string]bool
	cards    map[string][]workspace.Card
	back     map[string]bool
	counts   map[string]int
	columns  []workspace.TimeColumn
}

func (p *Page) rebuildIndex() {
	content := p.Host.Content()
	x := planIndex{live: map[string]bool{}, cards: map[string][]workspace.Card{}, back: map[string]bool{}, counts: map[string]int{}, columns: content.Plan().Columns()}
	content.Tree().Walk(func(n *workspace.Node, _ int) {
		if !n.Folder {
			x.chapters = append(x.chapters, n)
			x.live[n.ID] = true
		}
	})
	first, last := map[string]int{}, map[string]int{}
	for i, e := range content.Plan().Cards {
		id := e.Chapter
		if !x.live[id] {
			id = ""
		}
		x.cards[id] = append(x.cards[id], e)
		x.counts[e.Line]++
		if _, ok := first[id]; !ok {
			first[id] = i
		}
		last[id] = i
	}
	before := -1
	for _, n := range x.chapters {
		if i, ok := first[n.ID]; ok {
			x.back[n.ID] = i < before
			before = max(before, last[n.ID])
		}
	}
	p.index = x
}

func (p *Page) load() {
	p.loadErr = p.Host.Content().ReloadPlan()
	p.loaded = true
	p.geometry = nil
	p.rebuildIndex()
}

func (p *Page) change(err error) {
	p.Host.Notices().SetError(err)
	if err == nil {
		p.loadErr = nil
		p.geometry = nil
		p.rebuildIndex()
	}
}

func (p *Page) chapters() []*workspace.Node {
	return p.index.chapters
}

func (p *Page) cardChapter(e workspace.Card) string {
	if p.index.live[e.Chapter] {
		return e.Chapter
	}
	return ""
}

func (p *Page) cardsOf(id string) []workspace.Card {
	return p.index.cards[id]
}

func storyWhen(cards []workspace.Card) string {
	var w []string
	for _, e := range cards {
		if e.When != "" && !slices.Contains(w, e.When) {
			w = append(w, e.When)
		}
	}
	if len(w) > 1 {
		return w[0] + " — " + w[len(w)-1]
	}
	if len(w) == 1 {
		return w[0]
	}
	return ""
}

func (p *Page) back(id string) bool {
	return p.index.back[id]
}

func (p *Page) cardFound(e workspace.Card) bool {
	q := kit.Trim(p.query)
	if q == "" {
		return true
	}
	title := ""
	if n, ok := p.Host.Content().Tree().Node(e.Chapter); ok {
		title = n.Title
	}
	return q == "" || slices.ContainsFunc([]string{e.Title, e.When, e.About, title}, func(s string) bool { return strings.Contains(s, q) })
}

func (p *Page) nodeFound(n *workspace.Node) bool {
	q := kit.Trim(p.query)
	return q == "" || strings.Contains(n.Title, q) || strings.Contains(p.Host.Content().Outlines()[n.ID], q)
}

func (p *Page) counts() (cards, nodes int) {
	content := p.Host.Content()
	for _, e := range content.Plan().Cards {
		if p.cardFound(e) {
			cards++
		}
	}
	if p.mode == 0 {
		content.Tree().Walk(func(n *workspace.Node, _ int) {
			if p.nodeFound(n) {
				nodes++
			}
		})
	}
	return
}

// View draws the page: the chapter table with the cards not yet placed, or
// the timeline, and the setting beside it while Pane is set. A broken
// outline shows what is wrong instead, and is read again when it is due.
func (p *Page) View(c *ui.Context) {
	content := p.Host.Content()
	if !p.checksPaused && p.loadErr != nil && time.Since(p.checked) >= p.Host.Content().Timing.CheckEvery {
		p.loaded = false // a broken outline may have been put right outside
	}
	if !p.loaded {
		p.checked = time.Now()
		p.loadErr = content.ReloadTree()
		p.loaded = true
		if p.loadErr == nil {
			p.load()
		}
		if p.loadErr == nil {
			p.loadErr = content.ReloadOutlines()
		}
	} else {
		p.rebuildIndex()
	}
	if p.loadErr != nil {
		p.broken(c)
		return
	}
	if !p.cardOpen && !p.timeOpen && p.outlineDraft.Node == "" && p.lineID == "" && c.Shortcut(0, ui.KeyEscape) {
		p.query = ""
	}
	if p.seen != "" {
		if wait := planFlash - time.Since(p.seenAt); wait > 0 {
			c.After(wait)
		} else {
			p.seen = ""
		}
	}
	ui.Row(c).FillWidth().MinWidth(0).Grow(1).Shrink(1).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).FillHeight().Children(func() {
			p.top(c)
			cards, nodes := p.counts()
			if q := kit.Trim(p.query); q != "" && cards+nodes == 0 {
				about := locales.PlanSearchChapters()
				if p.mode == 1 {
					about = locales.PlanSearchTimeline()
				}
				p.Host.Colors().Blank(c, kit.IcSearch, locales.PlanNotFound(q), about, func() {
					if p.Host.Colors().Secondary(c, locales.PlanClearSearch()).Label("空页清除搜索").Clicked() {
						p.query = ""
					}
				})
			} else if p.mode == 0 {
				p.grid(c)
				p.tray(c)
			} else {
				p.timeline(c)
			}
		})
		if p.Pane {
			p.Host.SettingColumn(c)
		}
	})
}

// broken stands in for the page while the outline does not read: one
// line, as when a search finds nothing, and no retry, as the file is read
// again as often as the open chapter is looked at on the disk.
func (p *Page) broken(c *ui.Context) {
	pal := p.Host.Colors()
	if !p.checksPaused {
		c.After(p.Host.Content().Timing.CheckEvery - time.Since(p.checked))
	}
	title, about, why := locales.PlanReadError(), "", p.loadErr.Error()
	var bad *workspace.PlanError
	if errors.As(p.loadErr, &bad) {
		title, about, why = locales.PlanBrokenTitle(), locales.PlanBrokenAbout(), ""
		switch {
		case bad.Line > 0:
			why = locales.PlanBrokenLine(bad.Line)
		case errors.Is(bad.Err, io.ErrUnexpectedEOF):
			why = locales.PlanBrokenEnd()
		}
	}
	ui.Row(c).FillWidth().Height(kit.TitleHeight).Padding(0, 16, 0, 24).AlignItems(ui.Center).Shrink(0).BorderWidth(0, 0, 1, 0).BorderColor(pal.Line).Children(func() {
		ui.Text(c, locales.PagePlan()).FontSize(kit.FsBody).FontWeight(600).SingleLine()
	})
	pal.Blank(c, kit.IcAlert, title, about, func() {
		if why != "" {
			ui.Text(c, why).FontSize(kit.FsNote).TextColor(pal.Faint).TextAlign(ui.Center).MaxWidth(440)
		}
		_, reveal := p.Host.Dialogs()
		if reveal != nil && pal.Secondary(c, locales.CommonOpenFolder()).Clicked() {
			p.Host.Notices().Warn(reveal(p.Host.WorkPath()))
		}
	})
}

func (p *Page) top(c *ui.Context) {
	pal := p.Host.Colors()
	content := p.Host.Content()
	cards, nodes := p.counts()
	chs := p.chapters()
	loose := len(p.cardsOf(""))
	sum := locales.PlanSummary(len(chs), len(content.Plan().Lines), len(content.Plan().Cards))
	if loose > 0 {
		sum = locales.PlanSummaryLoose(len(chs), len(content.Plan().Lines), len(content.Plan().Cards), loose)
	}
	if kit.Trim(p.query) != "" {
		switch {
		case nodes > 0 && cards > 0:
			sum = locales.PlanFoundBoth(nodes, cards)
		case nodes > 0:
			sum = locales.PlanFoundChapters(nodes)
		case cards > 0:
			sum = locales.PlanFoundCards(cards)
		default:
			sum = locales.PlanNoResults()
		}
	}
	ui.Row(c).FillWidth().Height(kit.TitleHeight).Padding(0, 16, 0, 24).Gap(12).AlignItems(ui.Center).Shrink(0).BorderWidth(0, 0, 1, 0).BorderColor(pal.Line).Children(func() {
		ui.Text(c, locales.PagePlan()).FontSize(kit.FsBody).FontWeight(600).SingleLine()
		ui.Text(c, sum).FontSize(kit.FsNote).TextColor(pal.Theme.TextMuted).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
		hint := locales.PlanSearchCardsAndOutlines()
		if p.mode == 1 {
			hint = locales.PlanSearchCards()
		}
		ui.Row(c).Width(planCardWidth).Height(kit.HControlSm).AlignItems(ui.Center).Gap(6).Padding(0, 8).Radius(kit.RControl).Border(1, pal.Theme.Border).Children(func() {
			kit.Glyph(c, kit.IcSearch, kit.IconSm, pal.Faint)
			ui.TextInputBase(c, &p.query).Label(locales.PlanSearch()).Placeholder(hint).FontSize(kit.FsControl).Grow(1).Shrink(1)
			if p.query != "" {
				if kit.IconButton(c, kit.IcX, locales.PlanClearSearch(), ui.Color{}).Size(kit.HTag, kit.HTag).Clicked() {
					p.query = ""
				}
			}
		})
		was := p.mode
		pal.Segments(c, &p.mode, locales.PlanViewAria(), []string{locales.PlanByChapter(), locales.PlanTimelineView()})
		if was != p.mode {
			p.FinishOutline()
			p.finishLine()
			p.timeOpen = false
			p.seen = ""
		}
	})
}

func (p *Page) text(c *ui.Context, text string, small bool) ui.Element {
	size := float32(kit.FsBody)
	if small {
		size = kit.FsNote
	}
	return ui.RichText(c, kit.Marked(text, kit.Trim(p.query), p.Host.Colors().Ink)...).FontSize(size).LineHeight(kit.LhBody).FillWidth()
}

// ChapterStory draws, by the open chapter's 章纲, when the chapter is in the
// story, from its cards; clicking it opens the timeline at the first card
// with a time.
func (p *Page) ChapterStory(c *ui.Context) {
	if !p.loaded {
		p.load()
	} else {
		p.rebuildIndex()
	}
	if p.loadErr != nil {
		return
	}
	id, _, _ := p.Host.Chapter()
	cards := p.cardsOf(id)
	when := storyWhen(cards)
	if when == "" {
		return
	}
	if p.Host.Colors().Ghost(c, locales.PlanWhenNote(when), ui.Color{}).Clicked() {
		p.Host.ShowOutline()
		p.mode = 1
		p.query = ""
		p.loaded = false
		for _, e := range cards {
			if e.When != "" {
				p.seeCard(e.ID)
				break
			}
		}
	}
}

// Enter has the outline read again when the page is next drawn.
func (p *Page) Enter() { p.loaded = false }

// Leave saves the 章纲 and the plot-line name being typed.
func (p *Page) Leave() {
	p.FinishOutline()
	p.finishLine()
}
