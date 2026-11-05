package workspace

import "testing"

func TestSnapshotFreezesConversationSourceTitle(t *testing.T) {
	s, err := Create(t.TempDir(), "book", "网文", "zh-Hans")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	id, err := s.NewChapter("", "Original title", false)
	if err != nil {
		t.Fatal(err)
	}
	session, err := s.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Append(session, Event{Role: RoleUser, Origin: AIDraft, Chapter: id, Text: "draft"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SnapshotForTalks(id, "original prose", nil, []string{session}); err != nil {
		t.Fatal(err)
	}
	snaps, err := s.Snapshots(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 1 || len(snaps[0].Sources) != 1 {
		t.Fatal(snaps)
	}
	want := SnapshotSource{ID: session, Title: ConversationTitle{Kind: AIDraft, Text: "Original title"}}
	if snaps[0].Sources[0] != want {
		t.Fatal(snaps[0].Sources)
	}
	if err = s.RenameChapter(id, "Renamed title"); err != nil {
		t.Fatal(err)
	}
	snaps, err = s.Snapshots(id)
	if err != nil || snaps[0].Sources[0] != want {
		t.Fatal(snaps, err)
	}
}
