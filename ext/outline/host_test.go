package outline

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/editor"
	"writer/workspace"
)

type testHost struct {
	Store                   *workspace.Store
	book                    *book.Book
	pal                     *kit.Palette
	status                  kit.Status
	menu                    kit.MenuState
	plan                    Page
	current                 string
	writing                 bool
	Reveal                  func(string) error
	syncedID, syncedOutline string
}

func newTestHost(t *testing.T) *testHost {
	t.Helper()
	store, err := workspace.Create(t.TempDir(), "测试作品", "网文")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	h := &testHost{Store: store, book: book.Open(store), pal: kit.LightPalette()}
	h.plan.Services = Services{Book: testBook{h}, Palette: h, Notices: h, Dialogs: h, Desktop: h}
	return h
}

func (h *testHost) Colors() *kit.Palette { return h.pal }

func (h *testHost) Content() *book.Book { return h.book }

func (h *testHost) Notices() *kit.Status { return &h.status }

func (h *testHost) WorkPath() string { return h.Store.Root() }

func (h *testHost) Chapter() (string, *editor.Editor, bool) {
	n, ok := h.book.Tree().Node(h.current)
	return h.current, nil, ok && !n.Folder
}

func (h *testHost) OpenWriting(id string) { h.current, h.writing = id, true }

func (h *testHost) ShowOutline() { h.writing = false }

func (h *testHost) SettingColumn(*ui.Context) {}

func (h *testHost) Dialogs() (func(string, string) (string, error), func(string) error) {
	return nil, h.Reveal
}

func (h *testHost) DropMenu(c *ui.Context, button ui.Element, key string, place func(ui.Element), build func(*kit.Menu)) {
	h.menu.DropMenu(c, h.pal, button, key, place, build)
}

func (h *testHost) draw(c *ui.Context) {
	c.SetTheme(h.pal.Theme)
	ui.Column(c).Fill().Children(func() {
		if h.writing {
			h.plan.ChapterStory(c)
		} else {
			h.plan.View(c)
		}
	})
	h.plan.Dialog(c)
}

func click(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	tt.Frame()
}

func find(t *testing.T, tt *ui.Tester, label string) ui.Rect {
	t.Helper()
	rect, ok := tt.Find(label)
	if !ok {
		t.Fatal(label, tt.Texts())
	}
	return rect
}

func hasText(t *testing.T, tt *ui.Tester, text string) {
	t.Helper()
	if !tt.HasText(text) {
		t.Fatal(text, tt.Texts())
	}
}

func outline(h *testHost) workspace.Plan { return h.book.Plan() }

func outlineErr(h *testHost) error { return h.plan.loadErr }

func forgetOutline(h *testHost) { h.plan.Enter() }

type testBook struct{ h *testHost }

func (b testBook) Store() *workspace.Store { return b.h.Store }

func (b testBook) Content() *book.Book { return b.h.book }

func (b testBook) Flush() {}

func (b testBook) Chapter() (string, *editor.Editor, bool) { return b.h.Chapter() }

func (b testBook) ChapterTitle(id string) (string, bool) {
	n, ok := b.h.book.Tree().Node(id)
	if !ok {
		return "", false
	}
	return n.Title, !n.Folder
}

func (b testBook) Dir() string { return b.h.Store.Root() }

func (h *testHost) ScreenColors() (*kit.Palette, *kit.Palette) { return h.pal, h.pal }

func (h *testHost) CanPickFolder() bool { return false }

func (h *testHost) PickInto(*ui.Context, string, func(string)) bool { return false }

func (h *testHost) Writing() bool { return h.writing }

func (h *testHost) SlideEdge(ui.Element) {}

func (h *testHost) OpenSettings(string) {}

func (h *testHost) ShowPage(string) { h.writing = false }

func (h *testHost) SameFolder(a, b string) bool { return a == b }

func (h *testHost) Selected() (string, bool, bool) { return h.current, false, false }
