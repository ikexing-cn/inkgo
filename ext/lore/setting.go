package lore

import (
	"cmp"
	"writer/ext/lore/loretext"

	"slices"
	"strings"
	"time"
	"writer/locales"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/workspace"
)

// settingBase is what lies under the setting panel's other screens: the
// list, what waits on the author, or the deleted entries. One at a time.
type settingBase int

const (
	baseList settingBase = iota
	baseWaiting
	baseDropped
)

// Route is where the setting panel is: a base screen, an entry's
// page over it, a form over that, and the revisions over everything. The
// form's text stays in Form: it is what is typed, not where the panel
// is.
type Route struct {
	base    settingBase
	shown   string // the entry whose page shows
	editing string // the entry being edited, "new" for a new one
	asking  string // with editing "new", the AI's question it answers
	change  string // the change of the entry shown being edited, "new" for a new one
	history bool   // the revisions show
}

// Screen is what the setting panel shows.
type Screen int

const (
	screenList Screen = iota
	screenWaiting
	screenDropped
	screenEntry
	screenNewEntry
	screenEntryForm
	screenChange
	screenHistory
)

// Settle drops the layers that point at nothing: the form of an entry
// gone, a new entry whose question is gone or no longer waits, the page
// of an entry gone with its change, and a change gone or no longer in
// the body.
func (r *Route) Settle(set workspace.Setting) {
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

// Screen says what the panel shows, and the entry or change it is about:
// blank for the screens that name neither.
func (r Route) Screen() (Screen, string, string) {
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

// Entry returns the shown entry.
func (r Route) Entry() string { return r.shown }

// EditID returns the entry being edited.
func (r Route) EditID() string { return r.editing }

// Question returns the question being answered.
func (r Route) Question() string { return r.asking }

// ChangeID returns the change being edited.
func (r Route) ChangeID() string { return r.change }

// ShowingHistory reports whether revisions are shown.
func (r Route) ShowingHistory() bool { return r.history }

// ShowingWaiting reports whether the pending list is shown.
func (r Route) ShowingWaiting() bool { return r.base == baseWaiting }

// OpenEntry shows an entry's page over what lies below it.
func (r *Route) OpenEntry(id string) { r.shown = id }

// CloseEntry goes back from the entry's page and its change.
func (r *Route) CloseEntry() { r.shown, r.change = "", "" }

// JumpToEntry shows id's page, closing every layer: a link to an entry
// from elsewhere.
func (r *Route) JumpToEntry(id string) { *r = Route{shown: id} }

// JumpToWaiting shows what waits on the author, closing every layer.
func (r *Route) JumpToWaiting() { *r = Route{base: baseWaiting} }

// ShowWaiting lists what waits on the author.
func (r *Route) ShowWaiting() { r.base = baseWaiting }

// CloseWaiting goes back to the list of entries from what waits.
func (r *Route) CloseWaiting() {
	if r.base == baseWaiting {
		r.base = baseList
	}
}

// ShowDropped lists the deleted entries.
func (r *Route) ShowDropped() { r.base = baseDropped }

// CloseDropped goes back to the list of entries from the deleted ones.
func (r *Route) CloseDropped() {
	if r.base == baseDropped {
		r.base = baseList
	}
}

// ShowHistory shows the revisions, over what lies below.
func (r *Route) ShowHistory() { r.history = true }

// CloseHistory goes back from the revisions.
func (r *Route) CloseHistory() { r.history = false }

// NewEntry opens the form for a new entry.
func (r *Route) NewEntry() { r.editing, r.asking = "new", "" }

// Answer opens the form for a new entry answering question.
func (r *Route) Answer(question string) { r.editing, r.asking = "new", question }

// EditEntry opens the form of entry id.
func (r *Route) EditEntry(id string) { r.editing, r.asking = id, "" }

// CloseForm goes back from the entry's form.
func (r *Route) CloseForm() { r.editing, r.asking = "", "" }

// EditChange opens the form of change id.
func (r *Route) EditChange(id string) { r.change = id }

// CloseChange goes back from the change's form.
func (r *Route) CloseChange() { r.change = "" }

var placeNames = map[string]func() string{workspace.PlaceBody: loretext.LoreWhereBody, workspace.PlaceOpen: loretext.LoreToConfirm, workspace.PlaceDropped: loretext.LoreDropped}

func settingPlaceName(place string) string {
	if say := placeNames[place]; say != nil {
		return say()
	}
	return ""
}

// revise records the author's change and shows the setting after it.
func (a *Panel) revise(ops ...workspace.Op) {
	_, err := a.services.Book.Content().Revise(workspace.Revision{By: workspace.ByAuthor, Ops: ops})
	if err == nil {
		err = a.services.Edits.Refresh()
	}
	a.services.Notices.Notices().SetError(err)
}

// settingPanel is the setting in the right column: its entries a row
// each, or the page of what a row was clicked for.
func (a *Panel) settingPanel(c *ui.Context) {
	a.services.Palette.Colors().PaneTop(c).Children(func() {
		ui.Text(c, locales.RecordsLore()).FontSize(kit.FsControl).Margin(0, 0, 0, 10).Label("右栏设定标题")
		if open := a.ideas(); len(open) > 0 {
			ui.Textf(c, "%d", len(open)).FontSize(kit.FsNote).TextColor(a.services.Palette.Colors().Faint).Padding(0, 0, 0, 4)
		}
	})
	a.body(c)
}

func (a *Panel) body(c *ui.Context) {
	d := &a.Doc
	d.Route.Settle(a.services.Book.Content().Setting())
	switch screen, entry, _ := d.Route.Screen(); screen {
	case screenHistory:
		a.history(c)
	case screenNewEntry:
		a.entryForm(c, workspace.Entry{})
	case screenEntryForm:
		e, _ := a.services.Book.Content().Setting().Entry(entry)
		a.entryForm(c, e)
	case screenChange:
		e, _ := a.services.Book.Content().Setting().Entry(entry)
		a.changeForm(c, e)
	case screenEntry:
		e, _ := a.services.Book.Content().Setting().Entry(entry)
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
func (a *Panel) bodyCount() int {
	n := 0
	for _, e := range a.services.Book.Content().Setting().Entries {
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
func (a *Panel) settingList(c *ui.Context) {
	d := &a.Doc
	open := a.ideas()
	waiting := map[string]bool{}
	for _, it := range open {
		waiting[it.About] = true
	}
	dropped := 0
	for _, e := range a.services.Book.Content().Setting().Entries {
		if e.Place == workspace.PlaceDropped {
			dropped++
		}
	}
	if a.paneFind(c, &d.Query, loretext.LoreSearch(), loretext.CommonNew(), func(m *kit.Menu) {
		m.Item(loretext.CheckDeleted()).Hint(kit.Count(dropped)).Do(d.Route.ShowDropped)
		m.Item(loretext.LoreLog()).Do(d.Route.ShowHistory)
		if a.services.Desktop.HasPage("cats.categories") {
			m.Item(locales.CmdCategories()).Do(func() { a.services.Desktop.ShowPage("cats.categories") })
		}
	}) {
		d.Route.NewEntry()
		d.Text, d.Category, d.Name, d.Names = "", a.firstCategory(), "", ""
	}
	q := kit.Trim(d.Query)
	// On the writing page each entry reads as it does at the chapter open,
	// elsewhere as it does now; the end of its row says from where.
	tree, here := a.services.Book.Content().Tree(), a.here()
	o := tree.Order()
	row := func(e workspace.Entry) {
		side := changeSide(tree, o, e, here)
		e.Text, _, _ = e.Reading(o, here)
		name, gist := entryName(e)
		if q != "" {
			gist = kit.NearFind(gist, q)
		}
		b := a.services.Palette.Colors().PaneRow(c.Key(e.ID), "设定-"+e.ID, name, gist, side, q, waiting[e.ID])
		// One a talk just noted is tinted for a moment.
		if at, ok := a.services.Book.Flashed(e.ID); ok {
			if left := flashFor - time.Since(at); left > 0 {
				b.Background(a.services.Palette.Colors().Ink.Alpha(0.2 * float32(left) / float32(flashFor)))
				c.AnimationFrame()
			} else {
				a.services.Book.ClearFlash(e.ID)
			}
		}
		if b.Clicked() {
			d.Route.OpenEntry(e.ID)
		}
	}
	a.services.Palette.Colors().PaneRows(c, "setting", func() {
		if a.renamed.count > 0 {
			ui.Box(c).FillWidth().Padding(8, 0, 0, 0).Children(func() { a.renameNotice(c) })
		}
		if q != "" {
			var found []workspace.Entry
			for _, e := range a.services.Book.Content().Setting().Entries {
				if e.Place == workspace.PlaceBody && (strings.Contains(e.Text, q) || slices.ContainsFunc(e.Names, func(n string) bool { return strings.Contains(n, q) })) {
					found = append(found, e)
				}
			}
			if len(found) == 0 {
				a.services.Palette.Colors().PaneEmpty(c, locales.CommonNotFound(), "")
				return
			}
			a.services.Palette.Colors().PaneGroup(c, locales.CommonFound(len(found)))
			for _, e := range found {
				row(e)
			}
			return
		}
		if len(open) > 0 && a.services.Palette.Colors().PaneTodo(c, loretext.LoreToConfirm(), len(open)).Clicked() {
			d.Route.ShowWaiting()
		}
		if a.bodyCount() == 0 {
			how := loretext.LoreEmptyAi()
			if !a.services.AI.AIEnabled() {
				how = loretext.LoreEmptyNoAi()
			}
			a.services.Palette.Colors().PaneEmpty(c, loretext.LoreEmpty(), how)
			return
		}
		for _, cat := range a.services.Book.Store().Work().Categories {
			first := true
			for _, e := range a.services.Book.Content().Setting().Entries {
				if e.Place != workspace.PlaceBody || e.Category != cat.Name {
					continue
				}
				if first {
					a.services.Palette.Colors().PaneGroup(c, cat.Name)
					first = false
				}
				row(e)
			}
		}
	})
}

// droppedList is the entries the author deleted, which a click shows and
// its page puts back.
func (a *Panel) droppedList(c *ui.Context) {
	d := &a.Doc
	if back, _ := a.services.Palette.Colors().PaneBack(c, loretext.LoreAll(), ""); back {
		d.Route.CloseDropped()
	}
	a.services.Palette.Colors().PaneRows(c, "dropped", func() {
		a.services.Palette.Colors().PaneGroup(c, loretext.CheckDeleted())
		n := 0
		for _, e := range a.services.Book.Content().Setting().Entries {
			if e.Place != workspace.PlaceDropped {
				continue
			}
			n++
			name, gist := entryName(e)
			if a.services.Palette.Colors().PaneRow(c.Key(e.ID), "设定-"+e.ID, name, gist, "", "", false).Clicked() {
				d.Route.OpenEntry(e.ID)
			}
		}
		if n == 0 {
			a.services.Palette.Colors().PaneEmpty(c, loretext.LoreNoneDropped(), "")
		}
	})
}

// entryPage is one entry: all it says, where it comes from, and what the
// AI suggests about it.
func (a *Panel) entryPage(c *ui.Context, e workspace.Entry) {
	d := &a.Doc
	from, act := loretext.LoreAll(), loretext.LoreDelete()
	switch e.Place {
	case workspace.PlaceDropped:
		from, act = loretext.CheckDeleted(), loretext.LoreRestore()
	case workspace.PlaceOpen:
		act = ""
	}
	back, acted := a.services.Palette.Colors().PaneBack(c, from, act)
	if back {
		d.Route.CloseEntry()
	}
	if acted {
		place := workspace.PlaceDropped
		if e.Place == place {
			place = workspace.PlaceBody
		}
		a.revise(workspace.Op{Op: workspace.OpMove, Entry: e.ID, Place: place})
		d.Route.CloseEntry()
	}
	open := a.ideas()
	a.services.Palette.Colors().PanePage(c, "entry-"+e.ID, func() {
		name, sub := e.Category, ""
		if len(e.Names) > 0 {
			name = e.Names[0]
			if len(e.Names) > 1 {
				sub = loretext.LoreAliasesOf(strings.Join(e.Names[1:], locales.DraftListSep()))
			}
		}
		if a.services.Palette.Colors().PaneTitle(c, name, sub, loretext.CommonEdit()) {
			a.editSetting(e)
		}
		text := ui.Text(c, e.Text).FontSize(kit.FsBody).LineHeight(kit.LhLoose).Selectable().FillWidth()
		if e.Place == workspace.PlaceDropped {
			text.TextColor(a.services.Palette.Colors().Theme.TextMuted)
		}
		if e.Asked != "" {
			a.services.Palette.Colors().PaneNote(c, loretext.LoreAnswer(e.Asked))
		}
		if i := slices.IndexFunc(e.Sources, func(s workspace.Source) bool { return s.Chapter != "" }); i >= 0 {
			src := e.Sources[i]
			where := "“" + src.Quote + "”"
			if title, ok := a.services.Book.ChapterTitle(src.Chapter); ok {
				where = "「" + title + "」" + where
			}
			if a.services.Palette.Colors().PaneWhere(c, where, "原文-"+e.ID) {
				a.services.Search.FindInChapter(src.Chapter, src.Quote)
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
			a.services.Palette.Colors().Hairline(c)
			if first {
				ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
					kit.Dot(c, a.services.Palette.Colors().Amber, 6)
					ui.Text(c, loretext.LoreAiSuggest()).FontSize(kit.FsNote).FontWeight(600)
				})
				first = false
			}
			from := ""
			if it.Change.ID != "" {
				from = kit.FromLabel(a.services.Book.Content().Tree(), it.Change.From)
			}
			a.idea(c, it, from)
		}
	})
}

// entryForm changes e's name, text and category, or writes a new entry
// when e is empty: the answer to the AI's question when one is asked.
func (a *Panel) entryForm(c *ui.Context, e workspace.Entry) {
	d := &a.Doc
	question := d.Route.Question()
	q, asked := a.services.Book.Content().Setting().Entry(question)
	asked = asked && question != "" && e.ID == ""
	from := loretext.LoreAll()
	switch {
	case e.ID != "":
		from, _ = entryName(e)
		from = kit.Clip(from, 16)
	case asked:
		from = loretext.LoreToConfirm()
	}
	if back, _ := a.services.Palette.Colors().PaneBack(c, from, ""); back {
		d.Route.CloseForm()
	}
	a.services.Palette.Colors().PanePage(c, "form-"+e.ID, func() {
		// A new entry starts at its text, a changed one at its name: the
		// keyboard is in that field as the form comes.
		content := func() {
			a.services.Palette.Colors().PaneField(c, locales.ExportContent(), func() {
				in := ui.TextAreaBase(c, &d.Text).Label(locales.DialogSettingText()).FillWidth().Height(96).FontSize(kit.FsBody).LineHeight(kit.LhLoose).
					Padding(8, 10).Radius(kit.RControl).Background(a.services.Palette.Colors().Canvas).Border(1, a.services.Palette.Colors().Theme.Border)
				if e.ID == "" {
					in.AutoFocus()
				}
			})
		}
		name, alias := loretext.LoreName(), loretext.LoreAliases()
		if e.ID == "" {
			if asked {
				ui.Text(c, loretext.CommonAnswer()).FontSize(kit.FsGroup).FontWeight(600)
				a.services.Palette.Colors().PaneNote(c, q.Text)
			} else {
				ui.Text(c, loretext.LoreNewLore()).FontSize(kit.FsGroup).FontWeight(600)
			}
			name, alias = loretext.LoreNameOptional(), loretext.LoreAliasesOptional()
			content()
		}
		a.services.Palette.Colors().PaneField(c, name, func() {
			in := ui.TextInput(c, &d.Name).Label(loretext.LoreName()).FillWidth().FontSize(kit.FsBody)
			if e.ID != "" {
				in.AutoFocus()
			}
		})
		a.services.Palette.Colors().PaneField(c, alias, func() {
			ui.TextInput(c, &d.Names).Label(loretext.LoreAliases()).Placeholder(loretext.LoreAliasHint()).FillWidth().FontSize(kit.FsBody)
		})
		if e.ID != "" {
			content()
		}
		a.services.Palette.Colors().PaneField(c, loretext.LoreCategory(), func() {
			a.services.Palette.Colors().Select(c, &d.Category, a.categoryNames()).Label(locales.DialogSettingCat()).FillWidth()
		})
		text := kit.Trim(d.Text)
		names := splitNames(d.Name + "、" + d.Names)
		kit.PaneButtons(c, func() {
			if a.services.Palette.Colors().PaneButton(c, locales.CommonCancel(), false).Clicked() {
				d.Route.CloseForm()
			}
			if !a.services.Palette.Colors().PaneButton(c, locales.CommonSave(), true).Disabled(text == "").Clicked() {
				return
			}
			a.services.Notices.Notices().SetError(a.SaveEdits())
			if a.services.Notices.Notices().ErrorText() == "" {
				d.Route.CloseForm()
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
func (a *Panel) history(c *ui.Context) {
	if back, _ := a.services.Palette.Colors().PaneBack(c, loretext.LoreAll(), ""); back {
		a.Doc.Route.CloseHistory()
	}
	a.services.Palette.Colors().Hairline(c)
	ui.Scroll(c.Key("history")).Grow(1).FillWidth().Children(func() {
		ui.Column(c).FillWidth().Gap(10).Padding(16, 20, 20, 20).Children(func() {
			ui.Text(c, loretext.LoreLog()).FontSize(kit.FsGroup).FontWeight(600)
			shown := 0
			for n := len(a.services.Book.Content().Setting().Revisions) - 1; n >= 0; n-- {
				r := a.services.Book.Content().Setting().Revisions[n]
				if len(r.Ops) == 0 || r.By != workspace.ByAuthor && !a.services.AI.AIEnabled() {
					continue // the bookkeeper's that failed changed nothing
				}
				shown++
				ui.Column(c.Key(r.ID)).FillWidth().Padding(10, 12).Gap(6).Radius(kit.RCard).Background(a.services.Palette.Colors().Raised).Border(1, a.services.Palette.Colors().Line).Children(func() {
					ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
						who := loretext.LoreWhoManual()
						if r.By != workspace.ByAuthor {
							who = locales.SettingsJobChat()
						}
						ui.Text(c, who).FontSize(kit.FsNote).FontWeight(600)
						ui.Spacer(c)
						ui.Text(c, kit.When(r.At, time.Now())).FontSize(kit.FsSmall).TextColor(a.services.Palette.Colors().Faint)
					})
					for _, op := range r.Ops {
						ui.Text(c, a.DescribeOp(op)).FontSize(kit.FsNote).LineHeight(kit.LhBody).Selectable().FillWidth()
					}
					if r.By != workspace.ByAuthor {
						if talk, on := a.services.Talk(); on {
							if words, ok := talk.TalkTitle(r.Session); ok {
								if a.services.Palette.Colors().PaneLink(c, loretext.LoreNotedIn(words), loretext.ChatViewTalk(), "讨论-"+r.ID) {
									talk.OpenTalk(r.Session)
								}
							}
						}
					}
				})
			}
			switch {
			case shown > 0:
			case a.services.AI.AIEnabled():
				a.services.Palette.Colors().PaneNote(c, loretext.LoreLogEmptyAi())
			default:
				a.services.Palette.Colors().PaneNote(c, loretext.LoreLogEmpty())
			}
		})
	})
}

// DescribeOp says what op did, in the words of the panel.
func (a *Panel) DescribeOp(op workspace.Op) string {
	e, _ := a.services.Book.Content().Setting().Entry(op.Entry)
	if op.Op == workspace.OpChange || op.Op == workspace.OpEditChange || op.Op == workspace.OpDropChange {
		var g workspace.Change
		if k := slices.IndexFunc(e.Changes, func(x workspace.Change) bool { return x.ID == op.Change }); k >= 0 {
			g = e.Changes[k]
		}
		from, text := cmp.Or(op.From, g.From), cmp.Or(op.Text, g.Text)
		said := shortName(e) + " · " + kit.FromLabel(a.services.Book.Content().Tree(), from)
		switch {
		case op.Op == workspace.OpDropChange:
			return loretext.LoreChangeRemoved(shortName(e), kit.FromLabel(a.services.Book.Content().Tree(), from))
		case op.Op == workspace.OpChange && op.Place == workspace.PlaceOpen:
			return loretext.LoreAddedWaitingChange(said, kit.Clip(text, 24))
		}
		return loretext.LoreChangeLog(shortName(e), kit.FromLabel(a.services.Book.Content().Tree(), from), kit.Clip(text, 24))
	}
	switch op.Op {
	case workspace.OpAdd:
		return loretext.LoreAddedPlace(settingPlaceName(op.Place), op.Category, op.Text)
	case workspace.OpAnswer:
		return loretext.LoreAddedActive(op.Category, op.Text)
	case workspace.OpEdit:
		return loretext.LoreRewritten(op.Text)
	case workspace.OpName:
		if len(op.Names) == 0 {
			return loretext.LoreNamesRemoved(kit.Clip(e.Text, 24))
		}
		return loretext.LoreNamesChanged(strings.Join(op.Names, locales.DraftListSep()))
	}
	text := kit.Clip(e.Text, 24)
	if op.Place != "" {
		return loretext.LoreMovedTo(settingPlaceName(op.Place), text)
	}
	return loretext.LoreRecat(op.Category, text)
}

// SaveEdits persists the active entry form before leaving or disabling the extension.
func (a *Panel) SaveEdits() error {
	d := &a.Doc
	screen, id, _ := d.Route.Screen()
	if screen == screenChange {
		return a.saveChange()
	}
	if screen != screenNewEntry && screen != screenEntryForm {
		return nil
	}
	e, _ := a.services.Book.Content().Setting().Entry(id)
	q, asked := a.services.Book.Content().Setting().Entry(d.Route.Question())
	asked = asked && d.Route.Question() != "" && e.ID == ""
	text := kit.Trim(d.Text)
	if text == "" {
		return nil
	}
	names := splitNames(d.Name + "、" + d.Names)
	var ops []workspace.Op
	switch {
	case asked:
		ops = append(ops, workspace.Op{Op: workspace.OpAnswer, Entry: q.ID, Category: d.Category, Text: text, Names: names})
	case e.ID == "":
		ops = append(ops, workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: d.Category, Text: text, Names: names})
	default:
		if text != e.Text {
			ops = append(ops, workspace.Op{Op: workspace.OpEdit, Entry: e.ID, Text: text})
		}
		if d.Category != e.Category {
			ops = append(ops, workspace.Op{Op: workspace.OpMove, Entry: e.ID, Category: d.Category})
		}
		if !slices.Equal(names, e.Names) {
			ops = append(ops, workspace.Op{Op: workspace.OpName, Entry: e.ID, Names: names})
		}
	}
	if len(ops) > 0 {
		_, err := a.services.Book.Content().Revise(workspace.Revision{By: workspace.ByAuthor, Ops: ops})
		if err != nil {
			return err
		}
		if err = a.services.Edits.Refresh(); err != nil {
			return err
		}
	}
	d.Route.CloseForm()
	return nil
}
