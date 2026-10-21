package workspace

// Reading is how e reads at the node at, or after its last change at
// none. The bool reports whether it reads as a change.
func (e Entry) Reading(o Order, at string) (string, Change, bool) {
	if at == "" {
		if hold := e.Holding(o); len(hold) > 0 {
			return hold[len(hold)-1].Text, hold[len(hold)-1], true
		}
		return e.Text, Change{}, false
	}
	if g, ok := e.At(o, at); ok {
		return g.Text, g, true
	}
	return e.Text, Change{}, false
}

// ReadingBefore is how e reads just before the node at: what a change
// from it starts from.
func (e Entry) ReadingBefore(o Order, at string) string {
	text := e.Text
	for _, g := range e.Holding(o) {
		if o[g.From] < o[at] {
			text = g.Text
		}
	}
	return text
}
