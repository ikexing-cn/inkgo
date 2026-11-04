package ext

import (
	"context"
	"path/filepath"
	"sync"
)

// Session owns tasks and immutable paths for one activation in one book.
type Session struct {
	id        string
	book      Book
	localRoot string
	bookDir   func(string, string) (string, error)
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.Mutex
	stopped   bool
	wg        sync.WaitGroup
	onPanic   func(any)
}

func newSession(id string, book Book, local string) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	return &Session{id: id, book: book, localRoot: local, ctx: ctx, cancel: cancel}
}

// Book returns the immutable book identity on any goroutine.
func (s *Session) Book() Book { return s.book }

// ID returns the owning module ID.
func (s *Session) ID() string { return s.id }

// BookDir returns the book data directory without creating it.
func (s *Session) BookDir() (string, bool) {
	if s.book.Dir == "" || s.bookDir == nil {
		return "", false
	}
	dir, err := s.bookDir(s.book.Dir, s.id)
	return dir, err == nil
}

// LocalDir returns the local data directory without creating it.
func (s *Session) LocalDir() string { return filepath.Join(s.localRoot, "ext", s.id) }

// Go starts a task owned by this session. Calls after stopping are ignored.
// Tasks must return when ctx is canceled; shutdown waits for all of them.
func (s *Session) Go(f Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			if v := recover(); v != nil {
				if s.onPanic != nil {
					s.onPanic(v)
				} else {
					panic(v)
				}
			}
		}()
		f(s.ctx)
	}()
}
func (s *Session) stop() {
	s.mu.Lock()
	s.stopped = true
	s.cancel()
	s.mu.Unlock()
	s.wg.Wait()
}
