package desk

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// The outline page saves its cell after a pause, not on every key.
func TestOutlineSavesAfterAPause(t *testing.T) {
	saved := saveAfter
	defer func() { saveAfter = saved }()
	a, tt, f := outlineFixture(t)
	saveAfter = time.Hour
	click(t, tt, "章纲-"+f.one)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("新的章纲")
	tt.Frame()
	if ch, err := a.Store.ReadChapter(f.one); err != nil || ch.Outline != "何主任要一份灯数。" {
		t.Fatalf("the outline was written before the pause: %q %v", ch.Outline, err)
	}
	saveAfter = 0
	tt.Frame()
	if ch, err := a.Store.ReadChapter(f.one); err != nil || ch.Outline != "新的章纲" {
		t.Fatalf("the outline was not written after the pause: %q %v", ch.Outline, err)
	}
}

// The outline page's cell is saved as the work closes.
func TestOutlineDraftSavedOnClose(t *testing.T) {
	saved := saveAfter
	defer func() { saveAfter = saved }()
	a, tt, f := outlineFixture(t)
	saveAfter = time.Hour
	click(t, tt, "章纲-"+f.one)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("关作品前写的章纲")
	tt.Frame()
	a.leaveWork()
	if ch, err := a.Store.ReadChapter(f.one); err != nil || ch.Outline != "关作品前写的章纲" {
		t.Fatalf("the draft was lost at close: %q %v", ch.Outline, err)
	}
}
