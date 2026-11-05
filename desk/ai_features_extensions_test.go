package desk

import (
	"reflect"
	"testing"
	"time"
	"writer/ext"
	"writer/ext/ai/aiapi"
	"writer/ext/hostapi"
	"writer/workspace"
)

func TestContinueAndUsageSwitchesRemoveTheirContributionsAndRestoreThem(t *testing.T) {
	for _, id := range []string{"continue", "usage"} {
		for _, book := range []bool{false, true} {
			t.Run(id+map[bool]string{false: "/global", true: "/book"}[book], func(t *testing.T) {
				a, tt, _ := writingFixture(t, "正文。")
				count := func() int {
					n := 0
					for _, item := range ext.Items(a.extensions, hostapi.SettingsSections) {
						if item.Owner == id {
							n++
						}
					}
					for _, item := range ext.Items(a.extensions, aiapi.Usage) {
						if item.Owner == id {
							n++
						}
					}
					return n
				}
				before := count()
				if before == 0 {
					t.Fatal("no contributions")
				}
				a.setExtension(id, false, book)
				tt.Frame()
				if count() != 0 {
					t.Fatal("disabled contribution remained")
				}
				if id == "continue" {
					if !shownEditor(a).NoAI() || len(ext.Items(a.extensions, hostapi.Predictions)) != 0 {
						t.Fatal("prediction remained")
					}
				}
				if id == "usage" && len(ext.Items(a.extensions, aiapi.Calls)) != 0 {
					t.Fatal("record observer remained")
				}
				a.setExtension(id, true, book)
				tt.Frame()
				if count() != before {
					t.Fatal("contributions did not return")
				}
			})
		}
	}
}

func TestUsageObserverWritesEveryOutcomeWithUnchangedFieldsAndOffWritesNothing(t *testing.T) {
	a, _, _ := writingFixture(t, "正文。")
	var want []workspace.Record
	for _, outcome := range []string{workspace.OutcomeDone, workspace.OutcomeFailed, workspace.OutcomeCancelled} {
		r := workspace.Record{Schema: workspace.Schema, At: time.Now().UTC(), Took: 12, Feature: workspace.FeatureContinue, Chapter: a.write.node, Model: "local/model", Effort: "high", Parts: []workspace.Part{{Name: workspace.PartRules, Tokens: 10}}, Usage: workspace.Usage{Input: 10, Output: 3}, Outcome: outcome}
		if outcome == workspace.OutcomeFailed {
			r.Error = "failed"
		}
		want = append(want, r)
		for _, item := range ext.Items(a.extensions, aiapi.Calls) {
			if err := ext.Guard(a.extensions, item.Owner, item.ID, func() { item.Value(aiapi.Call{Store: a.Store, Record: r}) }); err != nil {
				t.Fatal(err)
			}
		}
	}
	got, err := a.Store.Records()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal(got, want, err)
	}
	a.setExtension("usage", false, false)
	for _, item := range ext.Items(a.extensions, aiapi.Calls) {
		item.Value(aiapi.Call{Store: a.Store, Record: want[0]})
	}
	got, err = a.Store.Records()
	if err != nil || len(got) != 3 {
		t.Fatal(got, err)
	}
}

func TestContinueAndUsageLedgersListTheirContributionsWhileOff(t *testing.T) {
	a, _, _ := writingFixture(t, "")
	for _, id := range []string{"continue", "usage"} {
		before := a.extensions.Ledger(id)
		if len(before.Contributions) == 0 {
			t.Fatal(id, "empty ledger")
		}
		a.setExtension(id, false, false)
		after := a.extensions.Ledger(id)
		if len(after.Contributions) != len(before.Contributions) {
			t.Fatal(id, "lost ledger")
		}
	}
}
