// Package backup copies a work, all of it but .writer/, into one zip file
// in a folder of the author's choosing, keeps the latest few, and unpacks
// one again to compare or restore.
package backup

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DefaultKeep is how many backups are kept when the author set nothing.
const DefaultKeep = 30

// stampLayout is the time in a backup's file name: sortable, and without
// the colons Windows refuses.
const stampLayout = "2006-01-02 15.04.05"

// Settings are how one work is backed up on this machine.
type Settings struct {
	// Dir is the folder the backups go to; empty until the author picks one.
	Dir string `json:"dir,omitempty"`
	// When: as the work closes, once a day it was written in, and every
	// hour of writing.
	OnClose bool `json:"on_close,omitempty"`
	Daily   bool `json:"daily,omitempty"`
	Hourly  bool `json:"hourly,omitempty"`
	// Keep is how many backups to keep; 0 is DefaultKeep.
	Keep int `json:"keep,omitempty"`
	// Last is when the last backup succeeded.
	Last time.Time `json:"last,omitzero"`
	// Later is when the author put off choosing a folder; Hush, the day
	// a failed backup was not to be shown again.
	Later time.Time `json:"later,omitzero"`
	Hush  time.Time `json:"hush,omitzero"`
}

// Kept is how many backups the settings keep.
func (s Settings) Kept() int {
	if s.Keep > 0 {
		return s.Keep
	}
	return DefaultKeep
}

// Meta is what a backup says of the work it holds, in the zip's comment.
type Meta struct {
	Work     string `json:"work"` // the work's ID
	Sum      string `json:"sum"`  // of the files, to skip a backup like the last
	Words    int    `json:"words"`
	Chapters int    `json:"chapters"`
}

// Info is one backup found in the folder.
type Info struct {
	Path string
	At   time.Time
	Size int64
	Meta
}

// File is a file of the work, as a backup holds it.
type File struct {
	Rel  string // its path under the work, with slashes
	Data []byte
	Mode fs.FileMode
	Mod  time.Time
}

// Files is the work as it was read, to be written out as a backup.
type Files struct {
	List []File
	Sum  string // of the paths and contents, to skip a backup like the last
}

// Read reads the work's files under root, all but .writer/, in order. It
// is quick, so it can run while the work is held still; writing the zip,
// to a slow disk perhaps, comes after.
func Read(root string) (Files, error) {
	var out Files
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel == ".writer" {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		st, err := d.Info()
		if err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out.List = append(out.List, File{Rel: rel, Data: b, Mode: st.Mode(), Mod: st.ModTime()})
		return nil
	})
	if err != nil {
		return Files{}, err
	}
	sort.Slice(out.List, func(i, j int) bool { return out.List[i].Rel < out.List[j].Rel })
	h := sha256.New()
	for _, f := range out.List {
		fmt.Fprintf(h, "%q %d\n", f.Rel, len(f.Data))
		h.Write(f.Data)
	}
	out.Sum = hex.EncodeToString(h.Sum(nil))
	return out, nil
}

// base is the start of the file names of the backups of the work named
// name: the name, with what file names cannot hold replaced.
func base(name string) string {
	r := strings.NewReplacer(`\`, "_", "/", "_", ":", "_", "*", "_", "?", "_", `"`, "_", "<", "_", ">", "_", "|", "_")
	name = strings.TrimSpace(r.Replace(name))
	if name == "" {
		name = "作品"
	}
	return name + " "
}

// Make writes fl into dir as a backup of the work named name, unless the
// latest backup there holds the same files; made says whether it wrote
// one. It then removes the oldest beyond keep.
func Make(fl Files, dir, name string, meta Meta, keep int, now time.Time) (info Info, made bool, err error) {
	meta.Sum = fl.Sum
	if _, err = os.Stat(dir); err != nil {
		return Info{}, false, err // a disk unplugged is not a folder to make
	}
	list, err := List(dir, meta.Work)
	if err != nil {
		return Info{}, false, err
	}
	if len(list) > 0 && list[0].Sum == meta.Sum {
		return list[0], false, nil
	}
	tmp, err := os.CreateTemp(dir, ".backup-*.tmp")
	if err != nil {
		return Info{}, false, err
	}
	defer os.Remove(tmp.Name())
	if err = write(tmp, fl, meta); err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return Info{}, false, err
	}
	p := filepath.Join(dir, base(name)+now.Format(stampLayout)+".zip")
	for i := 2; exists(p); i++ {
		p = filepath.Join(dir, fmt.Sprintf("%s%s (%d).zip", base(name), now.Format(stampLayout), i))
	}
	if err = os.Rename(tmp.Name(), p); err != nil {
		return Info{}, false, err
	}
	st, err := os.Stat(p)
	if err != nil {
		return Info{}, false, err
	}
	info = Info{Path: p, At: now, Size: st.Size(), Meta: meta}
	return info, true, Prune(dir, meta.Work, keep)
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func write(w io.Writer, fl Files, meta Meta) error {
	z := zip.NewWriter(w)
	for _, f := range fl.List {
		h := &zip.FileHeader{Name: f.Rel, Method: zip.Deflate, Modified: f.Mod}
		h.SetMode(f.Mode)
		out, err := z.CreateHeader(h)
		if err != nil {
			return err
		}
		if _, err = out.Write(f.Data); err != nil {
			return err
		}
	}
	c, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	if err = z.SetComment(string(c)); err != nil {
		return err
	}
	return z.Close()
}

// List returns the backups of work in dir, the latest first, by the ID
// each holds, so a work renamed still finds its own. Zips there that are
// not this work's backups are left out.
func List(dir, work string) ([]Info, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var list []Info
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".zip") {
			continue
		}
		stamp := strings.TrimSuffix(n, ".zip")
		if i := strings.LastIndex(stamp, " ("); i >= 0 && strings.HasSuffix(stamp, ")") {
			stamp = stamp[:i]
		}
		if len(stamp) < len(stampLayout) {
			continue
		}
		at, err := time.ParseInLocation(stampLayout, stamp[len(stamp)-len(stampLayout):], time.Local)
		if err != nil {
			continue
		}
		p := filepath.Join(dir, n)
		meta, size, err := readMeta(p)
		if err != nil || meta.Work != work {
			continue
		}
		list = append(list, Info{Path: p, At: at, Size: size, Meta: meta})
	}
	sort.SliceStable(list, func(i, j int) bool {
		if !list[i].At.Equal(list[j].At) {
			return list[i].At.After(list[j].At)
		}
		return list[i].Path > list[j].Path
	})
	return list, nil
}

func readMeta(p string) (Meta, int64, error) {
	z, err := zip.OpenReader(p)
	if err != nil {
		return Meta{}, 0, err
	}
	defer z.Close()
	var m Meta
	if err = json.Unmarshal([]byte(z.Comment), &m); err != nil {
		return Meta{}, 0, err
	}
	st, err := os.Stat(p)
	if err != nil {
		return Meta{}, 0, err
	}
	return m, st.Size(), nil
}

// Prune removes the backups of work in dir beyond the latest keep.
func Prune(dir, work string, keep int) error {
	if keep <= 0 {
		keep = DefaultKeep
	}
	list, err := List(dir, work)
	if err != nil {
		return err
	}
	for _, b := range list[min(keep, len(list)):] {
		if err := os.Remove(b.Path); err != nil {
			return err
		}
	}
	return nil
}

// Unpack writes the files of the backup at p into the empty folder into,
// with an empty .writer/ beside them, so it opens as a work.
func Unpack(p, into string) error {
	z, err := zip.OpenReader(p)
	if err != nil {
		return err
	}
	defer z.Close()
	if err = os.MkdirAll(filepath.Join(into, ".writer"), 0755); err != nil {
		return err
	}
	for _, f := range z.File {
		// A name is taken as this system reads it: one that is a path of
		// its own here, as with the other system's separator or a drive
		// before it, would be written outside into, or into .writer/.
		local := filepath.FromSlash(f.Name)
		first, _, _ := strings.Cut(local, string(filepath.Separator))
		if !fs.ValidPath(f.Name) || !filepath.IsLocal(local) || strings.EqualFold(first, ".writer") || strings.HasSuffix(f.Name, "/") {
			return fmt.Errorf("%s: unexpected entry %q", p, f.Name)
		}
		dst := filepath.Join(into, local)
		if err = os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}
		in, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(dst)
		if err == nil {
			_, err = io.Copy(out, in)
			if cerr := out.Close(); err == nil {
				err = cerr
			}
		}
		in.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
