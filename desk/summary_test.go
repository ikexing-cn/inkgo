package desk

import (
	"strings"
	"testing"

	"writer/writing"
)

var longProse = strings.Repeat("雨从傍晚开始下，一直没有停。", 20)

// Leaving a chapter changed has its summary written in the background; the
// summary the author wrote is theirs, and only marked when the prose moves
// on.
func TestLeavingAChapterSummarizesIt(t *testing.T) {
	a, tt, one := writingFixture(t, longProse)
	f := &stream{pieces: []string{"林晚冒雨", "进城。"}}
	penWith(a, f)
	a.ck.when = checkOnDemand // the checks ask the same model
	two, err := a.Store.NewChapter("", "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	a.setError(a.loadTree())

	// Opened and left unchanged: nothing to do.
	a.openNode(two)
	tt.Frame()
	if len(f.asked) != 0 || a.sums.running != "" {
		t.Fatal("summarized an unchanged chapter")
	}
	if tt.HasText("本章摘要 · 暂无") {
		t.Fatal("a summary box over an empty chapter")
	}
	a.openNode(one)
	a.write.ed.MoveTo(a.write.ed.Doc.Len(), false)
	a.write.ed.Insert("她推开门。")
	a.openNode(two)
	until(t, tt, "the summary", func() bool {
		sum, _ := a.Store.ReadSummary(one)
		return sum.Text == "林晚冒雨进城。"
	})
	sum, _ := a.Store.ReadSummary(one)
	ch, _ := a.Store.ReadChapter(one)
	if sum.By != "deepseek/deepseek-flash" || sum.Stale(ch.Text) {
		t.Fatalf("%+v", sum)
	}

	// Shown under the outline; the author rewrites it.
	a.openNode(one)
	tt.Frame()
	if !tt.HasText("本章摘要") || a.write.summary != "林晚冒雨进城。" {
		t.Fatal(tt.Texts())
	}
	a.write.summary = "林晚进城修灯。"
	a.write.ed.Insert("又一句。")
	a.openNode(two)
	tt.Frame()
	tt.Frame()
	sum, _ = a.Store.ReadSummary(one)
	if sum.Text != "林晚进城修灯。" || sum.By != "" || len(f.asked) != 1 {
		t.Fatalf("%+v asked %d", sum, len(f.asked))
	}

	// The prose moves on: the author's summary stays, marked stale.
	a.openNode(one)
	a.write.ed.Insert("再一句。")
	a.flush()
	tt.Frame()
	if !tt.HasText("本章摘要 · 可能过时") {
		t.Fatal(tt.Texts())
	}
	a.openNode(two)
	tt.Frame()
	tt.Frame()
	if len(f.asked) != 1 {
		t.Fatal("rewrote the author's summary")
	}

	// Asked for, it is written again.
	a.openNode(one)
	a.write.summaryOpen = true
	until(t, tt, "the summary open", func() bool {
		r, ok := tt.Find("重新生成摘要")
		return ok && r.H > 0
	})
	if err := tt.Click("重新生成摘要"); err != nil {
		t.Fatal(err)
	}
	until(t, tt, "the summary again", func() bool { return a.write.sum.By != "" })
	if a.write.summary != "林晚冒雨进城。" {
		t.Fatal(a.write.summary)
	}
}

// The draft dialog says which chapters before have no summary, and fills
// them in when asked.
func TestDraftOffersToFillInSummaries(t *testing.T) {
	a, tt, one := writingFixture(t, longProse)
	f := &stream{pieces: []string{"林晚进城。"}}
	penWith(a, f)
	a.write.autoSummary = false
	two, err := a.Store.NewChapter("", "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.Store.WriteOutline(two, "她修好井边的灯。"); err != nil {
		t.Fatal(err)
	}
	a.setError(a.loadTree())
	a.openNode(two)
	a.openDraft()
	tt.Frame()
	if !tt.HasText("前文有 1 章尚无摘要，起草时 AI 无法了解这些章节的内容。") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("补齐摘要"); err != nil {
		t.Fatal(err)
	}
	until(t, tt, "the summary", func() bool {
		sum, _ := a.Store.ReadSummary(one)
		return sum.Text != ""
	})
	tt.Frame()
	if tt.HasText("本章摘要 · 暂无") {
		t.Fatal(tt.Texts())
	}
	// The dialog, still open, says what the draft will have now.
	until(t, tt, "the note", func() bool { return tt.HasText("起草时 AI 会参考前 1 章的摘要，以衔接前文。") })
	a.write.draftOpen = false
	a.openDraft()
	tt.Frame()
	if !tt.HasText("起草时 AI 会参考前 1 章的摘要，以衔接前文。") {
		t.Fatal(tt.Texts())
	}
	pw, _ := aiOf(a).NewWriter(a.Store)
	job, err := pw.Draft(workspaceScene(two), 0)
	if err != nil || !strings.Contains(job.Request.Messages[0].Content, "《第一章》林晚进城。") {
		t.Fatal(err, job.Request.Messages)
	}
}

func workspaceScene(id string) writing.Scene { return writing.Scene{Chapter: id} }
