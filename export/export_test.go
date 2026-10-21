package export

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"writer/workspace"
)

// book is a volume of two chapters, the first with a blank line between
// its scenes, and a chapter outside any volume with no prose yet.
var book = []workspace.Piece{
	{ID: "v", Title: "第一卷", Folder: true, Outline: "卷纲"},
	{ID: "1", Title: "第一章", Depth: 1, Text: "雨下着。\n\n她 走了。\n", Outline: "她走了\n\n雨停了"},
	{ID: "2", Title: "第二章", Depth: 1, Text: "天晴了。"},
	{ID: "3", Title: "番外", Text: "  "},
}

// The prose as plain text is what it always was: the work's name only for
// the whole of it, a blank line kept as the author left it, unindented.
func TestProseAsText(t *testing.T) {
	cover := Cover{Title: "测试作品", Pen: "雨声", Words: "11 字"}
	if got, want := Prose(cover, book, true).Text(true), "测试作品\n\n第一卷\n\n第一章\n\n　　雨下着。\n\n　　她 走了。\n\n第二章\n\n　　天晴了。\n\n番外\n"; got != want {
		t.Fatalf("whole:\n%q\nwant\n%q", got, want)
	}
	if got, want := Prose(cover, book[2:3], false).Text(false), "第二章\n\n天晴了。\n"; got != want {
		t.Fatalf("a chapter:\n%q\nwant\n%q", got, want)
	}
}

func TestOutlineAndSettingAsText(t *testing.T) {
	if got, want := Outline("测试作品", book).Text(false), "测试作品 · 大纲\n\n第一卷\n\n卷纲\n\n第一章\n\n她走了\n雨停了\n\n第二章\n\n番外\n"; got != want {
		t.Fatalf("outline:\n%q\nwant\n%q", got, want)
	}
	cats := []workspace.Category{{Name: "人物"}, {Name: "地点"}, {Name: "其他"}}
	entries := []workspace.Entry{
		{Place: workspace.PlaceBody, Category: "其他", Text: "雨季有九个月。"},
		{Place: workspace.PlaceBody, Category: "人物", Names: []string{"林晚", "晚晚", "阿晚"}, Text: "修灯的人。\n怕黑。"},
		{Place: workspace.PlaceOpen, Category: "人物", Names: []string{"周祥"}, Text: "还没定。"},
		{Place: workspace.PlaceDropped, Category: "地点", Names: []string{"井"}, Text: "不要了。"},
		{Place: workspace.PlaceBody, Category: "人物", Names: []string{"老陈"}, Text: "门卫。"},
	}
	if got, want := Setting("测试作品", cats, entries, nil).Text(false), "测试作品 · 设定\n\n人物\n\n林晚（晚晚、阿晚）：修灯的人。\n怕黑。\n老陈：门卫。\n\n其他\n\n雨季有九个月。\n"; got != want {
		t.Fatalf("setting:\n%q\nwant\n%q", got, want)
	}
	// How an entry reads from a chapter on comes under it.
	later := func(e workspace.Entry) []string {
		if len(e.Names) > 0 && e.Names[0] == "林晚" {
			return []string{"第40章起：不怕黑了。\n还修灯。"}
		}
		return nil
	}
	if got, want := Setting("测试作品", cats, entries, later).Text(false), "测试作品 · 设定\n\n人物\n\n林晚（晚晚、阿晚）：修灯的人。\n怕黑。\n第40章起：不怕黑了。\n还修灯。\n老陈：门卫。\n\n其他\n\n雨季有九个月。\n"; got != want {
		t.Fatalf("setting:\n%q\nwant\n%q", got, want)
	}
	if Settled(entries[2:4]) || !Settled(entries) {
		t.Fatal("what is settled is not told")
	}
}

// unzip reads a .docx's parts, each of them whole XML.
func unzip(t *testing.T, b []byte) map[string]string {
	t.Helper()
	z, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	parts := map[string]string{}
	for _, f := range z.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		d := xml.NewDecoder(bytes.NewReader(data))
		for {
			if _, err := d.Token(); errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				t.Fatalf("%s is not XML: %v", f.Name, err)
			}
		}
		parts[f.Name] = string(data)
	}
	return parts
}

// The manuscript: a cover of its own section, unnumbered, then each
// chapter from a new page under Word's own heading styles, a volume's
// title on the page of its first chapter, the pages numbered at the foot.
func TestProseAsWord(t *testing.T) {
	b, err := Prose(Cover{Title: "测试<作品>", Pen: "雨声", Words: "11 字"}, book, true).Docx()
	if err != nil {
		t.Fatal(err)
	}
	parts := unzip(t, b)
	for _, name := range []string{"[Content_Types].xml", "_rels/.rels", "docProps/core.xml", "word/_rels/document.xml.rels", "word/document.xml", "word/styles.xml", "word/footer1.xml"} {
		if _, ok := parts[name]; !ok {
			t.Fatalf("no %s in %v", name, parts)
		}
	}
	doc := parts["word/document.xml"]
	for _, want := range []string{`<w:pStyle w:val="Title"/>`, "测试&lt;作品&gt;", "作者：雨声", "字数：11 字", `<w:pStyle w:val="Heading1"/>`, `<w:pStyle w:val="Heading2"/>`, `w:fldSimple`} {
		if !strings.Contains(doc+parts["word/footer1.xml"], want) {
			t.Fatalf("no %s in\n%s", want, doc)
		}
	}
	if strings.Count(doc, "<w:sectPr>") != 2 || !strings.Contains(doc, `<w:pgNumType w:start="1"/></w:sectPr></w:body>`) {
		t.Fatalf("the cover is not a section of its own, the rest numbered from 1:\n%s", doc)
	}
	if strings.Count(doc[:strings.Index(doc, "</w:sectPr>")], "footerReference") != 0 {
		t.Fatal("the cover has a page number")
	}
	// 第一卷 and 第一章 share a page; 第二章 and 番外 begin their own.
	if n := strings.Count(doc, "<w:pageBreakBefore/>"); n != 2 {
		t.Fatalf("%d page breaks, want 2:\n%s", n, doc)
	}
	if !strings.Contains(doc, `<w:pageBreakBefore/></w:pPr><w:r><w:t xml:space="preserve">番外`) {
		t.Fatal("a chapter outside the volume does not begin a page")
	}
	styles := parts["word/styles.xml"]
	for _, want := range []string{`w:styleId="Normal"`, `<w:name w:val="heading 1"/>`, `<w:name w:val="heading 2"/>`, `w:eastAsia="宋体"`, `w:eastAsia="黑体"`, `w:firstLineChars="200" w:firstLine="480"`, `w:line="360"`, `<w:sz w:val="24"/>`} {
		if !strings.Contains(styles, want) {
			t.Fatalf("no %s in the styles", want)
		}
	}
	if !strings.Contains(parts["docProps/core.xml"], "<dc:creator>雨声</dc:creator>") {
		t.Fatal("the pen name is not the document's author")
	}
}

// The setting as Word: no cover, the name of each entry in bold.
func TestSettingAsWord(t *testing.T) {
	entries := []workspace.Entry{{Place: workspace.PlaceBody, Category: "人物", Names: []string{"林晚", "晚晚"}, Text: "修灯的人。"}}
	b, err := Setting("测试作品", []workspace.Category{{Name: "人物"}}, entries, nil).Docx()
	if err != nil {
		t.Fatal(err)
	}
	doc := unzip(t, b)["word/document.xml"]
	if strings.Contains(doc, "字数") || !strings.Contains(doc, "测试作品 · 设定") ||
		!strings.Contains(doc, `<w:rPr><w:b/><w:bCs/></w:rPr><w:t xml:space="preserve">林晚</w:t>`) || !strings.Contains(doc, "（晚晚）：修灯的人。") {
		t.Fatalf("setting:\n%s", doc)
	}
}

// A volume's chapters, one chapter, and a run of them across two
// volumes: each volume once, above its first chapter in the range.
func TestWithin(t *testing.T) {
	pieces := []workspace.Piece{
		{ID: "v1", Folder: true}, {ID: "1", Depth: 1}, {ID: "2", Depth: 1},
		{ID: "v2", Folder: true}, {ID: "p", Depth: 1, Folder: true}, {ID: "3", Depth: 2}, {ID: "4", Depth: 1},
		{ID: "5"},
	}
	ids := func(chapters ...string) string {
		var in map[string]bool
		if chapters != nil {
			in = map[string]bool{}
			for _, c := range chapters {
				in[c] = true
			}
		}
		var out []string
		for _, p := range Within(pieces, in) {
			out = append(out, p.ID)
		}
		return strings.Join(out, " ")
	}
	for _, c := range []struct{ got, want string }{
		{ids(), "v1 1 2 v2 p 3 4 5"},
		{ids("1", "2"), "v1 1 2"},
		{ids("5"), "5"},
		{ids("2", "3", "4"), "v1 2 v2 p 3 4"},
		{ids("4", "5"), "v2 4 5"},
		{ids("v1"), ""},
	} {
		if c.got != c.want {
			t.Fatalf("%q, want %q", c.got, c.want)
		}
	}
}

// A folder in a folder is 标题 2, a chapter in it 标题 3, and no deeper.
func TestHeadingLevels(t *testing.T) {
	pieces := []workspace.Piece{{Title: "卷", Folder: true}, {Title: "篇", Depth: 1, Folder: true}, {Title: "章", Depth: 2}, {Title: "节", Depth: 3}}
	var got []int
	for _, p := range Prose(Cover{}, pieces, true).Paras {
		got = append(got, p.Level)
	}
	if want := []int{1, 2, 3, 3}; !slices.Equal(got, want) {
		t.Fatalf("levels %v, want %v", got, want)
	}
}
