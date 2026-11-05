package talk

import (
	"writer/ext"
	"writer/ext/talk/talkapi"
)

type suggestionReader struct {
	revision uint64
	items    []ext.Item[talkapi.Suggestions]
	point    *ext.PointReader[talkapi.Suggestions]
}

func (s *suggestionReader) each(fn func(talkapi.Suggestions)) {
	revision := s.point.Revision()
	if s.revision != revision || s.revision == 0 {
		s.items = s.point.Items()
		s.revision = revision
	}
	for _, i := range s.items {
		_ = s.point.Guard(i.Owner, i.ID, func() { fn(i.Value) })
	}
}
func (s *suggestionReader) Begin() { s.mutate("Begin", struct{}{}, talkapi.InvokeBegin) }
func (s *suggestionReader) Follow(f talkapi.ProposalFrame) {
	mutateSuggestion(s, "Follow", f, talkapi.InvokeFollow)
}
func (s *suggestionReader) Finish(id string) { mutateSuggestion(s, "Finish", id, talkapi.InvokeFinish) }
func (s *suggestionReader) Drop()            { s.mutate("Drop", struct{}{}, talkapi.InvokeDrop) }
func (s *suggestionReader) State(id, call string) (ok, writing bool) {
	s.each(func(p talkapi.Suggestions) { ok, writing = p.State(id, call) })
	return
}
func (s *suggestionReader) ShowProposal(id, call string) {
	mutateSuggestion(s, "ShowProposal", talkapi.ProposalAddress{Session: id, Call: call}, talkapi.InvokeShowProposal)
}
func (s *suggestionReader) mutate(op string, in struct{}, get func(talkapi.Suggestions) func(struct{}) struct{}) {
	mutateSuggestion(s, op, in, get)
}
func mutateSuggestion[In any](s *suggestionReader, op string, in In, get func(talkapi.Suggestions) func(In) struct{}) {
	revision := s.point.Revision()
	if s.revision != revision || s.revision == 0 {
		s.items = s.point.Items()
		s.revision = revision
	}
	for _, i := range s.items {
		_, _ = ext.CallReadOperation(s.point, op, i.ID, in, get(i.Value))
	}
}
