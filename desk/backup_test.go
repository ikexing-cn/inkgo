package desk

import (
	"os"
	"path/filepath"
	"testing"
	"time"
	"writer/ext/backup"
	"writer/ext/backup/backuptext"

	"github.com/egoist/mygo/ui"

	"writer/ext/backup/archive"
	"writer/internal/testshot"
)

// waitBackup runs frames until the backup in flight is done.
func waitBackup(t *testing.T, a *App, tt *ui.Tester) {
	t.Helper()
	for range 500 {
		tt.Frame()
		if !backupOf(a).State.Running {
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
// starting backs up at once, and the footer says so.
func TestFirstBackup(t *testing.T) {
	a, tt, _ := fixture(t)
	sync := filepath.Join(t.TempDir(), "OneDrive")
	if err := os.Mkdir(sync, 0755); err != nil {
		t.Fatal(err)
	}
	was := backup.SyncFolders
	backup.SyncFolders = func() []backup.Place {
		return []backup.Place{{Name: func() string { return backuptext.BackupSyncFolder("OneDrive") }, Dir: sync}}
	}
	t.Cleanup(func() { backup.SyncFolders = was })
	enableBackupQuestion(a)
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
	click(t, tt, "正文")
	if zips(t, sync) != 1 || !tt.HasText("已备份") {
		t.Fatal("not backed up", zips(t, sync), tt.Texts())
	}
	click(t, tt, "备份状态")
	hasText(t, tt, "备份至 "+sync)
	shut(tt, &backupOf(a).State.Open)
	s, ok := backupOf(a).Settings()
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
	a, _, _ := writingFixture(t, "雨停了。")
	dir := t.TempDir()
	backupOf(a).Configure(archive.Settings{Dir: dir})
	// Started with no frame after it, the backup is still in flight as the
	// work closes: a click runs frames, which may hand a small book's backup
	// back first.
	backupOf(a).Run(ui.Services{})
	if !backupOf(a).State.Running {
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

func TestDisabledBackupDiscardsAnOutstandingFolderChoice(t *testing.T) {
	a, tt, _ := fixture(t)
	started, release := make(chan struct{}), make(chan struct{})
	dir := t.TempDir()
	a.PickFolder = func(string) (string, error) { close(started); <-release; return dir, nil }
	backupOf(a).OpenAsk()
	tt.Frame()
	click(t, tt, backuptext.BackupOtherFolder())
	select {
	case <-started:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("picker never started")
	}
	requireExtension(t, a.extensions.Set("backup", false))
	close(release)
	for n := 0; !a.jobs.pending(); n++ {
		if n == 500 {
			t.Fatal("picker never completed")
		}
		time.Sleep(time.Millisecond)
	}
	tt.Frame()
	requireExtension(t, a.extensions.Set("backup", true))
	tt.Frame()
	if s, _ := backupOf(a).Settings(); s.Dir != "" || zips(t, dir) != 0 {
		t.Fatal("disabled extension accepted stale picker result", s)
	}
}

func TestAskBackupFindsSyncFoldersOnce(t *testing.T) {
	a, tt, _ := fixture(t)
	calls := 0
	was := backup.SyncFolders
	backup.SyncFolders = func() []backup.Place {
		calls++
		return []backup.Place{{Name: func() string { return "同步文件夹" }, Dir: filepath.Join(t.TempDir(), "写作备份")}}
	}
	t.Cleanup(func() { backup.SyncFolders = was })
	backupOf(a).OpenAsk()
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
	enableBackupQuestion(a)
	if err := a.Open(a.Store.Root()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("稍后再说"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	if s, ok := backupOf(a).Settings(); !ok || s.Later.IsZero() || s.Dir != "" {
		t.Fatalf("settings %+v", s)
	}
	if err := a.Open(a.Store.Root()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if backupOf(a).State.Ask || tt.HasText("作品目前仅保存在本机") {
		t.Fatal("asked again so soon")
	}
}

// A backup that fails says why in the footer and on the writing toolbar,
// until it is put off for the day; the settings page shows the backups.
func TestBackupFails(t *testing.T) {
	a, tt, _ := writingFixture(t, "整条街的灯是同时灭的。")
	gone := filepath.Join(t.TempDir(), "备份")
	backupOf(a).Configure(archive.Settings{Dir: gone, OnClose: true})
	tt.Frame()
	hasText(t, tt, "尚未备份")
	click(t, tt, "备份状态")
	if err := tt.Click("立即备份"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	waitBackup(t, a, tt)
	shut(tt, &backupOf(a).State.Open)
	if !tt.HasText("备份失败") || !tt.HasText("备份失败 · 找不到备份文件夹") || !tt.HasText("待处理 1") {
		t.Fatal("failure not shown", tt.Texts())
	}
	if err := tt.Click("备份状态"); err != nil {
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
	click(t, tt, "备份状态")
	if err := tt.Click("重试"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	waitBackup(t, a, tt)
	if tt.HasText("备份失败") || !tt.HasText("已备份") || zips(t, gone) != 1 {
		t.Fatal("not backed up", tt.Texts())
	}
	shut(tt, &backupOf(a).State.Open)
	if err := clickMenu(tt, "文件", "菜单-设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("设置-备份"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.settings.open || tt.HasText("为整部作品另存一份副本") || !tt.HasText("11 字 · 1 章 · 已存 1 份") {
		t.Fatal("settings page", tt.Texts())
	}
	if err := tt.Click("写作时每小时一次"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s, _ := backupOf(a).Settings(); !s.Hourly || !s.OnClose {
		t.Fatalf("settings %+v", s)
	}
	if err := tt.Click("设置-外观"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("这台电脑上的所有作品") {
		t.Fatal("appearance", tt.Texts())
	}
}

func TestPendingBackupRetries(t *testing.T) {
	a, tt, _ := writingFixture(t, "雨停了。")
	dir := filepath.Join(t.TempDir(), "备份")
	s := archive.Settings{Dir: dir}
	backupOf(a).Configure(s)
	backupOf(a).RecordResult(s, os.ErrNotExist)
	tt.Frame()
	groups := a.pendingGroups()
	if len(groups) != 1 || groups[0].name != "备份" || len(groups[0].items) != 1 {
		t.Fatalf("pending groups = %+v", groups)
	}
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("待处理"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(groups[0].items[0].text); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.write.pendOpen {
		t.Fatal("retry left the pending list open")
	}
	waitBackup(t, a, tt)
	if backupOf(a).State.Policy.Failed() || zips(t, dir) != 1 {
		t.Fatal("the pending backup did not retry")
	}
}

// TestShotBackup writes screenshots of the backups when SHOT is a
// directory: SHOT=dir go test ./desk -run ShotBackup
func TestShotBackup(t *testing.T) {
	testshot.Dir(t)
	a, tt, _ := writingFixture(t, "整条街的灯是同时灭的。\n林晚站在巷口，手电在工具箱最底下。她没有去摸。")
	tt.SetTitleBar(ui.TitleBar{Height: 40, Right: 138})

	sync := filepath.Join(t.TempDir(), "OneDrive", "写作备份")
	was := backup.SyncFolders
	backup.SyncFolders = func() []backup.Place {
		return []backup.Place{{Name: func() string { return backuptext.BackupSyncFolder("OneDrive") }, Dir: sync}}
	}
	t.Cleanup(func() { backup.SyncFolders = was })
	a.PickFolder = func(string) (string, error) { return "", nil }
	backupOf(a).OpenAsk()
	shot(t, tt, "backup-ask.png")
	backupOf(a).State.Ask = false
	if err := os.MkdirAll(sync, 0755); err != nil {
		t.Fatal(err)
	}
	backupOf(a).Configure(archive.Settings{Dir: sync, OnClose: true, Daily: true})
	tt.Frame()
	click(t, tt, "备份状态")
	if err := tt.Click("立即备份"); err != nil {
		t.Fatal(err)
	}
	waitBackup(t, a, tt)
	shot(t, tt, "backup-line.png")
	shut(tt, &backupOf(a).State.Open)
	a.openPrefs("backup.settings")
	shot(t, tt, "backup-settings-ready.png")
	shut(tt, &a.settings.open)
	backupOf(a).State.Policy.RecordResult(sync, &os.PathError{Op: "stat", Path: `E:\雨城来信备份`, Err: os.ErrNotExist}, time.Now())
	s, _ := backupOf(a).Settings()
	s.Dir = `E:\雨城来信备份`
	backupOf(a).Configure(s)
	tt.SetDark(true)
	backupOf(a).State.Chip = true
	shot(t, tt, "backup-fail-dark.png")
	backupOf(a).State.Chip = false
	tt.SetDark(false)
	backupOf(a).State.Policy.ForgetFailure()
	s.Dir = sync
	backupOf(a).Configure(s)
	a.openPrefs("backup.settings")
	shot(t, tt, "backup-settings.png")
	tt.SetDark(true)
	a.openPrefs("appearance")
	shot(t, tt, "settings-look-dark.png")
	a.settings.tab = "AI"
	shot(t, tt, "settings-models-dark.png")
	aiViewOf(a).ChooseProvider("deepseek")
	aiViewOf(a).ServiceForm.Edit = "deepseek"
	tt.Scroll(700, 500, 0, 600)
	shot(t, tt, "settings-service-dark.png")
	a.settings.open = false
	showPage(a, pageTalk)
	shot(t, tt, "discussion-dark.png")
}

func TestTroubleSaysWhatWasFoundAsItFailed(t *testing.T) {
	missing := &os.PathError{Op: "stat", Path: `X:\备份`, Err: os.ErrNotExist}
	a, _, _ := fixture(t)
	s := archive.Settings{Dir: t.TempDir()}
	backupOf(a).RecordResult(s, missing)
	short, _, _ := backup.TroubleText(backupOf(a).State.Policy.Trouble(s.Dir))
	if !backupOf(a).State.Policy.Failed() || short != "找不到备份文件夹" {
		t.Fatalf("failed %v, trouble %q", backupOf(a).State.Policy.Failed(), short)
	}
}
