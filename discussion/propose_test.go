package discussion

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
	"writer/aiport"

	"writer/inference"
	"writer/model"
	"writer/workspace"
)

// shelf writes a volume with an outline, and in it three chapters, each
// with an outline and prose; it returns the IDs of the volume and the
// chapters.
func shelf(t *testing.T, e *Engine) (vol, one, two, three string) {
	t.Helper()
	node := func(parent, title string, folder bool, outline, text string) string {
		id, err := e.Store.NewChapter(parent, title, folder)
		if err != nil {
			t.Fatal(err)
		}
		if outline != "" {
			if err := e.Store.WriteOutline(id, outline); err != nil {
				t.Fatal(err)
			}
		}
		if text != "" {
			if _, err := e.Store.WriteChapter(id, text, nil); err != nil {
				t.Fatal(err)
			}
		}
		return id
	}
	vol = node("", "第一卷", true, "秋天，旧城。", "")
	one = node(vol, "第一章", false, "她回到旧城。", "雨下了一夜。")
	two = node(vol, "第二章", false, "停电。", "她推开门。\n灯灭了。")
	three = node(vol, "第三章", false, "", "她摸黑下楼。")
	return
}

// A talk opened to draft a chapter keeps to it: its requests carry the
// chapter's outlines, the chapter before and what it has, and the setting
// as it stands there, and the record counts them as drafting it. What the
// reply proposes is kept with the call, sent again with the reply, and
// what came of it is told with the author's next words.
func TestATalkOpenedToDraftKeepsToItsChapter(t *testing.T) {
	e, session := setup(t)
	_, _, two, three := shelf(t, e)
	r := revise(t, e, workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Names: []string{"林晚"}, Text: "林晚用左手拧灯座。"})
	revise(t, e, workspace.Op{Op: workspace.OpChange, Entry: r.Ops[0].Entry, From: three, Place: workspace.PlaceBody, Text: "林晚没了左手。"})
	f := &fake{steps: []model.Reply{
		{Calls: []model.Call{call("c1", toolPropose, map[string]any{"chapter": "第二章", "at": "end", "text": "  灯又亮了。\n\n　她笑了。 "})}},
		{Text: "接着写了两段。"},
	}, replies: []string{"那就不笑。"}}
	e.Talk = inference.Model{Model: limits, Completer: f}
	if _, err := e.Say(context.Background(), session, workspace.Event{Text: "按章纲起草本章，约 800 字，接在已有内容之后", Origin: workspace.AIDraft, Chapter: two}); err != nil {
		t.Fatal(err)
	}
	asked := f.calls[0].System
	for _, want := range []string{originRules[workspace.AIDraft], "【这段讨论起草的章：第二章】\n章纲：\n停电。", "【所在卷：第一卷】卷纲：\n秋天，旧城。",
		"【本卷前面各章的章纲】\n第一章：\n她回到旧城。", "【上一章全文：第一章】\n雨下了一夜。", "【本章已有的正文】\n她推开门。\n灯灭了。",
		"林晚用左手拧灯座。", "以上是写到「第二章」时的设定；后面的章节里还有 1 处变化"} {
		if !strings.Contains(asked, want) {
			t.Fatalf("lacks %q:\n%s", want, asked)
		}
	}
	if strings.Contains(asked, "没了左手") || strings.Contains(asked, besideRules) {
		t.Fatalf("sent the setting after the chapter, or the rules beside the prose:\n%s", asked)
	}
	for _, p := range f.purposes {
		if p.Feature != workspace.FeatureDraft || p.Chapter != two || p.Talk != session {
			t.Fatalf("the record would say %+v", p)
		}
	}
	events, err := e.Store.Events(session)
	if err != nil {
		t.Fatal(err)
	}
	res, reply := events[2], events[3]
	if p := res.Proposal; p == nil || *p != (workspace.Proposal{Chapter: two, Title: "第二章", At: workspace.ProposeEnd, Text: "灯又亮了。\n她笑了。"}) || events[1].Calls[0].Label != "提议修改「第二章」" {
		t.Fatalf("result %+v", res)
	}
	var items []string
	for _, it := range reply.Context {
		if it.ID == "" {
			items = append(items, it.Text)
		}
	}
	if !slices.Contains(items, "上一章全文：第一章 6 字") || !slices.Contains(items, "本章已有正文 9 字") || reply.Sent.AsOf != "第二章" || reply.Sent.Ahead != 1 {
		t.Fatalf("told the author it sent %v, as of %q with %d ahead", items, reply.Sent.AsOf, reply.Sent.Ahead)
	}
	if _, err := e.Store.Append(session, workspace.Event{Role: workspace.RoleFate, Call: "c1", Fate: workspace.FateTaken}); err != nil {
		t.Fatal(err)
	}
	turn, err := e.Say(context.Background(), session, workspace.Event{Text: "结尾别让她笑"})
	if err != nil {
		t.Fatal(err)
	}
	if turn.User.Told != "「第二章」接在章末的提议：已采纳。" || turn.User.Origin != "" {
		t.Fatalf("told %q", turn.User.Told)
	}
	msgs := f.calls[2].Messages
	if len(msgs) != 3 || msgs[1].Content != "接着写了两段。\n\n【这一轮在正文里的提议】\n「第二章」接在章末：\n灯又亮了。\n她笑了。" ||
		msgs[2].Content != "【之前的提议】「第二章」接在章末的提议：已采纳。\n\n结尾别让她笑" || !strings.Contains(f.calls[2].System, "【本章已有的正文】") {
		t.Fatalf("messages %+v", msgs)
	}
	if _, err := e.Say(context.Background(), session, workspace.Event{Text: "再起草", Origin: workspace.AIDraft, Chapter: two}); err == nil {
		t.Fatal("a talk under way was opened again")
	}
	if err := e.Store.TrashChapter(two); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Say(context.Background(), session, workspace.Event{Text: "还在吗"}); err == nil || !strings.Contains(err.Error(), "chapter no longer exists") {
		t.Fatalf("went on with the chapter gone: %v", err)
	}
}

// A talk opened to rewrite carries the chapter whole, and looks the
// setting up as it stands there.
func TestATalkOpenedToRewriteKeepsToItsChapter(t *testing.T) {
	e, session := setup(t)
	_, _, two, three := shelf(t, e)
	r := revise(t, e, workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Names: []string{"林晚"}, Text: "林晚用左手拧灯座。"})
	revise(t, e, workspace.Op{Op: workspace.OpChange, Entry: r.Ops[0].Entry, From: three, Place: workspace.PlaceBody, Text: "林晚没了左手。"})
	f := &fake{replies: []string{"好"}}
	e.Talk = inference.Model{Model: limits, Completer: f}
	words := workspace.Event{Text: "放慢", Origin: workspace.AIRewrite, Chapter: two, Quotes: []workspace.Quote{{Chapter: two, Title: "第二章", Text: "灯灭了。"}}}
	if _, err := e.Say(context.Background(), session, words); err != nil {
		t.Fatal(err)
	}
	asked := f.calls[0]
	if !strings.Contains(asked.System, originRules[workspace.AIRewrite]) || !strings.Contains(asked.System, "【这段讨论改写的章：第二章】") ||
		!strings.Contains(asked.System, "【本章全文】\n她推开门。\n灯灭了。") || strings.Contains(asked.System, "上一章全文") ||
		f.purposes[0].Feature != workspace.FeatureRewrite {
		t.Fatalf("asked %s", asked.System)
	}
	box := &toolbox{allowed: tools(workspace.Work{}), store: e.Store, session: session, model: e.Talk, at: two}
	if out := box.lookUp(lookUpArgs{Query: "林晚"}); !strings.Contains(out.text, "左手拧灯座") || strings.Contains(out.text, "没了左手") {
		t.Fatalf("looked up %s", out.text)
	}
	if n, err := Preloaded(e.Store, aiport.Material{Setting: true, Outline: true}, workspace.AIDraft, two); err != nil || n.Tokens < 1000 {
		t.Fatalf("preloaded %d, %v", n.Tokens, err)
	}
}

// A proposal finds its place: the chapter named, and in it the words it
// replaces or follows, there once.
func TestAProposalFindsItsPlace(t *testing.T) {
	e, session := setup(t)
	_, _, two, _ := shelf(t, e)
	if _, err := e.Store.WriteChapter(two, "她推开门。\n灯灭了。\n灯灭了。她没动。", nil); err != nil {
		t.Fatal(err)
	}
	box := &toolbox{allowed: tools(workspace.Work{}), store: e.Store, session: session}
	for _, c := range []struct {
		in   proposeArgs
		want string // in the refusal
		got  workspace.Proposal
	}{
		{in: proposeArgs{Chapter: "第二章", At: "replace", Quote: "“她推开门”", Text: "她撞开门。"}, got: workspace.Proposal{Chapter: two, Title: "第二章", At: "replace", Quote: "她推开门", Text: "她撞开门。"}},
		{in: proposeArgs{Chapter: "第二章", At: "after", Quote: "她没动。", Text: "\n她等着。"}, got: workspace.Proposal{Chapter: two, Title: "第二章", At: "after", Quote: "她没动。", Text: "\n她等着。"}},
		{in: proposeArgs{Chapter: "第二章", At: "all", Text: "全新的一章。"}, got: workspace.Proposal{Chapter: two, Title: "第二章", At: "all", Text: "全新的一章。"}},
		{in: proposeArgs{Chapter: "第二章", At: "replace", Quote: "灯灭了。", Text: "x"}, want: "出现了 2 次"},
		{in: proposeArgs{Chapter: "第二章", At: "replace", Quote: "她跑了", Text: "x"}, want: "找不到 quote"},
		{in: proposeArgs{Chapter: "第二章", At: "after", Text: "x"}, want: "要在 quote 里逐字复制"},
		{in: proposeArgs{Chapter: "第一卷", At: "end", Text: "x"}, want: "是一卷"},
		{in: proposeArgs{Chapter: "第九章", At: "end", Text: "x"}, want: "没有叫「第九章」的章"},
		{in: proposeArgs{Chapter: "第二章", At: "middle", Text: "x"}, want: "at 只能是"},
		{in: proposeArgs{Chapter: "第二章", At: "end", Text: " \n "}, want: "text 要写新的正文"},
	} {
		out := box.propose(c.in)
		switch {
		case c.want != "" && (!out.failed || !strings.Contains(out.text, c.want) || out.proposal != nil):
			t.Errorf("%+v: %s", c.in, out.text)
		case c.want == "" && (out.failed || out.proposal == nil || *out.proposal != c.got):
			t.Errorf("%+v: %s %+v", c.in, out.text, out.proposal)
		}
	}
}

// A proposal shows as its call streams: once its place is found, then as
// its text grows, a piece of JSON at a time, escapes cut in two and all.
func TestAProposalShowsAsItStreams(t *testing.T) {
	e, _ := setup(t)
	_, _, two, _ := shelf(t, e)
	var told []workspace.Proposal
	s := &proposing{store: e.Store, tell: func(call string, p workspace.Proposal) {
		if call != "c1" {
			t.Fatalf("told of %s", call)
		}
		told = append(told, p)
	}}
	for _, piece := range []string{`{"chapter":"第二`, `章","at":"rep`, `lace","quote":"灯灭`, `了","te`, `xt":"灯`, `闪了一下\`, `n又灭了`, `"}`} {
		s.feed(model.CallDelta{ID: "c1", Name: map[bool]string{true: toolPropose}[piece[0] == '{'], Input: piece})
	}
	s.feed(model.CallDelta{ID: "c2", Name: toolSearchBook, Input: `{"query":"灯"}`})
	var texts []string
	for _, p := range told {
		if p.Chapter != two || p.At != workspace.ProposeReplace || p.Quote != "灯灭了" {
			t.Fatalf("told %+v", p)
		}
		texts = append(texts, p.Text)
	}
	if strings.Join(texts, "|") != "|灯|灯闪了一下|灯闪了一下\n又灭了" {
		t.Fatalf("told %q", texts)
	}
	told = nil
	s.feed(model.CallDelta{ID: "c3", Name: toolPropose, Input: `{"chapter":"第二章","at":"replace","quote":"她跑了","text":"x"}`})
	if len(told) != 0 {
		t.Fatalf("showed a proposal with no place: %+v", told)
	}
}

func TestPartialJSON(t *testing.T) {
	got := partialJSON(`{"n": 3, "list": ["a,}", {"b": 1}], "s": "\u6797\"晚\"", "cut": "半`)
	if got["s"] != (field{`林"晚"`, true}) || got["cut"] != (field{"半", false}) || len(got) != 2 {
		t.Fatalf("%+v", got)
	}
	if got := partialJSON(`{"a":"x\u67`); got["a"] != (field{"x", false}) {
		t.Fatalf("%+v", got)
	}
}

// This whole-prefix oracle keeps preview behavior at incomplete escapes and
// UTF-8 boundaries independent of the incremental parser's state machine.
func referencePartialJSON(s string) map[string]field {
	out := map[string]field{}
	i := 0
	space := func() {
		for i < len(s) && strings.IndexByte(" \t\r\n", s[i]) >= 0 {
			i++
		}
	}
	// str reads a string at i, past its opening quote: as far as it goes,
	// and whether it closed.
	str := func() (string, bool) {
		var b strings.Builder
		for i < len(s) {
			switch c := s[i]; c {
			case '"':
				i++
				return b.String(), true
			case '\\':
				if i+1 >= len(s) {
					return b.String(), false
				}
				switch e := s[i+1]; e {
				case 'n':
					b.WriteByte('\n')
				case 't':
					b.WriteByte('\t')
				case 'r':
					b.WriteByte('\r')
				case 'b', 'f':
				case 'u':
					if i+6 > len(s) {
						return b.String(), false
					}
					var r rune
					if _, err := fmt.Sscanf(s[i+2:i+6], "%04x", &r); err == nil {
						if utf8.ValidRune(r) {
							b.WriteRune(r)
						}
					}
					i += 4
				default:
					b.WriteByte(e)
				}
				i += 2
			default:
				b.WriteByte(c)
				i++
			}
		}
		return b.String(), false
	}
	// skip passes over a value not a string: up to the comma or the brace
	// that ends it at its own depth.
	skip := func() {
		depth := 0
		for i < len(s) {
			switch s[i] {
			case '"':
				i++
				str()
				continue
			case '{', '[':
				depth++
			case '}', ']':
				if depth == 0 {
					return
				}
				depth--
			case ',':
				if depth == 0 {
					return
				}
			}
			i++
		}
	}
	space()
	if i >= len(s) || s[i] != '{' {
		return out
	}
	i++
	for {
		space()
		if i >= len(s) || s[i] != '"' {
			return out
		}
		i++
		key, ok := str()
		if !ok {
			return out
		}
		space()
		if i >= len(s) || s[i] != ':' {
			return out
		}
		i++
		space()
		if i >= len(s) {
			return out
		}
		if s[i] == '"' {
			i++
			v, done := str()
			out[key] = field{v, done}
			if !done {
				return out
			}
		} else {
			skip()
		}
		space()
		if i >= len(s) || s[i] != ',' {
			return out
		}
		i++
	}
}

func TestPartialJSONChunkBoundaries(t *testing.T) {
	samples := []string{
		`{"chapter":"第二章","at":"after","quote":"灯灭了","text":" \n  林晚\n\n　走过雨中的街道。\r\n abc\t "}`,
		`{"text":"\u6797\u665a\n\"灯\" \\ \/ \b\f\t\r\n\ud83d\ude00"}`,
		`{"n": 3, "list": ["a,}", {"b": 1}], "s": "x", "text":"甲😀乙\n丙"}`,
		`{"text":"第一段\n末尾", "text":"换成\n新段落", "at":"after"}`,
		`{"text":"abc\u67xz\q", "skip":{"x":"\"},["},"text":""}`,
		` { "text" : "　\r\n 甲\n\n乙 \t" } trailing`,
		`{"text":"`, `{"text":,"later":"x"}`, `{"x":"a" "text":"b"}`, `[]`,
	}
	check := func(s string, chunks []int) {
		t.Helper()
		var parsed partialObject
		var prose streamedProse
		at := 0
		for _, size := range chunks {
			next := min(len(s), at+size)
			parsed.feed(s[at:next])
			got, want := parsed.fields(), referencePartialJSON(s[:next])
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("prefix %q, chunks %v: got %+v, want %+v", s[:next], chunks, got, want)
			}
			lead := got["at"].value == workspace.ProposeAfter
			if got, want := prose.clean(got["text"].value, parsed.textVersion, lead), cleanProse(want["text"].value, lead); got != want {
				t.Fatalf("clean prefix %q: got %q, want %q", s[:next], got, want)
			}
			at = next
		}
	}
	rng := rand.New(rand.NewPCG(3, 7))
	for _, s := range samples {
		for split := 0; split <= len(s); split++ {
			check(s, []int{split, len(s) - split})
		}
		for range 30 {
			var chunks []int
			for total := 0; total < len(s); {
				n := rng.IntN(9) + 1
				chunks = append(chunks, n)
				total += n
			}
			check(s, chunks)
		}
		chunks := make([]int, len(s))
		for i := range chunks {
			chunks[i] = 1
		}
		check(s, chunks)
	}
}

func TestFixedOutlinesIgnoreDamagedMarks(t *testing.T) {
	e, _ := setup(t)
	_, _, _, three := shelf(t, e)
	path := filepath.Join(e.Store.Root(), "chapters", "第一卷", "第一章.marks.json")
	if err := os.WriteFile(path, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	tree, err := e.Store.Chapters()
	if err != nil {
		t.Fatal(err)
	}
	fixed, err := fixedOf(e.Store, aiport.Material{Setting: true, Outline: true}, tree, workspace.AIDraft, three)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(fixed.Pieces, func(p Piece) bool {
		return p.Part == workspace.PartOutlines && strings.Contains(p.Text, "她回到旧城。")
	}) {
		t.Fatalf("outlines = %+v", fixed.Pieces)
	}
}

func TestFixedOmitsOutlinesOnBatchFailure(t *testing.T) {
	e, _ := setup(t)
	_, _, _, three := shelf(t, e)
	path := filepath.Join(e.Store.Root(), "chapters", "第一卷", "第一章.outline.txt")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	tree, err := e.Store.Chapters()
	if err != nil {
		t.Fatal(err)
	}
	fixed, err := fixedOf(e.Store, aiport.Material{Setting: true, Outline: true}, tree, workspace.AIDraft, three)
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(fixed.Pieces, func(p Piece) bool {
		return p.Part == workspace.PartOutlines || p.Part == workspace.PartOutline && !strings.HasPrefix(p.Text, "【这段讨论")
	}) {
		t.Fatalf("outlines survived a failed batch: %+v", fixed.Pieces)
	}
	if !slices.ContainsFunc(fixed.Pieces, func(p Piece) bool { return strings.HasPrefix(p.Text, "【这段讨论起草的章") }) {
		t.Fatalf("the chapter's own outline was lost: %+v", fixed.Pieces)
	}
	if !slices.ContainsFunc(fixed.Pieces, func(p Piece) bool {
		return p.Part == workspace.PartChapter && strings.Contains(p.Text, "她摸黑下楼。")
	}) {
		t.Fatalf("chapter was lost: %+v", fixed.Pieces)
	}
}

func partialJSON(s string) map[string]field {
	var p partialObject
	p.feed(s)
	return p.fields()
}
