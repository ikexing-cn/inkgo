// Package atomicfile replaces whole files through a synced temporary file
// in the destination directory.
package atomicfile

import (
	"crypto/rand"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Error identifies the step so callers can keep their own error messages.
// Op is create, write (including sync and close), or rename.
type Error struct {
	Op  string
	Err error
}

func (e *Error) Error() string { return e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

type File struct {
	file *os.File
	path string
}

// Create starts a streamed replacement in dir; Commit chooses its final name.
func Create(dir string, perm fs.FileMode) (*File, error) {
	for {
		path := filepath.Join(dir, ".tmp-"+rand.Text())
		// OpenFile, unlike chmod after CreateTemp, applies the process umask.
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, perm)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return nil, &Error{Op: "create", Err: err}
		}
		return &File{file: f, path: path}, nil
	}
}

func (f *File) Write(p []byte) (int, error) { return f.file.Write(p) }

// Commit syncs and closes the stream before replacing path. The destination
// must be on the same filesystem as the directory passed to Create.
func (f *File) Commit(path string) error {
	defer f.Discard()
	if err := f.file.Sync(); err != nil {
		return &Error{Op: "write", Err: err}
	}
	if err := f.file.Close(); err != nil {
		return &Error{Op: "write", Err: err}
	}
	if err := os.Rename(f.path, path); err != nil {
		return &Error{Op: "rename", Err: err}
	}
	f.path = ""
	return nil
}

func (f *File) Discard() {
	_ = f.file.Close()
	if f.path != "" {
		_ = os.Remove(f.path)
	}
}

func Write(path string, data []byte, perm fs.FileMode) error {
	f, err := Create(filepath.Dir(path), perm)
	if err != nil {
		return err
	}
	defer f.Discard()
	if _, err := f.Write(data); err != nil {
		return &Error{Op: "write", Err: err}
	}
	return f.Commit(path)
}
