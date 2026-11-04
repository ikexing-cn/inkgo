package aiapi

import (
	"sync"
	"writer/talkbook"
	"writer/workspace"
)

// ToolShared lazily reads book documents and setting once per reply.
// Values returned are read-only; writers use Store.Revise for fresh state.
type ToolShared struct {
	store             *workspace.Store
	bookOnce, setOnce sync.Once
	book              talkbook.Book
	bookErr           error
	set               workspace.Setting
	setErr            error
}

// NewToolShared creates the shared reads for one reply.
func NewToolShared(s *workspace.Store) *ToolShared { return &ToolShared{store: s} }

// Book returns documents with their names for the model.
func (s *ToolShared) Book() (talkbook.Book, error) {
	s.bookOnce.Do(func() {
		var t workspace.Tree
		t, s.bookErr = s.store.Chapters()
		if s.bookErr == nil {
			s.book = talkbook.Of(t)
		}
	})
	return s.book, s.bookErr
}

// Setting returns the setting as first read during this reply.
func (s *ToolShared) Setting() (workspace.Setting, error) {
	s.setOnce.Do(func() { s.set, s.setErr = s.store.Setting() })
	return s.set, s.setErr
}

// Named resolves a model's chapter name for a label.
func (s *ToolShared) Named(n string) string {
	if n == "" {
		return ""
	}
	b, err := s.Book()
	if err != nil {
		return n
	}
	return b.Named(n)
}
