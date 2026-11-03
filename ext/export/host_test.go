package export

import (
	"encoding/json"
	"testing"

	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/editor"
	"writer/ext"
	"writer/ext/hostapi"
	"writer/workspace"
)

type testHost struct {
	store   *workspace.Store
	content *book.Book
	prefs   struct{ Export *Choices }
	pal     *kit.Palette
	status  kit.Status
	current string
}

func (h *testHost) Colors() *kit.Palette { return h.pal }

func (h *testHost) Content() *book.Book { return h.content }

func (h *testHost) Workspace() (*workspace.Store, string) { return h.store, h.store.Root() }

func (h *testHost) Notices() *kit.Status { return &h.status }

func (h *testHost) SaveShelf() {}

func (h *testHost) Chapter() (string, *editor.Editor, bool) {
	n, ok := h.content.Tree().Node(h.current)
	return h.current, nil, ok && !n.Folder
}

func (h *testHost) Flush() {}

func (h *testHost) CanPickFolder() bool { return false }

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
	var d *Dialog
	host, err := ext.New(ext.Options{Settings: testSettings{h}, DeclareHost: func(r *ext.Registrar) {
		ext.Provide(r, hostapi.BookService, func(*ext.Session) hostapi.Book { return h })
		ext.Provide(r, hostapi.PaletteService, func(*ext.Session) hostapi.Palette { return h })
		ext.Provide(r, hostapi.NoticesService, func(*ext.Session) hostapi.Notices { return h })
		ext.Provide(r, hostapi.JobsService, func(*ext.Session) hostapi.Jobs { return h })
		ext.Provide(r, hostapi.DialogsService, func(*ext.Session) hostapi.Dialogs { return h })
	}}, ext.Module{ID: "export", Setup: func(r *ext.Registrar) { d = registerDialog(r) }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(host.Close)
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

func (h *testHost) Store() *workspace.Store { return h.store }

func (h *testHost) Dir() string { return h.store.Root() }

func (h *testHost) ChapterTitle(id string) (string, bool) {
	n, ok := h.content.Tree().Node(id)
	return n.Title, ok && !n.Folder
}

func (h *testHost) ScreenColors() (*kit.Palette, *kit.Palette) { return h.pal, h.pal }

type testSettings struct{ h *testHost }

func (s testSettings) Load(ext.Book, string) ([]byte, bool) {
	if s.h.prefs.Export == nil {
		return nil, false
	}
	b, _ := json.Marshal(s.h.prefs.Export)
	return b, true
}

func (s testSettings) Save(_ ext.Book, _ string, data []byte) {
	_ = json.Unmarshal(data, &s.h.prefs.Export)
}
