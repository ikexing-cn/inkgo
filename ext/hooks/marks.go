package hooks

import (
	"writer/editor"
	"writer/workspace"
)

var markKinds = map[editor.MarkKind]string{
	editor.MarkForeshadow: workspace.MarkForeshadow,
	editor.MarkAI:         workspace.MarkAI,
	editor.MarkPayoff:     workspace.MarkPayoff,
}

func storedMarks(ms []editor.Mark) []workspace.ChapterMark {
	var out []workspace.ChapterMark
	for _, m := range ms {
		out = append(out, workspace.ChapterMark{Kind: markKinds[m.Kind], Start: m.Start, End: m.End, Note: m.Note, Orphaned: m.Orphaned, Quote: m.Quote,
			ID: m.Key, Of: m.Ref, At: m.At, Dropped: m.Dropped})
	}
	return out
}

func editorMarks(ms []workspace.ChapterMark) []editor.Mark {
	var out []editor.Mark
	for _, m := range ms {
		for k, name := range markKinds {
			if name == m.Kind {
				out = append(out, editor.Mark{Kind: k, Start: m.Start, End: m.End, Note: m.Note, Orphaned: m.Orphaned, Quote: m.Quote,
					Key: m.ID, Ref: m.Of, At: m.At, Dropped: m.Dropped})
			}
		}
	}
	return out
}

func liveMarks(d *editor.Doc) []workspace.ChapterMark {
	ms := storedMarks(d.Marks())
	for i := range ms {
		if !ms[i].Orphaned {
			ms[i].Quote = d.Slice(ms[i].Start, ms[i].End)
		}
	}
	return ms
}
