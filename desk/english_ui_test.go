package desk

import (
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"writer/ext/backup/backuptext"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/ext/backup"
	"writer/locales"
	"writer/workspace"
)

// paintedTexts reads text nodes, not accessibility labels, in mygo 0.7.4.
// Reflection is confined to this adapter because Tester.Texts includes hooks.
func paintedTexts(tt *ui.Tester) []string {
	rt := reflect.ValueOf(tt).Elem().FieldByName("rt").Elem()
	var out []string
	var walk func(reflect.Value)
	walk = func(ptr reflect.Value) {
		if ptr.IsNil() {
			return
		}
		n := ptr.Elem()
		if text := n.FieldByName("text"); text.IsValid() && text.String() != "" {
			out = append(out, text.String())
		}
		for child := n.FieldByName("first"); !child.IsNil(); child = child.Elem().FieldByName("next") {
			walk(child)
		}
	}
	walk(rt.FieldByName("c").FieldByName("root"))
	return out
}

func TestEnglishInterfaceDrawsNoHan(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(testLocale) })
	root := t.TempDir()
	store, err := workspace.Create(filepath.Join(root, "book"), "English Book", "Fantasy", "zh-Hans")
	if err != nil {
		t.Fatal(err)
	}
	work := store.Work()
	work.Language = "en"
	work.Categories = workspace.Categories(work.Genre, "en")
	if err := store.SaveWork(work); err != nil {
		t.Fatal(err)
	}
	vol, err := store.NewChapter("", "Volume One", true)
	if err != nil {
		t.Fatal(err)
	}
	id, err := store.NewChapter(vol, "Chapter One", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.WriteChapter(id, "The rain stopped.", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "People", Text: "A traveller."}}}); err != nil {
		t.Fatal(err)
	}
	store.Close()
	was := Library
	Library = func() string { return filepath.Join(root, "library") }
	t.Cleanup(func() { Library = was })
	a := newApp(filepath.Join(root, "providers.yaml"))
	t.Cleanup(func() { a.Close(); delete(controllers, a) })
	tt := stillTester(a.View, 1266, 780)
	check := func(state string) {
		t.Helper()
		tt.Frame()
		tt.Frame()
		texts := paintedTexts(tt)
		if len(texts) == 0 {
			t.Fatal(state, "no text collected")
		}
		// Each language is named in itself, as in the prototype.
		autonyms := map[string]bool{}
		for _, tag := range locales.Catalog.Languages() {
			autonyms[i18n.Autonym(tag)] = true
		}
		for _, text := range texts {
			if autonyms[text] {
				continue
			}
			if strings.ContainsFunc(text, func(r rune) bool { return unicode.Is(unicode.Han, r) }) {
				t.Errorf("%s: %q", state, text)
			}
		}
	}
	check("shelf")
	if err := a.Open(filepath.Join(root, "book")); err != nil {
		t.Fatal(err)
	}
	a.LocaleChanged()
	a.page = pageWrite
	a.openNode(id)
	check("write")
	for _, menu := range []string{"file", "edit", "view", "help"} {
		a.menu = kit.MenuState{Owner: barKey + menu, Drop: true}
		check("title menu " + menu)
		a.menu = kit.MenuState{}
	}
	a.keysOpen = true
	check("command shortcuts")
	a.keysOpen = false
	for _, tab := range []string{"appearance", "extensions", "AI", "reminders", "backup.settings"} {
		a.openPrefs(tab)
		check("settings " + tab)
	}
	a.openPrefs("extensions")
	a.settings.ext = "backup"
	check("extension details")
	a.closePrefs()
	runExport(t, a)
	check("export")
	closeExport(t, tt)
	for _, page := range []page{pageTalk, pageCheck, pagePlan} {
		a.page = page
		check("page " + strconv.Itoa(int(page)))
	}
	a.page = pageWrite
	for _, tab := range []paneTab{paneSetting, paneFore, paneMargin} {
		a.doc.hidden = false
		showTab(a, tab)
		check("pane " + string(tab))
	}
	a.menu = kit.MenuState{Owner: "prose", X: 400, Y: 200}
	check("prose context menu")
	a.menu = kit.MenuState{}
	wasFolders := backup.SyncFolders
	backup.SyncFolders = func() []backup.Place { return []backup.Place{{Dir: "backup", Name: backuptext.BackupNutstore}} }
	t.Cleanup(func() { backup.SyncFolders = wasFolders })
	backupOf(a).OpenAsk()
	check("backup destination question")
}

func TestPaintedTextsIgnoreLabelsButKeepTextOnLabeledNodes(t *testing.T) {
	tt := stillTester(func(c *ui.Context) { ui.Text(c, "English").Label("中文钩子") }, 200, 100)
	tt.Frame()
	texts := paintedTexts(tt)
	if !strings.Contains(strings.Join(texts, ""), "English") || strings.Contains(strings.Join(texts, ""), "中文") {
		t.Fatal(texts)
	}
}
