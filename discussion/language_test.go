package discussion

import (
	"strings"
	"testing"
	"writer/ext/ai/aiapi"
	"writer/prompt"
	"writer/workspace"
)

func TestEnglishDraftRewriteAndProposalRulesSpecifyProseLanguageOnly(t *testing.T) {
	for _, origin := range []string{workspace.AIDraft, workspace.AIRewrite} {
		zh := originRulesFor(origin, aiapi.Material{}, workspace.Chinese)
		en := originRulesFor(origin, aiapi.Material{}, "en")
		if !strings.Contains(en, "The prose is written in English.") || strings.Contains(zh, "English") {
			t.Fatal(en)
		}
	}
	c, err := Assemble(workspace.Work{Language: workspace.English}, 20000, workspace.Setting{}, workspace.Tree{}, nil, "Hello", Along{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(c.Request.System, "In edit proposals, the prose is written in English.") {
		t.Fatal(c.Request.System)
	}
}

func TestEnglishBesideCapCountsWords(t *testing.T) {
	text := "old " + strings.Repeat("word ", BesideCap)
	b := prompt.NewBudget(100000, "en")
	got, _ := (Writing{Before: text}).render(b, aiapi.Material{}, workspace.English)
	if strings.Contains(got, "old") || strings.Count(got, "word") != BesideCap {
		t.Fatal("English nearby prose was capped by characters")
	}
}

func TestStoredContextCountsUseTheBookUnit(t *testing.T) {
	for _, tc := range []struct {
		language    workspace.Language
		count, unit string
	}{
		{workspace.Chinese, "10", "字"}, {workspace.English, "2", "词"},
	} {
		t.Run(string(tc.language), func(t *testing.T) {
			st, err := workspace.Create(t.TempDir(), "Book", "novel", tc.language)
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			prev, err := st.NewChapter("", "Previous", false)
			if err != nil {
				t.Fatal(err)
			}
			current, err := st.NewChapter("", "Current", false)
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range []string{prev, current} {
				if _, err = st.WriteChapter(id, "Hello world", nil); err != nil {
					t.Fatal(err)
				}
			}
			tree, err := st.Chapters()
			if err != nil {
				t.Fatal(err)
			}
			for _, origin := range []string{workspace.AIDraft, workspace.AIRewrite} {
				fixed, err := fixedOf(st, aiapi.Material{}, tree, origin, current)
				if err != nil {
					t.Fatal(err)
				}
				var labels []string
				for _, p := range fixed.Pieces {
					labels = append(labels, p.Item)
				}
				wants := []string{"本章全文 " + tc.count + " " + tc.unit}
				if origin == workspace.AIDraft {
					wants = []string{"上一章全文：Previous " + tc.count + " " + tc.unit, "本章已有正文 " + tc.count + " " + tc.unit}
				}
				for _, want := range wants {
					if !strings.Contains(strings.Join(labels, "\n"), want) {
						t.Fatal(want, labels)
					}
				}
			}
			_, items := (Writing{Before: "Hello world"}).render(prompt.NewBudget(10000, tc.language), aiapi.Material{}, tc.language)
			if len(items) == 0 || items[len(items)-1].Text != "本章光标前 "+tc.count+" "+tc.unit {
				t.Fatal(items)
			}
		})
	}
}
