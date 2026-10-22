// Package book holds the content of an open work as the view reads it: the
// chapter tree, the counts, the marks, the setting, the plan and the
// chapters' outlines, and the stored checks. It reads the store and writes
// it; what is not the book's content (talks, records, snapshots, the trash
// list, preferences) stays with the store.
package book

import "writer/workspace"

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

// SetOutline keeps an outline written through the store, or one the view
// knows the store now has.
func (b *Book) SetOutline(id, outline string) {
	if b.outlines == nil {
		b.outlines = map[string]string{}
	}
	b.outlines[id] = outline
	b.version++
}

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
	b.tree = t
	b.version++
	return nil
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
