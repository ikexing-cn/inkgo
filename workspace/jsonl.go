package workspace

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
)

// readJSONL hands each line of an append-only file to line and returns the
// length of the recoverable prefix. Only a syntactically incomplete,
// unterminated last line, as a crash mid-append leaves, is recoverable:
// complete corrupt lines, unknown fields and unsupported schemas fail with
// the file and line.
func readJSONL(p string, line func(raw []byte) error) (int64, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		return 0, err
	}
	prefix, _, err := scanJSONL(p, data, 0, 0, line)
	return prefix, err
}

func scanJSONL(p string, data []byte, prefix int64, n int, line func(raw []byte) error) (int64, int, error) {
	offset := 0
	for offset < len(data) {
		n++
		end := bytes.IndexByte(data[offset:], '\n')
		terminated := end >= 0
		next := len(data)
		if terminated {
			next = offset + end + 1
		}
		raw := bytes.TrimSpace(data[offset:next])
		err := line(raw)
		if !terminated && (errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF)) {
			return prefix + int64(offset), n - 1, nil
		}
		if err != nil {
			return 0, 0, fmt.Errorf("%s:%d: %w", p, n, err)
		}
		offset = next
	}
	return prefix + int64(len(data)), n, nil
}

type jsonlCursor struct {
	prefix     int64
	lines      int
	info       os.FileInfo
	sum        hash.Hash
	terminated bool
	ids        map[string]bool
}

// cursor validates only the suffix after our last append. An external growth
// also checks the prefix's bytes: size, time and a last-line sentinel alone
// cannot detect an edit earlier in a file that was then appended to.
func (s *Store) cursor(p string, validate func(*jsonlCursor, []byte) error) (*jsonlCursor, error) {
	if s.cursors == nil {
		s.cursors = map[string]*jsonlCursor{}
	}
	f, err := os.Open(p)
	if err != nil {
		delete(s.cursors, p)
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	c := s.cursors[p]
	if c != nil {
		same := os.SameFile(c.info, info)
		unchanged := same && info.Size() == c.info.Size() && info.ModTime().Equal(c.info.ModTime())
		if !unchanged {
			growing := same && info.Size() > c.info.Size() && c.terminated
			if growing {
				sum := sha256.New()
				_, err = io.CopyN(sum, f, c.prefix)
				if err != nil {
					return nil, err
				}
				growing = bytes.Equal(sum.Sum(nil), c.sum.Sum(nil))
			}
			if !growing {
				c = nil
			}
		}
	}
	if c == nil {
		c = &jsonlCursor{sum: sha256.New(), ids: map[string]bool{}}
	}
	if _, err := f.Seek(c.prefix, io.SeekStart); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	prefix, lines, err := scanJSONL(p, data, c.prefix, c.lines, func(raw []byte) error { return validate(c, raw) })
	if err != nil {
		delete(s.cursors, p)
		return nil, err
	}
	count := prefix - c.prefix
	if count > 0 {
		c.sum.Write(data[:count])
		c.terminated = data[count-1] == '\n'
	}
	c.prefix, c.lines, c.info = prefix, lines, info
	s.cursors[p] = c
	return c, nil
}

func (s *Store) appendCursor(p string, c *jsonlCursor, v any) error {
	data, err := writeJSONL(p, c.prefix, v)
	if err != nil {
		delete(s.cursors, p)
		return err
	}
	info, err := os.Stat(p)
	if err != nil {
		delete(s.cursors, p)
		return err
	}
	// Windows Stat loads file identity lazily from the path. Pin it while
	// this is still the file we wrote, before an external replacement.
	if !os.SameFile(info, info) {
		delete(s.cursors, p)
		return nil
	}
	c.sum.Write(data)
	c.prefix += int64(len(data))
	c.info, c.terminated = info, true
	c.lines++
	return nil
}

// appendJSONL writes v as a line after the recoverable prefix, dropping an
// incomplete last line and delimiting a complete one missing its newline.
func appendJSONL(p string, prefix int64, v any) error {
	_, err := writeJSONL(p, prefix, v)
	return err
}

func writeJSONL(p string, prefix int64, v any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	f, err := os.OpenFile(p, os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if st.Size() != prefix {
		if err = f.Truncate(prefix); err != nil {
			return nil, err
		}
	}
	if prefix > 0 {
		var last [1]byte
		if _, err = f.ReadAt(last[:], prefix-1); err != nil {
			return nil, err
		}
		if last[0] != '\n' {
			data = append([]byte{'\n'}, data...)
		}
	}
	if _, err = f.Seek(prefix, io.SeekStart); err != nil {
		return nil, err
	}
	if _, err = f.Write(data); err != nil {
		return nil, err
	}
	if err := f.Sync(); err != nil {
		return nil, err
	}
	return data, nil
}
