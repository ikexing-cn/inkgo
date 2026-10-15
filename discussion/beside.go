package discussion

import (
	"fmt"
	"strings"

	"writer/prompt"
	"writer/workspace"
)

// Writing is what the author is writing as they talk beside the prose: a
// chapter, or a volume when a folder is open.
type Writing struct {
	Title, Outline string // the chapter's, or the volume's
	Volume         string // the title of the volume a chapter lies in
	VolumeOutline  string
	Folder         bool
	Before         string // the chapter's text before the caret
}

// BesideCap bounds the text before the caret sent with a turn.
const BesideCap = 2000

// render writes w for the author's turn, charged to budget as prose: the
// outlines must go, and the text before the caret goes as far as it holds,
// cut from its start. It lists what it sent.
func (w Writing) render(budget *prompt.Budget) (string, []workspace.ContextItem) {
	var s strings.Builder
	var items []workspace.ContextItem
	add := func(head, body, item string) {
		s.WriteString(head + "\n" + body + "\n\n")
		items = append(items, workspace.ContextItem{Text: item})
	}
	if w.Folder {
		s.WriteString("【作者正在看的卷：" + w.Title + "】\n\n")
		items = append(items, workspace.ContextItem{Text: "所在卷：" + w.Title})
		if o := strings.TrimSpace(w.VolumeOutline); o != "" {
			add("【卷纲】", o, "卷纲："+w.Title)
		}
	} else {
		if o := strings.TrimSpace(w.VolumeOutline); o != "" {
			add("【所在卷："+w.Volume+"】卷纲：", o, "卷纲："+w.Volume)
		}
		s.WriteString("【作者正在写的章：" + w.Title + "】\n\n")
		items = append(items, workspace.ContextItem{Text: "所在章：" + w.Title})
		if o := strings.TrimSpace(w.Outline); o != "" {
			add("【章纲】", o, "章纲："+w.Title)
		}
	}
	budget.Must(workspace.PartProse, s.String())
	before := []rune(w.Before)
	if len(before) > BesideCap {
		before = before[len(before)-BesideCap:]
	}
	for len(before) > 0 && !budget.Take(workspace.PartProse, string(before)) {
		before = before[min(len(before), max(len(before)/4, 1)):]
	}
	if len(before) > 0 {
		body := string(before)
		s.WriteString("【光标前的正文】\n" + body + "\n\n")
		items = append(items, workspace.ContextItem{Text: fmt.Sprintf("本章光标前 %d 字", workspace.Count(body))})
	}
	s.WriteString("【作者说】\n")
	return s.String(), items
}
