package desk

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"writer/backup"
)

// waitBackup runs frames until the backup in flight is done.
func waitBackup(t *testing.T, a *App, tt *ui.Tester) {
	t.Helper()
	for range 500 {
		tt.Frame()
		if !a.bk.running {
			tt.Frame()
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the backup never finished")
}

func zips(t *testing.T, dir string) int {
	t.Helper()
	m, err := filepath.Glob(filepath.Join(dir, "*.zip"))
	if err != nil {
		t.Fatal(err)
	}
	return len(m)
}

// The first time a work opens, the author is asked where to back it up;
// starting backs up at once, and the sidebar says so.
func TestFirstBackup(t *testing.T) {
	a, tt, _ := fixture(t)
	sync := filepath.Join(t.TempDir(), "OneDrive")
	if err := os.Mkdir(sync, 0755); err != nil {
		t.Fatal(err)
	}
	was := syncFolders
	syncFolders = func() []place { return []place{{"OneDrive 同步文件夹", sync}} }
	t.Cleanup(func() { syncFolders = was })
	a.Backups = true
	if err := a.Open(a.Store.Root()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("稿子现在只存在这台电脑上") || !tt.HasText("在这台电脑上找到了 · "+sync) || !tt.HasText("每次关闭作品时，再加每天一次") {
		t.Fatal("not asked", tt.Texts())
	}
	if err := tt.Click("开始备份"); err != nil {
		t.Fatal(err)
	}
	waitBackup(t, a, tt)
	if zips(t, sync) != 1 || !tt.HasText("已备份 · 今天 "+time.Now().Format("15:04")) {
		t.Fatal("not backed up", zips(t, sync), tt.Texts())
	}
	s, ok := a.backupSettings()
	if !ok || s.Dir != sync || !s.OnClose || !s.Daily || s.Hourly || s.Last.IsZero() {
		t.Fatalf("settings %+v", s)
	}
	// Opened again, it is not asked again; closing backs up what changed.
	if err := a.Open(a.Store.Root()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("稿子现在只存在这台电脑上") {
		t.Fatal("asked again")
	}
	if _, err := a.Store.NewChapter("", "第二章", false); err != nil {
		t.Fatal(err)
	}
	a.closeWork()
	if zips(t, sync) != 2 {
		t.Fatal("closing did not back up", zips(t, sync))
	}
}

// Put off, the question waits a week.
func TestBackupLater(t *testing.T) {
	a, tt, _ := fixture(t)
	a.Backups = true
	if err := a.Open(a.Store.Root()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("以后再说"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	if s, ok := a.backupSettings(); !ok || s.Later.IsZero() || s.Dir != "" {
		t.Fatalf("settings %+v", s)
	}
	if err := a.Open(a.Store.Root()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.bk.ask || tt.HasText("稿子现在只存在这台电脑上") {
		t.Fatal("asked again so soon")
	}
}

// A backup that fails says why in the sidebar and on the writing toolbar,
// until it is put off for the day; the settings page shows the backups.
func TestBackupFails(t *testing.T) {
	a, tt, _ := writingFixture(t, "整条街的灯是同时灭的。")
	gone := filepath.Join(t.TempDir(), "备份")
	a.setBackup(backup.Settings{Dir: gone, OnClose: true})
	tt.Frame()
	if !tt.HasText("还没备份过") {
		t.Fatal("no line", tt.Texts())
	}
	if err := tt.Click("备份状态"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("现在备份一次"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	waitBackup(t, a, tt)
	if !tt.HasText("备份没成功") || !tt.HasText("备份未完成 · 找不到备份文件夹") || !tt.HasText("待处理 1") {
		t.Fatal("failure not shown", tt.Texts())
	}
	if err := tt.Click("备份未完成"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("找不到备份文件夹，这次没备份上") || !tt.HasText("它可能被移走或改了名。稿子在这台电脑上好好的，换个地方或者把文件夹放回去再试一次。还没有成功过。") {
		t.Fatal("no reason", tt.Texts())
	}
	if err := tt.Click("今天别提醒"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("备份未完成 · 找不到备份文件夹") || !tt.HasText("备份没成功") {
		t.Fatal("still warned on the toolbar", tt.Texts())
	}
	// The folder back, trying again works, and the settings page says so.
	if err := os.Mkdir(gone, 0755); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("备份状态"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("再试一次"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	waitBackup(t, a, tt)
	if tt.HasText("备份没成功") || zips(t, gone) != 1 {
		t.Fatal("not backed up", tt.Texts())
	}
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("设置-备份"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.bk.prefs || !tt.HasText("把整部作品复制一份到别处。快照只防改错，备份防电脑坏。") || !tt.HasText("11 字 · 1 章 · 共保留 1 份，最早一份是 "+time.Now().Format("1 月 2 日")) {
		t.Fatal("settings page", tt.Texts())
	}
	if err := tt.Click("写作时每小时一次"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s, _ := a.backupSettings(); !s.Hourly || !s.OnClose {
		t.Fatalf("settings %+v", s)
	}
	if err := tt.Click("设置-外观"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("只在这台电脑上生效，所有作品都一样。") {
		t.Fatal("appearance", tt.Texts())
	}
}

func TestWhenSaid(t *testing.T) {
	now := time.Date(2026, 10, 9, 15, 0, 0, 0, time.Local)
	for _, c := range []struct {
		at   time.Time
		want string
	}{
		{now.Add(-time.Hour), "今天 14:00"},
		{now.Add(-20 * time.Hour), "昨天 19:00"},
		{time.Date(2026, 10, 6, 21, 40, 0, 0, time.Local), "10-06 21:40"},
		{time.Date(2025, 12, 31, 9, 5, 0, 0, time.Local), "2025-12-31 09:05"},
	} {
		if got := when(c.at, now); got != c.want {
			t.Errorf("when(%v) = %q, want %q", c.at, got, c.want)
		}
	}
	if got := whenSaid(true, true, false); got != "每次关闭作品时，再加每天一次" {
		t.Error(got)
	}
	if got := whenSaid(true, true, true); got != "每次关闭作品时、每天一次，再加写作时每小时一次" {
		t.Error(got)
	}
}

// TestShotBackup writes screenshots of the backups when SHOT is a
// directory: SHOT=dir go test ./desk -run ShotBackup
func TestShotBackup(t *testing.T) {
	dir := os.Getenv("SHOT")
	if dir == "" {
		t.Skip("set SHOT=dir")
	}
	a, tt, _ := writingFixture(t, "整条街的灯是同时灭的。\n林晚站在巷口，手电在工具箱最底下。她没有去摸。")
	tt.SetTitleBar(ui.TitleBar{Height: 40, Right: 138})
	shot := func(name string) {
		tt.Frame()
		tt.Frame()
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	sync := filepath.Join(t.TempDir(), "OneDrive", "写作备份")
	was := syncFolders
	syncFolders = func() []place { return []place{{"OneDrive 同步文件夹", sync}} }
	t.Cleanup(func() { syncFolders = was })
	a.PickFolder = func(string) (string, error) { return "", nil }
	a.openAsk()
	shot("backup-ask.png")
	a.bk.ask = false
	if err := os.MkdirAll(sync, 0755); err != nil {
		t.Fatal(err)
	}
	a.setBackup(backup.Settings{Dir: sync, OnClose: true, Daily: true})
	tt.Frame()
	if err := tt.Click("备份状态"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("现在备份一次"); err != nil {
		t.Fatal(err)
	}
	waitBackup(t, a, tt)
	a.bk.open = true
	shot("backup-line.png")
	a.bk.open = false
	a.bk.err = &os.PathError{Op: "stat", Path: `E:\雨城来信备份`, Err: os.ErrNotExist}
	s, _ := a.backupSettings()
	s.Dir = `E:\雨城来信备份`
	a.setBackup(s)
	tt.SetDark(true)
	a.bk.chip = true
	shot("backup-fail-dark.png")
	a.bk.chip = false
	tt.SetDark(false)
	a.bk.err = nil
	s.Dir = sync
	a.setBackup(s)
	a.bk.tab, a.page = "备份", 4
	shot("backup-settings.png")
	tt.SetDark(true)
	a.bk.tab, a.page = "外观", 4
	shot("settings-look-dark.png")
	a.bk.tab = "模型"
	shot("settings-models-dark.png")
	a.bk.edit = "deepseek"
	a.chooseProvider("deepseek")
	tt.Scroll(700, 500, 0, 600)
	shot("settings-service-dark.png")
	a.bk.prefs = false
	a.page = 0
	shot("discussion-dark.png")
}
