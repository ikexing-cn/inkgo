// Command sample writes the books to test with: each a work folder with
// its setting, volumes, chapters, outlines and foreshadowings, and prose
// at odds with the setting here and there for the checks to find.
//
//	go run ./cmd/sample <目录>
//	go run ./cmd/inkgo -work <目录>/<书名>
//
// A book is a text file in books/, read a line at a time:
//
//	# 一行注释
//	书名：雨城修灯人
//	题材：出版小说
//
//	= 设定
//	人物｜林晚、晚晚｜主角，怕黑。          类别｜名字、别名｜内容（没有名字就空着）
//	等你决定｜人物｜｜他的左手是不是有伤？   等作者决定的一条
//	已删除｜情节｜｜她最后离开了雨城。       已删除的一条
//
//	= 卷：第一卷 灯灭
//	卷纲：这一卷写什么。
//
//	== 第一章 停电
//	章纲：这一章写什么。
//	正文一行一段，空行不算。
//	{伏:铜钥匙|这句话记为伏笔，备注是「铜钥匙」。}
//	{收:铜钥匙|这句话回收备注是「铜钥匙」的伏笔。}
//	{弃:红雨伞|这句话是已放弃的伏笔。}
package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"writer/workspace"
)

//go:embed books/*.txt
var books embed.FS

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "用法：go run ./cmd/sample <目录>")
		os.Exit(2)
	}
	dirs, err := writeAll(os.Args[1])
	for _, d := range dirs {
		fmt.Printf("go run ./cmd/inkgo -work %q\n", d)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// writeAll writes every book into a folder of its name under dir and
// returns the folders written.
func writeAll(dir string) ([]string, error) {
	names, err := books.ReadDir("books")
	if err != nil {
		return nil, err
	}
	sort.Slice(names, func(i, j int) bool { return names[i].Name() < names[j].Name() })
	var dirs []string
	for _, n := range names {
		data, err := books.ReadFile("books/" + n.Name())
		if err != nil {
			return dirs, err
		}
		b, err := parse(string(data))
		if err != nil {
			return dirs, fmt.Errorf("%s: %w", n.Name(), err)
		}
		d := filepath.Join(dir, b.name)
		if err = b.write(d); err != nil {
			return dirs, fmt.Errorf("%s: %w", n.Name(), err)
		}
		dirs = append(dirs, d)
	}
	return dirs, nil
}

type book struct {
	name, genre string
	entries     []workspace.Op
	nodes       []node
}

// node is a volume, or a chapter of the volume before it.
type node struct {
	title, outline string
	volume         bool
	paras          []string
}

func parse(src string) (book, error) {
	var b book
	setting := false
	for i, line := range strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		last := len(b.nodes) - 1
		rest, is := func(prefix string) (string, bool) {
			r, ok := strings.CutPrefix(line, prefix)
			return strings.TrimSpace(r), ok
		}, false
		var v string
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case line == "= 设定":
			setting = true
		case strings.HasPrefix(line, "== "):
			v, _ = rest("== ")
			setting = false
			b.nodes = append(b.nodes, node{title: v})
		case strings.HasPrefix(line, "= 卷："):
			v, _ = rest("= 卷：")
			setting = false
			b.nodes = append(b.nodes, node{title: v, volume: true})
		case setting:
			op, err := entry(line)
			if err != nil {
				return b, fmt.Errorf("line %d: %w", i+1, err)
			}
			b.entries = append(b.entries, op)
		case last < 0:
			if v, is = rest("书名："); is {
				b.name = v
			} else if v, is = rest("题材："); is {
				b.genre = v
			} else {
				return b, fmt.Errorf("line %d: %q before any volume or chapter", i+1, line)
			}
		case b.nodes[last].volume:
			if v, is = rest("卷纲："); !is {
				return b, fmt.Errorf("line %d: prose in a volume", i+1)
			}
			b.nodes[last].outline = v
		default:
			if v, is = rest("章纲："); is {
				b.nodes[last].outline = v
			} else {
				b.nodes[last].paras = append(b.nodes[last].paras, line)
			}
		}
	}
	if b.name == "" || b.genre == "" {
		return b, fmt.Errorf("missing book title or genre")
	}
	return b, nil
}

// entry reads a line of the setting.
func entry(line string) (workspace.Op, error) {
	f := strings.Split(line, "｜")
	op := workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceBody}
	switch {
	case len(f) == 4 && f[0] == "等你决定":
		op.Place, f = workspace.PlaceOpen, f[1:]
	case len(f) == 4 && f[0] == "已删除":
		op.Place, f = workspace.PlaceDropped, f[1:]
	case len(f) != 3:
		return op, fmt.Errorf("%q is not category | name | content", line)
	}
	op.Category, op.Text = strings.TrimSpace(f[0]), strings.TrimSpace(f[2])
	op.Names = workspace.CleanNames(strings.Split(f[1], "、"))
	return op, nil
}

var marked = regexp.MustCompile(`\{(伏|收|弃):([^|}]*)\|([^}]*)\}`)

// write makes the work in dir, which must be missing or empty.
func (b book) write(dir string) error {
	s, err := workspace.Create(dir, b.name, b.genre)
	if err != nil {
		return err
	}
	defer s.Close()
	if len(b.entries) > 0 {
		if _, err = s.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: b.entries}); err != nil {
			return err
		}
	}
	at := time.Now().Format(time.RFC3339)
	buried := map[string]string{} // a foreshadowing's ID by its note
	volume, n := "", 0
	var chapterIDs []string
	for _, nd := range b.nodes {
		parent := volume
		if nd.volume {
			parent = ""
		}
		id, err := s.NewChapter(parent, nd.title, nd.volume)
		if err != nil {
			return err
		}
		if nd.outline != "" {
			if err = s.WriteOutline(id, nd.outline); err != nil {
				return err
			}
		}
		if nd.volume {
			volume = id
			continue
		}
		chapterIDs = append(chapterIDs, id)
		var text strings.Builder
		var marks []workspace.ChapterMark
		for _, p := range nd.paras {
			from := 0
			for _, m := range marked.FindAllStringSubmatchIndex(p, -1) {
				text.WriteString(p[from:m[0]])
				kind, note, quote := p[m[2]:m[3]], p[m[4]:m[5]], p[m[6]:m[7]]
				start := utf8.RuneCountInString(text.String())
				text.WriteString(quote)
				n++
				mk := workspace.ChapterMark{Kind: workspace.MarkForeshadow, Start: start, End: start + utf8.RuneCountInString(quote),
					Note: note, ID: fmt.Sprintf("sample-%d", n), At: at, Dropped: kind == "弃"}
				if kind == "收" {
					of, ok := buried[note]
					if !ok {
						return fmt.Errorf("%s: no foreshadowing %q to pay off", nd.title, note)
					}
					mk.Kind, mk.Of, mk.Note = workspace.MarkPayoff, of, ""
				} else if buried[note] != "" {
					return fmt.Errorf("%s: foreshadowing %q twice", nd.title, note)
				} else {
					buried[note] = mk.ID
				}
				marks = append(marks, mk)
				from = m[1]
			}
			text.WriteString(p[from:])
			text.WriteString("\n")
		}
		if _, err = s.WriteChapter(id, text.String(), marks); err != nil {
			return err
		}
	}
	return b.writePlan(s, chapterIDs)
}
