package backup

import (
	"writer/ext/backup/archive"
	"writer/locales"
)

// TroubleText translates a classified backup failure at display time.
func TroubleText(f archive.Failure) (short, title, body string) {
	switch f.Kind {
	case archive.NoDisk:
		return locales.BackupDriveShort(f.Disk), locales.BackupDriveTitle(f.Disk), locales.BackupDiskFix()
	case archive.NoFolder:
		return locales.BackupDirShort(), locales.BackupDirTitle(), locales.BackupDirFix()
	case archive.NoWrite:
		return locales.BackupDenyShort(), locales.BackupDenyTitle(), locales.BackupDenyFix()
	default:
		reason := ""
		if f.Err != nil {
			reason = f.Err.Error()
		}
		return locales.BackupFailed(), locales.BackupFailureTitle(), locales.BackupFailureBody(reason)
	}
}
