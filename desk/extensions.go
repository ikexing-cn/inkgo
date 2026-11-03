package desk

import (
	"path/filepath"
	"sync"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/editor"
	"writer/ext"
	"writer/ext/hostapi"
	"writer/locales"
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
	if a.ai.ConfigFile() != "" {
		root = filepath.Dir(a.ai.ConfigFile())
	}
	a.extStore = newExtensionStore(root, a.extensionError)
	if len(a.shelf.MovedPhone) > 0 {
		if err := a.extStore.migrate(a.shelf.MovedPhone); err != nil {
			a.extensionError(err)
		} else {
			a.shelf.MovedPhone = nil
			a.saveShelf()
		}
	}
	names := map[string]func() string{}
	for _, mod := range mods {
		name := mod.Name
		if name == nil {
			name = func() string { return mod.ID }
		}
		names[mod.ID] = name
	}
	h, err := ext.New(ext.Options{LocalRoot: root, LocalSwitches: localExtensions{a.extStore}, BookSwitches: bookExtensions{a.extStore}, Settings: a.extStore, Injections: injectionExtensions{a.extStore},
		OnFailure: func(id string, f ext.Failure) {
			a.extensionError(extensionFailedError{name: names[id], err: f.Err})
		},
		DeclareHost: func(r *ext.Registrar) {
			ext.Declare(r, hostapi.Commands)
			ext.Declare(r, hostapi.PaneTabs)
			ext.DeclareCalls(r, hostapi.Commands, commandCall)
			ext.DeclareCalls(r, hostapi.PaneTabs, paneCall)
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
	return n.Title, ok && !n.Folder
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
	if h.a.Store != nil {
		h.a.savePrefs()
	}
}

// extensionViews holds derived tables until the host revision changes.
type extensionViews struct {
	ready    bool
	changes  uint64
	commands []namedCommand
	list     []*command
	help     []*helpRow
	system   []namedCommand
	view     []namedCommand
	panes    []ext.Item[hostapi.PaneTab]
}

func (a *App) syncExtensions() {
	if a.extensions == nil {
		return
	}
	changes := a.extensions.Changes()
	if a.extViews.ready && a.extViews.changes == changes {
		return
	}
	cache := extensionViews{ready: true, changes: changes}
	cache.commands = a.buildExtensionCommands()
	cache.list = append([]*command(nil), commandList...)
	cache.help = append([]*helpRow(nil), commandHelp...)
	for _, row := range systemCommands {
		cache.system = append(cache.system, namedCommand{id: row.id, cmd: row.cmd})
	}
	for _, c := range cache.commands {
		cache.list = append(cache.list, c.cmd)
		if c.cmd.key != 0 || c.cmd.macKey != 0 {
			cache.help = append(cache.help, &helpRow{group: locales.HelpGroupGlobal, does: c.cmd.label, cmds: []*command{c.cmd}})
		}
		if c.menu.Menu == hostapi.MenuView {
			cache.system = append(cache.system, c)
		}
	}
	cache.view = orderMenuCommands(hostapi.MenuView, viewCommandRows, cache.commands)
	cache.panes = ext.Items(a.extensions, hostapi.PaneTabs)
	a.paneTabs = append(tabTable[paneTab](nil), a.hostPaneTabs...)
	for _, item := range cache.panes {
		p := item.Value
		a.paneTabs = append(a.paneTabs, tabEntry[paneTab]{id: paneTab(item.ID), name: p.Title, visible: p.Available})
	}
	a.extViews = cache
}

func (a *App) extensionTab(id paneTab) (ext.Item[hostapi.PaneTab], bool) {
	a.syncExtensions()
	for _, item := range a.extViews.panes {
		if item.ID == string(id) {
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
	id   string
	cmd  *command
	menu hostapi.MenuPosition
}

func (a *App) buildExtensionCommands() []namedCommand {
	var out []namedCommand
	if a.extensions == nil {
		return out
	}
	for _, item := range ext.Items(a.extensions, hostapi.Commands) {
		value := item.Value
		cmd := &command{label: value.Name, key: value.Key, mods: value.Mods, macKey: value.MacKey, macMods: value.MacMods, onMac: value.OnMac, page: pageAny, by: byDesk, systemMenu: value.Menu.Menu == hostapi.MenuView}
		cmd.can = func(*App) bool { return value.Can == nil || value.Can() }
		cmd.when = cmd.can
		cmd.run = func(*App) {
			_, _ = ext.Call(a.extensions, ext.HookOf(ext.CallPoint[hostapi.Command, struct{}, struct{}]{Point: hostapi.Commands}, item.ID), struct{}{}, commandCall(value))
		}
		out = append(out, namedCommand{item.ID, cmd, value.Menu})
	}
	return out
}

func (a *App) extensionCommands() []namedCommand { a.syncExtensions(); return a.extViews.commands }

func (a *App) commandList() []*command { a.syncExtensions(); return a.extViews.list }

func (a *App) commandHelp() []*helpRow { a.syncExtensions(); return a.extViews.help }

func (a *App) viewCommands() []namedCommand { a.syncExtensions(); return a.extViews.view }

// orderMenuCommands is shared by title and system menus. Equal anchors keep
// assembly and registration order; unknown anchors append to the menu.
func orderMenuCommands(menu hostapi.Menu, base, pending []namedCommand) []namedCommand {
	var out []namedCommand
	for _, row := range base {
		out = append(out, row)
		for _, c := range pending {
			if c.menu.Menu == menu && c.menu.After == row.id {
				out = append(out, c)
			}
		}
	}
	for _, c := range pending {
		if c.menu.Menu != menu {
			continue
		}
		found := false
		for _, row := range base {
			found = found || row.id == c.menu.After
		}
		if !found {
			out = append(out, c)
		}
	}
	return out
}
