// Package talk owns conversations and their model engine.
package talk

import (
	"context"
	"writer/ext/ai/aiapi"
	discussion "writer/ext/talk/engine"
	"writer/ext/talk/talkapi"
	"writer/model"
	"writer/workspace"
)

// talk is a discussion engine as aiapi has it.
type talk struct{ e *discussion.Engine }

func (t talk) Say(ctx context.Context, session string, words workspace.Event) (bool, error) {
	turn, err := t.e.Say(ctx, session, words)
	return turn.User.ID != "", err
}

func (t talk) Retry(ctx context.Context, session, userID string) error {
	_, err := t.e.Retry(ctx, session, userID)
	return err
}

// Build constructs a conversation engine on the frame thread. Callbacks run
// on the request worker; they must not retain frame objects.
func Build(s aiapi.Assistant, st *workspace.Store, material aiapi.Material, tools []aiapi.Tool, beside *talkapi.Beside, progress func(talkapi.Progress), step func(workspace.Event)) (talkapi.Talk, error) {
	m, err := s.Model(st, aiapi.RoleTalk)
	if err != nil {
		return nil, err
	}
	e := &discussion.Engine{Store: st, Talk: m, Step: step, Material: material, Tools: tools}
	if progress != nil {
		e.Progress = func(d model.Delta) { progress(talkapi.Progress{Reasoning: d.Reasoning, Text: d.Text}) }
		e.Proposing = func(call string, p workspace.Proposal) { progress(talkapi.Progress{Call: call, Proposal: &p}) }
	}
	if beside != nil {
		w := discussion.Writing(*beside)
		e.Beside = &w
	}
	return talk{e}, nil
}

// Preloaded describes fixed draft and rewrite context without calling a model.
func Preloaded(st *workspace.Store, material aiapi.Material, origin, chapter string, tools ...aiapi.Tool) (talkapi.Preload, error) {
	n, err := discussion.Preloaded(st, material, origin, chapter, tools...)
	if err != nil {
		return talkapi.Preload{}, err
	}
	out := talkapi.Preload{Tokens: n.Tokens, Cards: n.Cards, Previous: n.Previous, Outlines: talkapi.BookVolumes}
	if !material.Outline {
		out.Outlines = talkapi.NoOutlines
	} else if n.InVolume {
		out.Outlines = talkapi.VolumeChapters
	}
	return out, nil
}

// Extract saves selected conversation text as a reference entry.
func Extract(st *workspace.Store, session, message string, start, end int, category string) (workspace.Entry, error) {
	return (&discussion.Engine{Store: st}).Extract(session, message, start, end, category)
}
