package backup

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestBackupDue(t *testing.T) {
	now := time.Date(2041, 10, 22, 0, 1, 0, 0, time.FixedZone("local", 8*60*60))
	for _, tc := range []struct {
		name         string
		s            Settings
		writing, due bool
	}{
		{"daily crosses midnight", Settings{Dir: "backup", Daily: true, Last: now.Add(-2 * time.Minute)}, false, true},
		{"daily same date", Settings{Dir: "backup", Daily: true, Last: now.Add(-time.Minute)}, true, false},
		{"daily first", Settings{Dir: "backup", Daily: true}, false, true},
		{"hourly boundary", Settings{Dir: "backup", Hourly: true, Last: now.Add(-time.Hour)}, true, true},
		{"hourly before", Settings{Dir: "backup", Hourly: true, Last: now.Add(-time.Hour + time.Nanosecond)}, true, false},
		{"hourly off prose", Settings{Dir: "backup", Hourly: true, Last: now.Add(-time.Hour)}, false, false},
		{"close only", Settings{Dir: "backup", OnClose: true}, true, false},
		{"missing folder choice", Settings{Daily: true, Hourly: true}, true, false},
		{"future success", Settings{Dir: "backup", Hourly: true, Last: now.Add(time.Hour)}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var p Policy
			if got := p.Due(tc.s, tc.writing, now); got != tc.due {
				t.Fatalf("due %v, want %v", got, tc.due)
			}
		})
	}
}

func TestBackupRetryAndCheck(t *testing.T) {
	now := time.Date(2041, 10, 22, 9, 0, 0, 0, time.UTC)
	settings := Settings{Dir: "backup", Daily: true}
	for _, tc := range []struct {
		ago          time.Duration
		check, retry bool
	}{
		{-time.Minute, false, false},
		{time.Minute - time.Nanosecond, false, false},
		{time.Minute, true, false},
		{10*time.Minute - time.Nanosecond, true, false},
		{10 * time.Minute, true, true},
	} {
		p := Policy{err: fs.ErrPermission, failed: now.Add(-tc.ago), checked: now.Add(-tc.ago)}
		if p.Due(settings, true, now) != (tc.check && tc.retry) {
			t.Fatal(tc, p)
		}
		want := now.Add(-tc.ago)
		if tc.check {
			want = now
		}
		if !p.checked.Equal(want) {
			t.Fatal("check timestamp", p.checked, want)
		}
	}
	var p Policy
	if !p.Due(settings, true, now) || p.Due(settings, true, now.Add(time.Minute-time.Nanosecond)) || !p.Due(settings, true, now.Add(time.Minute)) {
		t.Fatal("minute throttle", p)
	}
	p = Policy{}
	if p.Due(Settings{}, true, now) || !p.checked.Equal(now) || p.Due(settings, true, now) {
		t.Fatal("an unconfigured check did not consume the minute", p)
	}
}

func TestAskBackupDue(t *testing.T) {
	now := time.Date(2041, 10, 22, 9, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		s   Settings
		due bool
	}{
		{Settings{}, true},
		{Settings{Dir: "chosen"}, false},
		{Settings{Later: now}, false},
		{Settings{Later: now.Add(-7*24*time.Hour + time.Nanosecond)}, false},
		{Settings{Later: now.Add(-7 * 24 * time.Hour)}, true},
		{Settings{Later: now.Add(time.Hour)}, false},
	} {
		var p Policy
		if got := p.Ask(tc.s, now); got != tc.due {
			t.Fatal(tc, got)
		}
		if !p.asked || p.Ask(Settings{}, now.Add(8*24*time.Hour)) {
			t.Fatal("an opening asked twice", p)
		}
	}
}

// Why a backup failed is said from what was found as it failed: saying it
// does not look for the disk again, as it is said in every frame.
func TestTroubleSaysWhatWasFoundAsItFailed(t *testing.T) {
	missing := &os.PathError{Op: "stat", Path: `X:\备份`, Err: os.ErrNotExist}
	if short, _, _ := (Policy{err: missing, gone: true}).Trouble(`X:\备份`); runtime.GOOS == "windows" && short != "找不到 X 盘" {
		t.Fatalf("with the disk gone: %q", short)
	}
	if short, _, _ := (Policy{err: missing}).Trouble(`X:\备份`); short != "找不到备份文件夹" {
		t.Fatalf("with the disk there: %q", short)
	}
	if volumeGone(t.TempDir()) {
		t.Fatal("the disk of a folder that is there counts as gone")
	}
	for _, tc := range []struct {
		err                error
		short, title, body string
	}{
		{fs.ErrPermission, "没有权限写入", "备份文件夹无法写入，本次备份未完成", "作品未受影响，该文件夹没有写入权限。"},
		{errors.New("full"), "备份失败", "本次备份未完成", "作品未受影响。原因：full"},
	} {
		if short, title, body := (Policy{err: tc.err}).Trouble("backup"); short != tc.short || title != tc.title || body != tc.body {
			t.Fatal(short, title, body)
		}
	}
	if volumeGone(filepath.Join("relative", "backup")) {
		t.Fatal("a relative path claims a missing volume")
	}
}

func TestForgetFailure(t *testing.T) {
	now := time.Date(2041, 10, 22, 9, 0, 0, 0, time.UTC)
	s := Settings{Dir: "backup", Daily: true}
	var p Policy
	p.RecordResult("relative", fs.ErrPermission, now)
	if !p.Failed() || p.Due(s, true, now) {
		t.Fatal(p)
	}
	p.ForgetFailure()
	if p.Failed() || p.Due(s, true, now) || !p.Due(s, true, now.Add(time.Minute)) {
		t.Fatal("forgetting a failure changed the minute check or kept the retry delay", p)
	}
}

func TestBackupResultAndHush(t *testing.T) {
	now := time.Date(2041, 10, 22, 23, 59, 0, 0, time.FixedZone("local", 8*60*60))
	missing := &os.PathError{Op: "stat", Path: "missing", Err: fs.ErrNotExist}
	var p Policy
	p.RecordResult(t.TempDir(), missing, now)
	if p.err != missing || !p.failed.Equal(now) || p.gone {
		t.Fatal(p)
	}
	p.RecordResult("relative", fs.ErrPermission, now.Add(time.Minute))
	if !errors.Is(p.err, fs.ErrPermission) || p.gone || !p.failed.Equal(now.Add(time.Minute)) {
		t.Fatal(p)
	}
	p.RecordResult("relative", nil, now.Add(2*time.Minute))
	if p.err != nil {
		t.Fatal("success kept a failure", p)
	}
	if !p.Due(Settings{Dir: "backup", Hourly: true, Last: now.Add(-time.Hour)}, true, now.Add(2*time.Minute)) {
		t.Fatal("success kept the retry delay", p)
	}
	s := Settings{Hush: now}
	if !s.Hushed(now.Add(-time.Hour)) || s.Hushed(now.Add(time.Minute)) || (Settings{}).Hushed(now) {
		t.Fatal("local-day hush")
	}
	if runtime.GOOS == "windows" {
		if !volumeGone("?:\\backup") {
			t.Fatal("an invalid volume counts as present")
		}
		p.RecordResult("?:\\backup", missing, now)
		if !p.gone {
			t.Fatal(p)
		}
	}
}
