package desk

import (
	"cmp"
	"context"
	"path/filepath"
	"slices"
	"sync"

	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/editor"
	"writer/ext"
	"writer/ext/ai/aiapi"
	"writer/ext/hostapi"
	"writer/locales"
	"writer/workspace"
)

type extensionFailedError struct {
	name func() string
	err  error
}

func (e extensionFailedError) Error() string { return locales.ExtFailed(e.name(), e.err.Error()) }

type extensionNotices struct {
	mu         sync.Mutex
	invalidate func()
	errors     []error
}

func (a *App) extensionError(err error) {
	if err == nil {
		return
	}
	n := &a.extNotices
	n.mu.Lock()
	n.errors = append(n.errors, err)
	invalidate := n.invalidate
	n.mu.Unlock()
	if invalidate != nil {
		invalidate()
	}
}

func (a *App) extensionFrame(c *ui.Context) {
	n := &a.extNotices
	n.mu.Lock()
	sv := c.Services()
	n.invalidate = sv.Invalidate
	n.mu.Unlock()
	if a.extensions != nil {
		a.extensions.Settle()
	}
	a.settleRequest()
	n.mu.Lock()
	errors := n.errors
	n.errors = nil
	n.mu.Unlock()
	for _, err := range errors {
		a.status.SetError(err)
	}
	a.syncExtensions()
}

func (a *App) initExtensions(mods []ext.Module) {
	root := ""
	if a.configFile != "" {
		root = filepath.Dir(a.configFile)
	}
	a.extStore = newExtensionStore(root, a.extensionError)
	changed, migrationErr := a.shelf.MigrateExtensions(a.extStore.migrate, a.extStore.mergeMigration)
	a.extensionError(migrationErr)
	if changed {
		a.saveShelf()
	}
	names := map[string]func() string{}
	for _, mod := range mods {
		name := mod.Name
		if name == nil {
			name = func() string { return mod.ID }
		}
		names[mod.ID] = name
	}
	h, err := ext.New(ext.Options{BookState: viewStateStore{a}, BookDir: workspace.ExtDir, LocalRoot: root, LocalSwitches: localExtensions{a.extStore}, BookSwitches: bookExtensions{a.extStore}, Settings: a.extStore, Injections: injectionExtensions{a.extStore},
		OnFailure: func(id string, f ext.Failure) {
			a.extensionError(extensionFailedError{name: names[id], err: f.Err})
		},
		HostKeys: hostKeys(),
		DeclareHost: func(r *ext.Registrar) {
			a.aiRef = ext.UseOptional(r, aiapi.Service)

			ext.Declare(r, hostapi.ErrorTexts, ext.Hidden)
			ext.Declare(r, hostapi.SettingsDetails, "prefs")
			ext.DeclareCalls(r, hostapi.SettingsDetails, func(d hostapi.SettingsDetail) func(*ui.Context) struct{} {
				return func(c *ui.Context) struct{} { d.Draw(c); return struct{}{} }
			})
			ext.Declare(r, hostapi.Commands, ext.Hidden)
			ext.Declare(r, hostapi.PaneTabs, "pane")
			ext.DeclareName(r, hostapi.PaneTabs, func(v hostapi.PaneTab) func() string { return v.Title })
			ext.Declare(r, hostapi.Overlays, "dialog")
			declareLifecycle(r, a)
			ext.DeclareCalls(r, hostapi.Overlays, overlayCall)
			ext.DeclareCalls(r, hostapi.Commands, commandCall)
			ext.DeclareCalls(r, hostapi.PaneTabs, paneCall)
			ext.Provide(r, hostapi.NoticesService, func(*ext.Session) hostapi.Notices { return extensionHost{a} })
			ext.Provide(r, hostapi.JobsService, func(*ext.Session) hostapi.Jobs { return jobsHost{extensionHost{a}} })
			ext.Provide(r, hostapi.DialogsService, func(*ext.Session) hostapi.Dialogs { return dialogsHost{extensionHost{a}} })
			ext.Provide(r, hostapi.EditingService, func(*ext.Session) hostapi.Editing { return extensionHost{a} })
			ext.Provide(r, hostapi.BookService, func(*ext.Session) hostapi.Book { return extensionHost{a} })
			ext.Provide(r, hostapi.PaletteService, func(*ext.Session) hostapi.Palette { return extensionHost{a} })
			ext.Provide(r, hostapi.PaneService, func(*ext.Session) hostapi.Pane { return extensionHost{a} })
		}}, mods...)
	if err != nil {
		panic(err)
	}
	a.extensions = h
	a.syncExtensions()
}

func commandCall(c hostapi.Command) func(struct{}) struct{} {
	return func(struct{}) struct{} {
		if c.Run != nil {
			c.Run()
		}
		return struct{}{}
	}
}

func paneCall(p hostapi.PaneTab) func(*ui.Context) struct{} {
	return func(c *ui.Context) struct{} {
		if p.Draw != nil {
			p.Draw(c)
		}
		return struct{}{}
	}
}

type extensionHost struct{ a *App }

func (h extensionHost) Chapter() (string, *editor.Editor, bool) {
	return (appHost{h.a}).Chapter()
}

func (h extensionHost) ChapterTitle(id string) (string, bool) {
	if h.a.book == nil {
		return "", false
	}
	n, ok := h.a.book.Tree().Node(id)
	if !ok || n == nil {
		return "", false
	}
	return n.Title, !n.Folder
}

func (h extensionHost) Dir() string {
	if h.a.Store == nil {
		return ""
	}
	return h.a.Store.Root()
}

func (h extensionHost) Colors() *kit.Palette { return h.a.pal }

func (h extensionHost) ScreenColors() (*kit.Palette, *kit.Palette) { return h.a.light, h.a.dark }

func (h extensionHost) Show(tab string) {
	h.a.doc.hidden, h.a.doc.tab, h.a.write.focus = false, paneTab(tab), false
	h.a.paneChanged(tab)
	if h.a.Store != nil {
		h.a.savePrefs()
	}
}

// extensionViews holds derived tables until the host or language changes.
type extensionViews struct {
	activities       []ext.Item[hostapi.Activity]
	beforeEditor     []ext.Item[hostapi.DrawItem]
	selectionButtons []ext.Item[hostapi.SelectionButton]
	beforeProse      []ext.Item[hostapi.DrawItem]
	// Host contributions precede module callbacks and survive module switches.
	hostTicks      []ext.Item[hostapi.FrameTick]
	hostLeave      []ext.Item[func()]
	localeRevision uint64
	outlines       bool
	ready          bool
	changes        uint64
	commands       []namedCommand
	list           []*command
	help           []*helpRow
	system         []namedCommand
	view           []namedCommand
	file           []namedCommand
	edit           []namedCommand
	overlays       []ext.Item[hostapi.Overlay]
	panes          []ext.Item[hostapi.PaneTab]
	footer, title  []ext.Item[hostapi.DrawItem]
	ticks          []ext.Item[hostapi.FrameTick]
	leave          []ext.Item[func()]
	pending        []ext.Item[hostapi.PendingSource]
	names          []ext.Item[hostapi.NameProvider]
	prose          []ext.Item[hostapi.ProseMarkProvider]
	settings       []ext.Item[hostapi.SettingsPage]
	sections       []ext.Item[hostapi.SettingsSection]
	settingsTabs   map[string]string
	modules        []ext.Module
	moduleViews    map[string]extensionView
	groups         []extensionGroup
	pageOwners     map[string][]string
}

func (a *App) syncExtensions() {
	if a.extensions == nil {
		return
	}
	service, on := a.aiRef.Get()
	if on && service != a.aiResolved {
		a.aiResolved = service
		a.ai = service
	}
	if a.aiWasOn != on {
		a.aiWasOn = on
		a.applyAI(on)
	}
	changes := a.extensions.Changes()
	revision := a.localeRevision.Load()
	if a.extViews.ready && a.extViews.changes == changes && a.extViews.localeRevision == revision {
		return
	}
	a.mergePages()
	cache := extensionViews{ready: true, changes: changes, localeRevision: revision}
	cache.selectionButtons = ext.Items(a.extensions, hostapi.SelectionButtons)
	cache.beforeEditor = ext.Items(a.extensions, hostapi.BeforeEditor)
	cache.beforeProse = ext.Items(a.extensions, hostapi.BeforeProse)
	a.cacheExtensionViews(&cache)
	_, cache.outlines = a.outlineValue(a.write.node)
	cache.activities = ext.Items(a.extensions, hostapi.Activities)
	cache.commands = a.buildExtensionCommands()
	for _, cmd := range commandList {
		if cmd != cmdTake || a.hasEntries() {
			cache.list = append(cache.list, cmd)
		}
	}
	for _, cmd := range commandList {
		a.applyKeyClaim(cmd, hostCommandID(cmd), ext.HostOwner)
	}
	cache.help = append(cache.help, commandHelp...)
	for _, row := range systemCommands {
		cache.system = append(cache.system, namedCommand{id: row.id, cmd: row.cmd})
	}
	for _, c := range cache.commands {
		cache.list = append(cache.list, c.cmd)
		if c.cmd.key != 0 || c.cmd.macKey != 0 {
			row := &helpRow{id: c.id, group: locales.HelpGroupGlobal, does: c.cmd.label, cmds: []*command{c.cmd}}
			value := c.value
			if value.HelpGroup != nil {
				row.group = value.HelpGroup
			}
			if value.HelpText != nil {
				row.does = value.HelpText
			}
			at := len(cache.help)
			if value.HelpAfter != "" {
				for i, h := range cache.help {
					if h.id == value.HelpAfter {
						at = i + 1
					}
				}
			}

			if value.HelpBefore != "" {
				for i, h := range cache.help {
					if h.id == value.HelpBefore {
						at = i
						break
					}
				}
			}

			cache.help = slices.Insert(cache.help, at, row)
		}
		if c.menu.Menu == hostapi.MenuView || c.menu.Menu == hostapi.MenuFile || c.menu.Menu == hostapi.MenuEdit {
			cache.system = append(cache.system, c)
		}
	}
	cache.edit = orderMenuCommands(hostapi.MenuEdit, editCommandRows, cache.commands)
	cache.file = orderMenuCommands(hostapi.MenuFile, fileCommandRows, cache.commands)
	cache.overlays = ext.Items(a.extensions, hostapi.Overlays)
	cache.view = orderMenuCommands(hostapi.MenuView, viewCommandRows, cache.commands)
	cache.panes = ext.Items(a.extensions, hostapi.PaneTabs)
	a.paneTabs = append(tabTable[paneTab](nil), a.hostPaneTabs...)
	for _, item := range cache.panes {
		p := item.Value
		id := item.ID
		if p.Key != "" {
			id = p.Key
		}
		row := tabEntry[paneTab]{id: paneTab(id), name: p.Title, visible: p.Available, fallback: p.Fallback}
		if p.Count != nil {
			row.count = func() int {
				n := 0
				_ = ext.Guard(a.extensions, item.Owner, item.ID, func() { n = p.Count() })
				return n
			}
		}
		at := len(a.paneTabs)
		before := p.Before
		if !slices.ContainsFunc(a.paneTabs, func(row tabEntry[paneTab]) bool { return string(row.id) == before }) {
			before = p.BeforeFallback
		}
		for i, existing := range a.paneTabs {
			if before != "" && string(existing.id) == before {
				at = i
				break
			}
		}
		a.paneTabs = slices.Insert(a.paneTabs, at, row)
	}
	cache.footer = ext.Items(a.extensions, hostapi.FooterItems)
	cache.title = ext.Items(a.extensions, hostapi.TitleItems)
	for _, item := range ext.Items(a.extensions, hostapi.FrameTicks) {
		if item.Owner == ext.HostOwner {
			cache.hostTicks = append(cache.hostTicks, item)
		} else {
			cache.ticks = append(cache.ticks, item)
		}
	}
	for _, item := range ext.Items(a.extensions, hostapi.BeforeLeave) {
		if item.Owner == ext.HostOwner {
			cache.hostLeave = append(cache.hostLeave, item)
		} else {
			cache.leave = append(cache.leave, item)
		}
	}
	cache.pending = ext.Items(a.extensions, hostapi.PendingGroups)
	cache.names = ext.Items(a.extensions, hostapi.NameHighlights)
	cache.prose = ext.Items(a.extensions, hostapi.ProseMarks)
	cache.settings = ext.Declared(a.extensions, hostapi.SettingsPages)
	cache.sections = ext.Items(a.extensions, hostapi.SettingsSections)
	cache.settingsTabs = map[string]string{}
	cache.pageOwners = map[string][]string{}
	slices.SortStableFunc(cache.settings, func(x, y ext.Item[hostapi.SettingsPage]) int { return cmp.Compare(x.Value.Order, y.Value.Order) })
	a.prefTabs = slices.Clone(a.hostPrefTabs)
	pagesByKey := map[string]string{}
	for _, item := range cache.settings {
		p := item.Value
		key := p.Key
		if key == "" {
			key = item.ID
		}
		id, shared := pagesByKey[key]
		if !shared {
			id = key
			if item.ID == "ai.settings" {
				id = "AI"
			}
			pagesByKey[key] = id
			a.prefTabs = append(a.prefTabs, tabEntry[string]{id: id, name: p.Title, label: p.Label, order: p.Order, fallback: item.Owner == "ai"})
		}
		cache.settingsTabs[item.ID] = id
		if !slices.Contains(cache.pageOwners[id], item.Owner) {
			cache.pageOwners[id] = append(cache.pageOwners[id], item.Owner)
		}
	}
	slices.SortStableFunc(a.prefTabs, func(x, y tabEntry[string]) int { return cmp.Compare(x.order, y.order) })
	a.extViews = cache
	a.cacheExtensionNotes()
}

func (a *App) extensionTab(id paneTab) (ext.Item[hostapi.PaneTab], bool) {
	a.syncExtensions()
	for _, item := range a.extViews.panes {
		key := item.Value.Key
		if key == "" {
			key = item.ID
		}
		if key == string(id) {
			return item, true
		}
	}
	return ext.Item[hostapi.PaneTab]{}, false
}

func (a *App) extensionPane(c *ui.Context, item ext.Item[hostapi.PaneTab]) {
	a.pal.PaneTop(c).Children(func() { a.panelTabs(c) })
	// The outer scope restores the parent after a nested scope panics.
	// Hide the partial subtree, including input targets, on failure.
	body := ui.Column(c).Fill()
	var err error
	body.Children(func() {
		_, err = ext.Call(a.extensions, ext.HookOf(ext.CallPoint[hostapi.PaneTab, *ui.Context, struct{}]{Point: hostapi.PaneTabs}, item.ID), c, paneCall(item.Value))
	})
	body.Hidden(err != nil)
	a.syncExtensions()
}

type namedCommand struct {
	id    string
	cmd   *command
	menu  hostapi.MenuPosition
	value hostapi.Command
}

func (a *App) buildExtensionCommands() []namedCommand {
	var out []namedCommand
	if a.extensions == nil {
		return out
	}
	for _, item := range ext.Items(a.extensions, hostapi.Commands) {
		value := item.Value
		cmd := &command{label: value.Name, key: value.Key, mods: value.Mods, macKey: value.MacKey, macMods: value.MacMods, onMac: value.OnMac, page: pageAny, by: byDesk, systemMenu: value.Menu.Menu == hostapi.MenuView || value.Menu.Menu == hostapi.MenuFile || value.Menu.Menu == hostapi.MenuEdit}
		a.applyKeyClaim(cmd, item.ID, item.Owner)

		cmd.presentNow = value.Present
		if value.Hint != nil {
			cmd.hint = func(*App) string { return value.Hint() }
		}
		cmd.can = func(*App) bool { return value.Can == nil || value.Can() }
		cmd.when = cmd.can
		cmd.run = func(*App) {
			_, _ = ext.Call(a.extensions, ext.HookOf(ext.CallPoint[hostapi.Command, struct{}, struct{}]{Point: hostapi.Commands}, item.ID), struct{}{}, commandCall(value))
		}
		id := item.ID
		if value.ID != "" {
			id = value.ID
		}
		out = append(out, namedCommand{id: id, cmd: cmd, menu: value.Menu, value: value})
	}
	return out
}

func (a *App) extensionCommands() []namedCommand { a.syncExtensions(); return a.extViews.commands }

func (a *App) commandList() []*command { a.syncExtensions(); return a.extViews.list }

func (a *App) commandHelp() []*helpRow { a.syncExtensions(); return a.extViews.help }

func (a *App) viewCommands() []namedCommand { a.syncExtensions(); return a.extViews.view }

// orderMenuCommands uses order when an anchor is missing, preserving menu groups.
func orderMenuCommands(menu hostapi.Menu, base, pending []namedCommand) []namedCommand {

	rows := append([]namedCommand(nil), base...)
	for i := range rows {
		rows[i].menu.Order = (i + 1) * 10
	}
	for _, c := range pending {
		if c.menu.Menu == menu {
			rows = append(rows, c)
		}
	}
	slices.SortStableFunc(rows, func(a, b namedCommand) int { return cmp.Compare(a.menu.Order, b.menu.Order) })
	ids := map[string]bool{}
	for _, r := range rows {
		ids[r.id] = true
	}
	var out []namedCommand
	seen := map[string]bool{}
	var emit func(namedCommand)
	emit = func(r namedCommand) {
		if seen[r.id] {
			return
		}
		seen[r.id] = true
		out = append(out, r)
		for _, c := range rows {
			if c.menu.After == r.id {
				emit(c)
			}
		}
	}
	for _, r := range rows {
		if r.menu.After == "" || !ids[r.menu.After] {
			emit(r)
		}
	}
	for _, r := range rows {
		emit(r)
	}

	return out
}
func (a *App) editCommands() []namedCommand { a.syncExtensions(); return a.extViews.edit }

var editCommandRows = []namedCommand{{id: "undo", cmd: cmdUndo}, {id: "redo", cmd: cmdRedo}, {id: "edit-find-separator"}, {id: "rename", cmd: cmdRename}}

func (h extensionHost) Store() *workspace.Store { return h.a.Store }

func (h extensionHost) Content() *book.Book { return h.a.book }

func (h extensionHost) Flush() { h.a.flush() }

func (h extensionHost) Notices() *kit.Status { return &h.a.status }

func (h extensionHost) StartJob(sv ui.Services, fn func(context.Context) func()) {
	h.a.jobs.start(sv, func() func() { return fn(context.Background()) })
}

func (h extensionHost) CanPickFolder() bool { return h.a.PickFolder != nil }

func (h extensionHost) Reveal(path string) error {
	if h.a.Reveal != nil {
		return h.a.Reveal(path)
	}
	return nil
}
func (h extensionHost) SaveInto(c *ui.Context, title, name string, picked func(string, error)) bool {
	return sessionDialogs{extensionHost: extensionHost{h.a}}.SaveInto(c, title, name, picked)
}

func (h extensionHost) PickInto(c *ui.Context, title string, picked func(string)) bool {
	return h.a.pickInto(c, title, picked)
}

func overlayCall(o hostapi.Overlay) func(*ui.Context) struct{} {
	return func(c *ui.Context) struct{} {
		if o.Draw != nil {
			o.Draw(c)
		}
		return struct{}{}
	}
}

func (a *App) extensionOverlays(c *ui.Context) {
	a.syncExtensions()
	for _, item := range a.extViews.overlays {
		body := ui.Box(c)
		var err error
		body.Children(func() {
			_, err = ext.Call(a.extensions, ext.HookOf(ext.CallPoint[hostapi.Overlay, *ui.Context, struct{}]{Point: hostapi.Overlays}, item.ID), c, overlayCall(item.Value))
		})
		body.Hidden(err != nil)
	}
	a.syncExtensions()
}

var fileCommandRows = []namedCommand{
	{id: "new-chapter", cmd: cmdNewChapter}, {id: "new-folder", cmd: cmdNewFolder},
	{id: "file-save-separator"}, {id: "save", cmd: cmdSave},
	{id: "file-organize-separator"}, {id: "categories"},
}

func (a *App) fileCommands() []namedCommand { a.syncExtensions(); return a.extViews.file }

// busyPresented reports whether an extension already shows the operation's progress.
func (a *App) busyPresented() bool {
	for _, i := range a.extViews.activities {
		shown := false
		_ = ext.Guard(a.extensions, i.Owner, i.ID, func() { shown = i.Value.Presented != nil && i.Value.Presented() })
		if shown {
			return true
		}
	}
	return false
}

func (a *App) nonblockingActivity() bool {
	for _, i := range a.extViews.activities {
		active := false
		_ = ext.Guard(a.extensions, i.Owner, i.ID, func() { active = i.Value.Nonblocking != nil && i.Value.Nonblocking() })
		if active {
			return true
		}
	}
	return false
}
