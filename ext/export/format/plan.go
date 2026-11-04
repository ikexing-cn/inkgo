package format

import (
	"slices"
	"strconv"

	"writer/workspace"
)

// The ranges of the export dialog.
const (
	ScopeWork    = "work"
	ScopeVolume  = "volume"
	ScopeChapter = "chapter"
	ScopeSpan    = "span"
)

// What the export dialog exports, each its own file.
const (
	ProseContent   = "prose"
	OutlineContent = "outline"
	SettingContent = "lore"
)

// Why something ticked is not exported, in Plan.Skip.
const (
	SkipNoChapters = "no-chapters" // the range has no chapters
	SkipNoOutlines = "no-outlines" // none of its chapters has an outline
	SkipNoLore     = "no-lore"     // nothing in the lore is settled
)

// What the dialog says the export will be, in Plan.About.
const (
	AboutMany    = "many"    // several files in one folder
	AboutLore    = "lore"    // the settled lore by category
	AboutOutline = "outline" // the outlines in chapter order
	AboutText    = "text"    // one plain text file
	AboutWord    = "word"    // one Word document
)

// ChapterNames are the chapters of pieces by the names the drop-downs of
// 指定范围 show, a number after a name some chapter before already has;
// ids are the chapters by those names.
func ChapterNames(pieces []workspace.Piece) (names []string, ids map[string]string) {
	ids = map[string]string{}
	for _, p := range pieces {
		if p.Folder {
			continue
		}
		name := p.Title
		for i := 2; ids[name] != ""; i++ {
			name = p.Title + " (" + strconv.Itoa(i) + ")"
		}
		names, ids[name] = append(names, name), p.ID
	}
	return names, ids
}

// Options selects the contents and plain text rather than Word documents.
type Options struct {
	Prose   bool `json:"prose,omitempty"`
	Outline bool `json:"outline,omitempty"`
	Setting bool `json:"setting,omitempty"`
	Text    bool `json:"text,omitempty"`
}

// Selection is the chosen range.
// From and To are ChapterNames labels, including suffixes for duplicate titles.
type Selection struct {
	Scope    string
	From, To string
}

// Plan is what the dialog would export as it stands.
type Plan struct {
	Scope   string            // the chosen range, or ScopeWork when it is unavailable
	Current string            // the chapter used to plan, or empty away from the writing page
	Names   []string          // chapter labels offered by the span selectors
	Ticked  []string          // selected contents, including those skipped
	Scopes  []string          // the ranges the dialog offers
	Pieces  []workspace.Piece // of the prose and the outline, in the range
	Whole   bool              // the range is the whole work
	Part    string            // the range in the names of the files
	Skip    map[string]string // why what is ticked cannot be exported, a Skip code
	Going   []string          // what is ticked and can be, in the dialog's order
	About   string            // the dialog's description, an About code
	Words   int               // of the prose in the range
}

// BuildPlan selects the available ranges, contents and explanation for the choices.
// span formats the two chapter titles in a multi-chapter range for file names.
func BuildPlan(pieces []workspace.Piece, entries []workspace.Entry, current string, options Options, selection Selection, span func(from, to string) string, language workspace.Language) Plan {
	plan := Plan{Current: current}
	scope, from, to := selection.Scope, selection.From, selection.To
	// The chapter open on the page of prose, and the volume it is in, or
	// the volume itself when that is what is open.
	cur, vol := -1, -1
	if current != "" {
		for i, p := range pieces {
			if p.ID == current {
				cur = i
			}
		}
	}
	if cur >= 0 && pieces[cur].Folder {
		cur, vol = -1, cur
	} else if cur >= 0 {
		for i := cur - 1; i >= 0 && pieces[cur].Depth > 0; i-- {
			if pieces[i].Depth < pieces[cur].Depth {
				vol = i
				break
			}
		}
	}
	names, ids := ChapterNames(pieces)
	plan.Names = names
	plan.Scopes = []string{ScopeWork}
	if vol >= 0 {
		plan.Scopes = append(plan.Scopes, ScopeVolume)
	}
	if cur >= 0 {
		plan.Scopes = append(plan.Scopes, ScopeChapter)
	}
	if len(names) > 1 {
		plan.Scopes = append(plan.Scopes, ScopeSpan)
	}
	if !slices.Contains(plan.Scopes, scope) {
		scope = ScopeWork
	}
	plan.Scope = scope
	chapters := map[string]bool{}
	switch scope {
	case ScopeWork:
		chapters, plan.Whole = nil, true
	case ScopeVolume:
		v := pieces[vol]
		for _, p := range pieces[vol+1:] {
			if p.Depth <= v.Depth {
				break
			}
			chapters[p.ID] = true
		}
		plan.Part = v.Title
	case ScopeChapter:
		chapters[pieces[cur].ID] = true
		plan.Part = pieces[cur].Title
	case ScopeSpan:
		in := false
		for _, name := range names {
			in = in || name == from
			if in {
				chapters[ids[name]] = true
			}
			if name == to {
				break
			}
		}
		plan.Part = titleOf(pieces, ids[from])
		if to != from {
			plan.Part = span(plan.Part, titleOf(pieces, ids[to]))
		}
	}
	plan.Pieces = Within(pieces, chapters)
	chaptered, outlined := false, false
	for _, p := range plan.Pieces {
		chaptered = chaptered || !p.Folder
		outlined = outlined || p.Outline != ""
		plan.Words += language.Count(p.Text)
	}
	plan.Skip = map[string]string{}
	if !chaptered {
		plan.Skip[ProseContent] = SkipNoChapters
		plan.Skip[OutlineContent] = SkipNoChapters
	} else if !outlined {
		plan.Skip[OutlineContent] = SkipNoOutlines
	}
	if !Settled(entries) {
		plan.Skip[SettingContent] = SkipNoLore
	}
	ticked := options.Ticked()
	plan.Ticked = ticked
	for _, what := range ticked {
		if plan.Skip[what] == "" {
			plan.Going = append(plan.Going, what)
		}
	}
	switch {
	case len(ticked) > 1:
		plan.About = AboutMany
	case options.Setting:
		plan.About = AboutLore
	case options.Outline:
		plan.About = AboutOutline
	case options.Text:
		plan.About = AboutText
	default:
		plan.About = AboutWord
	}
	return plan
}

// titleOf is the title of the piece id.
func titleOf(pieces []workspace.Piece, id string) string {
	for _, p := range pieces {
		if p.ID == id {
			return p.Title
		}
	}
	return ""
}

// Ticked is what is ticked, in the dialog's order.
func (o Options) Ticked() []string {
	var out []string
	for _, o := range []struct {
		on   bool
		what string
	}{{o.Prose, ProseContent}, {o.Outline, OutlineContent}, {o.Setting, SettingContent}} {
		if o.on {
			out = append(out, o.what)
		}
	}
	return out
}
