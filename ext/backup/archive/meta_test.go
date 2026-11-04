package archive

import "writer/workspace"

import "testing"

func TestBackupMetadata(t *testing.T) {
	files := Files{List: []File{
		{Rel: "work.yaml", Data: []byte("not prose")},
		{Rel: "chapters/甲.txt", Data: []byte("一 二\n三")},
		{Rel: "chapters/卷/乙.txt", Data: []byte("四五")},
		{Rel: "ext/outline/chapter-id.txt", Data: []byte("outline")},
		{Rel: "ext/outline/volume-id.txt", Data: []byte("volume outline")},
		{Rel: "chapters/甲.marks.json", Data: []byte("marks")},
		{Rel: "trash/丙.txt", Data: []byte("trash")},
		{Rel: "chapters-other/丁.txt", Data: []byte("outside")},
	}}
	if m := files.Meta("work-id", workspace.Chinese); m.Work != "work-id" || m.Words != 5 || m.Chapters != 2 || m.Sum != "" {
		t.Fatal(m)
	}
	if m := (Files{}).Meta("empty", workspace.Chinese); m.Work != "empty" || m.Words != 0 || m.Chapters != 0 {
		t.Fatal(m)
	}
}
