// Package workspace stores a work as canonical files: work.yaml, a JSONL
// file per session, and setting.jsonl, the revisions of the setting
// document. Nothing else is authoritative.
package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// Schema 2 replaced idea cards with the setting document; works of schema
// 1 are not migrated.
const Schema = 2
const Other = "其他"

// Message roles of a session file.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

type Category struct {
	Name   string `yaml:"name" json:"name"`
	Target string `yaml:"target" json:"target"`
}

// Selection is the model a use goes to. Effort is how hard it reasons:
// empty for the provider's default, or low, high, max.
type Selection struct {
	Provider string `yaml:"provider" json:"provider"`
	Model    string `yaml:"model" json:"model"`
	Effort   string `yaml:"effort,omitempty" json:"effort,omitempty"`
}
type Work struct {
	Schema      int        `yaml:"schema" json:"schema"`
	ID          string     `yaml:"id" json:"id"`
	Name        string     `yaml:"name" json:"name"`
	Genre       string     `yaml:"genre" json:"genre"`
	Categories  []Category `yaml:"categories" json:"categories"`
	Discussion  Selection  `yaml:"discussion" json:"discussion"`
	Bookkeeping Selection  `yaml:"bookkeeping" json:"bookkeeping"`
	// Writing continues, rewrites and drafts the prose; left empty, it is
	// the discussion's model.
	Writing     Selection `yaml:"writing,omitempty" json:"writing,omitempty"`
	TokenBudget int       `yaml:"token_budget" json:"token_budget"`
}

// ContextItem is an entry of the setting as one request showed it, or
// why it left the entry out.
type ContextItem struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Reason string `json:"reason"`
}

// Event is a message of a session.
type Event struct {
	Schema  int           `json:"schema"`
	ID      string        `json:"id"`
	Role    string        `json:"role"`
	Text    string        `json:"text"`
	At      time.Time     `json:"at"`
	Context []ContextItem `json:"context,omitempty"`
	Omitted []ContextItem `json:"omitted,omitempty"`
	ReplyTo string        `json:"reply_to,omitempty"`
	// What a model wrote is recorded with the model, provider/model, the
	// effort it ran at, and the reasoning it showed, for review afterwards.
	Model     string `json:"model,omitempty"`
	Effort    string `json:"effort,omitempty"`
	Reasoning string `json:"reasoning,omitempty"`
	// The author's words may come from the writing page: the chapter they
	// were writing, and the prose they quoted.
	Chapter string  `json:"chapter,omitempty"`
	Quotes  []Quote `json:"quotes,omitempty"`
}

// Quote is prose the author brought into the talk, with the chapter it
// came from and its title then.
type Quote struct {
	Chapter string `json:"chapter"`
	Title   string `json:"title"`
	Text    string `json:"text"`
}

// Said is the author's words as a model reads them: the prose they quoted,
// each under its chapter's title, then what they wrote.
func (e Event) Said() string {
	s, _ := e.said()
	return s
}

// QuoteAt returns the quote holding the runes [start, end) of Said, if one
// does.
func (e Event) QuoteAt(start, end int) (Quote, bool) {
	_, spans := e.said()
	for n, sp := range spans {
		if start >= sp[0] && end <= sp[1] {
			return e.Quotes[n], true
		}
	}
	return Quote{}, false
}

func (e Event) said() (string, [][2]int) {
	if len(e.Quotes) == 0 {
		return e.Text, nil
	}
	var b strings.Builder
	var spans [][2]int
	at := 0
	write := func(s string) {
		b.WriteString(s)
		at += utf8.RuneCountInString(s)
	}
	for _, q := range e.Quotes {
		write("【引用正文《" + q.Title + "》】\n")
		start := at
		write(q.Text)
		spans = append(spans, [2]int{start, at})
		write("\n\n")
	}
	write(e.Text)
	return b.String(), spans
}

// Places of an entry in the setting document.
const (
	PlaceBody    = "body"    // what the author decided
	PlaceOpen    = "open"    // waiting for the author: an AI's idea, a contradiction
	PlaceDropped = "dropped" // what the author does not want
)

// Source is where an entry comes from: words quoted verbatim from a
// message, or from a chapter's text as it was when the author took them. A
// message's words that are prose the author quoted name its chapter too.
type Source struct {
	Session string `json:"session,omitempty"`
	Message string `json:"message,omitempty"`
	Chapter string `json:"chapter,omitempty"`
	Quote   string `json:"quote"`
}

// Entry is one statement of the setting document.
type Entry struct {
	ID       string `json:"id"`
	Place    string `json:"place"`
	Category string `json:"category"`
	Text     string `json:"text"`
	// Names are the proper names the entry is about, the first the name
	// itself and the rest what else it is called: they complete as the
	// prose is typed.
	Names   []string `json:"names,omitempty"`
	Sources []Source `json:"sources,omitempty"`
	// Author is set once the author wrote or edited the entry: the
	// bookkeeper changes it only on the author's word.
	Author bool `json:"author,omitempty"`
	// Asked is the question of the AI's that the entry answers: the
	// author's answer took the question's place.
	Asked string `json:"asked,omitempty"`
	// Rev is the number of the last revision that changed it, from 1.
	Rev int `json:"-"`
}

// Ops a revision applies.
const (
	OpAdd  = "add"  // a new entry: Place, Category, Text, Sources
	OpEdit = "edit" // Entry's Text, and Sources added to its own
	OpMove = "move" // Entry to Place, or Category, or both, and Sources added
	OpName = "name" // Entry's Names, none to clear them
	// OpAnswer is the author's answer to an open entry that asks them
	// something: Entry becomes the answer, Text, of Category and Names,
	// settled, the question kept as its Asked.
	OpAnswer = "answer"
)

// NameCategories are the categories whose entries name something: a
// person, a place, a group, a thing, a rank or art of the power system.
var NameCategories = []string{"人物", "地点", "组织", "物品", "力量体系"}

// maxNameRunes bounds a name.
const maxNameRunes = 12

// CleanNames trims names and drops the empty and repeated ones.
func CleanNames(names []string) []string {
	var out []string
	for _, n := range names {
		if n = strings.TrimSpace(n); n != "" && !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	return out
}

// validNames checks names are short, one line and not repeated.
func validNames(names []string) error {
	for i, n := range names {
		switch {
		case strings.TrimSpace(n) != n || n == "":
			return fmt.Errorf("name %q has spaces around it or is empty", n)
		case strings.ContainsAny(n, "\n\r\t"):
			return fmt.Errorf("name %q spans lines", n)
		case utf8.RuneCountInString(n) > maxNameRunes:
			return fmt.Errorf("name %q is longer than %d characters", n, maxNameRunes)
		case slices.Contains(names[:i], n):
			return fmt.Errorf("name %q is repeated", n)
		}
	}
	return nil
}

type Op struct {
	Op       string   `json:"op"`
	Entry    string   `json:"entry"`
	Place    string   `json:"place,omitempty"`
	Category string   `json:"category,omitempty"`
	Text     string   `json:"text,omitempty"`
	Names    []string `json:"names,omitempty"`
	Sources  []Source `json:"sources,omitempty"`
}

// Who writes a revision.
const (
	ByAuthor     = "author"
	ByBookkeeper = "bookkeeper"
)

// Revision is one change of the setting document. The bookkeeper's cover
// the messages of a session from From through Through; one that failed
// has no ops and says why, and Redo names the failed one a retry redoes.
type Revision struct {
	Schema int       `json:"schema"`
	ID     string    `json:"id"`
	At     time.Time `json:"at"`
	By     string    `json:"by"`
	Ops    []Op      `json:"ops"`
	// Base is how many revisions the bookkeeper saw: ops on entries the
	// author changed after them are skipped, and listed here.
	Base      int    `json:"base,omitempty"`
	Skipped   []Op   `json:"skipped,omitempty"`
	Session   string `json:"session,omitempty"`
	From      string `json:"from,omitempty"`
	Through   string `json:"through,omitempty"`
	Redo      string `json:"redo,omitempty"`
	Failed    string `json:"failed,omitempty"`
	Model     string `json:"model,omitempty"`
	Effort    string `json:"effort,omitempty"`
	Reasoning string `json:"reasoning,omitempty"`
}

// NewID uses cryptographically secure randomness. Creation methods retry an ID
// collision; callers supplying their own ID receive an error on collision.
func NewID() string {
	var b [2]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Errorf("workspace ID entropy: %w", err))
	}
	return time.Now().Format("20060102-1504-") + hex.EncodeToString(b[:])
}

func Categories(genre string) []Category {
	c := []Category{{"人物", "设定库"}, {"关系", "设定库"}, {"地点", "设定库"}, {"组织", "设定库"}, {"物品", "设定库"}, {"规则", "设定库·世界观"}, {"情节", "大纲"}, {"主题", "作品说明"}, {"风格", "风格指南"}}
	switch genre {
	case "网文", "webnovel":
		c = append(c, Category{"力量体系", "设定库·世界观"})
	case "出版小说", "novel":
		c = append(c, Category{"叙事结构", "大纲"})
	case "剧本", "screenplay":
		c = append(c, Category{"场景", "大纲"})
	case "非虚构", "nonfiction":
		c = append(c, Category{"论点", "大纲"}, Category{"资料", "设定库"})
	}
	return append(c, Category{Other, "思路池"})
}

// AddCategory puts a new category in front of 其他, which stays last as the
// place everything without a better home ends up.
func AddCategory(cats []Category, add Category) []Category {
	at := slices.IndexFunc(cats, func(c Category) bool { return c.Name == Other })
	if at < 0 {
		at = len(cats)
	}
	return slices.Insert(cats, at, add)
}
