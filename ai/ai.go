// Package ai is the part of the writing software that sends: it keeps the
// services of models this machine has, and makes the talk and the pen of
// a work with them. It draws nothing; the interface reaches
// it through aiport alone.
package ai

import (
	"context"
	"errors"
	"sync/atomic"

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
	off  atomic.Bool
	// NewWriter builds the writer of s, as the work and the services are
	// now; tests replace it.
	NewWriter func(s *workspace.Store) (*writing.Writer, error)
}

var _ aiport.Service = (*Service)(nil)

// ErrOff is what asking for anything that sends gives while the AI is
// off.
var ErrOff = errors.New("AI 已关闭")

// ConfigPath is where this machine configures its services.
func ConfigPath() (string, error) { return model.ConfigPath() }

// New is the AI on the services configured in file, or on none for an
// empty file; Reload reads them.
func New(file string) *Service {
	s := &Service{file: file}
	s.NewWriter = s.writer
	return s
}

func (s *Service) ConfigFile() string { return s.file }

func (s *Service) SetOn(on bool) { s.off.Store(!on) }

func (s *Service) Providers() []aiport.Provider {
	out := make([]aiport.Provider, len(s.cfg.Providers))
	for i, p := range s.cfg.Providers {
		out[i] = aiport.Provider{ID: p.ID, Name: p.Name, BaseURL: p.BaseURL, Protocol: p.Protocol, Env: p.Env, Models: make([]aiport.Model, len(p.Models))}
		for j, m := range p.Models {
			out[i].Models[j] = aiport.Model(m)
		}
	}
	return out
}

// provider is p as package model has it.
func provider(p aiport.Provider) model.Provider {
	out := model.Provider{ID: p.ID, Name: p.Name, BaseURL: p.BaseURL, Protocol: p.Protocol, Env: p.Env, Models: make([]model.Model, len(p.Models))}
	for i, m := range p.Models {
		out.Models[i] = model.Model(m)
	}
	return out
}

func (s *Service) Limits(id string) (aiport.Model, bool) {
	m, ok := model.Lookup(id)
	return aiport.Model(m), ok
}

func (s *Service) Reload() error {
	cfg, err := model.Load(s.file)
	if err != nil {
		return err
	}
	s.cfg = cfg
	return nil
}

func (s *Service) Check(p aiport.Provider) aiport.Problems {
	got := model.Check(provider(p))
	say := func(err error) string {
		if err == nil {
			return ""
		}
		return err.Error()
	}
	return aiport.Problems{ID: say(got.ID), Name: say(got.Name), BaseURL: say(got.BaseURL), Env: say(got.Env), Models: say(got.Models)}
}

func (s *Service) SaveProvider(p aiport.Provider) error {
	cfg := s.cfg
	cfg.Providers = append([]model.Provider(nil), cfg.Providers...)
	replaced := false
	for i, old := range cfg.Providers {
		if old.ID == p.ID {
			cfg.Providers[i] = provider(p)
			replaced = true
			break
		}
	}
	if !replaced {
		cfg.Providers = append(cfg.Providers, provider(p))
	}
	if err := model.Save(s.file, cfg); err != nil {
		return err
	}
	s.cfg = cfg
	return nil
}

func (s *Service) SetKey(providerID, key string) error { return model.SetKey(providerID, key) }

func (s *Service) Catalog(providerID string) (aiport.Catalog, error) {
	if s.off.Load() {
		return nil, ErrOff
	}
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

func (s *Service) Talk(st *workspace.Store, beside *aiport.Beside, progress func(reasoning, text string), step func(workspace.Event)) (aiport.Talk, error) {
	if s.off.Load() {
		return nil, ErrOff
	}
	work := st.Work()
	if work.Discussion.Provider == "" || work.Discussion.Model == "" {
		return nil, errors.New("尚未选择讨论模型：请在设置中「AI」配置里选择")
	}
	m, err := open(s.cfg, work.Discussion, st)
	if err != nil {
		return nil, err
	}
	e := &discussion.Engine{Store: st, Talk: m, Step: step}
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

// open is the model sel names, with its limits as cfg has them; what each
// request to it takes and gives is counted as the work st spent.
func open(cfg model.Config, sel workspace.Selection, st *workspace.Store) (discussion.Model, error) {
	client, err := model.NewClient(cfg, sel.Provider, sel.Model)
	if err != nil {
		return discussion.Model{}, err
	}
	name := sel.Provider + "/" + sel.Model
	return discussion.Model{Completer: counted{client, st, name, sel.Model}, Model: cfg.Model(sel.Provider, sel.Model),
		Name: name, Effort: model.Effort(sel.Effort)}, nil
}

// counted is a model whose requests are counted as a work spent, at the
// prices built in for the model id.
type counted struct {
	model.Completer
	st       *workspace.Store
	name, id string
}

func (c counted) Complete(ctx context.Context, req model.Request, on func(model.Delta)) (model.Reply, error) {
	reply, err := c.Completer.Complete(ctx, req, on)
	if u := reply.Usage; u != (model.Usage{}) {
		cost, currency, _ := model.Cost(c.id, u)
		// What it spent is an estimate for the author to look at: failing
		// to count it does not fail the request.
		_ = c.st.Spend(c.name, workspace.Usage(u), cost, currency)
	}
	return reply, err
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
	m, err := open(s.cfg, sel, st)
	if err != nil {
		return nil, err
	}
	return &writing.Writer{Store: st, Model: m}, nil
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

func (p pen) Check(ctx context.Context, chapter string) (workspace.Check, error) {
	return p.w.Check(ctx, chapter)
}

func (s *Service) Pen(st *workspace.Store) (aiport.Pen, error) {
	if s.off.Load() {
		return nil, ErrOff
	}
	w, err := s.NewWriter(st)
	if err != nil {
		return nil, err
	}
	return pen{w}, nil
}

func (s *Service) MinCheck() int { return writing.MinCheck }
