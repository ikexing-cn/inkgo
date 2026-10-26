// Package book holds the content of an open work as the view reads it: the
// chapter tree, the counts, the marks, the setting, the plan and the
// chapters' outlines, and the stored checks. It reads the store and writes
// it; what is not the book's content (talks, records, snapshots, the trash
// list, preferences) stays with the store.
package book

import (
	"errors"

	"writer/workspace"
)

// Book is the content of one work, as of the last read. It is used on the
// view thread only: the store is handed to background work as it was, and a
// result that changes the setting comes back for Book to read again.
type Book struct {
	store   *workspace.Store
	version int

	tree     workspace.Tree
	counts   map[string]int
	marks    map[string][]workspace.ChapterMark
	setting  workspace.Setting
	plan     workspace.Plan
	planErr  error
	outlines map[string]string
	checks   map[string]workspace.Check
}

// Open reads everything the book holds once. A part that cannot be read is
// left empty; reading it again reports the error where the view can show it
// (the plan keeps it for the page that shows a broken outline).
func Open(store *workspace.Store) *Book {
	b := &Book{store: store}
	_ = b.ReloadAll()
	return b
}

// Version goes up by one whenever the book reads or writes any of its
// content.
func (b *Book) Version() int { return b.version }

// Tree is the chapter tree as of the last read or write.
func (b *Book) Tree() workspace.Tree { return b.tree }

// Counts is the count of each document's prose, by ID.
func (b *Book) Counts() map[string]int { return b.counts }

// Count is one document's count, 0 when it has none yet.
func (b *Book) Count(id string) int { return b.counts[id] }

// SetCount keeps the open chapter's count up with what is typed before it
// is saved; the prose on disk still has the word the last save gave it.
func (b *Book) SetCount(id string, n int) {
	if b.counts[id] == n {
		return
	}
	if b.counts == nil {
		b.counts = map[string]int{}
	}
	b.counts[id] = n
	b.version++
}

// SetCounts replaces every count the book holds, writing nothing: a test
// stands for the counts read with it; nil is counts not read yet.
func (b *Book) SetCounts(counts map[string]int) {
	b.counts = counts
	b.version++
}

// Marks is each chapter's marks, by ID, for the chapters that have any.
func (b *Book) Marks() map[string][]workspace.ChapterMark { return b.marks }

// Setting is the setting as of the last read or revision.
func (b *Book) Setting() workspace.Setting { return b.setting }

// Plan is the plan as of the last read or write.
func (b *Book) Plan() workspace.Plan { return b.plan }

// PlanErr is the error the outline page shows for the plan it could not
// read, nil when it can.
func (b *Book) PlanErr() error { return b.planErr }

// Outlines is the outline of each node, by ID, folders too, as of the last
// read or write. A node with no outline has "".
func (b *Book) Outlines() map[string]string { return b.outlines }

// Checks is the stored check of each chapter that has one, by ID.
func (b *Book) Checks() map[string]workspace.Check { return b.checks }

// ReloadAll reads every part again, as Open does, and returns the first
// error. A part that fails is left as far as its reload got.
func (b *Book) ReloadAll() error {
	var first error
	for _, load := range []func() error{
		b.ReloadTree, b.ReloadCounts, b.ReloadMarks, b.ReloadSetting,
		b.ReloadPlan, b.ReloadOutlines, b.ReloadChecks,
	} {
		if err := load(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// ReloadTree reads the chapter tree as of the disk.
func (b *Book) ReloadTree() error {
	t, err := b.store.Chapters()
	if err != nil {
		return err
	}
	if sameTree(b.tree, t) {
		return nil
	}
	b.tree = t
	b.version++
	return nil
}

// sameTree tells whether two reads of the tree show the same chapters in
// the same order, so a 2-second look that found nothing new moves no
// version.
func sameTree(a, b workspace.Tree) bool {
	if a.Root == nil || b.Root == nil {
		return a.Root == nil && b.Root == nil
	}
	return sameNodes(a.Root.Children, b.Root.Children)
}

func sameNodes(a, b []*workspace.Node) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Title != b[i].Title || a[i].Folder != b[i].Folder ||
			!sameNodes(a[i].Children, b[i].Children) {
			return false
		}
	}
	return true
}

// ReloadCounts reads the count of every document's prose.
func (b *Book) ReloadCounts() error {
	counts, err := b.store.Counts()
	if err != nil {
		return err
	}
	b.counts = counts
	b.version++
	return nil
}

// ReloadMarks reads the marks of every chapter that has any.
func (b *Book) ReloadMarks() error {
	marks, err := b.store.Marks()
	if err != nil {
		return err
	}
	b.marks = marks
	b.version++
	return nil
}

// ReloadSetting reads the setting as of the disk.
func (b *Book) ReloadSetting() error {
	set, err := b.store.Setting()
	if err != nil {
		return err
	}
	b.setting = set
	b.version++
	return nil
}

// ReloadPlan reads the plan as of the disk. A broken plan is kept with its
// error for the outline page, and reads as the zero plan.
func (b *Book) ReloadPlan() error {
	plan, err := b.store.Plan()
	b.plan, b.planErr = plan, err
	if err != nil {
		return err
	}
	b.version++
	return nil
}

// ReloadOutlines reads the outline of every node as of the disk.
func (b *Book) ReloadOutlines() error {
	outlines, err := b.store.Outlines()
	if err != nil {
		return err
	}
	b.outlines = outlines
	b.version++
	return nil
}

// ReloadChecks reads the stored check of every chapter that has one.
func (b *Book) ReloadChecks() error {
	checks, err := b.store.Checks()
	if err != nil {
		return err
	}
	b.checks = checks
	b.version++
	return nil
}

// SaveChapter writes a chapter's prose and marks, then keeps its count and
// reads its marks again: the store rewrites their quotes and clamps them to
// the text, so the copy is not the one passed in.
func (b *Book) SaveChapter(id, text string, marks []workspace.ChapterMark) (workspace.Stamp, error) {
	return b.saveChapter(id, text, marks, nil)
}

// SaveChapterOver writes as SaveChapter does when the text on disk is still
// the version over.
func (b *Book) SaveChapterOver(id, text string, marks []workspace.ChapterMark, over workspace.Stamp) (workspace.Stamp, error) {
	return b.saveChapter(id, text, marks, &over)
}

func (b *Book) saveChapter(id, text string, marks []workspace.ChapterMark, over *workspace.Stamp) (workspace.Stamp, error) {
	var stamp workspace.Stamp
	var err error
	if over == nil {
		stamp, err = b.store.WriteChapter(id, text, marks)
	} else {
		stamp, err = b.store.WriteChapterOver(id, text, marks, *over)
	}
	if err != nil {
		return stamp, err
	}
	b.version++
	b.setCount(id, workspace.Count(text))
	ch, err := b.store.ReadChapter(id)
	if err != nil {
		return stamp, err
	}
	b.version++
	if len(ch.Marks) == 0 {
		delete(b.marks, id)
	} else {
		if b.marks == nil {
			b.marks = map[string][]workspace.ChapterMark{}
		}
		b.marks[id] = ch.Marks
	}
	return stamp, nil
}

// SaveOutline writes a node's outline and keeps it; an empty one stays "".
func (b *Book) SaveOutline(id, outline string) error {
	if err := b.store.WriteOutline(id, outline); err != nil {
		return err
	}
	b.version++
	if b.outlines == nil {
		b.outlines = map[string]string{}
	}
	b.outlines[id] = outline
	return nil
}

// SaveCheck writes a chapter's check and keeps it; a check of no prose
// removes it.
func (b *Book) SaveCheck(id string, k workspace.Check) error {
	if err := b.store.WriteCheck(id, k); err != nil {
		return err
	}
	b.keepCheck(id, k)
	return nil
}

// SaveCheckOver replaces only the chapter result the caller acted on.
func (b *Book) SaveCheckOver(id string, k, over workspace.Check) error {
	if err := b.store.WriteCheckOver(id, k, over); err != nil {
		return err
	}
	b.keepCheck(id, k)
	return nil
}

// CheckUnchanged checks just this chapter on disk before a joint undo.
func (b *Book) CheckUnchanged(id string, over workspace.Check) error {
	now, err := b.store.ReadCheck(id)
	if err != nil {
		return err
	}
	if !now.Equal(over) {
		return errors.New("本章检查结果已再次修改，无法撤销")
	}
	return nil
}

func (b *Book) keepCheck(id string, k workspace.Check) {
	b.version++
	if b.checks == nil {
		b.checks = map[string]workspace.Check{}
	}
	if k.Of == "" {
		delete(b.checks, id)
	} else {
		b.checks[id] = k
	}
}

// Revise writes a setting revision and reads the setting again, as its
// entries and order come out of the store.
func (b *Book) Revise(r workspace.Revision) (workspace.Revision, error) {
	r, err := b.store.Revise(r)
	if err != nil {
		return r, err
	}
	b.version++
	return r, b.ReloadSetting()
}

// Unrevise removes the last revision and reads the setting left by it.
func (b *Book) Unrevise(id string) error {
	if err := b.store.Unrevise(id); err != nil {
		return err
	}
	b.version++
	return b.ReloadSetting()
}

// RemoveCategory removes a category, its entries going to 其他, and reads
// the setting again.
func (b *Book) RemoveCategory(name string) error {
	if err := b.store.RemoveCategory(name); err != nil {
		return err
	}
	b.version++
	return b.ReloadSetting()
}

// NewChapter adds a document or folder and reads the tree again.
func (b *Book) NewChapter(parent, title string, folder bool) (string, error) {
	id, err := b.store.NewChapter(parent, title, folder)
	if err != nil {
		return "", err
	}
	b.version++
	b.setCount(id, 0)
	return id, b.ReloadTree()
}

// RenameChapter renames a node and reads the tree again.
func (b *Book) RenameChapter(id, title string) error {
	if err := b.store.RenameChapter(id, title); err != nil {
		return err
	}
	b.version++
	return b.ReloadTree()
}

// MoveChapter moves a node and reads the tree again.
func (b *Book) MoveChapter(id, parent string, index int) error {
	if err := b.store.MoveChapter(id, parent, index); err != nil {
		return err
	}
	b.version++
	return b.ReloadTree()
}

// TrashChapter moves a node to the trash, reads the tree again, and counts
// the prose left.
func (b *Book) TrashChapter(id string) error {
	if err := b.store.TrashChapter(id); err != nil {
		return err
	}
	b.version++
	if err := b.ReloadTree(); err != nil {
		return err
	}
	return b.ReloadCounts()
}

// RestoreChapter takes a node out of the trash, reads the tree again, and
// counts the prose back in.
func (b *Book) RestoreChapter(id string) error {
	if err := b.store.RestoreChapter(id); err != nil {
		return err
	}
	b.version++
	if err := b.ReloadTree(); err != nil {
		return err
	}
	return b.ReloadCounts()
}

// PurgeChapter deletes a trashed node for good and reads the tree again.
func (b *Book) PurgeChapter(id string) error {
	if err := b.store.PurgeChapter(id); err != nil {
		return err
	}
	b.version++
	return b.ReloadTree()
}

// SaveCard saves a card and reads the plan again.
func (b *Book) SaveCard(c workspace.Card, after string) (workspace.Card, error) {
	c, err := b.store.SaveCard(c, after)
	if err != nil {
		return c, err
	}
	b.version++
	return c, b.ReloadPlan()
}

// DeleteCard deletes a card and reads the plan again.
func (b *Book) DeleteCard(id string) error {
	if err := b.store.DeleteCard(id); err != nil {
		return err
	}
	b.version++
	return b.ReloadPlan()
}

// PlaceCard puts a card in a chapter and line and reads the plan again.
func (b *Book) PlaceCard(id, chapter string, line *string) error {
	if err := b.store.PlaceCard(id, chapter, line); err != nil {
		return err
	}
	b.version++
	return b.ReloadPlan()
}

// DropCard puts a card in a time column and reads the plan again.
func (b *Book) DropCard(id string, column int, line string) error {
	if err := b.store.DropCard(id, column, line); err != nil {
		return err
	}
	b.version++
	return b.ReloadPlan()
}

// SaveTime names a time column and reads the plan again.
func (b *Book) SaveTime(column int, when string) error {
	if err := b.store.SaveTime(column, when); err != nil {
		return err
	}
	b.version++
	return b.ReloadPlan()
}

// SaveLine saves a plot line and reads the plan again.
func (b *Book) SaveLine(l workspace.PlotLine) (workspace.PlotLine, error) {
	l, err := b.store.SaveLine(l)
	if err != nil {
		return l, err
	}
	b.version++
	return l, b.ReloadPlan()
}

// MoveLine moves a plot line and reads the plan again.
func (b *Book) MoveLine(id string, delta int) error {
	if err := b.store.MoveLine(id, delta); err != nil {
		return err
	}
	b.version++
	return b.ReloadPlan()
}

// DeleteLine deletes a plot line and reads the plan again.
func (b *Book) DeleteLine(id string) error {
	if err := b.store.DeleteLine(id); err != nil {
		return err
	}
	b.version++
	return b.ReloadPlan()
}

func (b *Book) setCount(id string, n int) {
	if b.counts == nil {
		b.counts = map[string]int{}
	}
	b.counts[id] = n
}
