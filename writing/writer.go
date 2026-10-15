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

// A draft and a rewrite may take all the output the model gives; a
// sentence more takes little.
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

// Job is a request ready to run, and the record of what it shows.
type Job struct {
	Request model.Request
	Record  workspace.AIRecord
}

// Errors the author can act on.
var (
	ErrNoOutline = errors.New("章纲是空的：先写下这一章要发生什么，再让 AI 起草")
	ErrNoModel   = errors.New("请在设置中「AI」配置里选择写作模型")
)

// prepare builds the request of task for scene: the rules and the setting
// in the system prompt, the outlines and the text around the place in the
// message, as far as the budget allows.
func (w *Writer) prepare(op, task string, scene Scene, tail string) (Job, error) {
	job := Job{Record: workspace.AIRecord{Op: op, Model: w.Model.Name, Effort: string(w.Model.Effort)}}
	if w.Model.Completer == nil {
		return job, ErrNoModel
	}
	work := w.Store.Work()
	tree, err := w.Store.Chapters()
	if err != nil {
		return job, err
	}
	n, ok := tree.Node(scene.Chapter)
	if !ok || n.Folder {
		return job, errors.New("找不到这一章")
	}
	ch, err := w.Store.ReadChapter(n.ID)
	if err != nil {
		return job, err
	}
	set, err := w.Store.Setting()
	if err != nil {
		return job, err
	}
	b := prompt.NewBudget(w.Model.Input())
	b.Must(workspace.PartRules, rules+task)
	b.Must(workspace.PartInput, tail)
	// The parts of the message, in the order it reads; each says what it
	// shows for the record.
	type part struct{ heading, text, shown string }
	var volume, recap, outline, prev, near, far part
	take := func(p *part, kind, heading, text, shown string) {
		*p = part{heading, text, shown}
		b.Must(kind, heading+text)
	}
	// The outlines go whole, or the request does not.
	if p := n.Parent; p != nil && p.Parent != nil {
		if vol, err := w.Store.ReadChapter(p.ID); err == nil && strings.TrimSpace(vol.Outline) != "" {
			take(&volume, workspace.PartOutline, "【所在卷："+p.Title+"】卷纲：", strings.TrimSpace(vol.Outline), "卷纲："+p.Title)
		}
	}
	if strings.TrimSpace(ch.Outline) != "" {
		take(&outline, workspace.PartOutline, "【本章："+n.Title+"】章纲：", strings.TrimSpace(ch.Outline), "章纲："+n.Title)
	} else {
		outline = part{heading: "【本章：" + n.Title + "】"}
	}
	if b.Left() < 0 {
		return job, errors.New("纲和要求超出了模型的上下文上限：请缩短，或在设置中「AI」配置里调大上下文上限")
	}
	if t := fit(lastRunes(scene.Before, beforeCap), b.Left(), true); strings.TrimSpace(t) != "" {
		take(&near, workspace.PartProse, "【本章前文】", t, fmt.Sprintf("本章前文 %d 字", workspace.Count(t)))
	}
	if t := fit(firstRunes(scene.After, afterCap), b.Left(), false); strings.TrimSpace(t) != "" {
		take(&far, workspace.PartProse, "【本章后文】", t, fmt.Sprintf("本章后文 %d 字", workspace.Count(t)))
	}

	// The setting: the body binds the prose, then what the author dropped;
	// the newest first when it does not all fit.
	shown, left := b.Fit(set, []string{workspace.PlaceBody, workspace.PlaceDropped}, prompt.Whole)
	for _, p := range writePlaces {
		for _, e := range prompt.Ordered(work, set.Entries) {
			if e.Place == p.Place && shown[e.ID] {
				job.Record.Setting = append(job.Record.Setting, p.Item(e, prompt.Whole))
			}
		}
		for _, e := range left {
			if e.Place == p.Place {
				item := p.Item(e, prompt.Whole)
				item.Reason = "超出上下文上限"
				job.Record.Omitted = append(job.Record.Omitted, item)
			}
		}
	}
	system := rules + "\n\n" + prompt.Render(work, set, writePlaces, shown, prompt.Whole) + "\n\n" + task

	// What the chapters before told, for a draft, which sees no more of
	// them than the previous one's end.
	if op == workspace.AIDraft {
		text, chapters, stale, err := w.recap(tree, n, b.Left()-previousCap/2)
		if err != nil {
			return job, err
		}
		if chapters > 0 {
			shown := fmt.Sprintf("前情提要 %d 章", chapters)
			if stale > 0 {
				shown += fmt.Sprintf("（其中 %d 章正文改过，摘要可能过时）", stale)
			}
			take(&recap, workspace.PartRecap, "【前情提要】（前面各章的摘要，按顺序）", text, shown)
		}
	}

	// The previous chapter's end, while this one has little before.
	if p, ok := previous(tree, n); ok && len([]rune(scene.Before)) < beforeCap {
		if pc, err := w.Store.ReadChapter(p.ID); err == nil {
			if t := fit(lastRunes(pc.Text, previousCap), b.Left()-64, true); strings.TrimSpace(t) != "" {
				take(&prev, workspace.PartProse, "【上一章《"+p.Title+"》的结尾】", t, fmt.Sprintf("上一章《%s》结尾 %d 字", p.Title, workspace.Count(t)))
			}
		}
	}
	var parts []string
	for _, p := range []part{volume, recap, outline, prev, near, {text: tail}, far} {
		switch {
		case p.heading == "" && p.text == "":
		case p.heading == "":
			parts = append(parts, p.text)
		default:
			parts = append(parts, strings.TrimRight(p.heading+"\n"+p.text, "\n"))
		}
		if p.shown != "" {
			job.Record.Shown = append(job.Record.Shown, p.shown)
		}
	}
	job.Request = model.Request{System: system, Messages: []model.Message{{Role: workspace.RoleUser, Content: strings.Join(parts, "\n\n")}}}
	job.Record.Sent = b.Sent(rules+task, set)
	return job, nil
}

// Draft prepares drafting the chapter of scene from its outline, into
// about words characters, or as many as the outline wants for 0. With
// text before, it goes on from there.
func (w *Writer) Draft(scene Scene, words int) (Job, error) {
	ch, err := w.Store.ReadChapter(scene.Chapter)
	if err != nil {
		return Job{}, err
	}
	if strings.TrimSpace(ch.Outline) == "" {
		return Job{}, ErrNoOutline
	}
	ask := "请按章纲写出这一章。"
	if strings.TrimSpace(scene.Before) != "" {
		ask = "请接着本章前文，按章纲把这一章写完；前文已经写过的情节不要重复。"
	}
	if words > 0 {
		ask += fmt.Sprintf("篇幅约 %d 字。", words)
	}
	job, err := w.prepare(workspace.AIDraft, draftTask, scene, "【要求】"+ask)
	job.Request.Effort = w.Model.Effort
	if words > 0 {
		job.Record.Instruction = fmt.Sprintf("约 %d 字", words)
	}
	return job, err
}

// Rewrite prepares rewriting selected, the words between the scene's
// before and after, as the instruction asks.
func (w *Writer) Rewrite(scene Scene, selected, instruction string) (Job, error) {
	if strings.TrimSpace(instruction) == "" {
		return Job{}, errors.New("请写下要怎么改")
	}
	n := workspace.Count(selected)
	tail := fmt.Sprintf("【选中的原文】（%d 字）\n%s\n\n【作者的要求】\n%s\n\n【篇幅】改写后不超过 %d 字，除非作者的要求里另有字数。",
		n, selected, strings.TrimSpace(instruction), max(3*n, n+40))
	job, err := w.prepare(workspace.AIRewrite, rewriteTask, scene, tail)
	job.Request.Effort = w.Model.Effort
	job.Record.Instruction = strings.TrimSpace(instruction)
	return job, err
}

// Run runs job, passing the prose to onText as it streams, cleaned as the
// chapter stores it, and returns all of it. What came before a failure is
// returned with the error.
func (w *Writer) Run(ctx context.Context, job Job, onText func(string)) (string, error) {
	var p prose
	var out strings.Builder
	_, err := w.Model.Complete(ctx, job.Request, func(d model.Delta) {
		if s := p.feed(d.Text); s != "" {
			out.WriteString(s)
			if onText != nil {
				onText(s)
			}
		}
	})
	if err != nil && out.Len() > 0 && ctx.Err() == nil {
		err = fmt.Errorf("没有写完：%w", err)
	}
	return out.String(), err
}

// Predict continues the chapter of scene with one sentence. It asks for no
// reasoning and stops reading at the sentence's end.
func (w *Writer) Predict(ctx context.Context, scene Scene) (string, error) {
	job, err := w.prepare("continue", continueTask, scene, "【要求】请接着本章前文写下一句。")
	if err != nil {
		return "", err
	}
	job.Request.MaxTokens = nextOutput
	job.Request.Effort = model.EffortNone
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var got strings.Builder
	var line string
	_, err = w.Model.Complete(ctx, job.Request, func(d model.Delta) {
		got.WriteString(d.Text)
		if s, done := sentence(got.String()); done {
			line = s
			cancel()
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

// prose cleans streamed text as a chapter stores it: no indent, no blank
// line, no line break at the start or the end.
type prose struct {
	started, newline bool
}

func (p *prose) feed(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\r':
		case r == '\n':
			p.newline = p.started
		case unicode.IsSpace(r) && (!p.started || p.newline):
		default:
			if p.newline {
				b.WriteByte('\n')
				p.newline = false
			}
			p.started = true
			b.WriteRune(r)
		}
	}
	return b.String()
}
