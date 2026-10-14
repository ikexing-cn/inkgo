// Package ai is the part of the writing software that sends: it keeps the
// services of models this machine has, and makes the talk, the bookkeeper
// and the pen of a work with them. It draws nothing; the interface reaches
// it through aiport alone.
package ai

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"writer/desk/aiport"
	"writer/discussion"
	"writer/model"
	"writer/workspace"
	"writer/writing"
)

// Service is the AI of this machine, on the services configured in one
// file. Its methods but those of what it gives run on the interface's
// thread.
type Service struct {
	file string
	cfg  model.Config
	// NewWriter builds the writer of s, as the work and the services are
	// now; tests replace it.
	NewWriter func(s *workspace.Store) (*writing.Writer, error)
	// NewBookkeeper builds the bookkeeper of s, as the work and the
	// services on disk are now: it runs on the worker's goroutine. Tests
	// replace it.
	NewBookkeeper func(s *workspace.Store) (*discussion.Bookkeeper, error)
}

var _ aiport.Service = (*Service)(nil)

// ConfigPath is where this machine configures its services.
func ConfigPath() (string, error) { return model.ConfigPath() }

// New is the AI on the services configured in file, or on none for an
// empty file; Reload reads them.
func New(file string) *Service {
	s := &Service{file: file}
	s.NewWriter = s.writer
	s.NewBookkeeper = s.bookkeeper
	return s
}

func (s *Service) ConfigFile() string { return s.file }

func (s *Service) Providers() []aiport.Provider {
	out := make([]aiport.Provider, len(s.cfg.Providers))
	for i, p := range s.cfg.Providers {
		out[i] = aiport.Provider(p)
	}
	return out
}

func (s *Service) Reload() error {
	cfg, err := model.Load(s.file)
	if err != nil {
		return err
	}
	s.cfg = cfg
	return nil
}

func (s *Service) SaveProvider(p aiport.Provider) error {
	cfg := s.cfg
	cfg.Providers = append([]model.Provider(nil), cfg.Providers...)
	replaced := false
	for i, old := range cfg.Providers {
		if old.ID == p.ID {
			cfg.Providers[i] = model.Provider(p)
			replaced = true
			break
		}
	}
	if !replaced {
		cfg.Providers = append(cfg.Providers, model.Provider(p))
	}
	if err := model.Save(s.file, cfg); err != nil {
		return err
	}
	s.cfg = cfg
	return nil
}

func (s *Service) SetKey(providerID, key string) error { return model.SetKey(providerID, key) }

func (s *Service) Catalog(providerID string) (aiport.Catalog, error) {
	client, err := model.NewClient(s.cfg, providerID, "")
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (s *Service) Efforts() []string {
	out := make([]string, len(model.Efforts))
	for i, e := range model.Efforts {
		out[i] = string(e)
	}
	return out
}

func (s *Service) EffortLabel(effort string) string { return model.Effort(effort).Label() }

// talk is a discussion engine as aiport has it.
type talk struct{ e *discussion.Engine }

func (t talk) Say(ctx context.Context, session string, words workspace.Event) (bool, error) {
	turn, err := t.e.Say(ctx, session, words)
	return turn.User.ID != "", err
}

func (t talk) Retry(ctx context.Context, session, userID string) error {
	_, err := t.e.Retry(ctx, session, userID)
	return err
}

func (s *Service) Talk(st *workspace.Store, beside *aiport.Beside, progress func(reasoning, text string)) (aiport.Talk, error) {
	work := st.Work()
	if work.Discussion.Provider == "" || work.Discussion.Model == "" {
		return nil, errors.New("尚未选择讨论模型：请在“模型设置”中选择")
	}
	client, err := model.NewClient(s.cfg, work.Discussion.Provider, work.Discussion.Model)
	if err != nil {
		return nil, err
	}
	e := &discussion.Engine{Store: st, Talk: discussion.Model{Completer: client, Name: work.Discussion.Provider + "/" + work.Discussion.Model, Effort: model.Effort(work.Discussion.Effort)}}
	if progress != nil {
		e.Progress = func(d model.Delta) { progress(d.Reasoning, d.Text) }
	}
	if beside != nil {
		w := discussion.Writing(*beside)
		e.Beside = &w
	}
	return talk{e}, nil
}

func (s *Service) Extract(st *workspace.Store, session, message string, start, end int, category string) (workspace.Entry, error) {
	return (&discussion.Engine{Store: st}).Extract(session, message, start, end, category)
}

// bookBackoff is the waits between the bookkeeper's attempts at a batch.
var bookBackoff = []time.Duration{10 * time.Second, time.Minute}

// bookkeeper builds the bookkeeper of st from the services on disk: it
// runs on the worker's goroutine, which must not read what the interface's
// thread changes. The bookkeeping model left empty is the discussion's.
func (s *Service) bookkeeper(st *workspace.Store) (*discussion.Bookkeeper, error) {
	work := st.Work()
	sel := work.Bookkeeping
	if sel.Provider == "" {
		sel.Provider, sel.Model = work.Discussion.Provider, work.Discussion.Model
	}
	if sel.Provider == "" || sel.Model == "" {
		return nil, errors.New("尚未选择模型")
	}
	cfg, err := model.Load(s.file)
	if err != nil {
		return nil, err
	}
	client, err := model.NewClient(cfg, sel.Provider, sel.Model)
	if err != nil {
		return nil, err
	}
	return &discussion.Bookkeeper{Store: st, Model: discussion.Model{Completer: client, Name: sel.Provider + "/" + sel.Model, Effort: model.Effort(sel.Effort)}}, nil
}

// bookkeeping is the bookkeeper's worker as aiport has it.
type bookkeeping struct {
	*discussion.Worker
	stop context.CancelFunc
}

func (b bookkeeping) Stop() { b.stop() }

func (s *Service) Bookkeep(st *workspace.Store, changed func()) aiport.Bookkeeping {
	ctx, stop := context.WithCancel(context.Background())
	book := s.NewBookkeeper
	w := discussion.NewWorker(func() (*discussion.Bookkeeper, error) { return book(st) }, bookBackoff, changed)
	w.Log = bookLog(filepath.Join(st.Root(), ".writer", "bookkeeper.log"))
	go w.Run(ctx)
	w.Kick()
	return bookkeeping{w, stop}
}

// bookLog appends the worker's lines to path, the newest last, keeping the
// file short; it is for looking into a setting that does not fill.
func bookLog(path string) func(string) {
	return func(line string) {
		b, _ := os.ReadFile(path)
		lines := strings.Split(strings.TrimSpace(string(b)), "\n")
		lines = append(lines, time.Now().Format("01-02 15:04:05 ")+line)
		if len(lines) > 400 {
			lines = lines[len(lines)-400:]
		}
		_ = os.WriteFile(path, []byte(strings.TrimSpace(strings.Join(lines, "\n"))+"\n"), 0644)
	}
}

func (s *Service) Book(st *workspace.Store) (aiport.Book, error) {
	b, err := s.NewBookkeeper(st)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// writer builds the writer of st with its writing model, or the
// discussion's when the work leaves it empty.
func (s *Service) writer(st *workspace.Store) (*writing.Writer, error) {
	work := st.Work()
	sel := work.Writing
	if sel.Provider == "" {
		sel.Provider, sel.Model = work.Discussion.Provider, work.Discussion.Model
	}
	if sel.Provider == "" || sel.Model == "" {
		return nil, writing.ErrNoModel
	}
	client, err := model.NewClient(s.cfg, sel.Provider, sel.Model)
	if err != nil {
		return nil, err
	}
	return &writing.Writer{Store: st, Model: discussion.Model{Completer: client, Name: sel.Provider + "/" + sel.Model, Effort: model.Effort(sel.Effort)}}, nil
}

// pen is a writer as aiport has it.
type pen struct{ w *writing.Writer }

func (p pen) Name() string   { return p.w.Model.Name }
func (p pen) Effort() string { return string(p.w.Model.Effort) }

func (p pen) Predict(ctx context.Context, scene aiport.Scene) (string, error) {
	return p.w.Predict(ctx, writing.Scene(scene))
}

func (p pen) Rewrite(scene aiport.Scene, selected, instruction string) (aiport.Job, error) {
	job, err := p.w.Rewrite(writing.Scene(scene), selected, instruction)
	return p.job(job), err
}

func (p pen) Draft(scene aiport.Scene, words int) (aiport.Job, error) {
	job, err := p.w.Draft(writing.Scene(scene), words)
	return p.job(job), err
}

func (p pen) job(job writing.Job) aiport.Job {
	return aiport.Job{Record: job.Record, Run: func(ctx context.Context, onText func(string)) (string, error) {
		return p.w.Run(ctx, job, onText)
	}}
}

func (p pen) Summarize(ctx context.Context, chapter string) (workspace.Summary, error) {
	return p.w.Summarize(ctx, chapter)
}

func (p pen) Check(ctx context.Context, chapter string) (workspace.Check, error) {
	return p.w.Check(ctx, chapter)
}

func (s *Service) Pen(st *workspace.Store) (aiport.Pen, error) {
	w, err := s.NewWriter(st)
	if err != nil {
		return nil, err
	}
	return pen{w}, nil
}

func (s *Service) Unsummarized(st *workspace.Store, chapter string) ([]string, int, error) {
	return writing.Unsummarized(st, chapter)
}

func (s *Service) MinSummary() int { return writing.MinSummary }
func (s *Service) MinCheck() int   { return writing.MinCheck }
