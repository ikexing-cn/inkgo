package check

import (
	"context"
	"encoding/json"
	"github.com/egoist/mygo/ui"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	coreai "writer/ai"
	"writer/desk"
	"writer/desk/book"
	"writer/editor"
	"writer/ext"
	aimod "writer/ext/ai"
	"writer/ext/ai/aiapi"
	"writer/ext/backup"
	"writer/ext/cats"
	contextmod "writer/ext/context"
	continuation "writer/ext/continue"
	"writer/ext/draft"
	"writer/ext/export"
	"writer/ext/find"
	"writer/ext/focus"
	"writer/ext/goal"
	"writer/ext/hooks"
	"writer/ext/lore"
	"writer/ext/marks"
	"writer/ext/names"
	"writer/ext/notes"
	"writer/ext/outline"
	"writer/ext/phone"
	"writer/ext/propose"
	"writer/ext/snaps"
	"writer/ext/talk"
	"writer/ext/trash"
	"writer/ext/usage"
	"writer/inference"
	"writer/internal/testshot"
	"writer/model"
	"writer/workspace"
)

type testApp struct {
	*desk.App
	panel   *Panel
	backend *coreai.Service
	book    *book.Book
}
type testPage string

const (
	pageCheck testPage = "check.check"
	pageWrite testPage = "host.write"
)

func newApp(config string) *testApp {
	km, p := New()
	backend := coreai.New(config)
	bm, _ := backup.New(false)
	mods := []ext.Module{focus.Module, goal.Module, names.Module, find.Module, notes.Module, lore.Module, outline.Module, hooks.Module, talk.Module, continuation.Module, draft.Module, propose.Module, km, usage.Module, snaps.Module, trash.Module, cats.Module, bm, export.Module, phone.Module, contextmod.Module, marks.Module, aimod.New(aimod.Options{ConfigFile: config, Factory: func() aiapi.Backend { return backend }})}
	return &testApp{App: desk.New(config, mods...), panel: p, backend: backend}
}
func (a *testApp) Open(path string) error {
	if err := a.App.Open(path); err != nil {
		return err
	}
	a.book = a.panel.book()
	return nil
}
func fixture(t *testing.T) (*testApp, *ui.Tester, workspace.Entry) {
	t.Helper()
	root := t.TempDir()
	st, err := workspace.Create(filepath.Join(root, "work"), "测试作品", "网文", "zh-Hans")
	if err != nil {
		t.Fatal(err)
	}
	r, err := st.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "主角怕黑"}}})
	if err != nil {
		t.Fatal(err)
	}
	st.Close()
	if err = os.WriteFile(filepath.Join(root, "work", ".writer", "desk.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	was := desk.Library
	desk.Library = func() string { return filepath.Join(root, "library") }
	t.Cleanup(func() { desk.Library = was })
	a := newApp(filepath.Join(root, "providers.yaml"))
	if err = a.Open(filepath.Join(root, "work")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	tt := ui.NewTester(a.View, 1266, 780)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	e, _ := a.book.Setting().Entry(r.Ops[0].Entry)
	return a, tt, e
}
func writingFixture(t *testing.T, text string) (*testApp, *ui.Tester, string) {
	t.Helper()
	a, tt, _ := fixture(t)
	a.book.Timing.SaveAfter, a.book.Timing.CheckEvery = 0, time.Hour
	id, err := a.Store.NewChapter("", "第一章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Store.WriteChapter(id, text, nil); err != nil {
		t.Fatal(err)
	}
	click(t, tt, "正文")
	return a, tt, id
}
func checkOf(a *testApp) *Panel { return a.panel }
func (a *testApp) revise(ops ...workspace.Op) {
	if _, err := a.book.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: ops}); err != nil {
		panic(err)
	}
	if err := a.panel.revisions.Refresh(); err != nil {
		panic(err)
	}
}
func (a *testApp) warn(err error)     { a.panel.warn(err) }
func (a *testApp) loadTree() error    { return a.book.ReloadTree() }
func (a *testApp) loadChecks()        { a.panel.loadChecks() }
func (a *testApp) openNode(id string) { a.panel.desktop.OpenWriting(id) }
func (a *testApp) openPrefs(id string) {
	if id == "reminders" {
		id = "check.reminders"
	}
	a.panel.desktop.OpenSettings(id)
}
func (a *testApp) flush() { a.panel.bookHost.Flush() }
func (a *testApp) loadPrefs() {
	raw, _ := os.ReadFile(a.Store.PrefsPath())
	var state map[string]json.RawMessage
	_ = json.Unmarshal(raw, &state)
	a.panel.loadState(state)
}
func (a *testApp) checkPending() (hostPendingGroup, bool) {
	g, ok := a.panel.checkPending()
	out := hostPendingGroup{}
	for _, it := range g.Items {
		out.items = append(out.items, hostPendingItem{it.Text, it.Open})
	}
	return out, ok
}

type hostPendingGroup struct{ items []hostPendingItem }
type hostPendingItem struct {
	text string
	open func(ui.Services)
}

func (a *testApp) checkState() (string, int, bool, string) { return a.panel.checkState() }
func (a *testApp) errText(err error) string                { return a.panel.status().FormatError(err) }
func (a *testApp) attach(st *workspace.Store, path string, _ bool) error {
	st.Close()
	return a.Open(path)
}
func shownPage(a *testApp) testPage         { return testPage(a.panel.desktop.Page()) }
func showPage(a *testApp, p testPage)       { a.panel.desktop.ShowPage(string(p)) }
func shownEditor(a *testApp) *editor.Editor { _, ed, _ := a.panel.bookHost.Chapter(); return ed }
func shownChapter(a *testApp) string        { return a.panel.node() }
func setting(a *testApp) workspace.Setting  { return a.book.Setting() }
func hintOf(a *testApp) string              { return a.panel.status().HintText() }
func existsCheck(a *testApp, id string) bool {
	d, _ := a.Store.ExtDir("check")
	_, err := os.Stat(filepath.Join(d, id+".json"))
	return err == nil
}
func click(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
}
func until(t *testing.T, tt *ui.Tester, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatal("timed out: "+what, tt.Texts())
		}
		time.Sleep(5 * time.Millisecond)
		tt.Frame()
	}
}
func shot(t *testing.T, tt *ui.Tester, name string) {
	t.Helper()
	tt.Frame()
	tt.Frame()
	testshot.Save(t, name, tt.Image())
}
func penWith(a *testApp, f model.Completer) { penWithin(a, f, model.Builtin("deepseek-flash")) }
func penWithin(a *testApp, f model.Completer, m model.Model) {
	w := a.Store.Work()
	w.Discussion = workspace.Selection{Provider: "deepseek", Model: "deepseek-flash", Effort: string(model.EffortHigh)}
	if err := a.Store.SaveWork(w); err != nil {
		panic(err)
	}
	a.backend.Open = func(workspace.Selection, *workspace.Store) (inference.Model, error) {
		return inference.Model{Model: m, Completer: f, Name: "deepseek/deepseek-flash", Effort: model.EffortHigh}, nil
	}
}

type stream struct {
	pieces []string
	hold   chan struct{}
	fail   error
	mu     sync.Mutex
	asked  []model.Request
}

func (f *stream) Complete(ctx context.Context, r model.Request, on func(model.Delta)) (model.Reply, error) {
	f.mu.Lock()
	f.asked = append(f.asked, r)
	f.mu.Unlock()
	var all strings.Builder
	for i, p := range f.pieces {
		if i == 1 && f.hold != nil {
			select {
			case <-f.hold:
			case <-ctx.Done():
				return model.Reply{Text: all.String()}, ctx.Err()
			}
		}
		if ctx.Err() != nil {
			return model.Reply{Text: all.String()}, ctx.Err()
		}
		all.WriteString(p)
		on(model.Delta{Text: p})
	}
	return model.Reply{Text: all.String()}, f.fail
}
func savedText(a *testApp) string { return a.panel.bookHost.ChapterState().Saved }
func quick(a *testApp)            { a.book.Timing.SaveAfter, a.book.Timing.CheckEvery = 0, time.Hour }
