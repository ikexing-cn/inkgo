package workspace

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Each request is a line of the AI record, in the work's folder, the
// oldest first; a line written in half is dropped.
func TestTheRecordKeepsEachRequest(t *testing.T) {
	s := testStore(t)
	if got, err := s.Records(); err != nil || len(got) != 0 {
		t.Fatalf("a new work has %+v %v", got, err)
	}
	at := time.Date(2026, 10, 16, 10, 40, 0, 0, time.UTC)
	talk := Record{At: at, Took: 4200, Feature: FeatureTalk, Talk: "s1", Model: "deepseek/deepseek-flash", Effort: "high",
		Parts: []Part{{PartRules, 1100}, {PartTalk, 9600}}, Usage: Usage{Input: 12000, CacheRead: 9000, Output: 300},
		Cost: 0.004, Currency: "$", Outcome: OutcomeDone}
	check := Record{At: at.Add(time.Minute), Feature: FeatureCheck, Chapter: "c1", Model: "kimi/kimi-k3", Outcome: OutcomeFailed, Error: "无法连接模型服务"}
	for _, r := range []Record{talk, check} {
		if err := s.Log(r); err != nil {
			t.Fatal(err)
		}
	}
	p := filepath.Join(s.Root(), "ai-log.jsonl")
	b, err := os.ReadFile(p)
	if err != nil || strings.Count(string(b), "\n") != 2 {
		t.Fatalf("the file holds %q, %v", b, err)
	}
	if err = os.WriteFile(p, append(b, []byte(`{"schema":2,"at":"2026-`)...), 0600); err != nil {
		t.Fatal(err)
	}
	talk.Schema, check.Schema = Schema, Schema
	got, err := s.Records()
	if err != nil || !reflect.DeepEqual(got, []Record{talk, check}) {
		t.Fatalf("read %+v, %v", got, err)
	}
	cancelled := Record{At: at.Add(2 * time.Minute), Feature: FeatureContinue, Chapter: "c1", Model: "kimi/kimi-k3", Outcome: OutcomeCancelled}
	if err = s.Log(cancelled); err != nil {
		t.Fatal(err)
	}
	if got, err = s.Records(); err != nil || len(got) != 3 || got[2].Outcome != OutcomeCancelled {
		t.Fatalf("after the half line: %+v, %v", got, err)
	}
	for _, bad := range []Record{
		{At: at, Feature: FeatureTalk, Outcome: OutcomeDone},
		{At: at, Feature: "summary", Model: "a/b", Outcome: OutcomeDone},
		{Feature: FeatureTalk, Model: "a/b", Outcome: OutcomeDone},
		{At: at, Feature: FeatureTalk, Model: "a/b", Outcome: OutcomeFailed},
		{At: at, Feature: FeatureTalk, Model: "a/b", Outcome: OutcomeDone, Cost: 1},
		{At: at, Feature: FeatureTalk, Model: "a/b", Outcome: OutcomeDone, Usage: Usage{Input: -1}},
	} {
		if err := s.Log(bad); err == nil {
			t.Errorf("logged %+v", bad)
		}
	}
}

// A line gone wrong by hand is reported with the file and the line.
func TestTheRecordIsChecked(t *testing.T) {
	s := testStore(t)
	line := `{"schema":2,"at":"2026-10-16T10:40:00Z","feature":"talk","model":"a/b","usage":{"input":1,"output":1},"outcome":"done","words":"灯"}` + "\n"
	if err := os.WriteFile(filepath.Join(s.Root(), "ai-log.jsonl"), []byte(line), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Records(); err == nil || !strings.Contains(err.Error(), "ai-log.jsonl:1") || !strings.Contains(err.Error(), `unknown field "words"`) {
		t.Fatal(err)
	}
}

// What an older version counted in usage.json is a line for each model
// before the record's own, written with the first of them; the file is
// left as it was.
func TestTheRecordTakesOverWhatWasSpent(t *testing.T) {
	s := testStore(t)
	old := `{"models": [{"model": "deepseek/deepseek-flash", "requests": 3, "input": 4010, "cache_read": 2600, "output": 100, "cost": 0.003, "currency": "$"}, {"model": "kimi/kimi-k3", "requests": 1, "input": 50, "output": 5}]}`
	up := filepath.Join(s.Root(), "usage.json")
	if err := os.WriteFile(up, []byte(old), 0644); err != nil {
		t.Fatal(err)
	}
	want := []Record{
		{Schema: Schema, Feature: FeatureBefore, Model: "deepseek/deepseek-flash", Requests: 3, Usage: Usage{Input: 4010, CacheRead: 2600, Output: 100}, Cost: 0.003, Currency: "$", Outcome: OutcomeDone},
		{Schema: Schema, Feature: FeatureBefore, Model: "kimi/kimi-k3", Requests: 1, Usage: Usage{Input: 50, Output: 5}, Outcome: OutcomeDone},
	}
	if got, err := s.Records(); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("before any request: %+v, %v", got, err)
	}
	if want[0].Count() != 3 || (Record{Feature: FeatureTalk}).Count() != 1 {
		t.Fatal("counted the requests wrong")
	}
	now := Record{Schema: Schema, At: time.Date(2026, 10, 16, 10, 40, 0, 0, time.UTC), Feature: FeatureDraft, Chapter: "c1", Model: "kimi/kimi-k3", Outcome: OutcomeDone}
	if err := s.Log(now); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Records(); err != nil || !reflect.DeepEqual(got, append(want, now)) {
		t.Fatalf("after one: %+v, %v", got, err)
	}
	if b, err := os.ReadFile(up); err != nil || string(b) != old {
		t.Fatalf("usage.json is %q, %v", b, err)
	}
	// From then on the record alone counts: usage.json is not read again.
	if err := os.WriteFile(up, []byte(`{"models": [{"model": "", "requests": 1}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Records(); err != nil || len(got) != 3 {
		t.Fatalf("read usage.json again: %+v, %v", got, err)
	}
}

// A usage.json gone wrong by hand is reported with the file and the model.
func TestWhatWasSpentIsChecked(t *testing.T) {
	for _, c := range []struct{ file, want string }{
		{`{"models": [{"model": "", "requests": 1, "input": 1, "output": 1}]}`, "models[0]: model is empty"},
		{`{"models": [{"model": "a/b", "requests": 1, "input": 1, "output": 1}, {"model": "a/b", "requests": 1, "input": 1, "output": 1}]}`, `models[1]: model "a/b" is listed twice`},
		{`{"models": [{"model": "a/b", "requests": 1, "input": -1, "output": 1}]}`, "models[0]: counts cannot be negative"},
		{`{"models": [{"model": "a/b", "requests": 1, "input": 1, "output": 1, "cost": 1}]}`, "models[0]: cost needs its currency"},
		{`{"models": [{"model": "a/b", "tokens": 1}]}`, `unknown field "tokens"`},
	} {
		s := testStore(t)
		if err := os.WriteFile(filepath.Join(s.Root(), "usage.json"), []byte(c.file), 0644); err != nil {
			t.Fatal(err)
		}
		_, err := s.Records()
		if err == nil || !strings.Contains(err.Error(), "usage.json") || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v", c.file, err)
		}
		if err := s.Log(Record{At: time.Now(), Feature: FeatureTalk, Model: "a/b", Outcome: OutcomeDone}); err == nil {
			t.Errorf("%s: logged over it", c.file)
		}
	}
}

// A request's purpose goes with its context.
func TestThePurposeGoesWithTheContext(t *testing.T) {
	if p := PurposeOf(context.Background()); !reflect.DeepEqual(p, Purpose{}) {
		t.Fatal(p)
	}
	p := Purpose{Feature: FeatureCheck, Chapter: "c1", Parts: []Part{{PartProse, 900}}}
	if got := PurposeOf(WithPurpose(context.Background(), p)); !reflect.DeepEqual(got, p) {
		t.Fatal(got)
	}
}
