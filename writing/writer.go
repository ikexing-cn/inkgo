package writing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"writer/discussion"
	"writer/model"
	"writer/prompt"
	"writer/workspace"
)

// A sentence more takes little.
const (
	nextOutput  = 400 // a sentence, and what a model may say before it stops
	beforeCap   = 6000
	afterCap    = 1000
	previousCap = 3000
)

// writePlaces are the places of the setting as the prose sees them: what
// is open is not the author's yet.
var writePlaces = []prompt.Place{
	{Place: workspace.PlaceBody, Label: "正文", Heading: "设定正文（作者定下的，必须遵守）："},
	{Place: workspace.PlaceDropped, Label: "已放弃", Heading: "已放弃（不能出现在正文里）："},
}

// Writer writes the prose of one work with one model.
type Writer struct {
	Store *workspace.Store
	Model discussion.Model
}

// Scene is where in a chapter the model writes: the text before that
// place and after it, as the editor has them now.
type Scene struct {
	Chapter       string
	Before, After string
}

// ErrNoModel is what the author is told when the work names no model to
// write with.
var ErrNoModel = errors.New("请在设置中「AI」配置里选择续写和检查模型")

// prepare builds the request of task for scene: the rules and the setting
// in the system prompt, the outlines and the text around the place in the
// message, as far as the budget allows; and what it sends.
func (w *Writer) prepare(task string, scene Scene, tail string) (model.Request, *workspace.Sent, error) {
	var req model.Request
	if w.Model.Completer == nil {
		return req, nil, ErrNoModel
	}
	work := w.Store.Work()
	tree, err := w.Store.Chapters()
	if err != nil {
		return req, nil, err
	}
	n, ok := tree.Node(scene.Chapter)
	if !ok || n.Folder {
		return req, nil, errors.New("找不到这一章")
	}
	ch, err := w.Store.ReadChapter(n.ID)
	if err != nil {
		return req, nil, err
	}
	set, err := w.Store.Setting()
	if err != nil {
		return req, nil, err
	}
	// The prose is written to the setting as it stands at this chapter:
	// what changes later is not told, only that something does.
	set, ahead := set.At(tree.Order(), n.ID)
	later := ""
	if ahead > 0 {
		later = fmt.Sprintf("\n\n以上是写到本章时的设定；后面的章节里还有 %d 处变化，未列出。", ahead)
	}
	b := prompt.NewBudget(w.Model.Input())
	b.Must(workspace.PartRules, rules+task)
	b.Must(workspace.PartInput, tail)
	b.Must(workspace.PartSetting, later)
	// The parts of the message, in the order it reads.
	type part struct{ heading, text string }
	var volume, outline, prev, near, far part
	take := func(p *part, kind, heading, text string) {
		*p = part{heading, text}
		b.Must(kind, heading+text)
	}
	// The outlines go whole, or the request does not.
	if p := n.Parent; p != nil && p.Parent != nil {
		if vol, err := w.Store.ReadChapter(p.ID); err == nil && strings.TrimSpace(vol.Outline) != "" {
			take(&volume, workspace.PartOutline, "【所在卷："+p.Title+"】卷纲：", strings.TrimSpace(vol.Outline))
		}
	}
	if strings.TrimSpace(ch.Outline) != "" {
		take(&outline, workspace.PartOutline, "【本章："+n.Title+"】章纲：", strings.TrimSpace(ch.Outline))
	} else {
		outline = part{heading: "【本章：" + n.Title + "】"}
	}
	if b.Left() < 0 {
		return req, nil, errors.New("纲和要求超出了模型的上下文上限：请缩短，或在设置中「AI」配置里调大上下文上限")
	}
	if t := fit(lastRunes(scene.Before, beforeCap), b.Left(), true); strings.TrimSpace(t) != "" {
		take(&near, workspace.PartProse, "【本章前文】", t)
	}
	if t := fit(firstRunes(scene.After, afterCap), b.Left(), false); strings.TrimSpace(t) != "" {
		take(&far, workspace.PartProse, "【本章后文】", t)
	}

	// The setting: the body binds the prose, then what the author dropped;
	// the newest first when it does not all fit.
	shown, _ := b.Fit(set, []string{workspace.PlaceBody, workspace.PlaceDropped}, prompt.Whole)
	system := rules + "\n\n" + prompt.Render(work, set, writePlaces, shown, prompt.Whole) + later + "\n\n" + task

	// The previous chapter's end, while this one has little before.
	if p, ok := previous(tree, n); ok && len([]rune(scene.Before)) < beforeCap {
		if pc, err := w.Store.ReadChapter(p.ID); err == nil {
			if t := fit(lastRunes(pc.Text, previousCap), b.Left()-64, true); strings.TrimSpace(t) != "" {
				take(&prev, workspace.PartProse, "【上一章《"+p.Title+"》的结尾】", t)
			}
		}
	}
	var parts []string
	for _, p := range []part{volume, outline, prev, near, {text: tail}, far} {
		switch {
		case p.heading == "" && p.text == "":
		case p.heading == "":
			parts = append(parts, p.text)
		default:
			parts = append(parts, strings.TrimRight(p.heading+"\n"+p.text, "\n"))
		}
	}
	req = model.Request{System: system, Messages: []model.Message{{Role: workspace.RoleUser, Content: strings.Join(parts, "\n\n")}}}
	return req, b.Sent(rules+task, set), nil
}

// Predict continues the chapter of scene with one sentence. It asks for no
// reasoning and stops reading at the sentence's end.
func (w *Writer) Predict(ctx context.Context, scene Scene) (string, error) {
	req, sent, err := w.prepare(continueTask, scene, "【要求】请接着本章前文写下一句。")
	if err != nil {
		return "", err
	}
	req.MaxTokens = nextOutput
	req.Effort = model.EffortNone
	ctx, cancel := context.WithCancelCause(purpose(ctx, workspace.FeatureContinue, scene.Chapter, sent))
	defer cancel(nil)
	var got strings.Builder
	var line string
	_, err = w.Model.Complete(ctx, req, func(d model.Delta) {
		got.WriteString(d.Text)
		if s, done := sentence(got.String()); done {
			line = s
			cancel(workspace.ErrEnough)
		}
	})
	if line != "" {
		return line, nil
	}
	if err != nil {
		return "", err
	}
	s, _ := sentence(got.String())
	return s, nil
}

// purpose is ctx telling the AI record that its requests are for feature
// in chapter and send the parts of sent.
func purpose(ctx context.Context, feature, chapter string, sent *workspace.Sent) context.Context {
	p := workspace.Purpose{Feature: feature, Chapter: chapter}
	if sent != nil {
		p.Parts = sent.Parts
	}
	return workspace.WithPurpose(ctx, p)
}

// previous is the document before n in the order of the tree.
func previous(t workspace.Tree, n *workspace.Node) (*workspace.Node, bool) {
	var last, found *workspace.Node
	t.Walk(func(m *workspace.Node, _ int) {
		if m == n {
			found = last
		}
		if !m.Folder {
			last = m
		}
	})
	return found, found != nil
}

func lastRunes(s string, n int) string {
	r := []rune(s)
	return string(r[max(0, len(r)-n):])
}

func firstRunes(s string, n int) string {
	r := []rune(s)
	return string(r[:min(n, len(r))])
}

// fit cuts s to what budget tokens hold, keeping its end, or its start.
func fit(s string, budget int, end bool) string {
	for s != "" && prompt.Estimate(s) > budget {
		r := []rune(s)
		k := max(len(r)*2/3, len(r)-max((prompt.Estimate(s)-budget)*2/3, 1))
		if end {
			s = string(r[len(r)-k:])
		} else {
			s = string(r[:k])
		}
		if k == 0 || budget <= 16 {
			return ""
		}
	}
	return s
}

// closers may follow the end of a sentence and belong to it.
const closers = "”’」』）)】》"

// sentence returns the first sentence of s, on one line, and whether it
// ended.
func sentence(s string) (string, bool) {
	r := []rune(strings.TrimLeftFunc(s, unicode.IsSpace))
	for i := 0; i < len(r); i++ {
		if r[i] == '\n' {
			return strings.TrimSpace(string(r[:i])), i > 0
		}
		if !strings.ContainsRune("。！？!?…", r[i]) {
			continue
		}
		j := i + 1
		for j < len(r) && strings.ContainsRune("。！？!?…"+closers, r[j]) {
			j++
		}
		if j == len(r) {
			return string(r), false // more may close it
		}
		return string(r[:j]), true
	}
	return strings.TrimSpace(string(r)), false
}
