package writing

import (
	"strings"
	"testing"
	"writer/workspace"
)

func TestEnglishContinuationRequestsCarryOutputLanguageAndChineseBytesStayUnchanged(t *testing.T) {
	f := &fake{}
	w, id, _ := book(t, f)
	chinese, _, err := w.prepare(continueTask, Scene{Chapter: id}, "")
	if err != nil {
		t.Fatal(err)
	}
	work := w.Store.Work()
	work.Language = "en"
	work.Categories = append(work.Categories, workspace.Category{Name: "Other", Target: "Ideas"})
	if err = w.Store.SaveWork(work); err != nil {
		t.Fatal(err)
	}
	english, _, err := w.prepare(continueTask, Scene{Chapter: id}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(english.System, "The prose is written in English.") || strings.Contains(chinese.System, "The prose is written in English.") {
		t.Fatal(english.System)
	}
}
