package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"
)

// Stamp tells a file's version apart, for noticing it changed outside.
type Stamp struct {
	Size int64
	Mod  time.Time
}

// Chapter is what a node holds.
type Chapter struct {
	Text    string
	Outline string
	Marks   []ChapterMark
	Stamp   Stamp
}

// CleanProse keeps a paragraph to a line without the indent before it:
// the editor indents as it shows the text, and export as asked.
func CleanProse(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimLeftFunc(l, unicode.IsSpace)
	}
	return strings.Join(lines, "\n")
}

// ReadChapter returns what node id holds, finding its marks again by what
// they quote when the text changed outside.
func (s *Store) ReadChapter(id string) (Chapter, error) {
	unlock, err := s.lock()
	if err != nil {
		return Chapter{}, err
	}
	defer unlock()
	t, err := s.tree()
	if err != nil {
		return Chapter{}, err
	}
	n, ok := t.byID[id]
	if !ok {
		return Chapter{}, fmt.Errorf("chapters: unknown node %q", id)
	}
	return s.readChapter(n)
}

func (s *Store) readChapter(n *Node) (Chapter, error) {
	var ch Chapter
	read := func(ext string) (string, error) {
		p, err := s.nodePath(n, ext)
		if err != nil {
			return "", err
		}
		b, err := os.ReadFile(p)
		if errors.Is(err, os.ErrNotExist) && ext != textExt {
			return "", nil
		}
		return string(b), err
	}
	var err error
	if ch.Outline, err = read(outlineExt); err != nil {
		return Chapter{}, err
	}
	ch.Outline = strings.ReplaceAll(ch.Outline, "\r\n", "\n")
	if n.Folder {
		return ch, nil
	}
	if ch.Text, err = read(textExt); err != nil {
		return Chapter{}, err
	}
	ch.Text = CleanProse(ch.Text)
	if ch.Stamp, err = s.stamp(n); err != nil {
		return Chapter{}, err
	}
	marks, err := read(marksExt)
	if err != nil {
		return Chapter{}, err
	}
	if marks != "" {
		if err = json.Unmarshal([]byte(marks), &ch.Marks); err != nil {
			p, _ := s.nodePath(n, marksExt)
			return Chapter{}, fmt.Errorf("%s: %w", p, err)
		}
	}
	text := []rune(ch.Text)
	for i, m := range ch.Marks {
		ch.Marks[i] = anchor(text, m)
	}
	identify(n.ID, ch.Marks)
	return ch, nil
}

func (s *Store) stamp(n *Node) (Stamp, error) {
	p, err := s.nodePath(n, textExt)
	if err != nil {
		return Stamp{}, err
	}
	st, err := os.Stat(p)
	if err != nil {
		return Stamp{}, err
	}
	return Stamp{Size: st.Size(), Mod: st.ModTime()}, nil
}

// ChapterStamp returns the version of document id's text on disk.
func (s *Store) ChapterStamp(id string) (Stamp, error) {
	unlock, err := s.lock()
	if err != nil {
		return Stamp{}, err
	}
	defer unlock()
	t, err := s.tree()
	if err != nil {
		return Stamp{}, err
	}
	n, ok := t.byID[id]
	if !ok || n.Folder {
		return Stamp{}, fmt.Errorf("chapters: no document %q", id)
	}
	return s.stamp(n)
}

// ErrChangedOutside tells that a document's text on disk is not the
// version the caller last had, so nothing was written over it.
var ErrChangedOutside = Error("chapter changed outside the app")

// WriteChapter saves document id's text and marks, the quote of each mark
// taken from text but an orphaned one's kept, and returns the text's new
// stamp.
func (s *Store) WriteChapter(id, text string, marks []ChapterMark) (Stamp, error) {
	return s.writeChapter(id, text, marks, nil)
}

// WriteChapterOver saves as WriteChapter does when the text on disk is
// still the version over, the one the caller read or wrote last. When it
// is another, changed outside, it writes nothing and returns
// ErrChangedOutside: the caller looks at the disk before it goes on.
func (s *Store) WriteChapterOver(id, text string, marks []ChapterMark, over Stamp) (Stamp, error) {
	return s.writeChapter(id, text, marks, &over)
}

func (s *Store) writeChapter(id, text string, marks []ChapterMark, over *Stamp) (Stamp, error) {
	unlock, err := s.lock()
	if err != nil {
		return Stamp{}, err
	}
	defer unlock()
	t, err := s.tree()
	if err != nil {
		return Stamp{}, err
	}
	n, ok := t.byID[id]
	if !ok || n.Folder {
		return Stamp{}, fmt.Errorf("chapters: no document %q", id)
	}
	if over != nil {
		now, err := s.stamp(n)
		if err != nil {
			return Stamp{}, err
		}
		if now.Size != over.Size || !now.Mod.Equal(over.Mod) {
			return Stamp{}, ErrChangedOutside
		}
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	p, err := s.nodePath(n, textExt)
	if err != nil {
		return Stamp{}, err
	}
	if err = atomicWrite(p, []byte(text), false); err != nil {
		return Stamp{}, err
	}
	mp, err := s.nodePath(n, marksExt)
	if err != nil {
		return Stamp{}, err
	}
	if len(marks) == 0 {
		if err = os.Remove(mp); err != nil && !errors.Is(err, os.ErrNotExist) {
			return Stamp{}, err
		}
	} else {
		runes := []rune(text)
		out := make([]ChapterMark, len(marks))
		for i, m := range marks {
			m.Start, m.End = clampIndex(m.Start, len(runes)), clampIndex(m.End, len(runes))
			m.End = max(m.End, m.Start)
			if !m.Orphaned {
				m.Quote = string(runes[m.Start:m.End])
			}
			out[i] = m
		}
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return Stamp{}, err
		}
		if err = atomicWrite(mp, append(b, '\n'), false); err != nil {
			return Stamp{}, err
		}
	}
	return s.stamp(n)
}

// WriteOutline saves the outline of node id; an empty one removes its file.
func (s *Store) WriteOutline(id, outline string) error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	t, err := s.tree()
	if err != nil {
		return err
	}
	n, ok := t.byID[id]
	if !ok {
		return fmt.Errorf("chapters: unknown node %q", id)
	}
	p, err := s.nodePath(n, outlineExt)
	if err != nil {
		return err
	}
	if strings.TrimSpace(outline) == "" {
		if err = os.Remove(p); errors.Is(err, os.ErrNotExist) {
			err = nil
		}
		return err
	}
	return atomicWrite(p, []byte(outline), false)
}
