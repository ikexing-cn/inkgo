package writing

import (
	"context"
	"strings"
	"testing"

	"writer/model"
	"writer/workspace"
)

// A check shows the model the setting by number, never by ID, and keeps
// only what it can hold the model to: the prose's own words, an entry
// shown, a name the setting has or has not.
func TestCheckAChapterAgainstTheSetting(t *testing.T) {
	f := &fake{}
	w, two, r := book(t, f)
	s := w.Store
	if _, err := s.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{
		{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "主角靠维修路灯谋生，但她很怕黑。", Names: []string{"林晚"}},
	}}); err != nil {
		t.Fatal(err)
	}
	text := "林婉站在巷口，天晴得发白。她没有去摸手电，一个人走进了没有灯的巷子。老周在巷子那头等她。" + strings.Repeat("她蹲下去拧开灯座。", 30)
	if _, err := s.WriteChapter(two, text, nil); err != nil {
		t.Fatal(err)
	}
	f.pieces = []string{"```json\n", `{"findings":[
		{"kind":"conflict","title":"林晚怕黑，这里却一个人进了黑巷子","quote":"一个人走进了“没有灯”的巷子","entry":2,"says":"但她很怕黑"},
		{"kind":"conflict","title":"编的","quote":"她飞了起来","entry":1,"says":"但她很怕黑"},
		{"kind":"conflict","title":"没有这一条","quote":"天晴得发白","entry":99},
		{"kind":"name","title":"「林婉」是不是「林晚」写错了？","quote":"林婉","name":"林晚"},
		{"kind":"name","title":"设定里没有这个名字","quote":"老周","name":"老张"},
		{"kind":"new","title":"随便","quote":"老周在巷子那头等她","name":"老周","category":"人物","text":"在巷子那头等林晚的人"},
		{"kind":"new","title":"已经有了","quote":"林婉站在巷口","name":"林晚","category":"人物","text":"x"},
		{"kind":"new","title":"正文里没有","quote":"天晴得发白","name":"老李","category":"人物","text":"x"},
		{"kind":"odd","title":"x","quote":"天晴得发白"}
	]}`, "\n```"}
	got, err := w.Check(context.Background(), two)
	if err != nil {
		t.Fatal(err)
	}
	if got.Of != workspace.ProseHash(text) || got.By != "deepseek/deepseek-flash" || len(got.Findings) != 3 {
		t.Fatalf("%+v", got)
	}
	if got.Sent == nil || got.Sent.Part(workspace.PartSetting) == 0 || got.Sent.Part(workspace.PartProse) == 0 {
		t.Fatalf("sent %+v", got.Sent)
	}
	set, _ := s.Setting()
	var lin workspace.Entry
	for _, e := range set.Entries {
		if len(e.Names) > 0 {
			lin = e
		}
	}
	c, n, nw := got.Findings[0], got.Findings[1], got.Findings[2]
	if c.Kind != workspace.FindConflict || c.Quote != "一个人走进了没有灯的巷子" || c.Entry != lin.ID || c.Says != "但她很怕黑" || c.ID == "" {
		t.Fatalf("%+v", c)
	}
	if n.Kind != workspace.FindName || n.Quote != "林婉" || n.Name != "林晚" {
		t.Fatalf("%+v", n)
	}
	if nw.Kind != workspace.FindNew || nw.Name != "老周" || nw.Title != "老周" || nw.Category != "人物" || nw.Text != "在巷子那头等林晚的人" {
		t.Fatalf("%+v", nw)
	}
	req := f.got[0]
	msg := req.Messages[0].Content
	// The entries are numbered in the work's order, not the newest first.
	if req.System != checkRules || req.Effort != model.EffortHigh || !strings.Contains(msg, "[1]（人物｜已放弃）主角会飞\n[2]（人物｜林晚）主角靠维修路灯谋生") || strings.Contains(msg, "她有个妹妹") || !strings.Contains(msg, "【章名】第二章") {
		t.Fatalf("%+v", req)
	}
	for _, id := range append([]string{two, lin.ID}, r.Ops[0].Entry) {
		if strings.Contains(msg, id) {
			t.Fatal("the model saw an ID")
		}
	}

	// What the author called meant is kept, told to the model, and not
	// found again; what the prose no longer says is forgotten.
	got.Findings[0].Meant, got.Findings[1].Meant = true, true
	if err = s.WriteCheck(two, got); err != nil {
		t.Fatal(err)
	}
	text = strings.Replace(text, "林婉", "林晚", 1)
	if _, err = s.WriteChapter(two, text, nil); err != nil {
		t.Fatal(err)
	}
	f.pieces = []string{`{"findings":[{"kind":"conflict","title":"又来","quote":"一个人走进了没有灯的巷子","entry":2,"says":"但她很怕黑"}]}`}
	again, err := w.Check(context.Background(), two)
	if err != nil || len(again.Findings) != 1 || !again.Findings[0].Meant || again.Findings[0].ID != c.ID {
		t.Fatalf("%+v %v", again, err)
	}
	if msg = f.got[1].Messages[0].Content; !strings.Contains(msg, "【作者说过是故意的】\n- 一个人走进了没有灯的巷子\n") {
		t.Fatal(msg)
	}

	// An answer that is not the JSON asked for is an error; so are a
	// chapter too short and a setting with nothing settled.
	f.pieces = []string{"我看了一下，没有问题。"}
	if _, err = w.Check(context.Background(), two); err == nil {
		t.Fatal("took prose for an answer")
	}
	tree, _ := s.Chapters()
	node, _ := tree.Node(two)
	one, _ := previous(tree, node)
	if _, err = w.Check(context.Background(), one.ID); err == nil {
		t.Fatal("checked a chapter too short")
	}
}

// A chapter too long to give whole is checked from its start, and what
// is kept says how much of it that was.
func TestAChapterReadInPartSaysSo(t *testing.T) {
	f := &fake{}
	w, two, _ := book(t, f)
	s := w.Store
	if _, err := s.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{
		{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "主角靠维修路灯谋生，但她很怕黑。", Names: []string{"林晚"}},
	}}); err != nil {
		t.Fatal(err)
	}
	text := strings.Repeat("她蹲下去拧开灯座。", 4000)
	if _, err := s.WriteChapter(two, text, nil); err != nil {
		t.Fatal(err)
	}
	f.pieces = []string{`{"findings":[]}`}
	got, err := w.Check(context.Background(), two)
	if err != nil {
		t.Fatal(err)
	}
	if got.Words != workspace.Count(text) || got.Read <= 0 || got.Read >= got.Words {
		t.Fatalf("read %d of %d", got.Read, got.Words)
	}
	said := f.got[len(f.got)-1].Messages[0].Content
	if !strings.Contains(said, "（正文太长，只给了前面一部分）") || strings.Count(said, "她蹲下去拧开灯座。") >= 4000 {
		t.Fatal("the model was not told, or was given the whole")
	}
	if err = s.WriteCheck(two, got); err != nil {
		t.Fatal(err)
	}
	if kept, err := s.ReadCheck(two); err != nil || kept.Read != got.Read {
		t.Fatalf("kept %+v, %v", kept.Read, err)
	}

	// One that fits is read whole.
	short := strings.Repeat("她蹲下去拧开灯座。", 30)
	if _, err = s.WriteChapter(two, short, nil); err != nil {
		t.Fatal(err)
	}
	f.pieces = []string{`{"findings":[]}`}
	if got, err = w.Check(context.Background(), two); err != nil || got.Read != 0 {
		t.Fatalf("read %d, %v", got.Read, err)
	}
}
