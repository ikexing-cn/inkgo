package desk

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"writer/backup"
	"writer/internal/testshot"
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
	if !tt.HasText("作品目前仅保存在本机") || !tt.HasText("在本机检测到 · "+sync) || !tt.HasText("每天一次（当天有写作时）") {
		t.Fatal("not asked", tt.Texts())
	}
	// When is ticked in the dialog itself.
	if err := tt.Click("写作时每小时一次"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("开始备份"); err != nil {
		t.Fatal(err)
	}
	waitBackup(t, a, tt)
	if zips(t, sync) != 1 || !tt.HasText("已备份 · 今天 "+time.Now().Format("15:04")) {
		t.Fatal("not backed up", zips(t, sync), tt.Texts())
	}
	s, ok := a.backupSettings()
	if !ok || s.Dir != sync || !s.OnClose || !s.Daily || !s.Hourly || s.Last.IsZero() {
		t.Fatalf("settings %+v", s)
	}
	// Opened again, it is not asked again; closing backs up what changed.
	if err := a.Open(a.Store.Root()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("作品目前仅保存在本机") {
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

func TestBackupWaitsBeforeClosingStore(t *testing.T) {
	a, tt, _ := fixture(t)
	dir := t.TempDir()
	a.setBackup(backup.Settings{Dir: dir})
	a.bk.open = true
	tt.Frame()
	if err := tt.Click("立即备份"); err != nil {
		t.Fatal(err)
	}
	if !a.bk.running {
		t.Fatal("backup did not start")
	}
	st := a.Store
	a.closeWork()
	if a.Store != nil || zips(t, dir) != 1 {
		t.Fatalf("closed store = %v, backups = %d", a.Store, zips(t, dir))
	}
	if err := st.Hold(func(string) error { return nil }); err == nil {
		t.Fatal("store stayed open")
	}
}

func TestAskBackupFindsSyncFoldersOnce(t *testing.T) {
	a, tt, _ := fixture(t)
	calls := 0
	was := syncFolders
	syncFolders = func() []place {
		calls++
		return []place{{label: "同步文件夹", dir: filepath.Join(t.TempDir(), "写作备份")}}
	}
	t.Cleanup(func() { syncFolders = was })
	a.openAsk()
	for range 5 {
		tt.Frame()
	}
	if calls != 1 {
		t.Fatalf("sync folders probed %d times while dialog was open", calls)
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
	if err := tt.Click("稍后再说"); err != nil {
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
	if a.bk.ask || tt.HasText("作品目前仅保存在本机") {
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
	if !tt.HasText("尚未备份") {
		t.Fatal("no line", tt.Texts())
	}
	if err := tt.Click("备份状态"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("立即备份"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	waitBackup(t, a, tt)
	if !tt.HasText("备份失败") || !tt.HasText("备份失败 · 找不到备份文件夹") || !tt.HasText("待处理 1") {
		t.Fatal("failure not shown", tt.Texts())
	}
	if err := tt.Click("备份提示"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("找不到备份文件夹，本次备份未完成") || !tt.HasText("作品未受影响，文件夹可能已被移动或改名。尚无成功的备份。") {
		t.Fatal("no reason", tt.Texts())
	}
	// Its buttons at the right, the one that goes ahead last.
	hush, _ := tt.Find("今日不再提醒")
	again, _ := tt.Find("重试")
	title, ok := tt.Find("找不到备份文件夹，本次备份未完成")
	if !ok || hush.X <= title.X || again.X <= hush.X+hush.W {
		t.Fatalf("the buttons are not at the right, the main one last: %v %v under %v", hush, again, title)
	}
	if err := tt.Click("今日不再提醒"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("备份失败 · 找不到备份文件夹") || !tt.HasText("备份失败") {
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
	if err := tt.Click("重试"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	waitBackup(t, a, tt)
	if tt.HasText("备份失败") || zips(t, gone) != 1 {
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
	if !a.bk.prefs || tt.HasText("为整部作品另存一份副本") || !tt.HasText("11 字 · 1 章 · 已存 1 份") {
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
	if !tt.HasText("仅在本机生效，适用于所有作品。") {
		t.Fatal("appearance", tt.Texts())
	}
}

// TestShotBackup writes screenshots of the backups when SHOT is a
// directory: SHOT=dir go test ./desk -run ShotBackup
func TestShotBackup(t *testing.T) {
	testshot.Dir(t)
	a, tt, _ := writingFixture(t, "整条街的灯是同时灭的。\n林晚站在巷口，手电在工具箱最底下。她没有去摸。")
	tt.SetTitleBar(ui.TitleBar{Height: 40, Right: 138})

	sync := filepath.Join(t.TempDir(), "OneDrive", "写作备份")
	was := syncFolders
	syncFolders = func() []place { return []place{{"OneDrive 同步文件夹", sync}} }
	t.Cleanup(func() { syncFolders = was })
	a.PickFolder = func(string) (string, error) { return "", nil }
	a.openAsk()
	shot(t, tt, "backup-ask.png")
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
	if err := tt.Click("立即备份"); err != nil {
		t.Fatal(err)
	}
	waitBackup(t, a, tt)
	a.bk.open = true
	shot(t, tt, "backup-line.png")
	a.bk.open = false
	a.bk.err = &os.PathError{Op: "stat", Path: `E:\雨城来信备份`, Err: os.ErrNotExist}
	s, _ := a.backupSettings()
	s.Dir = `E:\雨城来信备份`
	a.setBackup(s)
	tt.SetDark(true)
	a.bk.chip = true
	shot(t, tt, "backup-fail-dark.png")
	a.bk.chip = false
	tt.SetDark(false)
	a.bk.err = nil
	s.Dir = sync
	a.setBackup(s)
	a.bk.tab, a.page = "备份", 4
	shot(t, tt, "backup-settings.png")
	tt.SetDark(true)
	a.bk.tab, a.page = "外观", 4
	shot(t, tt, "settings-look-dark.png")
	a.bk.tab = "AI"
	shot(t, tt, "settings-models-dark.png")
	a.bk.edit = "deepseek"
	a.chooseProvider("deepseek")
	tt.Scroll(700, 500, 0, 600)
	shot(t, tt, "settings-service-dark.png")
	a.bk.prefs = false
	a.page = 0
	shot(t, tt, "discussion-dark.png")
}

// Why a backup failed is said from what was found as it failed: saying it
// does not look for the disk again, as it is said in every frame.
func TestTroubleSaysWhatWasFoundAsItFailed(t *testing.T) {
	missing := &os.PathError{Op: "stat", Path: `X:\备份`, Err: os.ErrNotExist}
	if short, _, _ := trouble(`X:\备份`, missing, true); runtime.GOOS == "windows" && short != "找不到 X 盘" {
		t.Fatalf("with the disk gone: %q", short)
	}
	if short, _, _ := trouble(`X:\备份`, missing, false); short != "找不到备份文件夹" {
		t.Fatalf("with the disk there: %q", short)
	}
	if volumeGone(t.TempDir()) {
		t.Fatal("the disk of a folder that is there counts as gone")
	}
	a, _, _ := fixture(t)
	a.backupDone(backup.Settings{Dir: t.TempDir()}, missing)
	if a.bk.err == nil || a.bk.gone {
		t.Fatalf("err %v, gone %v", a.bk.err, a.bk.gone)
	}
}
