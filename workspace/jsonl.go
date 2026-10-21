package workspace

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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
	offset, n := 0, 0
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
			return int64(offset), nil
		}
		if err != nil {
			return 0, fmt.Errorf("%s:%d: %w", p, n, err)
		}
		offset = next
	}
	return int64(len(data)), nil
}

// appendJSONL writes v as a line after the recoverable prefix, dropping an
// incomplete last line and delimiting a complete one missing its newline.
func appendJSONL(p string, prefix int64, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	f, err := os.OpenFile(p, os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if st.Size() != prefix {
		if err = f.Truncate(prefix); err != nil {
			return err
		}
	}
	if prefix > 0 {
		var last [1]byte
		if _, err = f.ReadAt(last[:], prefix-1); err != nil {
			return err
		}
		if last[0] != '\n' {
			data = append([]byte{'\n'}, data...)
		}
	}
	if _, err = f.Seek(prefix, io.SeekStart); err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		return err
	}
	return f.Sync()
}
