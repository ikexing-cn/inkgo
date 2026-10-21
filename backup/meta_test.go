package backup

import "testing"

func TestBackupMetadata(t *testing.T) {
	files := Files{List: []File{
		{Rel: "work.yaml", Data: []byte("not prose")},
		{Rel: "chapters/甲.txt", Data: []byte("一 二\n三")},
		{Rel: "chapters/卷/乙.txt", Data: []byte("四五")},
		{Rel: "chapters/甲.outline.txt", Data: []byte("outline")},
		{Rel: "chapters/卷.outline.txt", Data: []byte("volume outline")},
		{Rel: "chapters/甲.marks.json", Data: []byte("marks")},
		{Rel: "trash/丙.txt", Data: []byte("trash")},
		{Rel: "chapters-other/丁.txt", Data: []byte("outside")},
	}}
	if m := files.Meta("work-id"); m.Work != "work-id" || m.Words != 5 || m.Chapters != 2 || m.Sum != "" {
		t.Fatal(m)
	}
	if m := (Files{}).Meta("empty"); m.Work != "empty" || m.Words != 0 || m.Chapters != 0 {
		t.Fatal(m)
	}
}
