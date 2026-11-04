package talk

import (
	"testing"
	"writer/ext/ai/aiapi"
	"writer/ext/talk/talkapi"
	"writer/workspace"
)

func TestPreloadedMapsTheTalksContext(t *testing.T) {
	st, err := workspace.Create(t.TempDir(), "雨城", "网文", "zh-Hans")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	vol, err := st.NewChapter("", "第一卷", true)
	if err != nil {
		t.Fatal(err)
	}
	one, err := st.NewChapter(vol, "第一章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.WriteChapter(one, "上一章的正文。", nil); err != nil {
		t.Fatal(err)
	}
	two, err := st.NewChapter(vol, "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SaveCard(workspace.Card{Title: "灯灭了", Chapter: two}, ""); err != nil {
		t.Fatal(err)
	}
	got, err := Preloaded(st, aiapi.Material{Setting: true, Outline: true}, workspace.AIDraft, two)
	if err != nil || !got.Previous || got.Outlines != talkapi.VolumeChapters || got.Tokens <= 0 || got.Cards != 1 {
		t.Fatal(got, err)
	}
	got, err = Preloaded(st, aiapi.Material{Setting: true, Outline: true}, workspace.AIRewrite, two)
	if err != nil || got.Previous || got.Outlines != talkapi.VolumeChapters || got.Tokens <= 0 || got.Cards != 1 {
		t.Fatal(got, err)
	}
	got, err = Preloaded(st, aiapi.Material{Setting: true, Outline: true}, workspace.AIDraft, one)
	if err != nil || got.Previous {
		t.Fatal(got, err)
	}
}

func TestPreloadedReportsNoOutlineProviderInsteadOfEmptyOutlines(t *testing.T) {
	st, err := workspace.Create(t.TempDir(), "书", "网文", "zh-Hans")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	chapter, err := st.NewChapter("", "章", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.WriteOutline(chapter, "章纲内容"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SaveCard(workspace.Card{Title: "本章卡片", Chapter: chapter}, ""); err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{workspace.AIDraft, workspace.AIRewrite} {
		off, err := Preloaded(st, aiapi.Material{}, origin, chapter)
		if err != nil {
			t.Fatal(err)
		}
		on, err := Preloaded(st, aiapi.Material{Setting: true, Outline: true}, origin, chapter)
		if err != nil {
			t.Fatal(err)
		}
		if off.Outlines != talkapi.NoOutlines || off.Cards != 0 || on.Outlines != talkapi.BookVolumes || on.Cards != 1 || off.Tokens >= on.Tokens {
			t.Fatal(off, on)
		}
	}
}
