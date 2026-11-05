package aiapi

import (
	"fmt"
	"strings"
	"writer/talkbook"
	"writer/workspace"
)

// PlaceProposal validates and locates a proposed edit against stored prose.
func PlaceProposal(st *workspace.Store, b talkbook.Book, chapter, at, quote string) (workspace.Proposal, string) {
	if at != workspace.ProposeReplace && at != workspace.ProposeAfter && at != workspace.ProposeEnd && at != workspace.ProposeAll {
		return workspace.Proposal{}, "at 只能是 replace、after、end、all"
	}
	n, missing := b.Find(chapter)
	if n == nil {
		return workspace.Proposal{}, missing
	}
	name := b.Name[n.ID]
	if n.Folder {
		return workspace.Proposal{}, "「" + name + "」是一卷，没有正文；提议改它下面的章。"
	}
	p := workspace.Proposal{Chapter: n.ID, Title: name, At: at}
	if at != workspace.ProposeReplace && at != workspace.ProposeAfter {
		return p, ""
	}
	if strings.TrimSpace(quote) == "" {
		return p, at + " 要在 quote 里逐字复制「" + name + "」里的原文"
	}
	ch, err := st.ReadChapter(n.ID)
	if err != nil {
		return p, fmt.Sprintf("读不出「%s」：%v", name, err)
	}
	words, ok := Locate(ch.Text, strings.TrimSpace(quote))
	if !ok {
		return p, "「" + name + "」里找不到 quote 这段原文；要逐字复制，可以先用 read_prose 读一下。"
	}
	if k := strings.Count(ch.Text, words); k > 1 {
		return p, fmt.Sprintf("quote 这段原文在「%s」里出现了 %d 次；多引几个字，让它只出现一次。", name, k)
	}
	p.Quote = words
	return p, ""
}

// CleanProposalText normalizes generated prose while retaining an append boundary.
func CleanProposalText(s string, lead bool) string {
	var b strings.Builder
	started := false
	for _, line := range strings.Split(strings.ReplaceAll(s, "\r", ""), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if started {
			b.WriteByte('\n')
		}
		b.WriteString(line)
		started = true
	}
	out := b.String()
	if lead && strings.HasPrefix(strings.TrimLeft(s, " \t　\r"), "\n") && out != "" {
		out = "\n" + out
	}
	return out
}
