package backup

import (
	"writer/ext/backup/archive"
	"writer/ext/backup/backuptext"
)

// TroubleText translates a classified backup failure at display time.
func TroubleText(f archive.Failure) (short, title, body string) {
	switch f.Kind {
	case archive.NoDisk:
		return backuptext.BackupDriveShort(f.Disk), backuptext.BackupDriveTitle(f.Disk), backuptext.BackupDiskFix()
	case archive.NoFolder:
		return backuptext.BackupDirShort(), backuptext.BackupDirTitle(), backuptext.BackupDirFix()
	case archive.NoWrite:
		return backuptext.BackupDenyShort(), backuptext.BackupDenyTitle(), backuptext.BackupDenyFix()
	default:
		reason := ""
		if f.Err != nil {
			reason = f.Err.Error()
		}
		return backuptext.BackupFailed(), backuptext.BackupFailureTitle(), backuptext.BackupFailureBody(reason)
	}
}
