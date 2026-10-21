package workspace

import (
	"reflect"
	"testing"
)

func TestEntryReading(t *testing.T) {
	e := Entry{Text: "最初", Changes: []Change{
		{ID: "last", From: "c", Place: PlaceBody, Text: "后来"},
		{ID: "lost", From: "gone", Place: PlaceBody, Text: "删除的章"},
		{ID: "open", From: "b", Place: PlaceOpen, Text: "待确认"},
		{ID: "first", From: "b", Place: PlaceBody, Text: "变了"},
		{ID: "drop", From: "c", Place: PlaceDropped, Text: "放弃的"},
	}}
	order := Order{"a": 0, "b": 1, "c": 2, "d": 3}
	for _, tc := range []struct{ at, text, change string }{
		{"a", "最初", ""}, {"b", "变了", "first"}, {"c", "后来", "last"}, {"d", "后来", "last"},
		{"", "后来", "last"}, {"gone", "最初", ""},
	} {
		text, change, ok := e.Reading(order, tc.at)
		if text != tc.text || change.ID != tc.change || ok != (tc.change != "") {
			t.Fatal(tc, text, change, ok)
		}
	}
	if text, change, ok := e.Reading(nil, ""); text != "最初" || !reflect.DeepEqual(change, Change{}) || ok {
		t.Fatal(text, change, ok)
	}
	if e.Changes[0].ID != "last" {
		t.Fatal("reading reordered stored changes", e.Changes)
	}
}

func TestEntryReadingBefore(t *testing.T) {
	e := Entry{Text: "最初", Changes: []Change{
		{ID: "last", From: "c", Place: PlaceBody, Text: "后来"},
		{ID: "first", From: "b", Place: PlaceBody, Text: "变了"},
		{ID: "open", From: "a", Place: PlaceOpen, Text: "待确认"},
		{ID: "gone", From: "gone", Place: PlaceBody, Text: "已删除"},
	}}
	for at, want := range map[string]string{"a": "最初", "b": "最初", "c": "变了", "d": "后来", "": "最初", "gone": "最初"} {
		if got := e.ReadingBefore(Order{"a": 0, "b": 1, "c": 2, "d": 3}, at); got != want {
			t.Fatal(at, got, want)
		}
	}
	if got := e.ReadingBefore(nil, ""); got != "最初" {
		t.Fatal(got)
	}
}
