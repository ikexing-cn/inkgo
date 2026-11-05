package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// indexedHook keeps the interleaving with other document annotations intact.
type indexedHook struct {
	Index int         `json:"index"`
	Mark  ChapterMark `json:"mark"`
}
type hookFile struct {
	Version int           `json:"version"`
	Marks   []indexedHook `json:"marks"`
}

func (s *Store) hooksPath(id string) (string, error) {
	if err := validID(id); err != nil {
		return "", err
	}
	return s.extensionPath("hooks", id+".json")
}
func (s *Store) readMarks(n *Node) ([]ChapterMark, error) {
	primary, err := s.nodePath(n, marksExt)
	if err != nil {
		return nil, err
	}
	hooks, err := s.hooksPath(n.ID)
	if err != nil {
		return nil, err
	}
	var core []ChapterMark
	b, err := os.ReadFile(primary)
	if err == nil {
		if err = json.Unmarshal(b, &core); err != nil {
			return nil, fmt.Errorf("%s: %w", primary, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	// Foreshadowing is read only from its extension directory; old books are not migrated.
	filtered := core[:0]
	for _, m := range core {
		if m.Kind != MarkForeshadow && m.Kind != MarkPayoff {
			filtered = append(filtered, m)
		}
	}
	core = filtered
	var file hookFile
	b, err = os.ReadFile(hooks)
	if err == nil {
		if err = json.Unmarshal(b, &file); err != nil {
			return nil, fmt.Errorf("%s: %w", hooks, err)
		}
		if file.Version != 1 {
			return nil, fmt.Errorf("%s: unsupported hooks version %d", hooks, file.Version)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if len(file.Marks) == 0 {
		return core, nil
	}
	out := make([]ChapterMark, 0, len(core)+len(file.Marks))
	next := 0
	for _, v := range file.Marks {
		if v.Index < len(out) || v.Index > len(core)+len(file.Marks)-1 {
			return nil, fmt.Errorf("%s: invalid hook index %d", hooks, v.Index)
		}
		for len(out) < v.Index {
			if next >= len(core) {
				return nil, fmt.Errorf("%s: invalid hook index %d", hooks, v.Index)
			}
			out = append(out, core[next])
			next++
		}
		out = append(out, v.Mark)
	}
	return append(out, core[next:]...), nil
}
func (s *Store) writeMarks(n *Node, marks []ChapterMark) error {
	primary, err := s.nodePath(n, marksExt)
	if err != nil {
		return err
	}
	hooks, err := s.hooksPath(n.ID)
	if err != nil {
		return err
	}
	var core []ChapterMark
	file := hookFile{Version: 1}
	for i, m := range marks {
		if m.Kind == MarkForeshadow || m.Kind == MarkPayoff {
			file.Marks = append(file.Marks, indexedHook{i, m})
		} else {
			core = append(core, m)
		}
	}
	save := func(path string, value any, empty bool) error {
		if empty {
			err := os.Remove(path)
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		b, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		return atomicWrite(path, append(b, '\n'), false)
	}
	if len(file.Marks) > 0 {
		if err = s.prepareExtension("hooks"); err != nil {
			return err
		}
	}
	if err = save(hooks, file, len(file.Marks) == 0); err != nil {
		return err
	}
	return save(primary, core, len(core) == 0)
}
