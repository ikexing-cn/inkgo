// Package workspace stores a work as human-editable canonical files. SQLite is
// a disposable search index and is never the authority for persisted content.
package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

const Schema = 1
const Other = "其他"

type Status string

const (
	Unprocessed Status = "unprocessed"
	Kept        Status = "kept"
	Deferred    Status = "deferred"
	Rejected    Status = "rejected"
	Adopted     Status = "adopted"
	Superseded  Status = "superseded"
)

type Category struct {
	Name   string `yaml:"name" json:"name"`
	Target string `yaml:"target" json:"target"`
}
type Selection struct {
	Provider string `yaml:"provider" json:"provider"`
	Model    string `yaml:"model" json:"model"`
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
type Span struct {
	Start int `yaml:"start" json:"start"`
	End   int `yaml:"end" json:"end"`
}
type Origin struct {
	Session string `yaml:"session" json:"session"`
	Message string `yaml:"message" json:"message"`
	Span    Span   `yaml:"span" json:"span"`
}
type History struct {
	At     time.Time `yaml:"at" json:"at"`
	From   Status    `yaml:"from" json:"from"`
	To     Status    `yaml:"to" json:"to"`
	Reason string    `yaml:"reason,omitempty" json:"reason,omitempty"`
}
type Similarity struct {
	ID         string `yaml:"id" json:"id"`
	Difference string `yaml:"difference" json:"difference"`
}
type Idea struct {
	Schema     int          `yaml:"schema" json:"schema"`
	ID         string       `yaml:"id" json:"id"`
	Status     Status       `yaml:"status" json:"status"`
	Category   string       `yaml:"category" json:"category"`
	Source     string       `yaml:"source" json:"source"`
	Fact       bool         `yaml:"fact" json:"fact"`
	Verified   bool         `yaml:"verified" json:"verified"`
	FactSource string       `yaml:"fact_source" json:"fact_source"`
	Topic      string       `yaml:"topic" json:"topic"`
	Origin     Origin       `yaml:"origin" json:"origin"`
	Conflicts  []string     `yaml:"conflicts" json:"conflicts"`
	Similar    []Similarity `yaml:"similar" json:"similar"`
	Supersedes []string     `yaml:"supersedes" json:"supersedes"`
	History    []History    `yaml:"history" json:"history"`
	Created    time.Time    `yaml:"created" json:"created"`
	Updated    time.Time    `yaml:"updated" json:"updated"`
	Text       string       `yaml:"-" json:"text"`
}
type ContextItem struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Reason string `json:"reason"`
}
type Event struct {
	Schema  int           `json:"schema"`
	ID      string        `json:"id"`
	Kind    string        `json:"kind"`
	Role    string        `json:"role"`
	Text    string        `json:"text"`
	At      time.Time     `json:"at"`
	Context []ContextItem `json:"context"`
	Omitted []ContextItem `json:"omitted"`
	ReplyTo string        `json:"reply_to"`
	IdeaIDs []string      `json:"idea_ids"`
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
