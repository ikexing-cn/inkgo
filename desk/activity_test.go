package desk

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDiskPollingPausesAndReturnsImmediately(t *testing.T) {
	a, tt, _ := writingFixture(t, "原稿")
	a.SetWindowActive(false)
	p := filepath.Join(a.Store.Root(), "chapters", "第一章.txt")
	if err := os.WriteFile(p, []byte("软件外改好的正文"), 0644); err != nil {
		t.Fatal(err)
	}
	markUnchecked(a)
	tt.Frame()
	tt.Frame()
	if diskChanged(a) || !a.write.checked.IsZero() {
		t.Fatal("inactive window polled the disk")
	}
	// Return must bypass the long test interval, not merely resume it.
	a.write.checked = time.Now()
	a.SetWindowActive(true)
	tt.Frame()
	if !diskChanged(a) {
		t.Fatal("return did not find the external change")
	}
}

func TestDiskPollingNoticesReturnBetweenFrames(t *testing.T) {
	a, tt, _ := writingFixture(t, "原稿")
	p := filepath.Join(a.Store.Root(), "chapters", "第一章.txt")
	if err := os.WriteFile(p, []byte("软件外改好的正文"), 0644); err != nil {
		t.Fatal(err)
	}
	a.SetWindowActive(true) // duplicate notification must not force a scan
	tt.Frame()
	if diskChanged(a) {
		t.Fatal("active notification reset the interval")
	}
	a.SetWindowActive(false)
	a.SetWindowActive(true)
	tt.Frame()
	if !diskChanged(a) {
		t.Fatal("blur and focus between frames were lost")
	}
}

func TestBackgroundSavingStillChecksConflicts(t *testing.T) {
	for _, outside := range []bool{false, true} {
		name := "save"
		if outside {
			name = "conflict"
		}
		t.Run(name, func(t *testing.T) {
			a, tt, _ := writingFixture(t, "原稿")
			shownEditor(a).Insert("新字")
			setChapterOutline(a, "后台也保存章纲")
			a.SetWindowActive(false)
			if outside {
				p := filepath.Join(a.Store.Root(), "chapters", "第一章.txt")
				if err := os.WriteFile(p, []byte("软件外改好的正文"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			markUnchecked(a)
			tt.Frame()
			want := shownEditor(a).Doc.String()
			if outside {
				want = "软件外改好的正文"
			}
			if got := disk(t, a, "第一章.txt"); got != want || diskChanged(a) != outside {
				t.Fatalf("background save: disk=%q external=%v", got, diskChanged(a))
			}
			if !a.write.checked.IsZero() {
				t.Fatal("background save also ran periodic polling")
			}
			if got := outlineDisk(t, a); got != "后台也保存章纲" {
				t.Fatalf("background outline was not saved: %q", got)
			}
		})
	}
}

func TestReturnFindsRemovedRenamedAndMovedChapters(t *testing.T) {
	for _, action := range []string{"delete", "rename", "move"} {
		t.Run(action, func(t *testing.T) {
			a, tt, _ := writingFixture(t, "原稿")
			a.SetWindowActive(false)
			tt.Frame()
			dir := filepath.Join(a.Store.Root(), "chapters")
			p := filepath.Join(dir, "第一章.txt")
			var err error
			switch action {
			case "delete":
				err = os.Remove(p)
			case "rename":
				err = os.Rename(p, filepath.Join(dir, "改名.txt"))
			case "move":
				if err = os.Mkdir(filepath.Join(dir, "新卷"), 0755); err == nil {
					err = os.Rename(p, filepath.Join(dir, "新卷", "第一章.txt"))
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if chapterLost(a) {
				t.Fatal("inactive window inspected the missing chapter")
			}
			a.SetWindowActive(true)
			tt.Frame()
			if !chapterLost(a) || shownEditor(a).Doc.String() != "原稿" {
				t.Fatal("return did not preserve the removed chapter for recovery")
			}
		})
	}
}

func TestBrokenOutlineRetryPausesWithWindow(t *testing.T) {
	a, tt, _ := outlineFixture(t)
	p := filepath.Join(a.Store.Root(), "ext", "outline", "outline.json")
	if err := os.WriteFile(p, []byte("{broken"), 0644); err != nil {
		t.Fatal(err)
	}
	outlinePage(a).Enter()
	tt.Frame()
	if !tt.HasText("大纲文件读不出来") {
		t.Fatal(tt.Texts())
	}
	a.SetWindowActive(false)
	a.book.Timing.CheckEvery = 0 // a retry is due, but the window is inactive
	if err := os.WriteFile(p, []byte(`{"schema":2,"lines":[],"cards":[]}`), 0644); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("大纲文件读不出来") {
		t.Fatal("background retry read the repaired file")
	}
	a.book.Timing.CheckEvery = time.Hour
	a.SetWindowActive(true)
	tt.Frame()
	if tt.HasText("大纲文件读不出来") || !tt.HasText("新增情节线") {
		t.Fatal("return did not retry the repaired outline", tt.Texts())
	}
}
