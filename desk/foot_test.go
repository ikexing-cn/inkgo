package desk

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"writer/backup"
	"writer/internal/testshot"
	"writer/workspace"
)

func TestFooterBackupOnEveryWritingView(t *testing.T) {
	a, tt, chapter := writingFixture(t, "雨停了。")
	vol, err := a.book.NewChapter("", "第一卷", true)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{chapter, vol, ""} {
		if id == "" {
			a.write.node, a.write.blank = "", true
		} else {
			a.openNode(id)
		}
		for _, ai := range []bool{false, true} {
			a.turnAI(ai)
			tt.Frame()
			tt.Frame()
			b, foot := find(t, tt, "备份状态"), find(t, tt, "底栏")
			if b.Y < foot.Y || b.Y+b.H > foot.Y+foot.H {
				t.Fatal("backup left footer", b, foot)
			}
			click(t, tt, "备份状态")
			hasText(t, tt, "作品目前仅保存在本机。")
			if find(t, tt, "选择位置…").Y >= foot.Y {
				t.Fatal("backup did not open upward")
			}
			shut(tt, &a.bk.open)
		}
	}
}

func TestFooterGivesUpCountsBeforeTheCheck(t *testing.T) {
	a, tt, _ := writingFixture(t, strings.Repeat("雨停了。", 70))
	a.sideHidden, a.doc.hidden = true, true
	a.setBackup(backup.Settings{Dir: t.TempDir(), Last: time.Now()})
	line := "本章已修改，尚未重新检查"
	// A stale check leaves the immediate-check link beside the status.
	if err := a.book.SaveCheck(a.write.node, workspace.Check{Of: "old", At: time.Now()}); err != nil {
		t.Fatal(err)
	}
	var full, prevCount float32
	for _, width := range []int{1000, 820, 700, 620, 540, 480} {
		tt.SetSize(width, 780)
		tt.Frame()
		tt.Frame()
		tt.Frame()
		status, count := find(t, tt, line), find(t, tt, "字数")
		if width == 1000 {
			full, prevCount = status.W, count.W
		}
		if status.W < full-1 && count.W > 1 {
			t.Fatalf("%d: check shrank before counts: %v %v (full %v)", width, status, count, full)
		}
		if count.W > prevCount+1 {
			t.Fatalf("counts grew in a smaller window: %v > %v", count.W, prevCount)
		}
		prevCount = count.W
		for _, label := range []string{"立即检查", "检查时机", "备份状态"} {
			r := find(t, tt, label)
			if r.X+r.W > float32(width)-24+1 || r.H > 30 {
				t.Fatal("fixed footer item clipped or wrapped", width, label, r)
			}
		}
		if find(t, tt, "本章 280 字 · 全书 280 字").W > count.W+1 {
			t.Fatal("count text escapes its item")
		}
	}
}

func TestFooterFollowsThePaper(t *testing.T) {
	a, tt, _ := writingFixture(t, strings.Repeat("雨停了。", 70))
	a.doc.hidden, a.sideHidden = true, true
	for _, width := range []int{1100, 1280, 2200} {
		tt.SetSize(width, 780)
		tt.Frame()
		tt.Frame()
		tt.Frame()
		r, ok := a.write.cur().ed.RangeBox(0, a.write.cur().ed.Doc.Len())
		state := find(t, tt, "本章尚未检查")
		if !ok || math.Abs(float64(state.X-a.LastProse.X-r.X)) > 1 {
			t.Fatal("footer does not follow prose", width, state, r)
		}
		foot := find(t, tt, "底栏")
		img := tt.Image()
		if img.At(2, int(foot.Y)) != img.At(2, int(foot.Y)-1) {
			t.Fatal("footer has a rule")
		}
	}
}

func TestFooterBackupTimeIsInItsTooltipAndPopover(t *testing.T) {
	a, tt, _ := writingFixture(t, "雨停了。")
	at := time.Now()
	a.setBackup(backup.Settings{Dir: t.TempDir(), Last: at})
	tt.Frame()
	tt.Frame()
	hasText(t, tt, "已备份")
	if tt.HasText("已备份 ·") {
		t.Fatal("backup time is always visible")
	}
	r := find(t, tt, "备份状态")
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(time.Second)
	hasText(t, tt, "已备份 · 今天 "+at.Format("15:04"))
	click(t, tt, "备份状态")
	hasText(t, tt, at.Format("15:04")+" 已备份")
}

func TestShotFooterBackup(t *testing.T) {
	testshot.Dir(t)
	a, tt, _ := writingFixture(t, strings.Repeat("雨停了。", 40))
	a.PickFolder = func(string) (string, error) { return "", nil }
	dir := t.TempDir()
	a.bk.open = true
	shot(t, tt, "footer-backup-none.png")
	s := backup.Settings{Dir: dir, Last: time.Now(), OnClose: true}
	a.setBackup(s)
	shot(t, tt, "footer-backup-ready.png")
	a.bk.running = true
	shot(t, tt, "footer-backup-running.png")
	a.bk.running = false
	a.backupDone(s, fmt.Errorf("backup: %w", os.ErrNotExist))
	shot(t, tt, "footer-backup-failed.png")
}
