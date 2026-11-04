package discussion

import (
	"context"
	"slices"
	"strings"

	"writer/inference"
	"writer/model"
	"writer/prompt"
	"writer/workspace"
)

// ask runs a request on m, streaming to on when it is set.
func ask(ctx context.Context, m inference.Model, req model.Request, on func(model.Delta)) (model.Reply, error) {
	req.Effort = m.Effort
	return m.Complete(ctx, req, on)
}

// Engine runs the talk of one work: a reply may look things up in the book
// and note the setting with tools, a step at a time. Its owner serializes
// calls; Talk may be unset when no model is configured, which only the
// calls that need a model refuse.
type Engine struct {
	Store *workspace.Store
	Talk  inference.Model
	// Progress, when set, hears what the reply streams, never empty, and
	// Step each step's calls and each call's result as they are written,
	// and each digest compacting the talk: with no ID as it is asked for,
	// then as written. Both run on the caller's goroutine.
	Progress func(model.Delta)
	Step     func(workspace.Event)
	// Beside, when set, is what the author is writing as they talk, sent
	// with the turn; a talk opened to draft or rewrite a chapter keeps to
	// it instead.
	Beside *Writing
	// Proposing, when set, hears each change of the prose a reply proposes
	// as its call streams, by the call's ID: once its place is found, then
	// each time its text grows. It runs on the caller's goroutine. Where a
	// provider does not stream a call's input, the change comes whole with
	// the result Step hears.
	Proposing func(call string, p workspace.Proposal)
}

// Turn is one exchange: the author's words and the reply.
type Turn struct {
	User, Reply workspace.Event
}

var errNoModel = &model.SetupError{Err: ErrNoModelReason}

// Say saves the author's words, with the prose they quote and the chapter
// they were writing, or the chapter a new talk opens to draft or rewrite,
// then asks for a reply. The words tell the model what came of what it
// proposed in the prose since it last heard. If the reply fails, the words
// stay saved and Retry asks again without sending them twice.
func (e *Engine) Say(ctx context.Context, session string, words workspace.Event) (Turn, error) {
	var turn Turn
	if strings.TrimSpace(words.Text) == "" {
		return turn, ErrInputRequired
	}
	if !e.Talk.Ready() {
		return turn, errNoModel
	}
	events, err := e.Store.Events(session)
	if err != nil {
		return turn, err
	}
	if words.Origin != "" && len(events) > 0 {
		return turn, ErrNewChatRequired
	}
	words = workspace.Event{Role: workspace.RoleUser, Text: words.Text, Quotes: words.Quotes, Chapter: words.Chapter, Origin: words.Origin, Told: told(events)}
	if _, _, err := e.assemble(session, -1, words); err != nil {
		return turn, err
	}
	user, err := e.Store.Append(session, words)
	if err != nil {
		return turn, err
	}
	turn.User = user
	return e.answer(ctx, session, turn, -1)
}

// Retry resumes an unanswered saved turn without appending a duplicate user message.
func (e *Engine) Retry(ctx context.Context, session, userID string) (Turn, error) {
	var turn Turn
	if !e.Talk.Ready() {
		return turn, errNoModel
	}
	events, err := e.Store.Events(session)
	if err != nil {
		return turn, err
	}
	pos := -1
	for n, m := range events {
		if m.ID == userID && m.Role == workspace.RoleUser {
			turn.User, pos = m, n
		}
		if m.Role == workspace.RoleAssistant && m.ReplyTo == userID {
			return turn, ErrAlreadyReplied
		}
	}
	if pos < 0 {
		return turn, ErrRetryMissing
	}
	return e.answer(ctx, session, turn, pos)
}

// answer brings the talk before event before, or all of it for -1, within
// the model's threshold, then asks for the reply to the turn.
func (e *Engine) answer(ctx context.Context, session string, turn Turn, before int) (Turn, error) {
	if err := e.compact(ctx, session, before, turn.User); err != nil {
		return turn, &CompactError{Err: err}
	}
	assembled, along, err := e.assemble(session, before, turn.User)
	if err != nil {
		return turn, err
	}
	return e.respond(ctx, session, turn, assembled, along)
}

// assemble builds the request for the author's words user from the
// setting and the talk before event before, or all of it for -1, and
// tells what went with them.
func (e *Engine) assemble(session string, before int, user workspace.Event) (Context, Along, error) {
	set, events, err := e.read(session, before)
	if err != nil {
		return Context{}, Along{}, err
	}
	tree, err := e.Store.Chapters()
	if err != nil {
		return Context{}, Along{}, err
	}
	along, err := e.along(tree, events, user)
	if err != nil {
		return Context{}, along, err
	}
	out, err := Assemble(e.Store.Work(), e.Talk.Input(), set, tree, events, user.Said(), along)
	return out, along, err
}

// along is what goes with the talk of events and the author's words user:
// the chapter of tree it keeps to, read now, when its first words opened
// it to draft or rewrite one, or else what the author is writing.
func (e *Engine) along(tree workspace.Tree, events []workspace.Event, user workspace.Event) (Along, error) {
	origin, chapter := originOf(append(slices.Clip(events), user))
	if origin == "" {
		return Along{Beside: e.Beside}, nil
	}
	f, err := fixedOf(e.Store, tree, origin, chapter)
	return Along{Fixed: f}, err
}

// read is the setting and the talk before event before, or all of it for
// -1. The digests written after it come along: a turn retried may have
// been compacted for.
func (e *Engine) read(session string, before int) (workspace.Setting, []workspace.Event, error) {
	set, err := e.Store.Setting()
	if err != nil {
		return set, nil, err
	}
	events, err := e.Store.Events(session)
	if err != nil || before < 0 {
		return set, events, err
	}
	talk := slices.Clip(events[:before])
	for _, ev := range events[before:] {
		if ev.Role == workspace.RoleDigest {
			talk = append(talk, ev)
		}
	}
	return set, talk, nil
}

// maxSteps bounds the requests of one reply: the last one may call no tool
// and answers from what was found.
const maxSteps = 10

// capNote tells the model of the last step it may look nothing more up.
const capNote = "\n\n（已达到本次查阅的上限，不能再查了，请用已经查到的内容回答。）"

// fullNote stands for what a tool returned when there is no room left for it.
const fullNote = "（这次查到的内容太多，放不下了；不能再查了，请用已经查到的内容回答。）"

// respond asks for the reply, one step at a time: each step's calls are
// written before they run and each result after, then the next step sees
// them. Each records what its request used, the reply its own.
//
// What the tools return goes while it leaves the next request within the
// model's input: the request before it as the provider counted it, its
// reply, and what came after it as estimated; or, where the provider
// counts nothing, while it fits in the room the assembly left.
func (e *Engine) respond(ctx context.Context, session string, turn Turn, assembled Context, along Along) (Turn, error) {
	box := &toolbox{store: e.Store, session: session, turn: turn.User.ID, model: e.Talk}
	if along.Fixed != nil {
		box.at = along.Fixed.Chapter
	}
	var on func(model.Delta)
	if e.Progress != nil || e.Proposing != nil {
		watch := &proposing{store: e.Store, tell: e.Proposing}
		on = func(d model.Delta) {
			if d.Call.ID != "" {
				watch.feed(d.Call)
			}
			if e.Progress != nil && (d.Reasoning != "" || d.Text != "") {
				e.Progress(model.Delta{Reasoning: d.Reasoning, Text: d.Text})
			}
		}
	}
	req := assembled.Request
	var thought []string
	looked, capped := 0, false
	write := func(ev workspace.Event) error {
		ev.ReplyTo = turn.User.ID
		ev, err := e.Store.Append(session, ev)
		if err == nil && e.Step != nil {
			e.Step(ev)
		}
		return err
	}
	for step := 1; ; step++ {
		capped = step == maxSteps
		req.NoCalls = req.NoCalls || capped
		reply, err := ask(e.purpose(ctx, session, withLooked(assembled.Sent, looked), along.Fixed), e.Talk, req, on)
		if err != nil {
			return turn, &ReplyError{Err: err}
		}
		used := reply.Usage
		if r := strings.TrimSpace(reply.Reasoning); r != "" {
			thought = append(thought, r)
		}
		if len(reply.Calls) == 0 || req.NoCalls {
			if strings.TrimSpace(reply.Text) == "" {
				return turn, ErrEmptyReply
			}
			sent := withLooked(assembled.Sent, looked)
			turn.Reply, err = e.Store.Append(session, workspace.Event{
				Role: workspace.RoleAssistant, Text: reply.Text, ReplyTo: turn.User.ID,
				Context: assembled.Included, Sent: sent, Capped: capped,
				Model: e.Talk.Name, Effort: string(e.Talk.Effort), Reasoning: strings.Join(thought, "\n\n"), Usage: (*workspace.Usage)(&used),
			})
			return turn, err
		}
		calls := box.labels(reply.Calls)
		if err = write(workspace.Event{Role: workspace.RoleCall, Text: reply.Text, Calls: calls, Model: e.Talk.Name, Effort: string(e.Talk.Effort), Usage: (*workspace.Usage)(&used)}); err != nil {
			return turn, err
		}
		spent := 0
		if used.Input > 0 {
			spent = used.Input + used.Output
		}
		req.Messages = append(slices.Clip(req.Messages), model.Message{Role: workspace.RoleAssistant, Content: reply.Text, Calls: reply.Calls, Thinking: reply.Thinking})
		for _, c := range reply.Calls {
			if ctx.Err() != nil {
				return turn, &ReplyError{Err: ctx.Err()}
			}
			out := box.run(c)
			if err = write(workspace.Event{Role: workspace.RoleResult, Call: c.ID, Text: out.text, Failed: out.failed, Revision: out.revision, Proposal: out.proposal}); err != nil {
				return turn, err
			}
			told := out.text
			if spent > 0 && spent+prompt.Estimate(told) > e.Talk.Input() || spent == 0 && looked+prompt.Estimate(told) > assembled.Left {
				told, req.NoCalls = fullNote, true
			}
			if spent > 0 {
				spent += prompt.Estimate(told)
			}
			looked += prompt.Estimate(told)
			req.Messages = append(req.Messages, model.Message{Role: "tool", CallID: c.ID, Content: told})
		}
		if step+1 == maxSteps {
			last := &req.Messages[len(req.Messages)-1]
			last.Content += capNote
		}
	}
}

// purpose is ctx telling the AI record that its requests are for the
// talk session and send the parts of sent: to draft or rewrite the
// chapter f when the talk keeps to it.
func (e *Engine) purpose(ctx context.Context, session string, sent *workspace.Sent, f *Fixed) context.Context {
	p := workspace.Purpose{Feature: workspace.FeatureTalk, Talk: session}
	if f != nil {
		p.Feature = map[string]string{workspace.AIDraft: workspace.FeatureDraft, workspace.AIRewrite: workspace.FeatureRewrite}[f.Origin]
		p.Chapter = f.Chapter
	}
	if sent != nil {
		p.Parts = sent.Parts
	}
	return workspace.WithPurpose(ctx, p)
}

// withLooked is sent with what the tools returned, in its place among the
// parts.
func withLooked(sent *workspace.Sent, looked int) *workspace.Sent {
	if sent == nil || looked == 0 {
		return sent
	}
	out := *sent
	out.Parts = append([]workspace.Part{}, sent.Parts...)
	out.Parts = append(out.Parts, workspace.Part{Name: workspace.PartLooked, Tokens: looked})
	slices.SortStableFunc(out.Parts, func(a, b workspace.Part) int {
		return slices.Index(workspace.Parts, a.Name) - slices.Index(workspace.Parts, b.Name)
	})
	return &out
}

// Extract adds the selected runes of a message to the setting's body as
// the author's entry, quoting them as its source; it asks no model.
func (e *Engine) Extract(session, message string, start, end int, category string) (workspace.Entry, error) {
	events, err := e.Store.Events(session)
	if err != nil {
		return workspace.Entry{}, err
	}
	for _, m := range events {
		if m.ID != message {
			continue
		}
		runes := []rune(m.Text)
		if start < 0 || end <= start || end > len(runes) {
			return workspace.Entry{}, ErrSelectionInvalid
		}
		text := string(runes[start:end])
		if strings.TrimSpace(text) == "" {
			return workspace.Entry{}, ErrSelectionBlank
		}
		r, err := e.Store.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{
			Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: category, Text: strings.TrimSpace(text),
			Sources: []workspace.Source{{Session: session, Message: message, Quote: text}},
		}}})
		if err != nil {
			return workspace.Entry{}, err
		}
		set, err := e.Store.Setting()
		if err != nil {
			return workspace.Entry{}, err
		}
		entry, _ := set.Entry(r.Ops[0].Entry)
		return entry, nil
	}
	return workspace.Entry{}, ErrSourceMissing
}
