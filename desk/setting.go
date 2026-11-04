package desk

import (
	"cmp"

	"slices"
	"strings"
	"time"
	"writer/locales"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/workspace"
)

type paneTab string

const (
	paneMargin  paneTab = "margin"
	paneSetting paneTab = "setting"
	paneTalk    paneTab = "talk"
	paneFore    paneTab = "fore"
)

// settingBase is what lies under the setting panel's other screens: the
// list, what waits on the author, or the deleted entries. One at a time.
type settingBase int

const (
	baseList settingBase = iota
	baseWaiting
	baseDropped
)

// settingRoute is where the setting panel is: a base screen, an entry's
// page over it, a form over that, and the revisions over everything. The
// form's text stays in docView: it is what is typed, not where the panel
// is.
type settingRoute struct {
	base    settingBase
	shown   string // the entry whose page shows
	editing string // the entry being edited, "new" for a new one
	asking  string // with editing "new", the AI's question it answers
	change  string // the change of the entry shown being edited, "new" for a new one
	history bool   // the revisions show
}

// settingScreen is what the setting panel shows.
type settingScreen int

const (
	screenList settingScreen = iota
	screenWaiting
	screenDropped
	screenEntry
	screenNewEntry
	screenEntryForm
	screenChange
	screenHistory
)

// settle drops the layers that point at nothing: the form of an entry
// gone, a new entry whose question is gone or no longer waits, the page
// of an entry gone with its change, and a change gone or no longer in
// the body.
func (r *settingRoute) settle(set workspace.Setting) {
	if r.editing != "" && r.editing != "new" {
		if _, ok := set.Entry(r.editing); !ok {
			r.editing, r.asking = "", ""
		}
	}
	if q, ok := set.Entry(r.asking); r.asking != "" && (!ok || q.Place != workspace.PlaceOpen) {
		r.editing, r.asking = "", "" // the question is gone
	}
	shown, ok := set.Entry(r.shown)
	if !ok {
		r.shown, r.change = "", ""
	}
	if r.change != "new" && !slices.ContainsFunc(shown.Changes, func(g workspace.Change) bool { return g.ID == r.change && g.Place == workspace.PlaceBody }) {
		r.change = "" // the change is gone, or the entry
	}
}

// screen says what the panel shows, and the entry or change it is about:
// blank for the screens that name neither.
func (r settingRoute) screen() (settingScreen, string, string) {
	switch {
	case r.history:
		return screenHistory, "", ""
	case r.editing == "new":
		return screenNewEntry, "", ""
	case r.editing != "":
		return screenEntryForm, r.editing, ""
	case r.shown != "" && r.change != "":
		return screenChange, r.shown, r.change
	case r.shown != "":
		return screenEntry, r.shown, ""
	case r.base == baseWaiting:
		return screenWaiting, "", ""
	case r.base == baseDropped:
		return screenDropped, "", ""
	}
	return screenList, "", ""
}

func (r settingRoute) entry() string        { return r.shown }
func (r settingRoute) editID() string       { return r.editing }
func (r settingRoute) question() string     { return r.asking }
func (r settingRoute) changeID() string     { return r.change }
func (r settingRoute) showingHistory() bool { return r.history }
func (r settingRoute) showingWaiting() bool { return r.base == baseWaiting }

// openEntry shows an entry's page over what lies below it.
func (r *settingRoute) openEntry(id string) { r.shown = id }

// closeEntry goes back from the entry's page and its change.
func (r *settingRoute) closeEntry() { r.shown, r.change = "", "" }

// jumpToEntry shows id's page, closing every layer: a link to an entry
// from elsewhere.
func (r *settingRoute) jumpToEntry(id string) { *r = settingRoute{shown: id} }

// jumpToWaiting shows what waits on the author, closing every layer.
func (r *settingRoute) jumpToWaiting() { *r = settingRoute{base: baseWaiting} }

// showWaiting lists what waits on the author.
func (r *settingRoute) showWaiting() { r.base = baseWaiting }

// closeWaiting goes back to the list of entries from what waits.
func (r *settingRoute) closeWaiting() {
	if r.base == baseWaiting {
		r.base = baseList
	}
}

// showDropped lists the deleted entries.
func (r *settingRoute) showDropped() { r.base = baseDropped }

// closeDropped goes back to the list of entries from the deleted ones.
func (r *settingRoute) closeDropped() {
	if r.base == baseDropped {
		r.base = baseList
	}
}

// showHistory shows the revisions, over what lies below.
func (r *settingRoute) showHistory() { r.history = true }

// closeHistory goes back from the revisions.
func (r *settingRoute) closeHistory() { r.history = false }

// newEntry opens the form for a new entry.
func (r *settingRoute) newEntry() { r.editing, r.asking = "new", "" }

// answer opens the form for a new entry answering question.
func (r *settingRoute) answer(question string) { r.editing, r.asking = "new", question }

// editEntry opens the form of entry id.
func (r *settingRoute) editEntry(id string) { r.editing, r.asking = id, "" }

// closeForm goes back from the entry's form.
func (r *settingRoute) closeForm() { r.editing, r.asking = "", "" }

// editChange opens the form of change id.
func (r *settingRoute) editChange(id string) { r.change = id }

// closeChange goes back from the change's form.
func (r *settingRoute) closeChange() { r.change = "" }

type docView struct {
	hidden bool
	query  string // what the list is searched for
	route  settingRoute
	// from is the node a change's form starts at, and the form is the
	// entry's text, category, name, and what else it is called as the
	// author writes them, apart by spaces or 、
	from     string
	text     string
	category string
	name     string
	names    string
	// The right column: its width as dragged, and, on the writing page,
	// its selected tab and whether a talk sends the chapter with it.
	width    float32
	resizing bool
	tab      paneTab
	ideas    cached[ideasKey, []workspace.Idea]
	// The talks are listed there, not the one open, and what the list is
	// searched for.
	talks     bool
	talkQuery string
	attach    bool
}

// setSideHidden puts the sidebar away, on every page, or brings it back.
func (a *App) setSideHidden(hidden bool) {
	a.sideHidden = hidden
	a.savePrefs()
}

// setDocHidden folds the panel away, for the talk alone, or brings it back.
func (a *App) setDocHidden(hidden bool) {
	if a.page == pagePlan {
		a.plan.Pane = !hidden
		return
	}
	a.doc.hidden = hidden
	a.savePrefs()
}

var placeNames = map[string]func() string{workspace.PlaceBody: locales.LoreWhereBody, workspace.PlaceOpen: locales.LoreToConfirm, workspace.PlaceDropped: locales.LoreDropped}

func settingPlaceName(place string) string {
	if say := placeNames[place]; say != nil {
		return say()
	}
	return ""
}

// revise records the author's change and shows the setting after it.
func (a *App) revise(ops ...workspace.Op) {
	_, err := a.book.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: ops})
	if err == nil {
		err = a.refreshFromSetting()
	}
	a.SetError(err)
}

// settingPanel is the setting in the right column: its entries a row
// each, or the page of what a row was clicked for.
func (a *App) settingPanel(c *ui.Context) {
	a.pal.PaneTop(c).Children(func() {
		if a.page == pageWrite {
			a.panelTabs(c)
		} else {
			// The count is what waits on the author, as in the tab.
			ui.Text(c, locales.RecordsLore()).FontSize(kit.FsControl).Margin(0, 0, 0, 10).Label("右栏设定标题")
			if open := a.ideas(); len(open) > 0 {
				ui.Textf(c, "%d", len(open)).FontSize(kit.FsNote).TextColor(a.pal.Faint).Padding(0, 0, 0, 4)
			}
		}
	})
	d := &a.doc
	d.route.settle(a.book.Setting())
	switch screen, entry, _ := d.route.screen(); screen {
	case screenHistory:
		a.history(c)
	case screenNewEntry:
		a.entryForm(c, workspace.Entry{})
	case screenEntryForm:
		e, _ := a.book.Setting().Entry(entry)
		a.entryForm(c, e)
	case screenChange:
		e, _ := a.book.Setting().Entry(entry)
		a.changeForm(c, e)
	case screenEntry:
		e, _ := a.book.Setting().Entry(entry)
		a.entryPage(c, e)
	case screenWaiting:
		a.ideasPage(c)
	case screenDropped:
		a.droppedList(c)
	default:
		a.settingList(c)
	}
}

// bodyCount is how many entries the author has settled.
func (a *App) bodyCount() int {
	n := 0
	for _, e := range a.book.Setting().Entries {
		if e.Place == workspace.PlaceBody {
			n++
		}
	}
	return n
}

// entryName is what a row calls the entry and the line of it that
// follows: its name and its text, or its text alone when it names nothing.
func entryName(e workspace.Entry) (name, gist string) {
	text := strings.Join(strings.Fields(e.Text), " ")
	if len(e.Names) == 0 {
		return text, ""
	}
	return kit.Clip(e.Names[0], 12), text
}

// settingList is the settled entries by category, a row each, under what
// waits on the author; or those that say what is searched for.
func (a *App) settingList(c *ui.Context) {
	d := &a.doc
	open := a.ideas()
	waiting := map[string]bool{}
	for _, it := range open {
		waiting[it.About] = true
	}
	dropped := 0
	for _, e := range a.book.Setting().Entries {
		if e.Place == workspace.PlaceDropped {
			dropped++
		}
	}
	if a.paneFind(c, &d.query, locales.LoreSearch(), locales.CommonNew(), func(m *kit.Menu) {
		m.Item(locales.CheckDeleted()).Hint(kit.Count(dropped)).Do(d.route.showDropped)
		m.Item(locales.LoreLog()).Do(d.route.showHistory)
		m.Item(locales.CmdCategories()).Do(func() { a.page = pageCategories })
	}) {
		d.route.newEntry()
		d.text, d.category, d.name, d.names = "", a.firstCategory(), "", ""
	}
	q := kit.Trim(d.query)
	// On the writing page each entry reads as it does at the chapter open,
	// elsewhere as it does now; the end of its row says from where.
	tree, here := a.book.Tree(), a.here()
	o := tree.Order()
	row := func(e workspace.Entry) {
		side := changeSide(tree, o, e, here)
		e.Text, _, _ = e.Reading(o, here)
		name, gist := entryName(e)
		if q != "" {
			gist = kit.NearFind(gist, q)
		}
		b := a.pal.PaneRow(c.Key(e.ID), "设定-"+e.ID, name, gist, side, q, waiting[e.ID])
		// One a talk just noted is tinted for a moment.
		if at, ok := a.flash[e.ID]; ok {
			if left := flashFor - time.Since(at); left > 0 {
				b.Background(a.pal.Ink.Alpha(0.2 * float32(left) / float32(flashFor)))
				c.AnimationFrame()
			} else {
				delete(a.flash, e.ID)
			}
		}
		if b.Clicked() {
			d.route.openEntry(e.ID)
		}
	}
	a.pal.PaneRows(c, "setting", func() {
		if a.renamed.count > 0 {
			ui.Box(c).FillWidth().Padding(8, 0, 0, 0).Children(func() { a.renameNotice(c) })
		}
		if q != "" {
			var found []workspace.Entry
			for _, e := range a.book.Setting().Entries {
				if e.Place == workspace.PlaceBody && (strings.Contains(e.Text, q) || slices.ContainsFunc(e.Names, func(n string) bool { return strings.Contains(n, q) })) {
					found = append(found, e)
				}
			}
			if len(found) == 0 {
				a.pal.PaneEmpty(c, locales.CommonNotFound(), "")
				return
			}
			a.pal.PaneGroup(c, locales.CommonFound(len(found)))
			for _, e := range found {
				row(e)
			}
			return
		}
		if len(open) > 0 && a.pal.PaneTodo(c, locales.LoreToConfirm(), len(open)).Clicked() {
			d.route.showWaiting()
		}
		if a.bodyCount() == 0 {
			how := locales.LoreEmptyAi()
			if !a.aiOn() {
				how = locales.LoreEmptyNoAi()
			}
			a.pal.PaneEmpty(c, locales.LoreEmpty(), how)
			return
		}
		for _, cat := range a.Store.Work().Categories {
			first := true
			for _, e := range a.book.Setting().Entries {
				if e.Place != workspace.PlaceBody || e.Category != cat.Name {
					continue
				}
				if first {
					a.pal.PaneGroup(c, cat.Name)
					first = false
				}
				row(e)
			}
		}
	})
}

// droppedList is the entries the author deleted, which a click shows and
// its page puts back.
func (a *App) droppedList(c *ui.Context) {
	d := &a.doc
	if back, _ := a.pal.PaneBack(c, locales.LoreAll(), ""); back {
		d.route.closeDropped()
	}
	a.pal.PaneRows(c, "dropped", func() {
		a.pal.PaneGroup(c, locales.CheckDeleted())
		n := 0
		for _, e := range a.book.Setting().Entries {
			if e.Place != workspace.PlaceDropped {
				continue
			}
			n++
			name, gist := entryName(e)
			if a.pal.PaneRow(c.Key(e.ID), "设定-"+e.ID, name, gist, "", "", false).Clicked() {
				d.route.openEntry(e.ID)
			}
		}
		if n == 0 {
			a.pal.PaneEmpty(c, locales.LoreNoneDropped(), "")
		}
	})
}

// entryPage is one entry: all it says, where it comes from, and what the
// AI suggests about it.
func (a *App) entryPage(c *ui.Context, e workspace.Entry) {
	d := &a.doc
	from, act := locales.LoreAll(), locales.LoreDelete()
	switch e.Place {
	case workspace.PlaceDropped:
		from, act = locales.CheckDeleted(), locales.LoreRestore()
	case workspace.PlaceOpen:
		act = ""
	}
	back, acted := a.pal.PaneBack(c, from, act)
	if back {
		d.route.closeEntry()
	}
	if acted {
		place := workspace.PlaceDropped
		if e.Place == place {
			place = workspace.PlaceBody
		}
		a.revise(workspace.Op{Op: workspace.OpMove, Entry: e.ID, Place: place})
		d.route.closeEntry()
	}
	open := a.ideas()
	a.pal.PanePage(c, "entry-"+e.ID, func() {
		name, sub := e.Category, ""
		if len(e.Names) > 0 {
			name = e.Names[0]
			if len(e.Names) > 1 {
				sub = locales.LoreAliasesOf(strings.Join(e.Names[1:], locales.DraftListSep()))
			}
		}
		if a.pal.PaneTitle(c, name, sub, locales.CommonEdit()) {
			a.editSetting(e)
		}
		text := ui.Text(c, e.Text).FontSize(kit.FsBody).LineHeight(kit.LhLoose).Selectable().FillWidth()
		if e.Place == workspace.PlaceDropped {
			text.TextColor(a.pal.Theme.TextMuted)
		}
		if e.Asked != "" {
			a.pal.PaneNote(c, locales.LoreAnswer(e.Asked))
		}
		if i := slices.IndexFunc(e.Sources, func(s workspace.Source) bool { return s.Chapter != "" }); i >= 0 {
			src := e.Sources[i]
			where := "“" + src.Quote + "”"
			if title, ok := a.chapterTitle(src.Chapter); ok {
				where = "「" + title + "」" + where
			}
			if a.pal.PaneWhere(c, where, "原文-"+e.ID) {
				a.findInChapter(src.Chapter, src.Quote)
			}
		}
		if e.Place != workspace.PlaceDropped {
			a.timeline(c, e)
		}
		first := true
		for _, it := range open {
			if it.About != e.ID {
				continue
			}
			a.pal.Hairline(c)
			if first {
				ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
					kit.Dot(c, a.pal.Amber, 6)
					ui.Text(c, locales.LoreAiSuggest()).FontSize(kit.FsNote).FontWeight(600)
				})
				first = false
			}
			from := ""
			if it.Change.ID != "" {
				from = kit.FromLabel(a.book.Tree(), it.Change.From)
			}
			a.idea(c, it, from)
		}
	})
}

// entryForm changes e's name, text and category, or writes a new entry
// when e is empty: the answer to the AI's question when one is asked.
func (a *App) entryForm(c *ui.Context, e workspace.Entry) {
	d := &a.doc
	question := d.route.question()
	q, asked := a.book.Setting().Entry(question)
	asked = asked && question != "" && e.ID == ""
	from := locales.LoreAll()
	switch {
	case e.ID != "":
		from, _ = entryName(e)
		from = kit.Clip(from, 16)
	case asked:
		from = locales.LoreToConfirm()
	}
	if back, _ := a.pal.PaneBack(c, from, ""); back {
		d.route.closeForm()
	}
	a.pal.PanePage(c, "form-"+e.ID, func() {
		// A new entry starts at its text, a changed one at its name: the
		// keyboard is in that field as the form comes.
		content := func() {
			a.pal.PaneField(c, locales.ExportContent(), func() {
				in := ui.TextAreaBase(c, &d.text).Label(locales.DialogSettingText()).FillWidth().Height(96).FontSize(kit.FsBody).LineHeight(kit.LhLoose).
					Padding(8, 10).Radius(kit.RControl).Background(a.pal.Canvas).Border(1, a.pal.Theme.Border)
				if e.ID == "" {
					in.AutoFocus()
				}
			})
		}
		name, alias := locales.LoreName(), locales.LoreAliases()
		if e.ID == "" {
			if asked {
				ui.Text(c, locales.CommonAnswer()).FontSize(kit.FsGroup).FontWeight(600)
				a.pal.PaneNote(c, q.Text)
			} else {
				ui.Text(c, locales.LoreNewLore()).FontSize(kit.FsGroup).FontWeight(600)
			}
			name, alias = locales.LoreNameOptional(), locales.LoreAliasesOptional()
			content()
		}
		a.pal.PaneField(c, name, func() {
			in := ui.TextInput(c, &d.name).Label(locales.LoreName()).FillWidth().FontSize(kit.FsBody)
			if e.ID != "" {
				in.AutoFocus()
			}
		})
		a.pal.PaneField(c, alias, func() {
			ui.TextInput(c, &d.names).Label(locales.LoreAliases()).Placeholder(locales.LoreAliasHint()).FillWidth().FontSize(kit.FsBody)
		})
		if e.ID != "" {
			content()
		}
		a.pal.PaneField(c, locales.LoreCategory(), func() {
			a.pal.Select(c, &d.category, a.categoryNames()).Label(locales.DialogSettingCat()).FillWidth()
		})
		text := kit.Trim(d.text)
		names := splitNames(d.name + "、" + d.names)
		kit.PaneButtons(c, func() {
			if a.pal.PaneButton(c, locales.CommonCancel(), false).Clicked() {
				d.route.closeForm()
			}
			if !a.pal.PaneButton(c, locales.CommonSave(), true).Disabled(text == "").Clicked() {
				return
			}
			var ops []workspace.Op
			switch {
			case asked:
				ops = append(ops, workspace.Op{Op: workspace.OpAnswer, Entry: q.ID, Category: d.category, Text: text, Names: names})
			case e.ID == "":
				ops = append(ops, workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: d.category, Text: text, Names: names})
			default:
				if text != e.Text {
					ops = append(ops, workspace.Op{Op: workspace.OpEdit, Entry: e.ID, Text: text})
				}
				if d.category != e.Category {
					ops = append(ops, workspace.Op{Op: workspace.OpMove, Entry: e.ID, Category: d.category})
				}
				if !slices.Equal(names, e.Names) {
					ops = append(ops, workspace.Op{Op: workspace.OpName, Entry: e.ID, Names: names})
				}
			}
			if len(ops) > 0 {
				a.revise(ops...)
			}
			if a.errorText() == "" {
				d.route.closeForm()
				if len(e.Names) > 0 && len(names) > 0 {
					a.noteRename(e.Names[0], names[0])
				}
			}
		})
	})
}

// history lists the revisions, newest first: what changed and who
// changed it; what a talk noted links to the talk. With the AI off, what
// talks noted is not listed.
func (a *App) history(c *ui.Context) {
	if back, _ := a.pal.PaneBack(c, locales.LoreAll(), ""); back {
		a.doc.route.closeHistory()
	}
	a.pal.Hairline(c)
	ui.Scroll(c.Key("history")).Grow(1).FillWidth().Children(func() {
		ui.Column(c).FillWidth().Gap(10).Padding(16, 20, 20, 20).Children(func() {
			ui.Text(c, locales.LoreLog()).FontSize(kit.FsGroup).FontWeight(600)
			shown := 0
			for n := len(a.book.Setting().Revisions) - 1; n >= 0; n-- {
				r := a.book.Setting().Revisions[n]
				if len(r.Ops) == 0 || r.By != workspace.ByAuthor && !a.aiOn() {
					continue // the bookkeeper's that failed changed nothing
				}
				shown++
				ui.Column(c.Key(r.ID)).FillWidth().Padding(10, 12).Gap(6).Radius(kit.RCard).Background(a.pal.Raised).Border(1, a.pal.Line).Children(func() {
					ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
						who := locales.LoreWhoManual()
						if r.By != workspace.ByAuthor {
							who = locales.SettingsJobChat()
						}
						ui.Text(c, who).FontSize(kit.FsNote).FontWeight(600)
						ui.Spacer(c)
						ui.Text(c, kit.When(r.At, time.Now())).FontSize(kit.FsSmall).TextColor(a.pal.Faint)
					})
					for _, op := range r.Ops {
						ui.Text(c, a.describeOp(op)).FontSize(kit.FsNote).LineHeight(kit.LhBody).Selectable().FillWidth()
					}
					if r.By != workspace.ByAuthor {
						if words, ok := a.talkName(r.Session); ok {
							if a.pal.PaneLink(c, locales.LoreNotedIn(words), locales.ChatViewTalk(), "讨论-"+r.ID) {
								a.page = pageTalk
								a.switchSession(r.Session)
							}
						}
					}
				})
			}
			switch {
			case shown > 0:
			case a.aiOn():
				a.pal.PaneNote(c, locales.LoreLogEmptyAi())
			default:
				a.pal.PaneNote(c, locales.LoreLogEmpty())
			}
		})
	})
}

// describeOp says what op did, in the words of the panel.
func (a *App) describeOp(op workspace.Op) string {
	e, _ := a.book.Setting().Entry(op.Entry)
	if op.Op == workspace.OpChange || op.Op == workspace.OpEditChange || op.Op == workspace.OpDropChange {
		var g workspace.Change
		if k := slices.IndexFunc(e.Changes, func(x workspace.Change) bool { return x.ID == op.Change }); k >= 0 {
			g = e.Changes[k]
		}
		from, text := cmp.Or(op.From, g.From), cmp.Or(op.Text, g.Text)
		said := shortName(e) + " · " + kit.FromLabel(a.book.Tree(), from)
		switch {
		case op.Op == workspace.OpDropChange:
			return locales.LoreChangeRemoved(shortName(e), kit.FromLabel(a.book.Tree(), from))
		case op.Op == workspace.OpChange && op.Place == workspace.PlaceOpen:
			return locales.LoreAddedWaitingChange(said, kit.Clip(text, 24))
		}
		return locales.LoreChangeLog(shortName(e), kit.FromLabel(a.book.Tree(), from), kit.Clip(text, 24))
	}
	switch op.Op {
	case workspace.OpAdd:
		return locales.LoreAddedPlace(settingPlaceName(op.Place), op.Category, op.Text)
	case workspace.OpAnswer:
		return locales.LoreAddedActive(op.Category, op.Text)
	case workspace.OpEdit:
		return locales.LoreRewritten(op.Text)
	case workspace.OpName:
		if len(op.Names) == 0 {
			return locales.LoreNamesRemoved(kit.Clip(e.Text, 24))
		}
		return locales.LoreNamesChanged(strings.Join(op.Names, locales.DraftListSep()))
	}
	text := kit.Clip(e.Text, 24)
	if op.Place != "" {
		return locales.LoreMovedTo(settingPlaceName(op.Place), text)
	}
	return locales.LoreRecat(op.Category, text)
}
