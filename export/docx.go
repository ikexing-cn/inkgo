package export

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
)

// The manuscript's look, in Word's units: twentieths of a point for
// lengths, half points for sizes. A4, 宋体 小四 at one and a half lines,
// the first line in by two characters; titles in 黑体.
const (
	pageWidth, pageHeight = 11906, 16838
	marginY, marginX      = 1440, 1800
	bodySize              = 24  // 小四, 12 pt
	lineSpacing           = 360 // 1.5 lines
	firstLine             = 2 * bodySize * 10
	coverDrop             = 3600 // how far down the cover's title stands
)

// headingSizes are 标题 1 to 3: 小二, 三号, 四号.
var headingSizes = [3]int{36, 32, 28}

const xmlHeader = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n"

// Word wants ind before jc.
const centered = `<w:ind w:firstLineChars="0" w:firstLine="0"/><w:jc w:val="center"/>`

const wordNS = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`

// esc is s as XML character data.
func esc(s string) string {
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

// Docx is d as a Word document. The cover is a section of its own, so the
// pages of the rest are numbered from 1 at the foot.
func (d Doc) Docx() ([]byte, error) {
	var body strings.Builder
	para := func(props, runs string) {
		body.WriteString("<w:p>")
		if props != "" {
			body.WriteString("<w:pPr>" + props + "</w:pPr>")
		}
		body.WriteString(runs + "</w:p>")
	}
	run := func(text string, bold bool) string {
		if text == "" {
			return ""
		}
		rPr := ""
		if bold {
			rPr = "<w:rPr><w:b/><w:bCs/></w:rPr>"
		}
		return `<w:r>` + rPr + `<w:t xml:space="preserve">` + esc(text) + `</w:t></w:r>`
	}
	page := fmt.Sprintf(`<w:pgSz w:w="%d" w:h="%d"/><w:pgMar w:top="%d" w:right="%d" w:bottom="%d" w:left="%d" w:header="851" w:footer="992" w:gutter="0"/>`,
		pageWidth, pageHeight, marginY, marginX, marginY, marginX)
	if c := d.Cover; c != nil {
		para(fmt.Sprintf(`<w:pStyle w:val="Title"/><w:spacing w:before="%d" w:after="1200"/>`, coverDrop), run(c.Title, false))
		if c.Pen != "" {
			para(centered, run("作者："+c.Pen, false))
		}
		// The cover's section ends with this paragraph; it has no footer.
		para(centered+`<w:sectPr>`+page+`</w:sectPr>`, run("字数："+c.Words, false))
	} else if d.Title != "" {
		para(`<w:pStyle w:val="Title"/>`, run(d.Title, false))
	}
	for _, p := range d.Paras {
		props := ""
		if p.Style == Heading {
			props = fmt.Sprintf(`<w:pStyle w:val="Heading%d"/>`, min(max(p.Level, 1), len(headingSizes)))
		}
		if p.NewPage {
			props += `<w:pageBreakBefore/>`
		}
		para(props, run(p.Lead, true)+run(p.Text, false))
	}
	body.WriteString(`<w:sectPr><w:footerReference w:type="default" r:id="rIdFooter"/>` + page + `<w:pgNumType w:start="1"/></w:sectPr>`)

	title, author := d.Title, ""
	if d.Cover != nil {
		title, author = d.Cover.Title, d.Cover.Pen
	}
	files := []struct{ name, data string }{
		{"[Content_Types].xml", contentTypes},
		{"_rels/.rels", rootRels},
		{"docProps/core.xml", xmlHeader +
			`<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/">` +
			`<dc:title>` + esc(title) + `</dc:title><dc:creator>` + esc(author) + `</dc:creator></cp:coreProperties>`},
		{"word/_rels/document.xml.rels", documentRels},
		{"word/document.xml", xmlHeader +
			`<w:document ` + wordNS + `><w:body>` + body.String() + `</w:body></w:document>`},
		{"word/styles.xml", styles()},
		{"word/footer1.xml", xmlHeader +
			`<w:ftr ` + wordNS + `><w:p><w:pPr>` + centered + `</w:pPr><w:fldSimple w:instr=" PAGE "><w:r><w:t>1</w:t></w:r></w:fldSimple></w:p></w:ftr>`},
	}
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	for _, f := range files {
		w, err := z.Create(f.name)
		if err != nil {
			return nil, err
		}
		if _, err = w.Write([]byte(f.data)); err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

const contentTypes = xmlHeader + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
	`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
	`<Default Extension="xml" ContentType="application/xml"/>` +
	`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
	`<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>` +
	`<Override PartName="/word/footer1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.footer+xml"/>` +
	`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>` +
	`</Types>`

const relationships = `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`

const rootRels = xmlHeader + relationships +
	`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>` +
	`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>` +
	`</Relationships>`

const documentRels = xmlHeader + relationships +
	`<Relationship Id="rIdStyles" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>` +
	`<Relationship Id="rIdFooter" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer" Target="footer1.xml"/>` +
	`</Relationships>`

// fonts sets Latin text in Times New Roman and Chinese in east, by the
// names Word and WPS know them by.
func fonts(east string) string {
	return `<w:rFonts w:ascii="Times New Roman" w:hAnsi="Times New Roman" w:cs="Times New Roman" w:eastAsia="` + east + `"/>`
}

// styles are Normal, Title and Heading 1 to 3 under the names Word gives
// its own, which it shows as 正文, 标题, 标题 1 to 3 in Chinese.
func styles() string {
	size := func(n int) string { return fmt.Sprintf(`<w:sz w:val="%d"/><w:szCs w:val="%d"/>`, n, n) }
	var b strings.Builder
	b.WriteString(xmlHeader + `<w:styles ` + wordNS + `>`)
	b.WriteString(`<w:docDefaults><w:rPrDefault><w:rPr>` + fonts("宋体") + size(bodySize) + `<w:lang w:val="en-US" w:eastAsia="zh-CN"/></w:rPr></w:rPrDefault>` +
		fmt.Sprintf(`<w:pPrDefault><w:pPr><w:spacing w:line="%d" w:lineRule="auto"/></w:pPr></w:pPrDefault></w:docDefaults>`, lineSpacing))
	b.WriteString(fmt.Sprintf(`<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:qFormat/>`+
		`<w:pPr><w:ind w:firstLineChars="200" w:firstLine="%d"/><w:jc w:val="both"/></w:pPr></w:style>`, firstLine))
	b.WriteString(`<w:style w:type="paragraph" w:styleId="Title"><w:name w:val="Title"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/>` +
		`<w:pPr><w:spacing w:after="480"/>` + centered + `</w:pPr><w:rPr>` + fonts("黑体") + size(44) + `</w:rPr></w:style>`)
	for i, n := range headingSizes {
		b.WriteString(fmt.Sprintf(`<w:style w:type="paragraph" w:styleId="Heading%d"><w:name w:val="heading %d"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/>`+
			`<w:pPr><w:keepNext/><w:spacing w:before="480" w:after="360"/>%s<w:outlineLvl w:val="%d"/></w:pPr><w:rPr>%s%s</w:rPr></w:style>`,
			i+1, i+1, centered, i, fonts("黑体"), size(n)))
	}
	b.WriteString(`</w:styles>`)
	return b.String()
}
