package propose

import (
	"writer/editor"
	"writer/workspace"
)

func storedMarks(ms []editor.Mark) []workspace.ChapterMark {
	var out []workspace.ChapterMark
	for _, m := range ms {
		out = append(out, workspace.ChapterMark{Kind: markKinds[m.Kind], Start: m.Start, End: m.End, Note: m.Note, Orphaned: m.Orphaned, Quote: m.Quote,
			ID: m.Key, Of: m.Ref, At: m.At, Dropped: m.Dropped})
	}
	return out
}

var markKinds = map[editor.MarkKind]string{
	editor.MarkForeshadow: workspace.MarkForeshadow,
	editor.MarkAI:         workspace.MarkAI,
	editor.MarkPayoff:     workspace.MarkPayoff,
}
