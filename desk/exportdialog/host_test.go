package exportdialog

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/editor"
	"writer/shelf"
	"writer/workspace"
)

type testHost struct {
	store   *workspace.Store
	content *book.Book
	prefs   shelf.Shelf
	pal     *kit.Palette
	status  kit.Status
	current string
}

func (h *testHost) Colors() *kit.Palette { return h.pal }

func (h *testHost) Content() *book.Book { return h.content }

func (h *testHost) Workspace() (*workspace.Store, string) { return h.store, h.store.Root() }

func (h *testHost) Notices() *kit.Status { return &h.status }

func (h *testHost) ShelfPrefs() *shelf.Shelf { return &h.prefs }

func (h *testHost) SaveShelf() {}

func (h *testHost) Chapter() (string, *editor.Editor, bool) {
	n, ok := h.content.Tree().Node(h.current)
	return h.current, nil, ok && !n.Folder
}

func (h *testHost) Flush() {}

func (h *testHost) PickInto(*ui.Context, string, func(string)) bool { return false }

func (h *testHost) StartJob(_ ui.Services, work func() func()) { work()() }

func (h *testHost) Dialogs() (func(string, string) (string, error), func(string) error) {
	return nil, nil
}

func dialogFixture(t *testing.T, text string) (*testHost, *Dialog, *ui.Tester, string) {
	t.Helper()
	store, err := workspace.Create(t.TempDir(), "测试作品", "网文")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	id, err := store.NewChapter("", "第一章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.WriteChapter(id, text, nil); err != nil {
		t.Fatal(err)
	}
	h := &testHost{store: store, content: book.Open(store), pal: kit.LightPalette(), current: id}
	d := &Dialog{Host: h}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(h.pal.Theme)
		ui.Box(c).Fill()
		if d.Open {
			d.View(c)
		}
	}, 1266, 780)
	tt.Frame()
	return h, d, tt, id
}
