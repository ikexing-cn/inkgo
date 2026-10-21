package backup

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Policy keeps scheduling and failure state for one opening of a work.
// Reset it when switching works; the caller owns the task and its settings.
type Policy struct {
	err     error     // why the last backup failed; nil once one succeeds
	gone    bool      // it failed with the disk of its folder not found
	failed  time.Time // when it failed, so it is not retried every frame
	checked time.Time // when it was last seen whether one is due
	asked   bool      // asked already this time the work is open
}

// Due advances the minute check when allowed and reports whether to start
// an automatic backup. Call it only while a work is open and no backup runs.
func (p *Policy) Due(s Settings, writing bool, now time.Time) bool {
	if now.Sub(p.checked) < time.Minute {
		return false
	}
	p.checked = now
	if s.Dir == "" || p.err != nil && now.Sub(p.failed) < retryAfter {
		return false
	}
	daily := s.Daily && s.Last.Format(time.DateOnly) != now.Format(time.DateOnly)
	hourly := s.Hourly && writing && now.Sub(s.Last) >= time.Hour
	return daily || hourly
}

// Ask marks this opening as considered and reports whether to ask for a
// folder. A configured or recently deferred work is considered only once.
func (p *Policy) Ask(s Settings, now time.Time) bool {
	if p.asked {
		return false
	}
	p.asked = true
	return s.Dir == "" && now.Sub(s.Later) >= askAgain
}

// RecordResult clears the error on success or records the failure time,
// probing a missing folder's volume once for later explanations.
func (p *Policy) RecordResult(dir string, err error, now time.Time) {
	p.err = err
	if err != nil {
		p.failed = now
		p.gone = errors.Is(err, fs.ErrNotExist) && volumeGone(dir)
	}
}

// Failed reports whether the last backup failed.
func (p Policy) Failed() bool {
	return p.err != nil
}

// ForgetFailure clears the error when starting backups or choosing a folder.
func (p *Policy) ForgetFailure() {
	p.err = nil
}

// retryAfter is how long a failed backup waits before it is tried again
// unasked.
const retryAfter = 10 * time.Minute

// askAgain is how long after "稍后再说" the author is asked again.
const askAgain = 7 * 24 * time.Hour

// volumeGone says the disk dir is on cannot be found, as one unplugged.
func volumeGone(dir string) bool {
	vol := filepath.VolumeName(dir)
	if vol == "" {
		return false
	}
	_, err := os.Stat(vol + string(filepath.Separator))
	return err != nil
}

// Trouble describes the last failure for the toolbar and its popover.
func (p Policy) Trouble(dir string) (short, title, body string) {
	if errors.Is(p.err, fs.ErrNotExist) {
		if p.gone {
			d := strings.TrimSuffix(filepath.VolumeName(dir), ":")
			return "找不到 " + d + " 盘", "找不到 " + d + " 盘，本次备份未完成",
				"作品未受影响，请接入硬盘后重试。"
		}
		return "找不到备份文件夹", "找不到备份文件夹，本次备份未完成",
			"作品未受影响，文件夹可能已被移动或改名。"
	}
	if errors.Is(p.err, fs.ErrPermission) {
		return "没有权限写入", "备份文件夹无法写入，本次备份未完成",
			"作品未受影响，该文件夹没有写入权限。"
	}
	return "备份失败", "本次备份未完成", "作品未受影响。原因：" + p.err.Error()
}
