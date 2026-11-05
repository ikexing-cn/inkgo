package talk

import (
	"writer/ext"
	"writer/ext/ai/aiapi"
)

type suggestionReader struct {
	revision uint64
	items    []ext.Item[aiapi.Suggestions]
	point    *ext.PointReader[aiapi.Suggestions]
}

func (s *suggestionReader) each(fn func(aiapi.Suggestions)) {
	revision := s.point.Revision()
	if s.revision != revision || s.revision == 0 {
		s.items = s.point.Items()
		s.revision = revision
	}
	for _, i := range s.items {
		_ = s.point.Guard(i.Owner, i.ID, func() { fn(i.Value) })
	}
}
func (s *suggestionReader) Begin() { s.mutate("Begin", struct{}{}, aiapi.InvokeBegin) }
func (s *suggestionReader) Follow(f aiapi.ProposalFrame) {
	mutateSuggestion(s, "Follow", f, aiapi.InvokeFollow)
}
func (s *suggestionReader) Finish(id string) { mutateSuggestion(s, "Finish", id, aiapi.InvokeFinish) }
func (s *suggestionReader) Drop()            { s.mutate("Drop", struct{}{}, aiapi.InvokeDrop) }
func (s *suggestionReader) State(id, call string) (ok, writing bool) {
	s.each(func(p aiapi.Suggestions) { ok, writing = p.State(id, call) })
	return
}
func (s *suggestionReader) ShowProposal(id, call string) {
	mutateSuggestion(s, "ShowProposal", aiapi.ProposalAddress{Session: id, Call: call}, aiapi.InvokeShowProposal)
}
func (s *suggestionReader) mutate(op string, in struct{}, get func(aiapi.Suggestions) func(struct{}) struct{}) {
	mutateSuggestion(s, op, in, get)
}
func mutateSuggestion[In any](s *suggestionReader, op string, in In, get func(aiapi.Suggestions) func(In) struct{}) {
	revision := s.point.Revision()
	if s.revision != revision || s.revision == 0 {
		s.items = s.point.Items()
		s.revision = revision
	}
	for _, i := range s.items {
		_, _ = ext.CallReadOperation(s.point, op, i.ID, in, get(i.Value))
	}
}
