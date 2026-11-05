package check

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"writer/ai"
	"writer/ext/ai/aiapi"
	"writer/ext/hostapi"
	"writer/inference"
	"writer/model"
	"writer/workspace"
	"writer/writing"
)

type requestBackend struct {
	aiapi.Backend
	handle inference.Model
	roles  []aiapi.Role
}

func (b *requestBackend) Model(_ *workspace.Store, role aiapi.Role) (aiapi.Handle, error) {
	b.roles = append(b.roles, role)
	return aiapi.Handle{Model: aiapi.Model(b.handle.Model), Name: b.handle.Name, Effort: b.handle.Effort, Completer: b.handle}, nil
}

type requestBook struct {
	hostapi.Book
	st *workspace.Store
}

func (b requestBook) Store() *workspace.Store { return b.st }

type requestMaterial struct{ entries []workspace.Entry }

func (m requestMaterial) Entries() ([]workspace.Entry, bool)  { return m.entries, true }
func (requestMaterial) HasOutlines() bool                     { return true }
func (requestMaterial) Outline(string) (string, bool)         { return "", true }
func (requestMaterial) Hooks() ([]workspace.Foreshadow, bool) { return nil, false }

type requestCapture struct{ requests []model.Request }

func (c *requestCapture) Complete(_ context.Context, r model.Request, on func(model.Delta)) (model.Reply, error) {
	c.requests = append(c.requests, r)
	if on != nil {
		on(model.Delta{Text: `{"findings":[]}`})
	}
	return model.Reply{Text: `{"findings":[]}`}, nil
}

func TestAllOnCheckUsesWritingModelAndPreservesLegacyRequestBytes(t *testing.T) {
	st, err := workspace.Create(t.TempDir(), "book", "网文", "zh-Hans")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	id, err := st.NewChapter("", "第一章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = st.WriteChapter(id, strings.Repeat("她蹲下去拧开灯座。", 30), nil); err != nil {
		t.Fatal(err)
	}
	if _, err = st.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Names: []string{"林晚"}, Text: "林晚怕黑"}}}); err != nil {
		t.Fatal(err)
	}
	capture := &requestCapture{}
	handle := inference.Model{Completer: capture, Model: model.Builtin("deepseek-flash"), Name: "deepseek/deepseek-flash", Effort: model.EffortHigh}
	backend := &requestBackend{Backend: ai.New(""), handle: handle}
	setting, err := st.Setting()
	if err != nil {
		t.Fatal(err)
	}
	p := Panel{bookHost: requestBook{st: st}, backend: backend, context: requestMaterial{setting.Entries}}
	legacy := &writing.Writer{Store: st, Model: handle, Material: aiapi.Material{Setting: true, Outline: true}}
	if _, err = legacy.Check(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	next, err := p.pen()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = next.Check(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if len(backend.roles) != 1 || backend.roles[0] != aiapi.RoleWriting {
		t.Fatal(backend.roles)
	}
	before, _ := json.Marshal(capture.requests[0])
	after, _ := json.Marshal(capture.requests[1])
	if !bytes.Equal(before, after) {
		t.Fatalf("request bytes changed\nbefore: %s\nafter: %s", before, after)
	}
}
