package backup

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/egoist/mygo/i18n"
	"writer/ext/backup/backuptext"
	"writer/locales"
)

func TestExtensionCatalogFollowsLocaleAndPseudoLocale(t *testing.T) {
	tag := "zh-Hans"
	i18n.SetLocaleFunc(func() string { return tag })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	chinese := backuptext.BackupBusy()
	if chinese != "正在备份…" {
		t.Fatal(chinese)
	}
	hostChinese := locales.CommonCancel()
	tag = "en"
	if got := backuptext.BackupBusy(); got != "Backing Up…" || got != backuptext.In("en").BackupBusy() || got == chinese {
		t.Fatal(got)
	}
	tag = "en-XA"
	for _, pair := range [][2]string{{backuptext.BackupBusy(), chinese}, {locales.CommonCancel(), hostChinese}} {
		if pair[0] == pair[1] || !strings.HasPrefix(pair[0], "[") || !strings.HasSuffix(pair[0], "]") || utf8.RuneCountInString(pair[0]) <= utf8.RuneCountInString(pair[1])+2 {
			t.Fatal(pair)
		}
	}
	tag = "zh-Hans"
	if got := backuptext.BackupBusy(); got != chinese {
		t.Fatal(got)
	}
}
