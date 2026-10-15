package prompt

import (
	"cmp"
	"slices"
	"strings"

	"writer/workspace"
)

// Place is a place of the setting as a request shows it: Label names an
// entry of it in a record, empty for the body, and Heading opens it.
type Place struct {
	Place, Label, Heading string
}

// Line is an entry as a request shows it.
type Line func(workspace.Entry) string

// Whole is an entry on one line: its names, if it has any, then its text,
// as the text of an entry under a name need not say the name.
func Whole(e workspace.Entry) string {
	if len(e.Names) == 0 {
		return OneLine(e.Text)
	}
	return strings.Join(e.Names, "、") + "：" + OneLine(e.Text)
}

// Short is an entry in an index: its category, its first name, and its
// first sentence.
func Short(e workspace.Entry) string {
	head := e.Category
	if len(e.Names) > 0 {
		head += " · " + e.Names[0]
	}
	return head + "：" + firstSentence(e.Text)
}

// Item is e in a record of what was sent, as line shows it, after its
// place's label.
func (p Place) Item(e workspace.Entry, line Line) workspace.ContextItem {
	text := line(e)
	if p.Label != "" {
		text = p.Label + "：" + text
	}
	return workspace.ContextItem{ID: e.ID, Text: text}
}

// Render lists the entries of set in places that shown holds, or all of
// them for nil, under each place's heading, each as line shows it: by
// category in the work's order, then in the order they were added. That
// order changes only where the setting does, so a provider's cache keeps
// what comes before. A setting with nothing to show says so.
func Render(w workspace.Work, set workspace.Setting, places []Place, shown map[string]bool, line Line) string {
	var b strings.Builder
	for _, p := range places {
		var lines []string
		category := ""
		for _, e := range Ordered(w, set.Entries) {
			if e.Place != p.Place || shown != nil && !shown[e.ID] {
				continue
			}
			if p.Place == workspace.PlaceBody && e.Category != category {
				category = e.Category
				lines = append(lines, "【"+category+"】")
			}
			lines = append(lines, "- "+line(e))
		}
		if len(lines) == 0 {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(p.Heading + "\n" + strings.Join(lines, "\n"))
	}
	if b.Len() == 0 {
		return "设定文档还是空的。"
	}
	return b.String()
}

// Ordered is entries by category in the work's order, then in the order
// they were added.
func Ordered(w workspace.Work, entries []workspace.Entry) []workspace.Entry {
	at := map[string]int{}
	for n, c := range w.Categories {
		at[c.Name] = n
	}
	out := slices.Clone(entries)
	slices.SortStableFunc(out, func(a, b workspace.Entry) int { return cmp.Compare(at[a.Category], at[b.Category]) })
	return out
}

// Fit charges to the setting the entries of set in places, each as line
// shows it, as many as b holds, the newest first when not all fit; it
// returns those it took, and those it left out, newest first.
func (b *Budget) Fit(set workspace.Setting, places []string, line Line) (shown map[string]bool, left []workspace.Entry) {
	shown = map[string]bool{}
	for _, place := range places {
		entries := slices.Clone(set.Entries)
		slices.SortStableFunc(entries, func(a, b workspace.Entry) int { return cmp.Compare(b.Rev, a.Rev) })
		for _, e := range entries {
			if e.Place != place {
				continue
			}
			if b.Take(workspace.PartSetting, "- "+line(e)) {
				shown[e.ID] = true
			} else {
				left = append(left, e)
			}
		}
	}
	return shown, left
}

// OneLine is s with its runs of space, line breaks among them, made one
// space.
func OneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// sentenceEnds end the first sentence of an entry in an index.
const sentenceEnds = "。！？；\n"

// indexRunes bounds an entry's first sentence in an index.
const indexRunes = 60

func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, sentenceEnds); i >= 0 {
		end := i
		if r := s[i:]; !strings.HasPrefix(r, "。") && !strings.HasPrefix(r, "；") && !strings.HasPrefix(r, "\n") {
			end = i + len(string([]rune(r)[0]))
		}
		s = s[:end]
	}
	s = OneLine(s)
	if r := []rune(s); len(r) > indexRunes {
		s = string(r[:indexRunes]) + "…"
	}
	return s
}
