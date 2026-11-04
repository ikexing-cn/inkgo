package aiapi

import (
	"sync"
	"testing"
	"writer/internal/testtmp"
	"writer/workspace"
)

func TestMain(m *testing.M) { testtmp.Main(m) }

func TestReplySharesBookAndSettingReadsAcrossCallsAndLabels(t *testing.T) {
	st, err := workspace.Create(t.TempDir(), "书", "网文", "zh-Hans")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	id, err := st.NewChapter("", "第一章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = st.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "林晚怕黑"}}}); err != nil {
		t.Fatal(err)
	}
	shared := NewToolShared(st)
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			b, err := shared.Book()
			if err != nil || b.Name[id] != "第一章" {
				t.Error(b, err)
			}
			s, err := shared.Setting()
			if err != nil || len(s.Entries) != 1 {
				t.Error(s, err)
			}
			if shared.Named("第一") != "第一章" {
				t.Error("label lost book name")
			}
		})
	}
	wg.Wait()
	st.Close()
	if b, err := shared.Book(); err != nil || b.Name[id] != "第一章" {
		t.Fatal("reread closed store", err)
	}
	if s, err := shared.Setting(); err != nil || len(s.Entries) != 1 {
		t.Fatal("reread closed store", err)
	}
}
