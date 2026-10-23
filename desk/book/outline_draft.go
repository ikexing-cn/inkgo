package book

import "time"

// OutlineDraft is one node's outline as it is being typed.
type OutlineDraft struct {
	Node, Text, Saved string
	Seen              string // text as of the last change
	at                time.Time
}

// Change notes when the text was last typed, and reports whether it did.
func (d *OutlineDraft) Change(now time.Time) bool {
	if d.Text == d.Seen {
		return false
	}
	d.Seen, d.at = d.Text, now
	return true
}

// Waits says whether the draft waits to be saved, and for how long more.
func (d *OutlineDraft) Waits(after time.Duration, now time.Time) (time.Duration, bool) {
	if d.Node == "" || d.Text == d.Saved {
		return 0, false
	}
	if wait := after - now.Sub(d.at); wait > 0 {
		return wait, true
	}
	return 0, true
}

// Save writes the draft through Book.
func (d *OutlineDraft) Save(b *Book) error {
	if d.Node == "" || d.Text == d.Saved {
		return nil
	}
	if err := b.SaveOutline(d.Node, d.Text); err != nil {
		return err
	}
	d.Saved = d.Text
	return nil
}
