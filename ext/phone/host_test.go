package phone

import (
	"testing"

	"encoding/json"
	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/editor"
	"writer/ext"
	"writer/ext/hostapi"
	"writer/workspace"
)

type testHost struct {
	store       *workspace.Store
	content     *book.Book
	prefs       struct{ Phone *Typesetting }
	light, dark *kit.Palette
	id          string
	ed          *editor.Editor
	prose       ui.Rect
}

func (h *testHost) Colors() *kit.Palette { return h.light }

func (h *testHost) ScreenColors() (*kit.Palette, *kit.Palette) { return h.light, h.dark }

func (h *testHost) Content() *book.Book { return h.content }

func (h *testHost) Dir() string { return h.store.Root() }

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
	var p *Panel
	host, err := ext.New(ext.Options{Settings: testSettings{h}, DeclareHost: func(r *ext.Registrar) {
		ext.Provide(r, hostapi.BookService, func(*ext.Session) hostapi.Book { return h })
		ext.Provide(r, hostapi.PaletteService, func(*ext.Session) hostapi.Palette { return h })
	}}, ext.Module{ID: "phone", Setup: func(r *ext.Registrar) {
		p = &Panel{book: ext.Use(r, hostapi.BookService), palette: ext.Use(r, hostapi.PaletteService), setting: ext.Setting(r, "typesetting", ext.Schema[Typesetting]{Default: defaultPhone()})}
		r.OnStart(func(s *ext.Session) error { p.session = s; return nil })
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err = host.Open(ext.Book{Dir: store.Root()}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(host.Close)
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

func (h *testHost) ChapterTitle(id string) (string, bool) {
	n, ok := h.content.Tree().Node(id)
	return n.Title, ok && !n.Folder
}

type testSettings struct{ h *testHost }

func (s testSettings) Load(ext.Book, string) ([]byte, bool) {
	if s.h.prefs.Phone == nil {
		return nil, false
	}
	data, _ := json.Marshal(s.h.prefs.Phone)
	return data, true
}
func (s testSettings) Save(_ ext.Book, _ string, data []byte) { json.Unmarshal(data, &s.h.prefs.Phone) }
