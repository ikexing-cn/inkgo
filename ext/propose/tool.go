package propose

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"writer/ext/ai/aiapi"
	"writer/workspace"
)

const proposeRules = `

改正文（propose_edit）：
- 作者要你改正文、写正文时，用 propose_edit 提议，不要把正文写在回复里。提议显示在那一章里，作者采纳了才写进正文。作者只是问看法、要思路时不要提议。
- 一次提议一处：chapter 写章名；at 写位置：replace 换掉 quote 那段原文，after 接在 quote 之后，end 接在章末，all 换掉全章。replace 和 after 的 quote 要从那一章逐字复制，在那一章里只出现一次；手头没有原文就先用 read_prose 读。
- text 是新写的正文，一段一行，不要空行、不要缩进、不要加标题和说明。
- 同一处再提议，新的会换掉作者还没定的旧的。作者采纳、放弃了，或者改动了原文，作者下次说话时，前面的【之前的提议】会告诉你。
- 提议之后，回复只用一两句说改了什么、为什么，不要把提议的正文再抄一遍。`

// TalkTool returns the proposal tool with its unchanged request metadata.
func TalkTool() aiapi.Tool {
	return aiapi.Tool{
		Name:        workspace.ProposeTool,
		Order:       70,
		Prompt:      aiapi.ToolPrompt{Order: 70, Section: proposeRules},
		Description: "在正文里提议一处修改，显示在那一章里，作者采纳了才写进正文。一次提议一处。参数按 chapter、at、quote、text 的次序写。",
		Schema:      fixedSchema(`{"type":"object","properties":{"chapter":{"type":"string","description":"改哪一章，章名和搜全书结果里写的一样"},"at":{"type":"string","enum":["replace","after","end","all"],"description":"replace 换掉 quote 那段原文；after 接在 quote 之后，在同一段里接着写，text 以换行开头就另起一段；end 接在章末；all 换掉全章"},"quote":{"type":"string","description":"at 是 replace 或 after 时，逐字复制那一章里的原文，在那一章里只出现一次"},"text":{"type":"string","description":"新写的正文，一段一行"}},"required":["chapter","at","text"]}`),
		Run:         runProposal,
		Label:       labelProposal,
	}
}
func fixedSchema(s string) func(workspace.Work) json.RawMessage {
	return func(workspace.Work) json.RawMessage { return json.RawMessage(s) }
}

type proposeArgs struct {
	Chapter string `json:"chapter"`
	At      string `json:"at"`
	Quote   string `json:"quote"`
	Text    string `json:"text"`
}

func runProposal(ctx context.Context, env aiapi.ToolEnv, raw json.RawMessage) aiapi.ToolResult {
	var in proposeArgs
	if err := json.Unmarshal(raw, &in); err != nil {
		return aiapi.ToolResult{Text: fmt.Sprintf("参数不是约定的 JSON：%v", err), Failed: true}
	}
	b, err := env.Shared.Book()
	if err != nil {
		return aiapi.ToolResult{Text: fmt.Sprintf("读不出章节：%v", err), Failed: true}
	}
	p, why := aiapi.PlaceProposal(env.Store, b, strings.TrimSpace(in.Chapter), in.At, in.Quote)
	if why != "" {
		return aiapi.ToolResult{Text: why, Failed: true}
	}
	p.Text = aiapi.CleanProposalText(in.Text, in.At == workspace.ProposeAfter)
	if strings.TrimSpace(p.Text) == "" {
		return aiapi.ToolResult{Text: "text 要写新的正文", Failed: true}
	}
	return aiapi.ToolResult{Text: "已在「" + p.Title + "」的正文里显示，等作者决定；作者采纳或放弃了，下次说话时会告诉你。", Proposal: &p}
}
func labelProposal(env aiapi.ToolEnv, raw json.RawMessage) string {
	var in proposeArgs
	_ = json.Unmarshal(raw, &in)
	return "提议修改「" + env.Shared.Named(strings.TrimSpace(in.Chapter)) + "」"
}

// proposalNotTaken is what the model hears when the talk that asked for a
// proposal stopped or the book changed before it could be shown.
const proposalNotTaken = "提议没有被接收：讨论已结束或已换书"
