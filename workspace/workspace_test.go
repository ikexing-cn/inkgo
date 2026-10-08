package workspace

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
func mustIdea(t *testing.T, s *Store, i Idea) Idea {
	t.Helper()
	v, err := s.NewIdea(i)
	if err != nil {
		t.Fatal(err)
	}
	return v
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
	for _, dir := range []string{"ideas", "sessions", "settings", "topics", "chapters", ".writer/embeddings"} {
		if st, err := os.Stat(filepath.Join(root, dir)); err != nil || !st.IsDir() {
			t.Fatalf("missing directory %s: %v", dir, err)
		}
	}
	if got := string(readBytes(t, filepath.Join(root, ".gitignore"))); got != ".writer/\n" {
		t.Fatal(got)
	}
	i := mustIdea(t, s, Idea{Text: "第一行\r\n\n---\n正文保持原样。\n", Fact: true, Verified: true, FactSource: "资料"})
	if !idPattern.MatchString(i.ID) || i.Status != Unprocessed || i.Schema != Schema {
		t.Fatalf("bad defaults: %+v", i)
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
	got, err := reopened.Idea(i.ID)
	if err != nil || got.Text != i.Text || got.FactSource != "资料" {
		t.Fatalf("roundtrip: %+v %v", got, err)
	}
}
func TestManualEditsAndStrictCanonicalValidation(t *testing.T) {
	s := testStore(t)
	i := mustIdea(t, s, Idea{Text: "旧文本"})
	p := filepath.Join(s.Root(), "ideas", i.ID+".md")
	original := readBytes(t, p)
	putBytes(t, p, bytes.Replace(original, []byte("旧文本"), []byte("新文本"), 1))
	ideas, err := s.Ideas()
	if err != nil || len(ideas) != 1 || !strings.Contains(ideas[0].Text, "新文本") {
		t.Fatalf("manual edit: %v %v", ideas, err)
	}
	for _, tc := range []struct{ name, old, new, field string }{
		{"unknown field", "category:", "categry:", "categry"},
		{"future schema", "schema: 1", "schema: 2", "schema"},
		{"unknown source", "source: human", "source: robot", "source"},
		{"unknown category", "category: 其他", "category: invented", "category"},
		{"duplicate key", "schema: 1", "schema: 1\nschema: 1", "schema"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			putBytes(t, p, bytes.Replace(original, []byte(tc.old), []byte(tc.new), 1))
			_, err := s.Ideas()
			if err == nil || !strings.Contains(err.Error(), i.ID+".md") || !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("missing filename/field: %v", err)
			}
			if opened, err := Open(s.Root()); err == nil {
				opened.Close()
				t.Fatal("Open accepted malformed canonical file")
			}
			putBytes(t, p, original)
		})
	}
	wp := filepath.Join(s.Root(), "work.yaml")
	work := readBytes(t, wp)
	putBytes(t, wp, append(work, []byte("typo: true\n")...))
	if _, err = s.Idea(i.ID); err == nil || !strings.Contains(err.Error(), "typo") {
		t.Fatalf("work not refreshed: %v", err)
	}
}
func TestStatusHistoryAndCategoryRemoval(t *testing.T) {
	s := testStore(t)
	i := mustIdea(t, s, Idea{Text: "一条思路", Category: "人物"})
	for _, status := range []Status{Kept, Rejected, Deferred, Kept, Adopted} {
		if i.Status == Rejected {
			if _, err := s.ChangeStatus(i.ID, Kept, "invalid restore"); err == nil {
				t.Fatal("rejected jumped to kept")
			}
		}
		next, err := s.ChangeStatus(i.ID, status, "because")
		if err != nil {
			t.Fatal(err)
		}
		if next.History[len(next.History)-1].From != i.Status {
			t.Fatal("history lost previous status")
		}
		i = next
	}
	if len(i.History) != 5 {
		t.Fatal(i.History)
	}
	if _, err := s.ChangeStatus(i.ID, Deferred, ""); err == nil {
		t.Fatal("adopted state changed")
	}
	i.Status = Deferred
	if err := s.SaveIdea(i); err == nil {
		t.Fatal("SaveIdea bypassed adopted protection")
	}
	if err := s.RemoveCategory("人物"); err != nil {
		t.Fatal(err)
	}
	got, err := s.Idea(i.ID)
	if err != nil || got.Category != Other || got.Status != Adopted {
		t.Fatalf("reclassify: %+v %v", got, err)
	}
	if err = s.RemoveCategory(Other); err == nil {
		t.Fatal("removed fallback category")
	}
	if err = s.SetCategory(i.ID, "bogus"); err == nil {
		t.Fatal("accepted invalid category")
	}
	got.History = nil
	if err = s.SaveIdea(got); err == nil {
		t.Fatal("history erased")
	}
}
func TestOriginUsesRunesAndValidatesSource(t *testing.T) {
	s := testStore(t)
	session := mustSession(t, s)
	e := mustEvent(t, s, session, Event{Role: "user", Text: "你好🙂world"})
	valid := Idea{Text: "好🙂", Source: "human", Origin: Origin{Session: session, Message: e.ID, Span: Span{Start: 1, End: 3}}}
	mustIdea(t, s, valid)
	for _, tc := range []struct {
		name   string
		mutate func(*Idea)
	}{
		{"byte offsets", func(i *Idea) { i.Origin.Span.End = 13 }},
		{"negative", func(i *Idea) { i.Origin.Span.Start = -1 }},
		{"reverse", func(i *Idea) { i.Origin.Span = Span{3, 1} }},
		{"empty", func(i *Idea) { i.Origin.Span = Span{1, 1} }},
		{"wrong source", func(i *Idea) { i.Source = "ai" }},
		{"unknown message", func(i *Idea) { i.Origin.Message = "20000101-0000-0000" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			i := valid
			tc.mutate(&i)
			if _, err := s.NewIdea(i); err == nil {
				t.Fatal("accepted invalid origin")
			}
		})
	}
}
func TestIDsCollisionsTraversalAndAtomicFailures(t *testing.T) {
	s := testStore(t)
	i := mustIdea(t, s, Idea{Text: "original"})
	p := filepath.Join(s.Root(), "ideas", i.ID+".md")
	before := readBytes(t, p)
	if _, err := s.NewIdea(Idea{ID: i.ID, Text: "replace"}); !errors.Is(err, os.ErrExist) {
		t.Fatalf("collision: %v", err)
	}
	if !bytes.Equal(before, readBytes(t, p)) {
		t.Fatal("collision overwrote record")
	}
	i.Source = "typo"
	if err := s.SaveIdea(i); err == nil {
		t.Fatal("invalid update succeeded")
	}
	if !bytes.Equal(before, readBytes(t, p)) {
		t.Fatal("failed validation changed file")
	}
	for _, id := range []string{"../work", "..\\work", "C:\\escape", "/absolute", "valid/../../bad", ""} {
		if _, err := s.Idea(id); err == nil {
			t.Fatalf("Idea accepted %q", id)
		}
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
	putBytes(t, p, append(append([]byte{}, original...), []byte(`{"schema":1,"id":"broken`)...))
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
		{"future", bytes.Replace(good, []byte(`"schema":1`), []byte(`"schema":2`), 1), "schema"},
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
func TestCreatedOrderingAndSaveWorkValidation(t *testing.T) {
	s := testStore(t)
	at := time.Now().UTC()
	ai := mustIdea(t, s, Idea{Text: "AI", Source: "ai", Created: at})
	human := mustIdea(t, s, Idea{Text: "human", Created: at})
	earlier := mustIdea(t, s, Idea{Text: "earlier", Created: at.Add(-time.Hour)})
	ideas, err := s.Ideas()
	if err != nil || len(ideas) != 3 || ideas[0].ID != earlier.ID || ideas[1].ID != human.ID || ideas[2].ID != ai.ID {
		t.Fatalf("ordering: %v %v", ideas, err)
	}
	w := s.Work()
	w.Discussion = Selection{Provider: "custom", Model: "model-id"}
	if err = s.SaveWork(w); err != nil {
		t.Fatal(err)
	}
	w.TokenBudget = -1
	if err = s.SaveWork(w); err == nil {
		t.Fatal("invalid budget persisted")
	}
	w = s.Work()
	w.Categories = append(w.Categories, Category{Name: Other, Target: "duplicate"})
	if err = s.SaveWork(w); err == nil {
		t.Fatal("duplicate category persisted")
	}
	w = s.Work()
	w.ID = NewID()
	if err = s.SaveWork(w); err == nil {
		t.Fatal("work identity changed")
	}
}
func TestBookkeepingJournalsReservedIDs(t *testing.T) {
	s := testStore(t)
	session := mustSession(t, s)
	user := mustEvent(t, s, session, Event{Role: "user", Text: "想法"})
	reply := mustEvent(t, s, session, Event{Role: "assistant", Text: "建议", ReplyTo: user.ID})
	reserved := NewID()
	mustEvent(t, s, session, Event{Kind: "bookkeeping", ReplyTo: reply.ID, Text: "[]", IdeaIDs: []string{reserved}})
	reopened, err := Open(s.Root())
	if err != nil {
		t.Fatalf("pending plan cannot reopen: %v", err)
	}
	reopened.Close()
	mustIdea(t, s, Idea{ID: reserved, Text: "journaled idea"})
	events, err := s.Events(session)
	if err != nil || events[2].IdeaIDs[0] != reserved {
		t.Fatalf("journal round trip: %v %v", events, err)
	}
	if _, err = s.Append(session, Event{Kind: "bookkeeping", IdeaIDs: []string{"../escape"}}); err == nil {
		t.Fatal("bookkeeping accepted invalid ID")
	}
}
