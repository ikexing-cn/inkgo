package discussion

import (
	"fmt"
	"strings"

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

// render writes w for the author's turn within budget, the text before the
// caret cut from its start first, and lists what it sent.
func (w Writing) render(budget int) (string, []workspace.ContextItem, int) {
	var b strings.Builder
	var items []workspace.ContextItem
	add := func(head, body, item string) {
		b.WriteString(head + "\n" + body + "\n\n")
		items = append(items, workspace.ContextItem{Text: item})
	}
	if w.Folder {
		b.WriteString("【作者正在看的卷：" + w.Title + "】\n\n")
		items = append(items, workspace.ContextItem{Text: "所在卷：" + w.Title})
		if o := strings.TrimSpace(w.VolumeOutline); o != "" {
			add("【卷纲】", o, "卷纲："+w.Title)
		}
	} else {
		if o := strings.TrimSpace(w.VolumeOutline); o != "" {
			add("【所在卷：" + w.Volume + "】卷纲：", o, "卷纲："+w.Volume)
		}
		b.WriteString("【作者正在写的章：" + w.Title + "】\n\n")
		items = append(items, workspace.ContextItem{Text: "所在章：" + w.Title})
		if o := strings.TrimSpace(w.Outline); o != "" {
			add("【章纲】", o, "章纲："+w.Title)
		}
	}
	head := b.String()
	cost := Estimate(head)
	before := []rune(w.Before)
	if len(before) > BesideCap {
		before = before[len(before)-BesideCap:]
	}
	for len(before) > 0 && cost+Estimate(string(before)) > budget {
		before = before[min(len(before), max(len(before)/4, 1)):]
	}
	if len(before) > 0 {
		body := string(before)
		b.WriteString("【光标前的正文】\n" + body + "\n\n")
		cost += Estimate(body)
		items = append(items, workspace.ContextItem{Text: fmt.Sprintf("本章光标前 %d 字", workspace.Count(body))})
	}
	b.WriteString("【作者说】\n")
	return b.String(), items, cost
}
