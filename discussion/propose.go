package discussion

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"writer/internal/textclip"
	"writer/model"
	"writer/workspace"
)

// toolPropose proposes a change of the prose: shown in the chapter, it
// goes in only when the author takes it.
const toolPropose = workspace.ProposeTool

// proposeTool is the tool's description and input. The input lists where
// before what: written in that order, the change shows in its place while
// its text streams.
var proposeTool = model.Tool{Name: toolPropose,
	Description: "在正文里提议一处修改，显示在那一章里，作者采纳了才写进正文。一次提议一处。参数按 chapter、at、quote、text 的次序写。",
	Schema: json.RawMessage(`{"type":"object","properties":{` +
		`"chapter":{"type":"string","description":"改哪一章，章名和搜全书结果里写的一样"},` +
		`"at":{"type":"string","enum":["replace","after","end","all"],"description":"replace 换掉 quote 那段原文；after 接在 quote 之后，在同一段里接着写，text 以换行开头就另起一段；end 接在章末；all 换掉全章"},` +
		`"quote":{"type":"string","description":"at 是 replace 或 after 时，逐字复制那一章里的原文，在那一章里只出现一次"},` +
		`"text":{"type":"string","description":"新写的正文，一段一行"}` +
		`},"required":["chapter","at","text"]}`)}

// atNames say where a proposal goes, as the model is told and the author
// shown.
var atNames = map[string]string{
	workspace.ProposeReplace: "换掉一段",
	workspace.ProposeAfter:   "接在一句之后",
	workspace.ProposeEnd:     "接在章末",
	workspace.ProposeAll:     "换掉全章",
}

// propose finds the place of a change the model proposes: the chapter, and
// in it the words the change replaces or follows, which must be there
// once. Found, the change shows in the chapter until the author takes it
// or not.
func (t *toolbox) propose(in input) outcome {
	b, err := t.book()
	if err != nil {
		return failed("读不出章节：%v", err)
	}
	p, why := place(t.store, b, in.Chapter, in.At, in.Quote)
	if why != "" {
		return failed("%s", why)
	}
	if p.Text = cleanProse(in.Text, in.At == workspace.ProposeAfter); strings.TrimSpace(p.Text) == "" {
		return failed("text 要写新的正文")
	}
	return outcome{text: "已在「" + p.Title + "」的正文里显示，等作者决定；作者采纳或放弃了，下次说话时会告诉你。", proposal: &p}
}

// place finds where a change of the prose goes: in the chapter named, at,
// after or in place of quote, which it holds once. It says why not when
// it cannot.
func place(st *workspace.Store, b book, chapter, at, quote string) (workspace.Proposal, string) {
	if at != workspace.ProposeReplace && at != workspace.ProposeAfter && at != workspace.ProposeEnd && at != workspace.ProposeAll {
		return workspace.Proposal{}, "at 只能是 replace、after、end、all"
	}
	n, missing := b.find(chapter)
	if n == nil {
		return workspace.Proposal{}, missing
	}
	name := b.name[n.ID]
	if n.Folder {
		return workspace.Proposal{}, "「" + name + "」是一卷，没有正文；提议改它下面的章。"
	}
	p := workspace.Proposal{Chapter: n.ID, Title: name, At: at}
	if at != workspace.ProposeReplace && at != workspace.ProposeAfter {
		return p, ""
	}
	if strings.TrimSpace(quote) == "" {
		return p, at + " 要在 quote 里逐字复制「" + name + "」里的原文"
	}
	ch, err := st.ReadChapter(n.ID)
	if err != nil {
		return p, fmt.Sprintf("读不出「%s」：%v", name, err)
	}
	words, ok := locate(ch.Text, strings.TrimSpace(quote))
	if !ok {
		return p, "「" + name + "」里找不到 quote 这段原文；要逐字复制，可以先用 read_prose 读一下。"
	}
	if k := strings.Count(ch.Text, words); k > 1 {
		return p, fmt.Sprintf("quote 这段原文在「%s」里出现了 %d 次；多引几个字，让它只出现一次。", name, k)
	}
	p.Quote = words
	return p, ""
}

// fateWords say what came of a proposal as the model is told. That a later
// proposal took its place it knows: it proposed it.
var fateWords = map[string]string{
	workspace.FateTaken:   "已采纳",
	workspace.FateDropped: "被作者放弃",
	workspace.FateVoid:    "作者改动了它要换掉的原文，已作废",
	workspace.FateClosed:  "作者关闭作品时丢弃了",
	workspace.FateOff:     "作者关闭 AI 时丢弃了",
}

// told is what the model is told with the author's next words of the talk
// of events: what came of each change it proposed in the prose since the
// author last spoke, a line each.
func told(events []workspace.Event) string {
	since := 0
	for n, e := range events {
		if e.Role == workspace.RoleUser {
			since = n + 1
		}
	}
	proposed := map[string]workspace.Proposal{}
	for _, e := range events {
		if e.Role == workspace.RoleResult && e.Proposal != nil {
			proposed[e.Call] = *e.Proposal
		}
	}
	var lines []string
	for _, e := range events[since:] {
		p, ok := proposed[e.Call]
		if e.Role != workspace.RoleFate || !ok || fateWords[e.Fate] == "" {
			continue
		}
		where := atNames[p.At]
		switch p.At {
		case workspace.ProposeReplace:
			where = "换掉「" + textclip.Clip(p.Quote, 12) + "」"
		case workspace.ProposeAfter:
			where = "接在「" + textclip.Clip(p.Quote, 12) + "」之后"
		}
		lines = append(lines, "「"+p.Title+"」"+where+"的提议："+fateWords[e.Fate]+"。")
	}
	return strings.Join(lines, "\n")
}

// proposing follows the calls of a reply that propose a change of the
// prose as their input streams: it tells each once its place is found,
// then again each time its text grows.
type proposing struct {
	store *workspace.Store
	tell  func(call string, p workspace.Proposal)
	calls map[string]*streaming
}

// streaming is a call as its input comes.
type streaming struct {
	name  string
	input partialObject
	prose streamedProse
	found *workspace.Proposal // its place, once found
	tried string              // where it was last looked for, not to look again
	told  *string             // the text last told, once told
}

func (s *proposing) feed(d model.CallDelta) {
	if s.calls == nil {
		s.calls = map[string]*streaming{}
	}
	c := s.calls[d.ID]
	if c == nil {
		c = &streaming{}
		s.calls[d.ID] = c
	}
	if d.Name != "" {
		c.name = d.Name
	}
	c.input.feed(d.Input)
	if c.name != toolPropose || s.tell == nil {
		return
	}
	got := c.input.fields()
	chapter, at, quote := got["chapter"], got["at"], got["quote"]
	if c.found == nil {
		quoted := at.value == workspace.ProposeReplace || at.value == workspace.ProposeAfter
		if !chapter.done || !at.done || quoted && !quote.done {
			return
		}
		key := chapter.value + "\x00" + at.value + "\x00" + quote.value
		if key == c.tried {
			return
		}
		c.tried = key
		tree, err := s.store.Chapters()
		if err != nil {
			return
		}
		p, why := place(s.store, bookOf(tree), strings.TrimSpace(chapter.value), at.value, quote.value)
		if why != "" {
			return // the call says why, once it is whole
		}
		c.found = &p
	}
	text := c.prose.clean(got["text"].value, c.input.textVersion, at.value == workspace.ProposeAfter)
	if c.told != nil && *c.told == text {
		return
	}
	c.told = &text
	p := *c.found
	p.Text = text
	s.tell(d.ID, p)
}

// field is a string an object of JSON holds, as far as it came, and
// whether it came whole.
type field struct {
	value string
	done  bool
}

type objectState uint8

const (
	objectStart objectState = iota
	objectKey
	objectColon
	objectValue
	objectAfter
	objectSkip
	objectStopped
)

type stringKind uint8

const (
	noString stringKind = iota
	keyString
	valueString
	skippedString
)

type pendingEscape struct {
	active  bool
	unicode bool
	digits  []byte
}

// partialObject deliberately has the preview's permissive string rules.
// The tool still validates the complete JSON before it can write anything.
type partialObject struct {
	state       objectState
	out         map[string]field
	key         string
	text        strings.Builder
	stringKind  stringKind
	escape      pendingEscape
	depth       int
	textVersion int
}

func (p *partialObject) fields() map[string]field {
	if p.stringKind == valueString {
		p.out[p.key] = field{p.text.String(), false}
	}
	return p.out
}

func (p *partialObject) feed(s string) {
	if p.out == nil {
		p.out = map[string]field{}
	}
	for i := 0; i < len(s); {
		c := s[i]
		if p.stringKind != noString {
			i++
			if p.escape.active {
				if !p.escape.unicode && c == 'u' {
					p.escape.unicode = true
					continue
				}
				if p.escape.unicode {
					p.escape.digits = utf8.AppendRune(p.escape.digits, rune(c))
					if len(p.escape.digits) < 4 {
						continue
					}
					var r rune
					if _, err := fmt.Sscanf(string(p.escape.digits), "%04x", &r); err == nil && utf8.ValidRune(r) {
						p.text.WriteRune(r)
					}
				} else {
					switch c {
					case 'n':
						p.text.WriteByte('\n')
					case 'r':
						p.text.WriteByte('\r')
					case 't':
						p.text.WriteByte('\t')
					case 'b', 'f':
					default:
						p.text.WriteByte(c)
					}
				}
				p.escape = pendingEscape{}
				continue
			}
			switch c {
			case '\\':
				p.escape.active = true
			case '"':
				switch p.stringKind {
				case keyString:
					p.key, p.state = p.text.String(), objectColon
				case valueString:
					p.out[p.key], p.state = field{p.text.String(), true}, objectAfter
				}
				p.stringKind = noString
			default:
				p.text.WriteByte(c)
			}
			continue
		}
		if p.state != objectSkip && strings.IndexByte(" \t\r\n", c) >= 0 {
			i++
			continue
		}
		switch p.state {
		case objectStart:
			if c != '{' {
				p.state = objectStopped
				return
			}
			p.state = objectKey
		case objectKey:
			if c != '"' {
				p.state = objectStopped
				return
			}
			p.text.Reset()
			p.stringKind = keyString
		case objectColon:
			if c != ':' {
				p.state = objectStopped
				return
			}
			p.state = objectValue
		case objectValue:
			if c == '"' {
				p.text.Reset()
				p.stringKind = valueString
				if p.key == "text" {
					p.textVersion++
				}
			} else {
				p.state, p.depth = objectSkip, 0
				continue
			}
		case objectAfter:
			if c != ',' {
				p.state = objectStopped
				return
			}
			p.state = objectKey
		case objectSkip:
			switch c {
			case '"':
				p.stringKind = skippedString
				p.text.Reset()
			case '{', '[':
				p.depth++
			case '}', ']':
				if p.depth == 0 {
					p.state = objectAfter
					continue
				}
				p.depth--
			case ',':
				if p.depth == 0 {
					p.state = objectAfter
					continue
				}
			}
		case objectStopped:
			return
		}
		i++
	}
}

type streamedProse struct {
	version, read int
	done, pending strings.Builder
}

func (p *streamedProse) clean(raw string, version int, lead bool) string {
	if version != p.version {
		*p = streamedProse{version: version}
	}
	more := raw[p.read:]
	p.read = len(raw)
	for {
		line, rest, newline := strings.Cut(more, "\n")
		p.pending.WriteString(line)
		if !newline {
			break
		}
		if line := cleanProse(p.pending.String(), false); line != "" {
			if p.done.Len() > 0 {
				p.done.WriteByte('\n')
			}
			p.done.WriteString(line)
		}
		p.pending.Reset()
		more = rest
	}
	text := p.done.String()
	if tail := cleanProse(p.pending.String(), false); tail != "" {
		if text != "" {
			text += "\n"
		}
		text += tail
	}
	if lead && strings.HasPrefix(strings.TrimLeft(raw, " \t　\r"), "\n") && text != "" {
		text = "\n" + text
	}
	return text
}
