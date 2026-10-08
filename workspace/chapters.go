package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"
)

// The prose lives under chapters/ as the chapter tree shows it: a folder is
// a directory, a document 标题.txt, and the outline of either 标题.outline.txt
// beside it. A document's marks are 标题.marks.json. Each directory's .order
// lists its nodes in order with their IDs and titles, which file names
// cannot hold; files that appear without an entry, as ones copied in by
// hand, become nodes of their own.
const (
	chaptersDir = "chapters"
	trashDir    = "trash"
	orderFile   = ".order"
	trashedFile = ".trashed.json"
	textExt     = ".txt"
	outlineExt  = ".outline.txt"
	marksExt    = ".marks.json"
)

// Node is a node of the chapter tree: a folder of nodes, or a document of
// prose. Either may have an outline.
type Node struct {
	ID       string
	Title    string
	Folder   bool
	Children []*Node
	Parent   *Node // nil for the root
	file     string
	dir      []string // the directory holding it, under chapters/
}

// Tree is the chapter tree as the disk had it when it was read.
type Tree struct {
	Root *Node
	byID map[string]*Node
}

// Node returns the node id.
func (t Tree) Node(id string) (*Node, bool) {
	n, ok := t.byID[id]
	return n, ok
}

// Walk calls fn with each node in reading order, folders before what they
// hold, and its depth from 0.
func (t Tree) Walk(fn func(n *Node, depth int)) {
	var walk func(ns []*Node, depth int)
	walk = func(ns []*Node, depth int) {
		for _, n := range ns {
			fn(n, depth)
			walk(n.Children, depth+1)
		}
	}
	walk(t.Root.Children, 0)
}

// Within reports whether n is ancestor or inside it.
func (n *Node) Within(ancestor *Node) bool {
	for p := n; p != nil; p = p.Parent {
		if p == ancestor {
			return true
		}
	}
	return false
}

// orderEntry is a node as its directory's .order records it.
type orderEntry struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	File   string `json:"file"`
	Folder bool   `json:"folder,omitempty"`
}

// Kinds of a chapter mark.
const (
	MarkForeshadow = "foreshadow"  // a foreshadowing the author marked
	MarkAIAccepted = "ai-accepted" // AI text the author accepted
	MarkAIDraft    = "ai-draft"    // AI text drafted from the outline
)

// ChapterMark is a mark of a document's text, in runes [Start, End). Quote
// is the text it covered when saved, which finds it again after the file
// changed outside; one that cannot be found is Orphaned.
type ChapterMark struct {
	Kind     string `json:"kind"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
	Quote    string `json:"quote"`
	Note     string `json:"note,omitempty"`
	Orphaned bool   `json:"orphaned,omitempty"`
}

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

// Trashed is a node in the trash with what it held.
type Trashed struct {
	ID     string    `json:"id"`
	Title  string    `json:"title"`
	Folder bool      `json:"folder,omitempty"`
	File   string    `json:"file"`
	Parent string    `json:"parent,omitempty"`
	At     time.Time `json:"at"`
}

// fileName turns title into a name its directory can hold beside the names
// taken: characters file systems refuse become their full-width forms, and
// a name clashing with another, letter case aside, is numbered.
func fileName(title string, taken func(string) bool) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(title) {
		switch {
		case r < 0x20 || r == 0x7f:
		case strings.ContainsRune(`<>:"/\|?*`, r):
			b.WriteRune(map[rune]rune{'<': '＜', '>': '＞', ':': '：', '"': '＂', '/': '／', '\\': '＼', '|': '｜', '?': '？', '*': '＊'}[r])
		default:
			b.WriteRune(r)
		}
	}
	name := strings.TrimRight(b.String(), ". ")
	if r := []rune(name); len(r) > 80 {
		name = strings.TrimRight(string(r[:80]), ". ")
	}
	if strings.HasPrefix(name, ".") {
		name = "．" + name[1:] // hidden files and .order are not nodes
	}
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".outline") || strings.HasSuffix(lower, ".marks") {
		name += "_"
	}
	stem := strings.ToUpper(name)
	if i := strings.IndexByte(stem, '.'); i >= 0 {
		stem = stem[:i]
	}
	switch stem {
	case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		name += "_"
	}
	if name == "" {
		name = "未命名"
	}
	out := name
	for n := 2; taken(out); n++ {
		out = fmt.Sprintf("%s (%d)", name, n)
	}
	return out
}

// takenIn returns whether a name clashes with a node of dir other than
// except, letter case aside, as on Windows and macOS.
func takenIn(dir *Node, except *Node) func(string) bool {
	return func(name string) bool {
		for _, n := range dir.Children {
			if n != except && strings.EqualFold(n.file, name) {
				return true
			}
		}
		return false
	}
}

// diskPath returns the path of parts under chapters/.
func (s *Store) diskPath(parts ...string) (string, error) {
	return s.path(append([]string{chaptersDir}, parts...)...)
}

// nodePath returns the path of n's file with ext; a folder's directory
// with ext "".
func (s *Store) nodePath(n *Node, ext string) (string, error) {
	return s.diskPath(append(slices.Clone(n.dir), n.file+ext)...)
}

// childDir is the directory, under chapters/, of what folder n holds.
func childDir(n *Node) []string {
	if n.Parent == nil {
		return nil
	}
	return append(slices.Clone(n.dir), n.file)
}

// tree reads the chapter tree, giving files without an entry nodes of
// their own and dropping entries without files, and saves each .order
// that changed.
func (s *Store) tree() (Tree, error) {
	root, err := s.diskPath()
	if err != nil {
		return Tree{}, err
	}
	if err = os.MkdirAll(root, 0755); err != nil {
		return Tree{}, err
	}
	t := Tree{Root: &Node{Folder: true}, byID: map[string]*Node{}}
	used, err := s.trashedIDs()
	if err != nil {
		return Tree{}, err
	}
	if err = s.readDir(t, t.Root, used); err != nil {
		return Tree{}, err
	}
	return t, nil
}

func (s *Store) readDir(t Tree, folder *Node, used map[string]bool) error {
	dir := childDir(folder)
	p, err := s.diskPath(dir...)
	if err != nil {
		return err
	}
	files, err := os.ReadDir(p)
	if err != nil {
		return err
	}
	folders, docs := map[string]bool{}, map[string]bool{}
	for _, f := range files {
		name := f.Name()
		switch {
		case strings.HasPrefix(name, "."):
		case f.Type()&os.ModeSymlink != 0:
			return fmt.Errorf("%s: symlinks are not allowed", filepath.Join(p, name))
		case f.IsDir():
			folders[name] = true
		case strings.HasSuffix(strings.ToLower(name), outlineExt):
		case strings.HasSuffix(strings.ToLower(name), textExt):
			docs[name[:len(name)-len(textExt)]] = true
		}
	}
	order, err := s.readOrder(dir)
	if err != nil {
		return err
	}
	changed := false
	add := func(e orderEntry) {
		if validID(e.ID) != nil || used[e.ID] {
			e.ID = freshID(used)
			changed = true
		}
		used[e.ID] = true
		if strings.TrimSpace(e.Title) == "" {
			e.Title = e.File
			changed = true
		}
		n := &Node{ID: e.ID, Title: e.Title, Folder: e.Folder, Parent: folder, file: e.File, dir: dir}
		folder.Children = append(folder.Children, n)
		t.byID[n.ID] = n
	}
	for _, e := range order {
		have := docs
		if e.Folder {
			have = folders
		}
		if !have[e.File] {
			changed = true // gone, or renamed outside: a new node below
			continue
		}
		delete(have, e.File)
		add(e)
	}
	for _, kind := range []struct {
		names  map[string]bool
		folder bool
	}{{folders, true}, {docs, false}} {
		var names []string
		for name := range kind.names {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			add(orderEntry{Title: name, File: name, Folder: kind.folder})
			changed = true
		}
	}
	if changed {
		if err = s.writeOrder(folder); err != nil {
			return err
		}
	}
	for _, n := range folder.Children {
		if n.Folder {
			if err = s.readDir(t, n, used); err != nil {
				return err
			}
		}
	}
	return nil
}

func freshID(used map[string]bool) string {
	id := NewID()
	for used[id] {
		id = NewID()
	}
	used[id] = true
	return id
}

func (s *Store) readOrder(dir []string) ([]orderEntry, error) {
	p, err := s.diskPath(append(slices.Clone(dir), orderFile)...)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var order []orderEntry
	if err = decodeStrict(b, &order); err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	return order, nil
}

// writeOrder saves the order of what folder holds.
func (s *Store) writeOrder(folder *Node) error {
	order := []orderEntry{}
	for _, n := range folder.Children {
		order = append(order, orderEntry{ID: n.ID, Title: n.Title, File: n.file, Folder: n.Folder})
	}
	b, err := json.MarshalIndent(order, "", "  ")
	if err != nil {
		return err
	}
	p, err := s.diskPath(append(childDir(folder), orderFile)...)
	if err != nil {
		return err
	}
	return atomicWrite(p, append(b, '\n'), false)
}

// Chapters returns the chapter tree.
func (s *Store) Chapters() (Tree, error) {
	unlock, err := s.lock()
	if err != nil {
		return Tree{}, err
	}
	defer unlock()
	return s.tree()
}

// folderOf returns the folder id, the root for "".
func (t Tree) folderOf(id string) (*Node, error) {
	if id == "" {
		return t.Root, nil
	}
	n, ok := t.byID[id]
	if !ok {
		return nil, fmt.Errorf("chapters: unknown node %q", id)
	}
	if !n.Folder {
		return nil, fmt.Errorf("chapters: %q is not a folder", n.Title)
	}
	return n, nil
}

// NewChapter adds a document, or a folder, titled title at the end of the
// folder parent ("" for the top) and returns its ID.
func (s *Store) NewChapter(parent, title string, folder bool) (string, error) {
	unlock, err := s.lock()
	if err != nil {
		return "", err
	}
	defer unlock()
	t, err := s.tree()
	if err != nil {
		return "", err
	}
	p, err := t.folderOf(parent)
	if err != nil {
		return "", err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return "", errors.New("标题不能为空")
	}
	used, err := s.usedIDs(t)
	if err != nil {
		return "", err
	}
	n := &Node{ID: freshID(used), Title: title, Folder: folder, Parent: p, dir: childDir(p)}
	n.file = fileName(title, takenIn(p, nil))
	if folder {
		path, err := s.nodePath(n, "")
		if err != nil {
			return "", err
		}
		if err = os.Mkdir(path, 0755); err != nil {
			return "", err
		}
	} else {
		path, err := s.nodePath(n, textExt)
		if err != nil {
			return "", err
		}
		if err = atomicWrite(path, nil, true); err != nil {
			return "", err
		}
	}
	p.Children = append(p.Children, n)
	return n.ID, s.writeOrder(p)
}

// usedIDs returns the IDs of the tree and of the trash.
func (s *Store) usedIDs(t Tree) (map[string]bool, error) {
	used, err := s.trashedIDs()
	if err != nil {
		return nil, err
	}
	for id := range t.byID {
		used[id] = true
	}
	return used, nil
}

// companions are the files of n beside its own, which move with it.
func companions(n *Node) []string {
	if n.Folder {
		return []string{"", outlineExt}
	}
	return []string{textExt, outlineExt, marksExt, summaryExt}
}

// moveFiles moves n's files from its place to file in dir.
func (s *Store) moveFiles(n *Node, dir []string, file string) error {
	for _, ext := range companions(n) {
		from, err := s.nodePath(n, ext)
		if err != nil {
			return err
		}
		to, err := s.diskPath(append(slices.Clone(dir), file+ext)...)
		if err != nil {
			return err
		}
		if err = os.Rename(from, to); err != nil && !(ext != textExt && ext != "" && errors.Is(err, os.ErrNotExist)) {
			return err
		}
	}
	return nil
}

// RenameChapter retitles node id, renaming its files to match.
func (s *Store) RenameChapter(id, title string) error {
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
	title = strings.TrimSpace(title)
	if title == "" {
		return errors.New("标题不能为空")
	}
	file := fileName(title, takenIn(n.Parent, n))
	if file != n.file {
		if err = s.moveFiles(n, n.dir, file); err != nil {
			return err
		}
		n.file = file
	}
	n.Title = title
	return s.writeOrder(n.Parent)
}

// MoveChapter puts node id into the folder parent ("" for the top) at
// index among what it holds, counted without the node itself.
func (s *Store) MoveChapter(id, parent string, index int) error {
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
	to, err := t.folderOf(parent)
	if err != nil {
		return err
	}
	if to.Within(n) {
		return errors.New("不能把文件夹移进它自己里面")
	}
	from := n.Parent
	from.Children = slices.DeleteFunc(from.Children, func(x *Node) bool { return x == n })
	index = clampIndex(index, len(to.Children))
	if to == from {
		to.Children = slices.Insert(to.Children, index, n)
		return s.writeOrder(to)
	}
	// The entry goes in first and out last: a crash between leaves the
	// node where its files are, as reading the tree drops an entry whose
	// files are gone.
	file := fileName(n.Title, takenIn(to, nil))
	moved := &Node{ID: n.ID, Title: n.Title, Folder: n.Folder, Parent: to, file: file, dir: childDir(to)}
	to.Children = slices.Insert(to.Children, index, moved)
	if err = s.writeOrder(to); err != nil {
		return err
	}
	if err = s.moveFiles(n, moved.dir, file); err != nil {
		return err
	}
	return s.writeOrder(from)
}

func clampIndex(i, n int) int { return max(0, min(i, n)) }

// TrashChapter moves node id, with what it holds, to the trash.
func (s *Store) TrashChapter(id string) error {
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
	dir, err := s.path(trashDir, n.ID)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	info := Trashed{ID: n.ID, Title: n.Title, Folder: n.Folder, File: n.file, Parent: n.Parent.ID, At: time.Now().UTC()}
	b, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	if err = atomicWrite(filepath.Join(dir, trashedFile), append(b, '\n'), false); err != nil {
		return err
	}
	for _, ext := range companions(n) {
		from, err := s.nodePath(n, ext)
		if err != nil {
			return err
		}
		if err = os.Rename(from, filepath.Join(dir, n.file+ext)); err != nil && !(ext != textExt && ext != "" && errors.Is(err, os.ErrNotExist)) {
			return err
		}
	}
	n.Parent.Children = slices.DeleteFunc(n.Parent.Children, func(x *Node) bool { return x == n })
	return s.writeOrder(n.Parent)
}

// trashedIDs returns the IDs in the trash, of the nodes trashed and what
// trashed folders hold.
func (s *Store) trashedIDs() (map[string]bool, error) {
	used := map[string]bool{}
	items, err := s.trash()
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		used[it.ID] = true
		ids, err := s.heldIDs(it)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			used[id] = true
		}
	}
	return used, nil
}

// heldIDs returns the IDs the .order files of a trashed folder list.
func (s *Store) heldIDs(it Trashed) ([]string, error) {
	if !it.Folder {
		return nil, nil
	}
	root, err := s.path(trashDir, it.ID, it.File)
	if err != nil {
		return nil, err
	}
	var ids []string
	err = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != orderFile {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		var order []orderEntry
		if json.Unmarshal(b, &order) == nil {
			for _, e := range order {
				ids = append(ids, e.ID)
			}
		}
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	return ids, err
}

func (s *Store) trash() ([]Trashed, error) {
	p, err := s.path(trashDir)
	if err != nil {
		return nil, err
	}
	dirs, err := os.ReadDir(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var items []Trashed
	for _, d := range dirs {
		if !d.IsDir() || validID(d.Name()) != nil {
			continue
		}
		b, err := os.ReadFile(filepath.Join(p, d.Name(), trashedFile))
		if err != nil {
			continue // half trashed by a crash: its files are still in there
		}
		var it Trashed
		if json.Unmarshal(b, &it) != nil || it.ID != d.Name() {
			continue
		}
		items = append(items, it)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].At.After(items[j].At) })
	return items, nil
}

// Trash returns what is in the trash, the latest first.
func (s *Store) Trash() ([]Trashed, error) {
	unlock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	return s.trash()
}

// RestoreChapter puts node id back at the end of the folder it was in, or
// at the top when that folder is gone.
func (s *Store) RestoreChapter(id string) error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	items, err := s.trash()
	if err != nil {
		return err
	}
	i := slices.IndexFunc(items, func(it Trashed) bool { return it.ID == id })
	if i < 0 {
		return fmt.Errorf("回收站里没有 %q", id)
	}
	it := items[i]
	t, err := s.tree()
	if err != nil {
		return err
	}
	to, err := t.folderOf(it.Parent)
	if err != nil {
		to = t.Root
	}
	dir, err := s.path(trashDir, id)
	if err != nil {
		return err
	}
	n := &Node{ID: it.ID, Title: it.Title, Folder: it.Folder, file: it.File}
	file := fileName(it.Title, takenIn(to, nil))
	moved := &Node{ID: it.ID, Title: it.Title, Folder: it.Folder, Parent: to, file: file, dir: childDir(to)}
	to.Children = append(to.Children, moved)
	if err = s.writeOrder(to); err != nil {
		return err
	}
	for _, ext := range companions(n) {
		dst, err := s.nodePath(moved, ext)
		if err != nil {
			return err
		}
		if err = os.Rename(filepath.Join(dir, it.File+ext), dst); err != nil && !(ext != textExt && ext != "" && errors.Is(err, os.ErrNotExist)) {
			return err
		}
	}
	return os.RemoveAll(dir)
}

// PurgeChapter deletes node id from the trash for good, with the
// snapshots of it and of what it held.
func (s *Store) PurgeChapter(id string) error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	items, err := s.trash()
	if err != nil {
		return err
	}
	i := slices.IndexFunc(items, func(it Trashed) bool { return it.ID == id })
	if i < 0 {
		return fmt.Errorf("回收站里没有 %q", id)
	}
	held, err := s.heldIDs(items[i])
	if err != nil {
		return err
	}
	for _, h := range append(held, id) {
		if validID(h) != nil {
			continue
		}
		p, err := s.path(historyDir, h)
		if err != nil {
			return err
		}
		if err = os.RemoveAll(p); err != nil {
			return err
		}
	}
	p, err := s.path(trashDir, id)
	if err != nil {
		return err
	}
	return os.RemoveAll(p)
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
	return ch, nil
}

// anchor finds m in text again: where it was if its quote is still there,
// else at the occurrence of its quote nearest to that, else nowhere.
func anchor(text []rune, m ChapterMark) ChapterMark {
	m.Start, m.End = clampIndex(m.Start, len(text)), clampIndex(m.End, len(text))
	if m.End < m.Start {
		m.End = m.Start
	}
	if m.Orphaned || string(text[m.Start:m.End]) == m.Quote {
		return m
	}
	q := []rune(m.Quote)
	best := -1
	for i := 0; len(q) > 0 && i+len(q) <= len(text); i++ {
		if string(text[i:i+len(q)]) == m.Quote && (best < 0 || abs(i-m.Start) < abs(best-m.Start)) {
			best = i
		}
	}
	if best < 0 {
		m.Orphaned, m.End = true, m.Start
		return m
	}
	m.Start, m.End = best, best+len(q)
	return m
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
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

// WriteChapter saves document id's text and marks, the quote of each mark
// taken from text, and returns the text's new stamp.
func (s *Store) WriteChapter(id, text string, marks []ChapterMark) (Stamp, error) {
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
			m.Quote = string(runes[m.Start:m.End])
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

// Count is how many characters a text has as writers count them: every
// character but white space.
func Count(s string) int {
	n := 0
	for _, r := range s {
		if !unicode.IsSpace(r) {
			n++
		}
	}
	return n
}

// Counts returns the count of each document's text, by ID.
func (s *Store) Counts() (map[string]int, error) {
	unlock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	t, err := s.tree()
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for id, n := range t.byID {
		if n.Folder {
			continue
		}
		p, err := s.nodePath(n, textExt)
		if err != nil {
			return nil, err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		counts[id] = Count(string(b))
	}
	return counts, nil
}

// Export joins the work into one text in the tree's order, each node under
// its title; indent puts two full-width spaces before each paragraph.
func (s *Store) Export(indent bool) (string, error) {
	unlock, err := s.lock()
	if err != nil {
		return "", err
	}
	defer unlock()
	t, err := s.tree()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(s.work.Name + "\n")
	var failed error
	t.Walk(func(n *Node, depth int) {
		if failed != nil {
			return
		}
		b.WriteString("\n" + n.Title + "\n")
		if n.Folder {
			return
		}
		ch, err := s.readChapter(n)
		if err != nil {
			failed = err
			return
		}
		if strings.TrimSpace(ch.Text) == "" {
			return
		}
		b.WriteString("\n")
		for _, p := range strings.Split(strings.TrimRight(ch.Text, "\n"), "\n") {
			if indent && strings.TrimSpace(p) != "" {
				p = "　　" + p
			}
			b.WriteString(p + "\n")
		}
	})
	return b.String(), failed
}
