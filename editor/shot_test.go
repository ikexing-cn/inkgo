package editor

import (
	"image/png"
	"os"
	"testing"
)

// TestShot writes a screenshot when SHOT is set, to look at the drawing.
func TestShot(t *testing.T) {
	path := os.Getenv("SHOT")
	if path == "" {
		t.Skip("set SHOT=file.png")
	}
	d := NewDoc("　　雨下了整整一夜。林晚站在檐下，看着院子里那口老井，井沿上的青苔被雨水洗得发亮。“你还记得那把钥匙吗？”沈砚从屋里走出来。\n　　她没有回答。")
	d.Add(Mark{Kind: MarkForeshadow, Start: 30, End: 36, Note: ""})
	e := New(d, Options{Entities: []Entity{{Name: "林晚", Kind: "人物"}}})
	e.MoveTo(d.Len(), false)
	tt := newTester(e)
	tt.Type("林")
	tt.Compose("shenyan", 7)
	tt.Frame()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}
