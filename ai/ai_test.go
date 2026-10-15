package ai

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"

	"writer/model"
	"writer/workspace"
	"writer/writing"
)

// replies is a model that gives each of its replies in turn.
type replies []model.Reply

func (r *replies) Complete(ctx context.Context, req model.Request, on func(model.Delta)) (model.Reply, error) {
	next := (*r)[0]
	*r = (*r)[1:]
	if next.Text == "" {
		return next, errors.New("服务不可用")
	}
	return next, nil
}

// Each request counts as the work spent, with its cost at the prices built
// in, failed ones too; one that tells no usage counts nothing.
func TestRequestsAreCounted(t *testing.T) {
	st, err := workspace.Create(filepath.Join(t.TempDir(), "work"), "测试作品", "网文")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	m := counted{&replies{
		{Text: "好。", Usage: model.Usage{Input: 1_000_000, CacheRead: 500_000, Output: 1000}},
		{Usage: model.Usage{Input: 1000}},
		{Text: "没有用量。"},
	}, st, "deepseek/deepseek-flash", "deepseek-flash"}
	for range 3 {
		m.Complete(context.Background(), model.Request{}, nil)
	}
	got, err := st.Spent()
	want := 0.5*0.3 + 0.5*0.006 + 0.001*1.2 + 0.001*0.3
	if err != nil || len(got) != 1 || got[0].Model != "deepseek/deepseek-flash" || got[0].Requests != 2 || got[0].Input != 1_001_000 ||
		got[0].Currency != "$" || got[0].Cost < want-1e-9 || got[0].Cost > want+1e-9 {
		t.Fatalf("spent %+v %v", got, err)
	}
}

// Off, nothing that sends is built: the catalog of a service, the talk
// and the pen fail at once. On again, they are built as before.
func TestOffBuildsNothing(t *testing.T) {
	st, err := workspace.Create(filepath.Join(t.TempDir(), "work"), "测试作品", "网文")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := New("")
	var built atomic.Int32
	s.NewWriter = func(*workspace.Store) (*writing.Writer, error) {
		built.Add(1)
		return &writing.Writer{Store: st}, nil
	}
	s.SetOn(false)
	if _, err := s.Catalog("deepseek"); !errors.Is(err, ErrOff) {
		t.Fatalf("catalog: %v", err)
	}
	if _, err := s.Talk(st, nil, nil, nil); !errors.Is(err, ErrOff) {
		t.Fatalf("talk: %v", err)
	}
	if _, err := s.Pen(st); !errors.Is(err, ErrOff) {
		t.Fatalf("pen: %v", err)
	}
	if n := built.Load(); n != 0 {
		t.Fatalf("built %d with the AI off", n)
	}
	s.SetOn(true)
	if _, err := s.Pen(st); err != nil {
		t.Fatal(err)
	}
	if n := built.Load(); n != 1 {
		t.Fatalf("built %d with the AI on again", n)
	}
}
