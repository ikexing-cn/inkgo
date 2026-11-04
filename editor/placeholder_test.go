package editor

import "testing"

func TestPlaceholderIsReadAgainWhenLanguageChanges(t *testing.T) {
	text := "First language"
	calls := 0
	ed := New(NewDoc(""), Options{Placeholder: func() string { calls++; return text }})
	tt := newTester(ed)
	before := calls
	text = "Second language"
	tt.Frame()
	if calls <= before || ed.opts.Placeholder() != text {
		t.Fatal("placeholder was cached", calls, before)
	}
}
