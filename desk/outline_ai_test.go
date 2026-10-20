package desk

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"writer/workspace"
)

// The draft dialog describes the cards of its chapter and keeps that
// count until it is opened again, including when the file breaks outside.
func TestDraftDialogWithCards(t *testing.T) {
	a, tt, id := writingFixture(t, "")
	penWith(a, &script{})
	a.write.outline = "雨夜修灯。"
	other, err := a.Store.NewChapter("", "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Store.SaveCard(workspace.Card{Title: "别章的事", Chapter: other}, ""); err != nil {
		t.Fatal(err)
	}
	tail := "起草在一段新的讨论里进行：写好的先在正文里显示为提议，采纳后才写入，写入前保存快照；想改哪里，在讨论里接着说。"
	oldHead := "AI 仅将章纲扩写为正文，不添加章纲之外的重大情节。" + tail
	newHead := "AI 仅将章纲和本章的卡片扩写为正文，不添加这些之外的重大情节。" + tail
	a.openDraft()
	tt.Frame()
	if !tt.HasText(oldHead) || !strings.HasPrefix(a.write.draftNote, "将带上各卷卷纲等，约 ") || a.write.draftCards != 0 {
		t.Fatal("dialog changed without cards of its chapter", tt.Texts())
	}
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"她拧开灯座", "她发现断口"} {
		if _, err := a.Store.SaveCard(workspace.Card{Title: title, Chapter: id}, ""); err != nil {
			t.Fatal(err)
		}
	}
	a.openDraft()
	tt.Frame()
	tt.Frame()
	if !tt.HasText(newHead) || tt.HasText(oldHead) || !strings.HasPrefix(a.write.draftNote, "将带上本章的 2 张卡片、各卷卷纲等，约 ") || a.write.draftCards != 2 {
		t.Fatal("dialog does not describe its chapter's cards", tt.Texts())
	}
	if dir := os.Getenv("SHOT"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(filepath.Join(dir, "draft-with-cards.png"))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, tt.Image())
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			t.Fatalf("screenshot: %v, %v", err, closeErr)
		}
	}
	note := a.write.draftNote
	if err := os.WriteFile(filepath.Join(a.Store.Root(), "outline.json"), []byte(`{broken`), 0644); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if !tt.HasText(newHead) || a.write.draftNote != note || a.write.draftCards != 2 {
		t.Fatal("a frame reread the cards", tt.Texts())
	}
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	a.openDraft()
	tt.Frame()
	if !tt.HasText(oldHead) || !strings.HasPrefix(a.write.draftNote, "将带上各卷卷纲等，约 ") || a.write.draftCards != 0 || a.Error != "" {
		t.Fatal("a broken outline prevents ordinary drafting", a.Error, tt.Texts())
	}
	if partNames[workspace.PartCards] != "本章的卡片" {
		t.Fatal("the cards have no part name")
	}
}

func TestDraftNoteCardsBeforePreviousChapter(t *testing.T) {
	a, _, one := writingFixture(t, "上一章的正文。")
	penWith(a, &script{})
	vol, err := a.Store.NewChapter("", "第一卷", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Store.MoveChapter(one, vol, 0); err != nil {
		t.Fatal(err)
	}
	two, err := a.Store.NewChapter(vol, "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Store.SaveCard(workspace.Card{Title: "修好路灯", Chapter: two}, ""); err != nil {
		t.Fatal(err)
	}
	if err := a.loadTree(); err != nil {
		t.Fatal(err)
	}
	a.openNode(two)
	a.openDraft()
	if !strings.HasPrefix(a.write.draftNote, "将带上本章的 1 张卡片、上一章全文、本卷各章章纲等，约 ") {
		t.Fatal(a.write.draftNote)
	}
}
