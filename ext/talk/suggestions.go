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
func (s *suggestionReader) Begin() { s.each(func(p aiapi.Suggestions) { p.Begin() }) }
func (s *suggestionReader) Follow(f aiapi.ProposalFrame) {
	s.each(func(p aiapi.Suggestions) { p.Follow(f) })
}
func (s *suggestionReader) Finish(id string) { s.each(func(p aiapi.Suggestions) { p.Finish(id) }) }
func (s *suggestionReader) Drop()            { s.each(func(p aiapi.Suggestions) { p.Drop() }) }
func (s *suggestionReader) State(id, call string) (ok, writing bool) {
	s.each(func(p aiapi.Suggestions) { ok, writing = p.State(id, call) })
	return
}
func (s *suggestionReader) ShowProposal(id, call string) {
	s.each(func(p aiapi.Suggestions) { p.ShowProposal(id, call) })
}
