package desk

import (
	"strings"
	"testing"
	"unicode"

	"github.com/egoist/mygo/i18n"
	"writer/desk/book"
	"writer/ext/backup"
	"writer/locales"
	"writer/shelf"
	"writer/workspace"
)

func TestRemainingErrorsTranslateInBothLocales(t *testing.T) {
	tag := "zh-Hans"
	i18n.SetLocaleFunc(func() string { return tag })
	t.Cleanup(func() { i18n.SetLocaleFunc(testLocale) })
	a := &App{}
	for _, next := range []string{"zh-Hans", "en"} {
		tag = next
		for _, tc := range []struct {
			err  error
			want string
		}{
			{workspace.ErrTitleEmpty, locales.WorkspaceTitleEmpty()},
			{workspace.ErrMoveIntoSelf, locales.WorkspaceMoveIntoSelf()},
			{workspace.ErrChangedOutside, locales.WorkspaceChangedOutside()},
			{workspace.ErrCheckChanged, locales.WorkspaceCheckChanged()},
			{book.ErrCheckChanged, locales.WorkspaceCheckChanged()},
			{workspace.ErrSettingChanged, locales.WorkspaceSettingChanged()},
			{workspace.ErrLegacySchema, locales.WorkspaceLegacySchema()},
			{&workspace.TrashMissingError{ID: "entry"}, locales.WorkspaceTrashMissing(`"entry"`)},
			{&shelf.NoBookError{Path: "folder"}, locales.ShelfNoBook("folder")},
		} {
			got := a.errText(tc.err)
			if got != tc.want || tag == "en" && strings.ContainsFunc(got, func(r rune) bool { return unicode.Is(unicode.Han, r) }) {
				t.Errorf("%s %T: %q want %q", tag, tc.err, got, tc.want)
			}
		}
	}
}

func TestLocaleChangeRefreshesTalkListPlaceholderAndDetectedBackupName(t *testing.T) {
	tag := "zh-Hans"
	i18n.SetLocaleFunc(func() string { return tag })
	t.Cleanup(func() { i18n.SetLocaleFunc(testLocale) })
	a, tt, id := writingFixture(t, "")
	if _, err := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleUser, Origin: workspace.AIDraft, Chapter: id}); err != nil {
		t.Fatal(err)
	}
	if err := a.refreshFromSetting(); err != nil {
		t.Fatal(err)
	}
	old := backup.SyncFolders
	backup.SyncFolders = func() []backup.Place { return []backup.Place{{Dir: "backup", Name: locales.BackupNutstore}} }
	t.Cleanup(func() { backup.SyncFolders = old })
	backupOf(a).OpenAsk()
	for _, next := range []string{"zh-Hans", "en", "zh-Hans"} {
		tag = next
		a.LocaleChanged()
		tt.Frame()
		tt.Frame()
		if got, _ := a.talkName(a.session); got != locales.SideDraftTitle(a.nodeTitle(id)) {
			t.Fatal(tag, got)
		}
		if !tt.HasText(locales.BackupNutstore()) {
			t.Fatal(tag, tt.Texts())
		}
	}
}

func TestTalkTitlesTranslateInBothLocales(t *testing.T) {
	tag := "zh-Hans"
	i18n.SetLocaleFunc(func() string { return tag })
	t.Cleanup(func() { i18n.SetLocaleFunc(testLocale) })
	for _, next := range []string{"zh-Hans", "en"} {
		tag = next
		for _, tc := range []struct {
			title workspace.ConversationTitle
			want  string
		}{
			{workspace.ConversationTitle{}, locales.SideNewTalk()},
			{workspace.ConversationTitle{Kind: workspace.AIDraft, Text: "Rain"}, locales.SideDraftTitle("Rain")},
			{workspace.ConversationTitle{Kind: workspace.AIDraft}, locales.SideDraftTitle(locales.AppChapterDeleted())},
			{workspace.ConversationTitle{Kind: workspace.AIRewrite, Text: "Rain"}, locales.SideRewriteTitle("Rain")},
			{workspace.ConversationTitle{Kind: workspace.TitleWords, Text: "Rain"}, "Rain"},
		} {
			if got := talkTitleText(tc.title); got != tc.want {
				t.Fatal(tag, got, tc.want)
			}
		}
	}
}
