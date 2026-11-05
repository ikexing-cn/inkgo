package desk

import (
	"context"
	"testing"
	"writer/ext/talk/talkapi"

	"github.com/egoist/mygo/ui"
	"writer/ext/ai/aiapi"
	"writer/workspace"
)

// materialSpy records the provider snapshots at the frame boundary.
type materialSpy struct {
	capturedStream
	tools       [][]aiapi.Tool
	talks, pens []aiapi.Material
	predictions int
}

func (s *materialSpy) Build(st *workspace.Store, m aiapi.Material, tools []aiapi.Tool, b *talkapi.Beside, progress func(talkapi.Progress), step func(workspace.Event)) (talkapi.Talk, error) {
	s.talks = append(s.talks, m)
	s.tools = append(s.tools, tools)
	return s.capturedStream.Build(st, m, tools, b, progress, step)
}

func (s *materialSpy) Pen(_ *workspace.Store, m aiapi.Material) (aiapi.Pen, error) {
	s.pens = append(s.pens, m)
	return materialPen{s}, nil
}

type materialPen struct{ spy *materialSpy }

func (p materialPen) Name() string { return "fake" }

func (p materialPen) Effort() string { return "" }

func (p materialPen) Predict(context.Context, aiapi.Scene) (string, error) {
	p.spy.predictions++
	return "句子。", nil
}

func (p materialPen) Check(context.Context, string) (workspace.Check, error) {
	return workspace.Check{}, nil
}

func TestNextTalkPredictionAndPreloadFollowGlobalAndBookMaterialSwitches(t *testing.T) {
	for _, id := range []string{"lore", "outline"} {
		for _, bookScope := range []bool{false, true} {
			t.Run(id+map[bool]string{false: "/global", true: "/book"}[bookScope], func(t *testing.T) {
				a, tt, _ := marginFixture(t, true)
				spy := &materialSpy{capturedStream: capturedStream{Backend: a.ai}}
				a.ai = spy
				continueViews[a].SetBackend(spy)
				talkOf(a).Build = spy.Build
				continueViews[a].Invalidate()
				for _, on := range []bool{true, false, true} {
					a.setExtension(id, on, bookScope)
					tt.Frame()
					var want = aiapi.Material{Setting: true, Outline: true}
					if id == "lore" {
						want.Setting = on
					} else {
						want.Outline = on
					}
					stillTester(func(c *ui.Context) {
						if _, err := talkOf(a).Talk(c); err != nil {
							t.Fatal(err)
						}
						draftOf(a).DraftNote()
					}, 64, 64)
					snapshot := spy.tools[len(spy.tools)-1]
					for _, name := range []string{"look_up_setting", "note_setting", "read_outline"} {
						present := false
						for _, tool := range snapshot {
							present = present || tool.Name == name
						}
						expected := want.Setting
						if name == "read_outline" {
							expected = want.Outline
						}
						if present != expected {
							t.Fatalf("%s present=%v want=%v", name, present, expected)
						}
					}
					if spy.talks[len(spy.talks)-1] != want || spy.pens[len(spy.pens)-1] != want {
						t.Fatal(want, spy.talks, spy.pens)
					}
					if _, err := continueViews[a].Predict(context.Background(), a.write.node, "正文"); err != nil {
						t.Fatal(err)
					}
					if id == "lore" {
						talkOf(a).SourceOpen = false
						talkOf(a).OpenSource(workspace.Event{ID: "message", Text: "讨论里的文字"}, 0, 0)
						if talkOf(a).SourceOpen != on {
							t.Fatal("extract entry", on)
						}
						talkOf(a).SourceOpen = false
					}
				}
				if spy.predictions != 3 {
					t.Fatal(spy.predictions)
				}
			})
		}
	}
}
