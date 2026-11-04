package snaps

import (
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"writer/desk"
	"writer/editor"
	aimod "writer/ext/ai"
	"writer/ext/context"
	"writer/ext/find"
	"writer/ext/lore"
	"writer/ext/outline"
	"writer/ext/talk"
	"writer/workspace"
)

type testApp struct {
	*desk.App
	controller *View
}

func writingFixture(t *testing.T, text string) (*testApp, *ui.Tester, string) {
	i18n.SetLocaleFunc(func() string { return "zh-Hans" })
	root := t.TempDir()
	s, err := workspace.Create(filepath.Join(root, "work"), "测试作品", "网文", "zh-Hans")
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.NewChapter("", "第一章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.WriteChapter(id, text, nil); err != nil {
		t.Fatal(err)
	}
	s.Close()
	was := desk.Library
	desk.Library = func() string { return filepath.Join(root, "library") }
	t.Cleanup(func() { desk.Library = was })
	mod, v := New()
	app := desk.New(filepath.Join(root, "providers.yaml"), find.Module, lore.Module, outline.Module, talk.Module, mod, context.Module, aimod.New(aimod.Options{}))
	a := &testApp{app, v}
	if err = app.Open(filepath.Join(root, "work")); err != nil {
		t.Fatal(err)
	}
	v.Book.Content().Timing.SaveAfter = 0
	v.Book.Content().Timing.CheckEvery = time.Hour
	v.Desktop.OpenWriting(id)
	tt := ui.NewTester(app.View, 1266, 780)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	tt.Frame()
	t.Cleanup(func() { app.Close() })
	return a, tt, id
}
func shownEditor(a *testApp) *editor.Editor { _, ed, _ := a.controller.Book.Chapter(); return ed }
func shownChapter(a *testApp) string        { id, _, _ := a.controller.Desktop.Selected(); return id }
func snapsOf(a *testApp) *View              { return a.controller }
func noticeOf(a *testApp) string            { return a.controller.Notices.Notices().NoticeText() }
func disk(t *testing.T, a *testApp, name string) string {
	b, e := os.ReadFile(filepath.Join(a.Store.Root(), "chapters", name))
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func clickMenu(tt *ui.Tester, menu, name string) error {
	if err := tt.Click("菜单栏-" + menu); err != nil {
		return err
	}
	tt.Frame()
	return tt.Click(name)
}
func pick(tt *ui.Tester, path ...string) error {
	for _, s := range path {
		tt.Frame()
		if err := tt.Click("菜单-" + strings.TrimSuffix(s, "…")); err != nil {
			return err
		}
	}
	tt.Frame()
	return nil
}
func rows(tt *ui.Tester) []string        { return tt.Texts() }
func chapters(a *testApp) workspace.Tree { return a.controller.Book.Content().Tree() }
func markUnchecked(a *testApp) {
	if err := a.controller.Book.Content().ReloadTree(); err != nil {
		panic(err)
	}
}

func (a *testApp) openSnapshots() { a.controller.openSnapshots() }
