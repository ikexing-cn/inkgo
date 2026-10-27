package desk

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/editor"
	"writer/shelf"
	"writer/workspace"
)

// appHost is what the pages of the packages under desk reach of App: the
// methods their Host interfaces name, which App does not export.
type appHost struct{ a *App }

func (h appHost) Colors() *kit.Palette { return h.a.pal }

func (h appHost) Content() *book.Book { return h.a.book }

func (h appHost) Workspace() (*workspace.Store, string) { return h.a.Store, h.a.path }

func (h appHost) WorkPath() string {
	if h.a.Store == nil {
		return ""
	}
	return h.a.Store.Root()
}

func (h appHost) Notices() *kit.Status { return &h.a.status }

func (h appHost) ShelfPrefs() *shelf.Shelf { return &h.a.shelf }

// Chapter gives an empty ID off the writing page; the editor is the selected tab's.
// open also checks that the chapter exists and is not a folder.
func (h appHost) Chapter() (id string, ed *editor.Editor, open bool) {
	if h.a.page == pageWrite {
		id = h.a.write.node
	}
	open, _ = h.a.chapterOpen()
	return id, h.a.write.cur().ed, open
}

func (h appHost) StartJob(sv ui.Services, fn func() func()) { h.a.jobs.start(sv, fn) }

func (h appHost) Dialogs() (func(string, string) (string, error), func(string) error) {
	return h.a.SaveFile, h.a.Reveal
}

func (h appHost) PhoneColors() (*kit.Palette, *kit.Palette) { return h.a.light, h.a.dark }

func (h appHost) SyncPlanOutline(id, text string) {
	if h.a.write.node == id {
		d := &h.a.write.cur().outline
		d.Text, d.Saved, d.Seen = text, text, text
	}
}

func (h appHost) OpenWriting(id string) {
	h.a.page = pageWrite
	h.a.openNode(id)
}

func (h appHost) ShowOutline() { h.a.page = pagePlan }

func (h appHost) DropMenu(c *ui.Context, b ui.Element, key string, place func(ui.Element), build func(*kit.Menu)) {
	h.a.dropMenu(c, b, key, place, build)
}

func (a *App) dropMenu(c *ui.Context, b ui.Element, key string, place func(ui.Element), build func(*kit.Menu)) {
	a.menu.DropMenu(c, a.pal, b, key, place, build)
}

func (h appHost) SaveShelf() { h.a.saveShelf() }

func (h appHost) Flush() { h.a.flush() }

func (h appHost) SettingColumn(c *ui.Context) { h.a.settingColumn(c) }

func (h appHost) PickInto(c *ui.Context, title string, picked func(string)) bool {
	return h.a.pickInto(c, title, picked)
}
