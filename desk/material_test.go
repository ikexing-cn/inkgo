package desk

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"
	"writer/aiport"
	"writer/workspace"
)

// materialSpy records the provider snapshots at the frame boundary.
type materialSpy struct {
	capturedStream
	talks, pens, preloads []aiport.Material
	predictions           int
}

func (s *materialSpy) Talk(st *workspace.Store, m aiport.Material, b *aiport.Beside, progress func(aiport.Progress), step func(workspace.Event)) (aiport.Talk, error) {
	s.talks = append(s.talks, m)
	return s.capturedStream.Talk(st, m, b, progress, step)
}

func (s *materialSpy) Pen(_ *workspace.Store, m aiport.Material) (aiport.Pen, error) {
	s.pens = append(s.pens, m)
	return materialPen{s}, nil
}

func (s *materialSpy) Preloaded(_ *workspace.Store, m aiport.Material, _, _ string) (aiport.Preload, error) {
	s.preloads = append(s.preloads, m)
	return aiport.Preload{}, nil
}

type materialPen struct{ spy *materialSpy }

func (p materialPen) Name() string { return "fake" }

func (p materialPen) Effort() string { return "" }

func (p materialPen) Predict(context.Context, aiport.Scene) (string, error) {
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
				spy := &materialSpy{capturedStream: capturedStream{Service: a.ai}}
				a.ai = spy
				a.modelsChanged()
				for _, on := range []bool{true, false, true} {
					a.setExtension(id, on, bookScope)
					tt.Frame()
					var want = aiport.Material{Setting: true, Outline: true}
					if id == "lore" {
						want.Setting = on
					} else {
						want.Outline = on
					}
					stillTester(func(c *ui.Context) {
						if _, err := a.talk(c); err != nil {
							t.Fatal(err)
						}
						a.draftNote()
					}, 64, 64)
					if spy.talks[len(spy.talks)-1] != want || spy.pens[len(spy.pens)-1] != want || spy.preloads[len(spy.preloads)-1] != want {
						t.Fatal(want, spy.talks, spy.pens, spy.preloads)
					}
					if _, err := (pen{writer: &a.write.pen, chapter: a.write.node}).Predict(context.Background(), "正文"); err != nil {
						t.Fatal(err)
					}
					if id == "lore" {
						a.sourceOpen = false
						a.openSource(workspace.Event{ID: "message", Text: "讨论里的文字"}, 0, 0)
						if a.sourceOpen != on {
							t.Fatal("extract entry", on)
						}
						a.sourceOpen = false
					}
				}
				if spy.predictions != 3 {
					t.Fatal(spy.predictions)
				}
			})
		}
	}
}
