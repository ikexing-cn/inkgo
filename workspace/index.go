package workspace

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	"modernc.org/sqlite"
)

// Open rebuilds the entire small M1 index. Query helpers re-read canonical files
// before indexing so manual edits and deleted files are visible immediately.
// Writes deliberately do not touch the index: an unavailable cache cannot turn
// a successfully persisted mutation into an apparent failure and unsafe retry.
func (s *Store) openIndex(ideas []Idea) error {
	dir, err := s.path(".writer")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	p, err := s.path(".writer", "index.sqlite")
	if err != nil {
		return err
	}
	rebuild := func() error {
		db, err := sql.Open("sqlite", p)
		if err != nil {
			return err
		}
		db.SetMaxOpenConns(1)
		s.db = db
		if _, err = db.Exec(`DROP TABLE IF EXISTS ideas; CREATE TABLE ideas (id TEXT PRIMARY KEY, status TEXT NOT NULL, category TEXT NOT NULL, source TEXT NOT NULL, text TEXT NOT NULL, created TEXT NOT NULL)`); err == nil {
			err = s.refreshIndex(ideas)
		}
		if err != nil {
			db.Close()
			s.db = nil
		}
		return err
	}
	err = rebuild()
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) && (sqliteErr.Code() == 26 || sqliteErr.Code() == 11) {
		// Only this verified disposable cache is removed after closing its handle.
		// A healthy cache is rebuilt in place, permitting multiple readers on Windows.
		if removeErr := os.Remove(p); removeErr != nil {
			return removeErr
		}
		return rebuild()
	}
	return err
}
func (s *Store) refreshIndex(ideas []Idea) error {
	if s.db == nil {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("DELETE FROM ideas"); err != nil {
		return err
	}
	stmt, err := tx.Prepare("INSERT INTO ideas (id,status,category,source,text,created) VALUES (?,?,?,?,?,?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, i := range ideas {
		if _, err = stmt.Exec(i.ID, string(i.Status), i.Category, i.Source, i.Text, i.Created.UTC().Format("2006-01-02T15:04:05.999999999Z")); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Search returns ideas containing the literal substring (case-insensitive for
// ASCII), in Ideas order. It intentionally makes no semantic-search claim.
func (s *Store) Search(query string) ([]Idea, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.check(); err != nil {
		return nil, err
	}
	if err := s.reloadWork(); err != nil {
		return nil, err
	}
	ideas, err := s.ideas()
	if err != nil {
		return nil, err
	}
	if err = s.refreshIndex(ideas); err != nil {
		return nil, fmt.Errorf("index: %w", err)
	}
	query = strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(query, `\`, `\\`), `%`, `\%`), `_`, `\_`)
	rows, err := s.db.Query(`SELECT id FROM ideas WHERE text LIKE ? ESCAPE '\'`, "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := map[string]bool{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	result := []Idea{}
	for _, i := range ideas {
		if ids[i.ID] {
			result = append(result, i)
		}
	}
	return result, nil
}
func (s *Store) Counts() (map[Status]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.check(); err != nil {
		return nil, err
	}
	if err := s.reloadWork(); err != nil {
		return nil, err
	}
	ideas, err := s.ideas()
	if err != nil {
		return nil, err
	}
	if err = s.refreshIndex(ideas); err != nil {
		return nil, err
	}
	rows, err := s.db.Query("SELECT status,COUNT(*) FROM ideas GROUP BY status")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[Status]int{}
	for rows.Next() {
		var status Status
		var count int
		if err = rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[status] = count
	}
	return counts, rows.Err()
}
