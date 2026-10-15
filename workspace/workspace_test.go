package workspace

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := Create(t.TempDir(), "测试作品", "非虚构")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func mustRevise(t *testing.T, s *Store, r Revision) Revision {
	t.Helper()
	v, err := s.Revise(r)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func entry(t *testing.T, s *Store, id string) Entry {
	t.Helper()
	set, err := s.Setting()
	if err != nil {
		t.Fatal(err)
	}
	e, ok := set.Entry(id)
	if !ok {
		t.Fatalf("no entry %s", id)
	}
	return e
}
func mustSession(t *testing.T, s *Store) string {
	t.Helper()
	id, err := s.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func mustEvent(t *testing.T, s *Store, id string, e Event) Event {
	t.Helper()
	v, err := s.Append(id, e)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func readBytes(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func putBytes(t *testing.T, p string, b []byte) {
	t.Helper()
	if err := os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestCreateAndRoundTrip(t *testing.T) {
	s := testStore(t)
	root := s.Root()
	for _, dir := range []string{"sessions", "chapters", ".writer"} {
		if st, err := os.Stat(filepath.Join(root, dir)); err != nil || !st.IsDir() {
			t.Fatalf("missing directory %s: %v", dir, err)
		}
	}
	if got := string(readBytes(t, filepath.Join(root, ".gitignore"))); got != ".writer/\n" {
		t.Fatal(got)
	}
	r := mustRevise(t, s, Revision{By: ByAuthor, Ops: []Op{{Op: OpAdd, Place: PlaceBody, Category: "人物", Text: "第一行\r\n正文保持原样。\n"}}})
	if !idPattern.MatchString(r.ID) || !idPattern.MatchString(r.Ops[0].Entry) || r.Schema != Schema {
		t.Fatalf("bad defaults: %+v", r)
	}
	w := s.Work()
	w.Categories[0].Name = "mutated copy"
	if s.Work().Categories[0].Name == "mutated copy" {
		t.Fatal("Work leaked mutable slice")
	}
	if _, err := Create(root, "overwrite", "novel"); err == nil {
		t.Fatal("Create overwrote workspace")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if e := entry(t, reopened, r.Ops[0].Entry); e.Text != "第一行\r\n正文保持原样。\n" || !e.Author || e.Rev != 1 {
		t.Fatalf("roundtrip: %+v", e)
	}
}

// A work of schema 1, with idea cards, is refused with a reason in Chinese.
func TestOldWorksAreRefused(t *testing.T) {
	s := testStore(t)
	p := filepath.Join(s.Root(), "work.yaml")
	putBytes(t, p, bytes.Replace(readBytes(t, p), []byte("schema: 2"), []byte("schema: 1"), 1))
	if _, err := Open(s.Root()); err == nil || !strings.Contains(err.Error(), "旧版本的作品") {
		t.Fatalf("schema 1 opened: %v", err)
	}
}

// A work written when it had a budget of its own opens without it, and is
// written back without it.
func TestTheOldBudgetIsIgnored(t *testing.T) {
	s := testStore(t)
	p := filepath.Join(s.Root(), "work.yaml")
	putBytes(t, p, append(readBytes(t, p), "token_budget: 16000\n"...))
	s.Close()
	again, err := Open(s.Root())
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	if err := again.SaveWork(again.Work()); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(readBytes(t, p), []byte("token_budget")) {
		t.Fatal("the budget was written back")
	}
}

func TestStrictWorkAndSettingValidation(t *testing.T) {
	s := testStore(t)
	r := mustRevise(t, s, Revision{By: ByAuthor, Ops: []Op{{Op: OpAdd, Place: PlaceBody, Category: "人物", Text: "旧文本"}}})
	p := filepath.Join(s.Root(), "setting.jsonl")
	original := readBytes(t, p)
	putBytes(t, p, bytes.Replace(original, []byte("旧文本"), []byte("新文本"), 1))
	if e := entry(t, s, r.Ops[0].Entry); e.Text != "新文本" {
		t.Fatalf("hand edit: %+v", e)
	}
	for _, tc := range []struct{ name, old, new, field string }{
		{"unknown field", `"place":`, `"placе":`, "placе"},
		{"future schema", `"schema":2`, `"schema":3`, "schema"},
		{"unknown op", `"op":"add"`, `"op":"delete"`, "delete"},
		{"unknown place", `"place":"body"`, `"place":"draft"`, "place"},
		{"author with model", `"by":"author"`, `"by":"author","model":"x"`, "by"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			putBytes(t, p, bytes.Replace(original, []byte(tc.old), []byte(tc.new), 1))
			_, err := s.Setting()
			if err == nil || !strings.Contains(err.Error(), "setting.jsonl:1") || !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("missing file/field: %v", err)
			}
			if opened, err := Open(s.Root()); err == nil {
				opened.Close()
				t.Fatal("Open accepted a malformed setting")
			}
			putBytes(t, p, original)
		})
	}
	// A crash mid-append leaves an incomplete last line, which is dropped.
	putBytes(t, p, append(append([]byte{}, original...), []byte(`{"schema":2,"id":"bro`)...))
	next := mustRevise(t, s, Revision{By: ByAuthor, Ops: []Op{{Op: OpMove, Entry: r.Ops[0].Entry, Place: PlaceDropped}}})
	set, err := s.Setting()
	if err != nil || len(set.Revisions) != 2 || set.Revisions[1].ID != next.ID || set.Entries[0].Place != PlaceDropped {
		t.Fatalf("tail recovery: %+v %v", set, err)
	}
	wp := filepath.Join(s.Root(), "work.yaml")
	putBytes(t, wp, append(readBytes(t, wp), []byte("typo: true\n")...))
	if _, err = s.Setting(); err == nil || !strings.Contains(err.Error(), "typo") {
		t.Fatalf("work not refreshed: %v", err)
	}
}

func TestRevisionsCheckOpsAndSources(t *testing.T) {
	s := testStore(t)
	session := mustSession(t, s)
	u := mustEvent(t, s, session, Event{Role: RoleUser, Text: "主角怕黑🙂，却修路灯"})
	a := mustEvent(t, s, session, Event{Role: RoleAssistant, Text: "可以让她绕路回家", ReplyTo: u.ID})
	src := Source{Session: session, Message: u.ID, Quote: "主角怕黑🙂"}
	add := mustRevise(t, s, Revision{By: ByBookkeeper, Session: session, From: u.ID, Through: a.ID,
		Ops: []Op{{Op: OpAdd, Place: PlaceBody, Category: "人物", Text: "主角怕黑", Sources: []Source{src}}}, Model: "deepseek/deepseek-flash", Effort: "high", Reasoning: "她说的"})
	id := add.Ops[0].Entry
	if e := entry(t, s, id); e.Author || e.Sources[0] != src {
		t.Fatalf("bookkeeper's entry: %+v", e)
	}
	for _, tc := range []struct {
		name string
		r    Revision
	}{
		{"quote not in message", Revision{By: ByAuthor, Ops: []Op{{Op: OpAdd, Place: PlaceBody, Category: "人物", Text: "x", Sources: []Source{{Session: session, Message: u.ID, Quote: "没说过"}}}}}},
		{"unknown message", Revision{By: ByAuthor, Ops: []Op{{Op: OpAdd, Place: PlaceBody, Category: "人物", Text: "x", Sources: []Source{{Session: session, Message: NewID(), Quote: "主角"}}}}}},
		{"unknown category", Revision{By: ByAuthor, Ops: []Op{{Op: OpAdd, Place: PlaceBody, Category: "虚构", Text: "x"}}}},
		{"unknown entry", Revision{By: ByAuthor, Ops: []Op{{Op: OpEdit, Entry: NewID(), Text: "x"}}}},
		{"edit moves", Revision{By: ByAuthor, Ops: []Op{{Op: OpEdit, Entry: id, Text: "x", Place: PlaceOpen}}}},
		{"empty move", Revision{By: ByAuthor, Ops: []Op{{Op: OpMove, Entry: id}}}},
		{"no ops", Revision{By: ByAuthor}},
		{"failed with ops", Revision{By: ByBookkeeper, Session: session, From: u.ID, Through: a.ID, Failed: "超时", Ops: []Op{{Op: OpMove, Entry: id, Place: PlaceOpen}}}},
		{"redo of nothing", Revision{By: ByBookkeeper, Session: session, From: u.ID, Through: a.ID, Redo: add.ID}},
		{"through unknown", Revision{By: ByBookkeeper, Session: session, From: u.ID, Through: NewID()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.Revise(tc.r); err == nil {
				t.Fatal("accepted")
			}
		})
	}
	mustRevise(t, s, Revision{By: ByAuthor, Ops: []Op{{Op: OpEdit, Entry: id, Text: "主角怕黑，但不说"}}})
	if e := entry(t, s, id); !e.Author || e.Text != "主角怕黑，但不说" || e.Rev != 2 {
		t.Fatalf("author's edit: %+v", e)
	}
	set, err := s.Setting()
	if err != nil || set.Processed(session, []Event{u, a}) != 2 {
		t.Fatalf("processed: %v", err)
	}
	failed := mustRevise(t, s, Revision{By: ByBookkeeper, Session: session, From: u.ID, Through: a.ID, Failed: "超时"})
	mustRevise(t, s, Revision{By: ByBookkeeper, Session: session, From: u.ID, Through: a.ID, Redo: failed.ID})
	if set, _ = s.Setting(); !set.Redone(failed.ID) {
		t.Fatal("a successful redo does not count")
	}
}

// What the author changed while the bookkeeper worked stands: its ops on
// that entry are skipped and become open entries.
func TestBookkeeperDefersToTheAuthorsConcurrentEdit(t *testing.T) {
	s := testStore(t)
	session := mustSession(t, s)
	u := mustEvent(t, s, session, Event{Role: RoleUser, Text: "雨从不停"})
	first := mustRevise(t, s, Revision{By: ByAuthor, Ops: []Op{{Op: OpAdd, Place: PlaceBody, Category: "规则", Text: "雨从不停"}, {Op: OpAdd, Place: PlaceBody, Category: "规则", Text: "晴天是传说"}}})
	rain, sun := first.Ops[0].Entry, first.Ops[1].Entry
	mustRevise(t, s, Revision{By: ByAuthor, Ops: []Op{{Op: OpEdit, Entry: rain, Text: "雨每年停一天"}}})
	r := mustRevise(t, s, Revision{By: ByBookkeeper, Base: 1, Session: session, From: u.ID, Through: u.ID,
		Ops: []Op{{Op: OpEdit, Entry: rain, Text: "雨从不停，连一天也不停"}, {Op: OpMove, Entry: sun, Place: PlaceOpen}}})
	if len(r.Skipped) != 1 || r.Skipped[0].Entry != rain || len(r.Ops) != 2 || r.Ops[0].Op != OpAdd || r.Ops[0].Place != PlaceOpen {
		t.Fatalf("revision: %+v", r)
	}
	if e := entry(t, s, rain); e.Text != "雨每年停一天" {
		t.Fatalf("the author's edit was overwritten: %+v", e)
	}
	if e := entry(t, s, sun); e.Place != PlaceOpen {
		t.Fatalf("an untouched entry was not moved: %+v", e)
	}
	if e := entry(t, s, r.Ops[0].Entry); !strings.Contains(e.Text, "雨从不停，连一天也不停") {
		t.Fatalf("open entry: %+v", e)
	}
}

func TestCategoryRemovalMovesEntries(t *testing.T) {
	s := testStore(t)
	r := mustRevise(t, s, Revision{By: ByAuthor, Ops: []Op{{Op: OpAdd, Place: PlaceBody, Category: "人物", Text: "一条设定"}}})
	w := s.Work()
	w.Categories = slices.DeleteFunc(w.Categories, func(c Category) bool { return c.Name == "人物" })
	if err := s.SaveWork(w); err == nil {
		t.Fatal("removed a category the setting uses")
	}
	if err := s.RemoveCategory("人物"); err != nil {
		t.Fatal(err)
	}
	if e := entry(t, s, r.Ops[0].Entry); e.Category != Other {
		t.Fatalf("reclassify: %+v", e)
	}
	if err := s.RemoveCategory(Other); err == nil {
		t.Fatal("removed fallback category")
	}
}

// 其他 stays the last category: a genre's own come before it, and so does
// one the author adds.
func TestOtherStaysLast(t *testing.T) {
	for _, genre := range []string{"网文", "出版小说", "剧本", "非虚构", ""} {
		c := Categories(genre)
		if c[len(c)-1].Name != Other {
			t.Fatalf("%s: %v", genre, c)
		}
	}
	c := AddCategory(Categories("网文"), Category{Name: "门派", Target: "设定库"})
	if n := len(c); c[n-1].Name != Other || c[n-2].Name != "门派" || c[n-3].Name != "力量体系" {
		t.Fatalf("added: %v", c)
	}
}

func TestTraversalAndDuplicateEvents(t *testing.T) {
	s := testStore(t)
	for _, id := range []string{"../work", "..\\work", "C:\\escape", "/absolute", "valid/../../bad", ""} {
		if _, err := s.Events(id); err == nil {
			t.Fatalf("Events accepted %q", id)
		}
		if _, err := s.Append(id, Event{Role: "user", Text: "escape"}); err == nil {
			t.Fatalf("Append accepted %q", id)
		}
	}
	session := mustSession(t, s)
	e := mustEvent(t, s, session, Event{Role: "user", Text: "first"})
	if _, err := s.Append(session, e); err == nil {
		t.Fatal("duplicate event accepted")
	}
	events, err := s.Events(session)
	if err != nil || len(events) != 1 {
		t.Fatalf("collision mutated events: %v %v", events, err)
	}
}

func TestSessionRecoverySnapshotsAndCorruption(t *testing.T) {
	s := testStore(t)
	session := mustSession(t, s)
	p := filepath.Join(s.Root(), "sessions", session+".jsonl")
	first := mustEvent(t, s, session, Event{Role: "user", Text: "想法"})
	assistant := mustEvent(t, s, session, Event{Role: "assistant", Text: "回答", ReplyTo: first.ID, Context: []ContextItem{{ID: "context-id", Text: "实际文本", Reason: "current topic"}}, Omitted: []ContextItem{{ID: "omitted-id", Text: "未发送", Reason: "budget"}}})
	original := readBytes(t, p)
	putBytes(t, p, append(append([]byte{}, original...), []byte(`{"schema":2,"id":"broken`)...))
	events, err := s.Events(session)
	if err != nil || len(events) != 2 {
		t.Fatalf("tail recovery read: %v %v", events, err)
	}
	reopened, err := Open(s.Root())
	if err != nil {
		t.Fatal(err)
	}
	reopened.Close()
	next := mustEvent(t, s, session, Event{Role: "user", Text: "继续"})
	events, err = s.Events(session)
	if err != nil || len(events) != 3 || events[2].ID != next.ID {
		t.Fatalf("tail recovery append: %v %v", events, err)
	}
	if !bytes.HasPrefix(readBytes(t, p), original) {
		t.Fatal("recovery modified completed events")
	}
	if events[1].Context[0].Text != assistant.Context[0].Text || events[1].Omitted[0].Reason != "budget" {
		t.Fatal("snapshot not preserved")
	}
	good := readBytes(t, p)
	for _, tc := range []struct {
		name  string
		data  []byte
		field string
	}{
		{"middle", append([]byte("{broken}\n"), good...), ".jsonl:1"},
		{"complete final", append(append([]byte{}, good...), []byte("{broken}\n")...), ".jsonl:4"},
		{"future", bytes.Replace(good, []byte(`"schema":2`), []byte(`"schema":3`), 1), "schema"},
		{"unknown field", bytes.Replace(good, []byte(`"role":`), []byte(`"rol":`), 1), "rol"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			putBytes(t, p, tc.data)
			if _, err := s.Events(session); err == nil || !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("accepted corruption: %v", err)
			}
			if _, err := s.Append(session, Event{Role: "user", Text: "new"}); err == nil {
				t.Fatal("append accepted corruption")
			}
			if !bytes.Equal(tc.data, readBytes(t, p)) {
				t.Fatal("corruption failure modified file")
			}
			putBytes(t, p, good)
		})
	}
	// A complete final event missing only its newline must not be discarded.
	putBytes(t, p, bytes.TrimSuffix(good, []byte("\n")))
	mustEvent(t, s, session, Event{Role: "user", Text: "last"})
	events, err = s.Events(session)
	if err != nil || len(events) != 4 {
		t.Fatalf("missing newline: %v %v", events, err)
	}
	raw, err := json.Marshal(events[1])
	if err != nil || bytes.Contains(raw, []byte(`"Context"`)) {
		t.Fatal("JSON field casing")
	}
}

func TestSaveWorkValidation(t *testing.T) {
	s := testStore(t)
	w := s.Work()
	w.Discussion = Selection{Provider: "custom", Model: "model-id"}
	if err := s.SaveWork(w); err != nil {
		t.Fatal(err)
	}
	w.Categories = append(w.Categories, Category{Name: Other, Target: "duplicate"})
	if err := s.SaveWork(w); err == nil {
		t.Fatal("duplicate category persisted")
	}
	w = s.Work()
	w.ID = NewID()
	if err := s.SaveWork(w); err == nil {
		t.Fatal("work identity changed")
	}
}

func TestModelOutputKeepsReasoningForReview(t *testing.T) {
	s := testStore(t)
	session := mustSession(t, s)
	user := mustEvent(t, s, session, Event{Role: RoleUser, Text: "她怕黑"})
	mustEvent(t, s, session, Event{Role: RoleAssistant, Text: "回答", ReplyTo: user.ID, Model: "deepseek/deepseek-flash", Effort: "max", Reasoning: "先想一想\n再回答"})
	events, err := s.Events(session)
	if err != nil || events[1].Reasoning != "先想一想\n再回答" || events[1].Model != "deepseek/deepseek-flash" || events[1].Effort != "max" {
		t.Fatalf("reasoning lost: %+v %v", events, err)
	}
	if _, err = s.Append(session, Event{Role: RoleUser, Text: "x", Reasoning: "伪造"}); err == nil {
		t.Fatal("the author's words recorded a model's reasoning")
	}
	if _, err = s.Append(session, Event{Role: RoleUser, Text: "x", Context: []ContextItem{{ID: "x", Text: "y"}}}); err == nil {
		t.Fatal("the author's words recorded a context")
	}
	if _, err = s.Append(session, Event{Role: RoleAssistant, Text: "x", ReplyTo: user.ID, Effort: "medium"}); err == nil {
		t.Fatal("accepted an effort not offered")
	}
	w := s.Work()
	w.Discussion = Selection{Provider: "deepseek", Model: "deepseek-flash", Effort: "high"}
	w.Bookkeeping = Selection{Effort: "low"}
	if err = s.SaveWork(w); err != nil {
		t.Fatal(err)
	}
	if s.Work().Bookkeeping.Effort != "low" {
		t.Fatal("bookkeeping effort lost")
	}
	w.Discussion.Effort = "xhigh"
	if err = s.SaveWork(w); err == nil {
		t.Fatal("accepted an unknown effort")
	}
}

// An entry's names come with add, change with edit and name, and must be
// short, one line and not repeated.
func TestEntryNames(t *testing.T) {
	s := testStore(t)
	r := mustRevise(t, s, Revision{By: ByAuthor, Ops: []Op{{Op: OpAdd, Place: PlaceBody, Category: "人物", Text: "林晚，修路灯的", Names: []string{"林晚", "晚晚"}}}})
	id := r.Ops[0].Entry
	mustRevise(t, s, Revision{By: ByAuthor, Ops: []Op{{Op: OpEdit, Entry: id, Text: "林晚，修路灯的姑娘"}}})
	set, err := s.Setting()
	if err != nil {
		t.Fatal(err)
	}
	if e, _ := set.Entry(id); !slices.Equal(e.Names, []string{"林晚", "晚晚"}) {
		t.Fatal("an edit without names lost them:", e.Names)
	}
	mustRevise(t, s, Revision{By: ByAuthor, Ops: []Op{{Op: OpEdit, Entry: id, Text: "苏晴", Names: []string{"苏晴"}}}})
	mustRevise(t, s, Revision{By: ByAuthor, Ops: []Op{{Op: OpName, Entry: id, Names: []string{"苏晴", "晴姐"}}}})
	if set, _ = s.Setting(); !slices.Equal(set.Entries[0].Names, []string{"苏晴", "晴姐"}) {
		t.Fatal("names", set.Entries[0].Names)
	}
	mustRevise(t, s, Revision{By: ByAuthor, Ops: []Op{{Op: OpName, Entry: id}}})
	if set, _ = s.Setting(); set.Entries[0].Names != nil {
		t.Fatal("not cleared", set.Entries[0].Names)
	}
	for _, names := range [][]string{{""}, {" 林晚"}, {"林\n晚"}, {"林晚", "林晚"}, {"一二三四五六七八九十一二三"}} {
		if _, err := s.Revise(Revision{By: ByAuthor, Ops: []Op{{Op: OpName, Entry: id, Names: names}}}); err == nil {
			t.Errorf("took %q", names)
		}
	}
	if _, err := s.Revise(Revision{By: ByAuthor, Ops: []Op{{Op: OpName, Entry: id, Text: "x"}}}); err == nil {
		t.Error("a name op changed the text")
	}
	if !slices.Equal(CleanNames([]string{" 林晚 ", "", "林晚", "晚晚"}), []string{"林晚", "晚晚"}) {
		t.Error("CleanNames")
	}
}

// The author answers an open entry that asks them something: it becomes
// the answer, settled, and keeps the question. Only the author answers,
// and only what is open.
func TestAnAnswerTakesTheQuestionsPlace(t *testing.T) {
	s := testStore(t)
	const q = "林晚怕黑，是不是和她母亲有关？"
	r := mustRevise(t, s, Revision{By: ByAuthor, Ops: []Op{
		{Op: OpAdd, Place: PlaceOpen, Category: Other, Text: q},
		{Op: OpAdd, Place: PlaceBody, Category: "人物", Text: "林晚，修路灯的"},
	}})
	id, body := r.Ops[0].Entry, r.Ops[1].Entry
	mustRevise(t, s, Revision{By: ByAuthor, Ops: []Op{{Op: OpAnswer, Entry: id, Category: "人物", Text: "是。她母亲死在停电的那一夜。", Names: []string{"林晚"}}}})
	set, err := s.Setting()
	if err != nil {
		t.Fatal(err)
	}
	e, _ := set.Entry(id)
	if e.Place != PlaceBody || e.Text != "是。她母亲死在停电的那一夜。" || e.Asked != q || e.Category != "人物" || !e.Author || !slices.Equal(e.Names, []string{"林晚"}) {
		t.Fatalf("answered %+v", e)
	}
	for _, op := range []Op{
		{Op: OpAnswer, Entry: body, Category: "人物", Text: "x"}, // settled already
		{Op: OpAnswer, Entry: id, Category: "人物", Text: "x"},   // answered already
		{Op: OpAnswer, Entry: "nope", Category: "人物", Text: "x"},
	} {
		if _, err := s.Revise(Revision{By: ByAuthor, Ops: []Op{op}}); err == nil {
			t.Errorf("took %+v", op)
		}
	}
	open := []Entry{{ID: "q", Place: PlaceOpen, Category: Other, Text: q}}
	if _, err := applyOps(open, Revision{By: ByBookkeeper, Ops: []Op{{Op: OpAnswer, Entry: "q", Category: Other, Text: "x"}}}, 1); err == nil {
		t.Error("the bookkeeper answered")
	}
	if _, err := applyOps(open, Revision{By: ByAuthor, Ops: []Op{{Op: OpAnswer, Entry: "q", Category: Other}}}, 1); err == nil {
		t.Error("an answer without text")
	}
}

// What is in sessions/ and is not a session does not keep the work from
// opening: it is passed over, and left as it is.
func TestFilesThatAreNotSessionsArePassedOver(t *testing.T) {
	dir := t.TempDir()
	s, err := Create(dir, "测试作品", "非虚构")
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	strays := []string{"desktop.ini", ".DS_Store", "Thumbs.db", id + " (冲突副本).jsonl", "笔记.jsonl"}
	for _, name := range strays {
		if err = os.WriteFile(filepath.Join(dir, "sessions", name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err = os.Mkdir(filepath.Join(dir, "sessions", "旧的"), 0755); err != nil {
		t.Fatal(err)
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ids, err := s.Sessions()
	if err != nil || len(ids) != 1 || ids[0] != id {
		t.Fatalf("sessions %v, %v", ids, err)
	}
	for _, name := range strays {
		if _, err = os.Stat(filepath.Join(dir, "sessions", name)); err != nil {
			t.Fatalf("%s was not left alone: %v", name, err)
		}
	}
}
