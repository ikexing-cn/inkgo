// Package workspace stores a work as canonical files: work.yaml, a JSONL
// file per session, and setting.jsonl, the revisions of the setting
// document. Nothing else is authoritative.
package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
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
	TokenBudget int        `yaml:"token_budget" json:"token_budget"`
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
}

// Places of an entry in the setting document.
const (
	PlaceBody    = "body"    // what the author decided
	PlaceOpen    = "open"    // waiting for the author: an AI's idea, a contradiction
	PlaceDropped = "dropped" // what the author does not want
)

// Source is where an entry comes from: words quoted verbatim from a message.
type Source struct {
	Session string `json:"session"`
	Message string `json:"message"`
	Quote   string `json:"quote"`
}

// Entry is one statement of the setting document.
type Entry struct {
	ID       string   `json:"id"`
	Place    string   `json:"place"`
	Category string   `json:"category"`
	Text     string   `json:"text"`
	Sources  []Source `json:"sources,omitempty"`
	// Author is set once the author wrote or edited the entry: the
	// bookkeeper changes it only on the author's word.
	Author bool `json:"author,omitempty"`
	// Rev is the number of the last revision that changed it, from 1.
	Rev int `json:"-"`
}

// Ops a revision applies.
const (
	OpAdd  = "add"  // a new entry: Place, Category, Text, Sources
	OpEdit = "edit" // Entry's Text, and Sources added to its own
	OpMove = "move" // Entry to Place, or Category, or both, and Sources added
)

type Op struct {
	Op       string   `json:"op"`
	Entry    string   `json:"entry"`
	Place    string   `json:"place,omitempty"`
	Category string   `json:"category,omitempty"`
	Text     string   `json:"text,omitempty"`
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
	c := []Category{{"人物", "设定库"}, {"关系", "设定库"}, {"地点", "设定库"}, {"组织", "设定库"}, {"物品", "设定库"}, {"规则", "设定库·世界观"}, {"情节", "大纲"}, {"主题", "作品说明"}, {"风格", "风格指南"}, {Other, "思路池"}}
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
	return c
}
