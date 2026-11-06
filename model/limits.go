package model

// Careful limits for a model the directory does not know, and the compaction
// threshold every model starts at.
const (
	guessWindow      = 32_000
	guessOutput      = 4096
	defaultCompactAt = 80
)

// Builtin is the model id as the directory has it, or with careful limits
// when the directory does not know it.
func Builtin(id string) Model {
	m, _ := Lookup(id)
	return m
}

// Lookup is Builtin, and whether the directory knows id.
func Lookup(id string) (Model, bool) {
	m := Model{ID: id, Window: guessWindow, Output: guessOutput, CompactAt: defaultCompactAt}
	b, ok := findModel(id)
	if ok {
		m.Window, m.Output = b.Limit.Context, b.Limit.Output
		if input := b.Limit.Input; input != nil && *input > 0 && *input < m.Window {
			m.Output = min(m.Output, m.Window-*input)
		} else if m.Output >= m.Window {
			m.Output = m.Window / 4
		}
	}
	return m, ok
}
