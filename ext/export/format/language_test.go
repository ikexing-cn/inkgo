package format

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"strings"
	"testing"
	"writer/workspace"
)

func TestExportLanguageFontsPunctuationAndIndentationFollowBook(t *testing.T) {
	base := Setting("Book", []workspace.Category{{Name: "People"}}, []workspace.Entry{{Place: workspace.PlaceBody, Category: "People", Names: []string{"Mara", "M"}, Text: "Sails."}}, nil)
	for _, language := range []workspace.Language{workspace.Chinese, workspace.English} {
		d := base
		d.Language = language
		d.Cover = &Cover{Title: "Book", Pen: "Mara", Words: "7"}
		text := d.Text(true)
		data, err := d.Docx()
		if err != nil {
			t.Fatal(err)
		}
		z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		files := map[string]string{}
		for _, f := range z.File {
			r, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			b, err := io.ReadAll(r)
			r.Close()
			if err != nil {
				t.Fatal(err)
			}
			files[f.Name] = string(b)
		}
		styles := files["word/styles.xml"]
		coverLabel := "字数：7"
		authorLabel := "作者：Mara"
		if language == "en" {
			coverLabel = "Word count: 7"
			authorLabel = "Author: Mara"
		}
		if !strings.Contains(files["word/document.xml"], coverLabel) || !strings.Contains(files["word/document.xml"], authorLabel) {
			t.Fatal(files["word/document.xml"])
		}
		if language == "en" {
			for _, want := range []string{`w:val="en"`, `Times New Roman`, `Georgia`, `w:firstLineChars="0"`} {
				if !strings.Contains(styles, want) {
					t.Fatal(want, styles)
				}
			}
			if strings.ContainsAny(text, "　：（）、") || !strings.Contains(text, "Mara (M): Sails.") {
				t.Fatal(text)
			}
		} else {
			if fmt.Sprintf("%x", sha256.Sum256([]byte(styles))) != "ee6f65df401cde9e8eca76758bb6fad4f922dc5cec9aaef3141500c5e43ccf3f" || text != base.Text(true) || !strings.Contains(styles, "zh-CN") || !strings.Contains(text, "　　Mara（M）：Sails.") {
				t.Fatal(text, styles)
			}
		}
	}
	plan := BuildPlan([]workspace.Piece{{Text: "Hello world 世界"}}, nil, "", Options{}, Selection{}, func(a, b string) string { return a + b }, workspace.English)
	if plan.Words != 4 {
		t.Fatal(plan.Words)
	}
}
