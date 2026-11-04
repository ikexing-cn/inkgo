package discussion

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"writer/ext/ai/aiapi"
	"writer/workspace"
)

func writePlan(t *testing.T, st *workspace.Store, p workspace.Plan) []byte {
	t.Helper()
	p.Schema = workspace.Schema
	used := map[string]bool{}
	for _, l := range p.Lines {
		used[l.ID] = true
	}
	for _, c := range p.Cards {
		if c.ID != "" {
			used[c.ID] = true
		}
	}
	for i := range p.Cards {
		if p.Cards[i].ID == "" {
			id := workspace.NewID()
			for used[id] {
				id = workspace.NewID()
			}
			p.Cards[i].ID = id
			used[id] = true
		}
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(st.Root(), "ext", "outline", "outline.json"), b, 0644); err != nil {
		t.Fatal(err)
	}
	return b
}
func TestFixedCardsAndPreloaded(t *testing.T) {
	e, _ := setup(t)
	_, one, two, _ := shelf(t, e)
	tree, err := e.Store.Chapters()
	if err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{workspace.AIDraft, workspace.AIRewrite} {
		t.Run(origin, func(t *testing.T) {
			writePlan(t, e.Store, workspace.Plan{})
			baseline, err := Preloaded(e.Store, aiapi.Material{Setting: true, Outline: true}, origin, two)
			if err != nil {
				t.Fatal(err)
			}
			p := workspace.Plan{Cards: []workspace.Card{{Title: "别章的事", Chapter: one}, {Title: "灯灭了", Chapter: two}, {Title: "她推开门", Chapter: two}}}
			writePlan(t, e.Store, p)
			f, err := fixedOf(e.Store, aiapi.Material{Setting: true, Outline: true}, tree, origin, two)
			if err != nil {
				t.Fatal(err)
			}
			i := slices.IndexFunc(f.Pieces, func(p Piece) bool { return p.Part == workspace.PartCards })
			if i != 2 || f.Pieces[i].Item != "本章的卡片 2 张" || f.Pieces[i].Text != "【本章的卡片】\n#2｜发生：灯灭了｜「第二章」\n#3｜发生：她推开门｜「第二章」" {
				t.Fatalf("pieces %+v", f.Pieces)
			}
			ctx, _, err := lay(e.Store.Work(), 1<<30, workspace.Setting{}, tree, talk{tools: allTools(), fixed: f, material: aiapi.Material{Setting: true, Outline: true}})
			if err != nil || ctx.Sent.Part(workspace.PartCards) == 0 || !slices.ContainsFunc(ctx.Included, func(it workspace.ContextItem) bool { return it.Text == "本章的卡片 2 张" }) {
				t.Fatalf("context %+v, %v", ctx, err)
			}
			n, err := Preloaded(e.Store, aiapi.Material{Setting: true, Outline: true}, origin, two)
			if err != nil || n.Tokens != baseline.Tokens+ctx.Sent.Part(workspace.PartCards) {
				t.Fatalf("preloaded %d, baseline %d, cards %d: %v", n.Tokens, baseline.Tokens, ctx.Sent.Part(workspace.PartCards), err)
			}
			for _, data := range [][]byte{writePlan(t, e.Store, workspace.Plan{Cards: p.Cards[:1]}), []byte(`{broken`)} {
				if err := os.WriteFile(filepath.Join(e.Store.Root(), "ext", "outline", "outline.json"), data, 0644); err != nil {
					t.Fatal(err)
				}
				f, err := fixedOf(e.Store, aiapi.Material{Setting: true, Outline: true}, tree, origin, two)
				if err != nil || slices.ContainsFunc(f.Pieces, func(p Piece) bool { return p.Part == workspace.PartCards }) {
					t.Fatalf("unexpected cards: %+v, %v", f, err)
				}
				if n, err := Preloaded(e.Store, aiapi.Material{Setting: true, Outline: true}, origin, two); err != nil || n.Tokens != baseline.Tokens {
					t.Fatalf("fallback preloaded %d, want %d: %v", n.Tokens, baseline.Tokens, err)
				}
			}
		})
	}
	for _, want := range []string{"read_outline", "故事里发生的先后", "可能和章节的先后不同", "还没写的打算", "以正文为准，并告诉作者哪里不一致"} {
		if !strings.Contains(talkRules, want) {
			t.Errorf("talk rules lack %q", want)
		}
	}
	for _, origin := range []string{workspace.AIDraft, workspace.AIRewrite} {
		if !strings.Contains(originRules[origin], "本章的卡片") {
			t.Errorf("%s rules omit cards", origin)
		}
	}
	if !strings.Contains(originRules[workspace.AIDraft], "照章纲和本章的卡片写") {
		t.Fatal("draft rules do not follow cards")
	}
}
