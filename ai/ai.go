// Package ai is the part of the writing software that sends: it keeps the
// services of models this machine has, and makes the talk and the pen of
// a work with them. It draws nothing; the interface reaches
// it through aiport alone.
package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"writer/aiport"
	"writer/discussion"
	"writer/inference"
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
	on   func() bool
	// Trace, when set, is a folder that gets the whole of each request
	// and its reply, a file for each; it is for tuning the prompts, and
	// no work is written to.
	Trace string
	// ErrorText words a failed request for the AI record, which keeps it
	// as the author saw it; unset, the error's own text.
	ErrorText func(error) string
	// Open is the model sel names for the work st, as the services are
	// now: each request to it goes into the AI record of st. Tests replace
	// it.
	Open func(sel workspace.Selection, st *workspace.Store) (inference.Model, error)
}

var (
	_ aiport.Settings  = (*Service)(nil)
	_ aiport.Assistant = (*Service)(nil)
)

// ConfigPath is where this machine configures its services.
func ConfigPath() (string, error) { return model.ConfigPath() }

// New is the AI on the services configured in file, or on none for an
// empty file; Reload reads them.
func New(file string) *Service {
	s := &Service{file: file}
	s.Open = s.open
	return s
}

func (s *Service) ConfigFile() string { return s.file }

// BindSwitch supplies the machine's switch, read on the interface's
// thread while Catalog, Talk or Pen builds. Unset or nil means on for
// standalone use; desk.New binds it for every desk. Background requests
// never read it.
func (s *Service) BindSwitch(on func() bool) { s.on = on }

func (s *Service) Providers() []aiport.Provider {
	out := make([]aiport.Provider, len(s.cfg.Providers))
	for i, p := range s.cfg.Providers {
		out[i] = aiport.Provider{ID: p.ID, Name: p.Name, BaseURL: p.BaseURL, Protocol: string(p.Protocol), Env: p.Env, Models: make([]aiport.Model, len(p.Models))}
		for j, m := range p.Models {
			out[i].Models[j] = aiport.Model(m)
		}
	}
	return out
}

// provider is p as package model has it.
func provider(p aiport.Provider) model.Provider {
	out := model.Provider{ID: p.ID, Name: p.Name, BaseURL: p.BaseURL, Protocol: model.Protocol(p.Protocol), Env: p.Env, Models: make([]model.Model, len(p.Models))}
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
		return setupError(err)
	}
	s.cfg = cfg
	return nil
}

func (s *Service) Check(p aiport.Provider) aiport.Problems {
	got := model.Check(provider(p))
	return aiport.Problems{ID: got.ID, Name: got.Name, Protocol: got.Protocol, BaseURL: got.BaseURL, Env: got.Env, Models: got.Models}
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
		return setupError(err)
	}
	s.cfg = cfg
	return nil
}

func (s *Service) SetKey(providerID, key string) error {
	return setupError(model.SetKey(providerID, key))
}

func (s *Service) Catalog(providerID string) (aiport.Catalog, error) {
	if s.on != nil && !s.on() {
		return nil, ErrOff
	}
	client, err := model.NewClient(s.cfg, providerID, "")
	if err != nil {
		return nil, setupError(err)
	}
	return catalog{client}, nil
}

func setupError(err error) error {
	if err == nil {
		return nil
	}
	return &aiport.SetupError{Err: err}
}

func requestError(err error) error {
	var port *aiport.SetupError
	if errors.As(err, &port) {
		return err
	}
	var setup *model.SetupError
	if errors.As(err, &setup) {
		return setupError(err)
	}
	return err
}

type catalog struct{ aiport.Catalog }

func (c catalog) ListModels(ctx context.Context) ([]string, error) {
	ids, err := c.Catalog.ListModels(ctx)
	return ids, requestError(err)
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
	return turn.User.ID != "", requestError(err)
}

func (t talk) Retry(ctx context.Context, session, userID string) error {
	_, err := t.e.Retry(ctx, session, userID)
	return requestError(err)
}

func (s *Service) Talk(st *workspace.Store, beside *aiport.Beside, progress func(aiport.Progress), step func(workspace.Event)) (aiport.Talk, error) {
	if s.on != nil && !s.on() {
		return nil, ErrOff
	}
	work := st.Work()
	if work.Discussion.Provider == "" || work.Discussion.Model == "" {
		return nil, setupError(ErrChatModel)
	}
	m, err := s.Open(work.Discussion, st)
	if err != nil {
		return nil, err
	}
	e := &discussion.Engine{Store: st, Talk: m, Step: step}
	if progress != nil {
		e.Progress = func(d model.Delta) { progress(aiport.Progress{Reasoning: d.Reasoning, Text: d.Text}) }
		e.Proposing = func(call string, p workspace.Proposal) { progress(aiport.Progress{Call: call, Proposal: &p}) }
	}
	if beside != nil {
		w := discussion.Writing(*beside)
		e.Beside = &w
	}
	return talk{e}, nil
}

func (s *Service) Preloaded(st *workspace.Store, origin, chapter string) (aiport.Preload, error) {
	n, err := discussion.Preloaded(st, origin, chapter)
	if err != nil {
		return aiport.Preload{}, requestError(err)
	}
	out := aiport.Preload{Tokens: n.Tokens, Cards: n.Cards, Previous: n.Previous, Outlines: aiport.BookVolumes}
	if n.InVolume {
		out.Outlines = aiport.VolumeChapters
	}
	return out, nil
}

func (s *Service) Extract(st *workspace.Store, session, message string, start, end int, category string) (workspace.Entry, error) {
	return (&discussion.Engine{Store: st}).Extract(session, message, start, end, category)
}

// open is the model sel names, with its limits as the services have
// them; each request to it is a line of the AI record of the work st.
func (s *Service) open(sel workspace.Selection, st *workspace.Store) (inference.Model, error) {
	client, err := model.NewClient(s.cfg, sel.Provider, sel.Model)
	if err != nil {
		return inference.Model{}, setupError(err)
	}
	name := (workspace.ModelRef{Provider: sel.Provider, Model: sel.Model}).String()
	return inference.Model{Completer: counted{client, st, name, sel.Model, s.Trace, s.ErrorText}, Model: s.cfg.Model(sel.Provider, sel.Model),
		Name: name, Effort: model.Effort(sel.Effort)}, nil
}

// counted is a model whose every request goes into the AI record of a
// work, for the purpose its context carries and at the prices built in
// for the model id; and, with a trace folder, whole into a file there.
type counted struct {
	model.Completer
	st       *workspace.Store
	name, id string
	trace    string
	words    func(error) string
}

func (c counted) Complete(ctx context.Context, req model.Request, on func(model.Delta)) (model.Reply, error) {
	start := time.Now()
	reply, err := c.Completer.Complete(ctx, req, on)
	p := workspace.PurposeOf(ctx)
	r := workspace.Record{At: start.UTC(), Took: time.Since(start).Milliseconds(), Feature: p.Feature, Chapter: p.Chapter, Talk: p.Talk,
		Model: c.name, Effort: string(req.Effort), Parts: p.Parts, Usage: workspace.Usage(reply.Usage), Outcome: workspace.OutcomeDone}
	r.Cost, r.Currency, _ = model.Cost(c.id, reply.Usage)
	switch {
	case err == nil || errors.Is(context.Cause(ctx), workspace.ErrEnough):
	case ctx.Err() != nil:
		r.Outcome = workspace.OutcomeCancelled
	default:
		r.Outcome, r.Error = workspace.OutcomeFailed, err.Error()
		if c.words != nil {
			r.Error = c.words(err)
		}
	}
	// The record is for the author to look at: failing to keep it does
	// not fail the request.
	_ = c.st.Log(r)
	if c.trace != "" {
		_ = traced(c.trace, r, req, reply, err)
	}
	return reply, requestError(err)
}

// traced writes request req, its reply and the line r of the record into
// a file of the folder dir, named by its time and feature.
func traced(dir string, r workspace.Record, req model.Request, reply model.Reply, err error) error {
	whole := struct {
		Record  workspace.Record
		Request model.Request
		Reply   model.Reply
		Error   string `json:",omitempty"`
	}{Record: r, Request: req, Reply: reply}
	if err != nil {
		whole.Error = err.Error()
	}
	b, err := json.MarshalIndent(whole, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := r.At.Local().Format("20060102-150405.000") + "-" + r.Feature
	for n := 1; ; n++ {
		p := filepath.Join(dir, name+".json")
		if n > 1 {
			p = filepath.Join(dir, fmt.Sprintf("%s-%d.json", name, n))
		}
		f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return err
		}
		_, err = f.Write(b)
		return errors.Join(err, f.Close())
	}
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
		return nil, setupError(writing.ErrNoModel)
	}
	m, err := s.Open(sel, st)
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
	text, err := p.w.Predict(ctx, writing.Scene(scene))
	return text, requestError(err)
}

func (p pen) Check(ctx context.Context, chapter string) (workspace.Check, error) {
	check, err := p.w.Check(ctx, chapter)
	return check, requestError(err)
}

func (s *Service) Pen(st *workspace.Store) (aiport.Pen, error) {
	if s.on != nil && !s.on() {
		return nil, ErrOff
	}
	w, err := s.writer(st)
	if err != nil {
		return nil, err
	}
	return pen{w}, nil
}

func (s *Service) MinCheck() int { return writing.MinCheck }

func (s *Service) Cost(name string, u workspace.Usage) (float64, string, bool) {
	ref, _ := workspace.ParseModelRef(name)
	return model.Cost(model.PriceID(ref.Model), model.Usage(u))
}
