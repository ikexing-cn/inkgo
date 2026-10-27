package phone

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
	store       *workspace.Store
	content     *book.Book
	prefs       shelf.Shelf
	light, dark *kit.Palette
	id          string
	ed          *editor.Editor
	prose       ui.Rect
}

func (h *testHost) Colors() *kit.Palette { return h.light }

func (h *testHost) PhoneColors() (*kit.Palette, *kit.Palette) { return h.light, h.dark }

func (h *testHost) Content() *book.Book { return h.content }

func (h *testHost) WorkPath() string { return h.store.Root() }

func (h *testHost) ShelfPrefs() *shelf.Shelf { return &h.prefs }

func (h *testHost) SaveShelf() {}

func (h *testHost) Chapter() (string, *editor.Editor, bool) {
	n, ok := h.content.Tree().Node(h.id)
	return h.id, h.ed, ok && !n.Folder && h.ed != nil
}

func (h *testHost) openChapter(t *testing.T, id string) {
	t.Helper()
	if err := h.content.ReloadTree(); err != nil {
		t.Fatal(err)
	}
	ch, err := h.store.ReadChapter(id)
	if err != nil {
		t.Fatal(err)
	}
	h.id = id
	h.ed = editor.New(editor.NewDoc(ch.Text), editor.Options{Prose: true, Font: ui.Font{Size: kit.TextSizeDefault}, LineHeight: kit.LhPaper, NoAI: true})
}

func previewFixture(t *testing.T, text string) (*testHost, *Panel, *ui.Tester, string) {
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
	h := &testHost{store: store, content: book.Open(store), light: kit.LightPalette(), dark: kit.DarkPalette()}
	h.openChapter(t, id)
	p := &Panel{Host: h}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(h.light.Theme)
		ui.Row(c).Fill().Children(func() {
			ui.Box(c).Grow(1).Height(320).Children(func() { h.prose = h.ed.View(c).Fill().Bounds() })
			ui.Column(c).Width(420).FillHeight().Children(func() { p.View(c) })
		})
	}, 1000, 780)
	tt.Frame()
	tt.Frame()
	return h, p, tt, id
}

func click(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
}
