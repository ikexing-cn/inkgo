package backup

import (
	"errors"
	"github.com/egoist/mygo/i18n"
	"strings"
	"testing"
	"unicode"
	"writer/ext/backup/archive"
	"writer/ext/backup/backuptext"
)

func TestBackupFailureMappingsInBothLocales(t *testing.T) {
	tag := "zh-Hans"
	i18n.SetLocaleFunc(func() string { return tag })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	for _, next := range []string{"zh-Hans", "en"} {
		tag = next
		for _, tc := range []struct {
			failure            archive.Failure
			short, title, body string
		}{
			{archive.Failure{Kind: archive.NoDisk, Disk: "X"}, backuptext.BackupDriveShort("X"), backuptext.BackupDriveTitle("X"), backuptext.BackupDiskFix()},
			{archive.Failure{Kind: archive.NoFolder}, backuptext.BackupDirShort(), backuptext.BackupDirTitle(), backuptext.BackupDirFix()},
			{archive.Failure{Kind: archive.NoWrite}, backuptext.BackupDenyShort(), backuptext.BackupDenyTitle(), backuptext.BackupDenyFix()},
			{archive.Failure{Kind: archive.Failed, Err: errors.New("full")}, backuptext.BackupFailed(), backuptext.BackupFailureTitle(), backuptext.BackupFailureBody("full")},
		} {
			short, title, body := TroubleText(tc.failure)
			if short != tc.short || title != tc.title || body != tc.body {
				t.Fatal(tag, short, title, body)
			}
			if tag == "en" && strings.ContainsFunc(short+title+body, func(r rune) bool { return unicode.Is(unicode.Han, r) }) {
				t.Fatal(short, title, body)
			}
		}
	}
}
