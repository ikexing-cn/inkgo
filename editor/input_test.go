package editor

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func useMacKeys(t *testing.T, mac bool) {
	t.Helper()
	old := macKeys
	macKeys = mac
	t.Cleanup(func() { macKeys = old })
}

func TestPlatformKeyMovement(t *testing.T) {
	for _, mac := range []bool{false, true} {
		name := "PC"
		if mac {
			name = "Mac"
		}
		t.Run(name, func(t *testing.T) {
			useMacKeys(t, mac)
			word := ui.Cmd
			if mac {
				word = ui.Alt
			}
			type move struct {
				name     string
				mods     ui.Modifiers
				key      ui.Key
				from, to int
			}
			cases := []move{
				{"word left", word, ui.KeyLeft, 7, 4},
				{"word right", word, ui.KeyRight, 4, 7},
				{"doc start", ui.Cmd, ui.KeyHome, 7, 0},
				{"doc end", ui.Cmd, ui.KeyEnd, 7, 11},
				{"home", 0, ui.KeyHome, 7, 4},
				{"end", 0, ui.KeyEnd, 5, 11},
			}
			if mac {
				cases = append(cases,
					move{"cmd left", ui.Cmd, ui.KeyLeft, 7, 4},
					move{"cmd right", ui.Cmd, ui.KeyRight, 5, 11},
					move{"cmd up", ui.Cmd, ui.KeyUp, 7, 0},
					move{"cmd down", ui.Cmd, ui.KeyDown, 7, 11})
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					for _, ro := range []bool{false, true} {
						for _, shift := range []bool{false, true} {
							e := New(NewDoc("one\ntwo end"), Options{ReadOnly: ro})
							newTester(e)
							e.MoveTo(tc.from, false)
							mods := tc.mods
							if shift {
								mods |= ui.Shift
							}
							if !e.key(mods, tc.key) || e.caret != tc.to {
								t.Fatalf("ro=%v shift=%v caret=%d want=%d", ro, shift, e.caret, tc.to)
							}
							wantAnchor := tc.to
							if shift {
								wantAnchor = tc.from
							}
							if e.anchor != wantAnchor {
								t.Fatalf("anchor=%d want=%d", e.anchor, wantAnchor)
							}
						}
					}
				})
			}
		})
	}
}

func TestPlatformKeyDeletion(t *testing.T) {
	for _, mac := range []bool{false, true} {
		name := "PC"
		if mac {
			name = "Mac"
		}
		t.Run(name, func(t *testing.T) {
			useMacKeys(t, mac)
			word := ui.Cmd
			if mac {
				word = ui.Alt
			}
			type deletion struct {
				mods ui.Modifiers
				key  ui.Key
				from int
				want string
			}
			cases := []deletion{
				{word, ui.KeyBackspace, 7, "one\n end"},
				{word, ui.KeyDelete, 4, "one\n end"},
			}
			if mac {
				cases = append(cases,
					deletion{ui.Cmd, ui.KeyBackspace, 9, "one\nnd"},
					deletion{ui.Cmd, ui.KeyDelete, 5, "one\nt"})
			}
			for _, tc := range cases {
				for _, selected := range []bool{false, true} {
					e := New(NewDoc("one\ntwo end"), Options{})
					newTester(e)
					e.MoveTo(tc.from, false)
					want := tc.want
					if selected {
						e.Select(1, 5)
						want = "owo end"
					}
					beforeCaret, beforeAnchor := e.caret, e.anchor
					if !e.key(tc.mods, tc.key) || e.Doc.String() != want {
						t.Fatalf("key=%v selection=%v got=%q want=%q", tc.key, selected, e.Doc.String(), want)
					}
					e.key(ui.Cmd, ui.KeyZ)
					if e.Doc.String() != "one\ntwo end" || e.caret != beforeCaret || e.anchor != beforeAnchor || e.CanUndo() {
						t.Fatal("deletion did not undo in one step")
					}
					if mac && e.key(ui.Cmd, ui.KeyY) {
						t.Fatal("Cmd+Y handled on Mac")
					}
					if mac {
						e.key(ui.Cmd|ui.Shift, ui.KeyZ)
					} else {
						e.key(ui.Cmd, ui.KeyY)
					}
					if e.Doc.String() != want {
						t.Fatal("redo failed")
					}
					e.opts.ReadOnly = true
					if e.key(tc.mods, tc.key) || e.Doc.String() != want {
						t.Fatal("read-only deletion")
					}
				}
			}
		})
	}
}

func TestMacWrappedLineKeys(t *testing.T) {
	useMacKeys(t, true)
	text := strings.Repeat("一二三四五六七八九十", 30)
	for _, end := range []bool{false, true} {
		e := New(NewDoc(text), Options{})
		newTester(e)
		ln := e.lay.lines[1]
		from := e.lay.toDoc(ln.start + 2)
		to := e.lay.toDoc(ln.start)
		k, del := ui.KeyLeft, ui.KeyBackspace
		if end {
			to = e.lay.toDoc(ln.end - 1)
			k, del = ui.KeyRight, ui.KeyDelete
		}
		for _, shift := range []bool{false, true} {
			e.MoveTo(from, false)
			mods := ui.Cmd
			if shift {
				mods |= ui.Shift
			}
			e.key(mods, k)
			if e.caret != to || (shift && e.anchor != from) {
				t.Fatal("wrapped line movement")
			}
		}
		e.MoveTo(from, false)
		e.key(ui.Cmd, del)
		a, b := min(from, to), max(from, to)
		if e.Doc.String() != string([]rune(text)[:a])+string([]rune(text)[b:]) {
			t.Fatal("wrapped line deletion")
		}
		e.key(ui.Cmd, ui.KeyZ)
		if e.Doc.String() != text || e.caret != from {
			t.Fatal("wrapped line undo")
		}
	}
}

func TestMacCompositionKeepsKeys(t *testing.T) {
	useMacKeys(t, true)
	e := New(NewDoc("one two"), Options{})
	e.MoveTo(4, false)
	e.preedit = "ni"
	for _, mods := range []ui.Modifiers{ui.Cmd, ui.Alt, ui.Cmd | ui.Shift, ui.Alt | ui.Shift} {
		for _, k := range []ui.Key{ui.KeyLeft, ui.KeyRight, ui.KeyUp, ui.KeyDown, ui.KeyBackspace, ui.KeyDelete} {
			if e.key(mods, k) || e.caret != 4 || e.preedit != "ni" || e.Doc.String() != "one two" {
				t.Fatal("key handled during composition")
			}
		}
	}
}

func TestMacPageKeys(t *testing.T) {
	useMacKeys(t, true)
	for _, ro := range []bool{false, true} {
		for _, shift := range []bool{false, true} {
			for _, k := range []ui.Key{ui.KeyPageUp, ui.KeyPageDown} {
				e := New(NewDoc(strings.Repeat("one two\n", 100)), Options{ReadOnly: ro})
				newTester(e)
				from := 400
				e.MoveTo(from, false)
				mods := ui.Modifiers(0)
				if shift {
					mods = ui.Shift
				}
				if !e.key(mods, k) || (k == ui.KeyPageUp && e.caret >= from) || (k == ui.KeyPageDown && e.caret <= from) {
					t.Fatal("page movement failed")
				}
				if (shift && e.anchor != from) || (!shift && e.anchor != e.caret) {
					t.Fatal("page selection failed")
				}
			}
		}
	}
}

// At the line's edge Cmd+Backspace and Cmd+Delete take one character, so
// the lines join; at the text's ends they do nothing.
func TestMacLineDeletionBoundaries(t *testing.T) {
	useMacKeys(t, true)
	for _, text := range []string{"", "one\n\ntwo"} {
		for _, tc := range []struct {
			at  int
			key ui.Key
		}{
			{0, ui.KeyBackspace}, {len([]rune(text)), ui.KeyDelete},
		} {
			e := New(NewDoc(text), Options{})
			newTester(e)
			e.MoveTo(tc.at, false)
			if !e.key(ui.Cmd, tc.key) || e.Doc.String() != text || e.CanUndo() {
				t.Fatal("boundary deletion changed document")
			}
		}
	}
	for _, tc := range []struct {
		at   int
		key  ui.Key
		want string
	}{
		{5, ui.KeyBackspace, "one\ntwo"}, // start of "two"
		{3, ui.KeyDelete, "one\ntwo"},    // end of "one"
		{4, ui.KeyBackspace, "one\ntwo"}, // the empty line
	} {
		e := New(NewDoc("one\n\ntwo"), Options{})
		newTester(e)
		e.MoveTo(tc.at, false)
		if !e.key(ui.Cmd, tc.key) || e.Doc.String() != tc.want {
			t.Fatalf("at %d: %q, want %q", tc.at, e.Doc.String(), tc.want)
		}
		e.key(ui.Cmd, ui.KeyZ)
		if e.Doc.String() != "one\n\ntwo" {
			t.Fatalf("at %d: undo gave %q", tc.at, e.Doc.String())
		}
	}
}
