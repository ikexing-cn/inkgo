# Agent Note: AI 基建：模型按窗口配上限，讨论用工具查全书、当场记设定，长了就压缩

Status: proposed

## Problem

路线走到第 2 步 M2（[完整产品提案](../feature/2026-10-07-idea-pool-discussion.md)），原定做 embedding 检索。作者说长篇「绝对撑不住」，不用再测，先把 AI 相关的基建做好，并要求先调研现在的 agent harness 怎么做上下文管理、工具、权限和自带密钥（BYOK）。

现在的 AI 层（[模型层](../../implemented/architecture/2026-10-08-model-layer-on-goai.md)、[把会发送的部分拆出去](../../implemented/architecture/2026-10-14-ai-out-of-the-core.md)、[设定文档与后台记账](../../implemented/feature/2026-10-08-setting-document-and-background-bookkeeper.md)）有这些问题：

- **长书装不下**。讨论只能把设定和最近几轮对话塞进上下文，书里其他地方 AI 看不到。
- **预算和模型无关**。每部作品有一个 `token_budget`，默认 16000（`workspace/store.go`），作者手填。DeepSeek 的窗口远大于它，默认值白白浪费容量；作者换了窗口小的模型，又可能超出。
- **预算各算各的**。讨论（`discussion/context.go` 的 `Assemble`）、续写改写起草（`writing/writer.go`）、检查（`writing/check.go`）、摘要（`writing/summary.go`）四处各自扣预算、各自拼设定，`renderSetting` 和 `oneLine` 两个包里各有一份。讨论的报错还写着「提高作品预算」（`discussion/context.go`），不合 DESIGN.md「文案里的地方和界面一字不差」那条。
- **记账员重复花钱**。后台记账员每一批都要重新发一遍设定和那几轮对话；讨论模型其实已经带着它们。讨论里不算设定的东西（作者的偏好、「这条线先放着」）只活在原文里，原文一旦不再发送就丢了。
- **用量被扔掉**。GoAI 每次请求都返回输入、输出、推理、缓存命中的 token 数，`model` 包没有取。
- **请求还原不全**。讨论事件记了附带和省略的设定条目，但没记规则的版本和设定的修订号，还原不出当时发了什么。
- **起草直接写进正文**。续写按 Tab 才进，改写点「采纳」才进，起草存完快照就流式写进正文（`desk/ai.go` 的 `startDraft`），没有采纳这一步。

### 调研

2026-10-15 读了三个开源 harness 的文档和部分源码（链接见文末）：

- **DeepSeek Harness**：「模型看到的必须记在日志里」：只追加的会话日志是唯一的真相，每次请求都从日志加代码重建。压缩到窗口的 80% 触发，保留最近 16%，摘要替换一段历史，原文留在日志里。审批的结果只有四种（允许一次、拒绝、撤回、没人回答），除了允许一次一律不放行。配置里只写密钥所在的环境变量名，每次请求现取密钥，任何接口都不返回密钥本身。
- **pi**：默认只有四个工具，系统提示加工具定义不到 1000 token。会话存成树，离窗口上限不到 16k 时压缩，保留最近约 2 万 token，摘要格式固定（目标、约束、进展、关键决定、下一步、关键上下文）。立场是不在作者背后往上下文里塞东西。不内建权限，交给容器。
- **Codex**：沙箱档位 × 审批策略，`.git` 等目录在可写范围里也只读。改文件用 `apply_patch`：模型交结构化的改动，由程序解析后写入，不走命令行。压缩时写一份交接摘要，另外原样保留最近 2 万 token 以内的用户发言。项目里的配置不能定义服务商。

三家都没用向量检索找资料，都是给模型搜和读的工具，让它自己去取。我们已经在走的路和它们一致：服务商配置只写环境变量名，密钥在钥匙串（`model/keys.go`）；作品文件只引用服务商和模型的名字。

## Proposal

作者 2026-10-15 逐条定了下面的做法。代码分五步，每步单独提交，带测试，并把这篇对应的部分转进 `implemented/`。界面上的改动先一次性做进 `prototype/`，作者确认以后才写代码；原型同日确认，看原型时补定的几处写在各步里。

### a 模型层

2026-10-15 已落地，见[上下文上限配在模型上](../../implemented/architecture/2026-10-15-model-limits-and-usage.md)。「记账模型」（`Work.Bookkeeping`）已随 c 删掉。

### b 共用的上下文拼装器

2026-10-15 已落地，见[每次请求由同一个拼装器装](../../implemented/architecture/2026-10-15-one-budget-for-every-request.md)。讨论设定只发目录时，「AI 参考内容」那句「需要时 AI 会查全文」已随 c 加上。

### c 讨论用工具，去掉记账员

2026-10-15 已落地，见[讨论用工具查全书、当场记设定](../../implemented/feature/2026-10-15-talk-looks-up-and-notes-the-setting.md)。

### d 压缩

2026-10-15 已落地，见[讨论长了压缩成摘要](../../implemented/architecture/2026-10-15-compacting-a-long-talk.md)。

### e AI 写的字要采纳，显示用量

- 起草写好后和改写一样先作为提议显示，点「采纳」才进正文，采纳前存快照。写进 DESIGN.md：AI 写的字，要作者采纳才进正文，没有例外。
- 每条讨论回复显示这次用了多少 token；作品累计的用量和估算花费显示在「AI」配置的「本作品使用」下面，只能看，不能清零。token 数按 K、M、B 写（DESIGN.md 文案那一条），不用「万」。
- AI 关掉时，还没采纳的起草丢掉，和改写一样；原来是停在写到的地方。
- 「AI」配置去掉预算以后没有保存按钮，不再单独查讨论模型填没填全：发送时没配讨论模型，照旧提示「请先配置讨论模型」。

### 先放着的

- **额外请求头和查询参数**：接中转网关（Azure 等）时要用，Codex 有这一项。值可以引用环境变量或存进钥匙串。等作者真要接网关再定要不要做。不要密钥的本地模型同样等到那时。
- **联网**：路线第 7 步的联网核实和联网搜索工具移出 M2，以后单独讨论。调研时想到的方向（默认关、每次讨论第一次联网时问、网页内容标成外来资料、不从网页内容里记设定）到时再定。
- **按意思相近来找**：如果搜全书按词搜不到、意思相近的又找不着，再在工具里加向量检索，不另开一条路。

## Alternatives considered

- **向量检索（原 M2 方案）**：最强的理由是能按意思找，不怕换说法；原提案为它预留了缓存目录和「向量」用途。没有用：要另配一个 embedding 服务商（原提案记着 DeepSeek、Anthropic 官方据了解不提供），纯 Go、不用 CGO 又难以带一个本地模型；索引要跟着正文修改更新；小说里要找的多是名字和事件，设定条目本来就有 `Names`，按词搜够准；三个 harness 都不用它。留一条后路：按词搜不够时把它加成一个工具。
- **起草保留直接写入，在 DESIGN.md 注明例外**：最强的理由是作者明确点了「起草」，写进去的位置是空的或者接在后面，有快照可以撤销，少点一下。没有用：规则留了例外就说不清，「AI 写的字要作者采纳」才简单。

## Acceptance criteria

- e：起草写完后正文不变，点「采纳」才写入；回复显示用量，设置里有累计和估算花费。
- 每一步 `npm run verify` 全绿；界面改动先过原型，`npm run verify-prototype` 的步骤跟着改。

## Risks

- **估算和实际不符**：拼装器仍按估算装，可能和服务商的实际计数差得多。有了 a 取到的实际用量，可以拿来对比；差太多就调整 `Estimate` 或预留空间。
- **改动面大**：去掉记账员要改 `discussion`、`ai`、`desk`、`workspace` 和几篇笔记，提示词里的规则要搬家。分五步提交，每一步都能单独跑通。

## Sources

- DeepSeek Harness：[architecture](https://raw.githubusercontent.com/deepseek-ai/deepseek-harness/master/docs/architecture.md)、[compaction](https://raw.githubusercontent.com/deepseek-ai/deepseek-harness/master/docs/subsystems/compaction.md)、[approval](https://raw.githubusercontent.com/deepseek-ai/deepseek-harness/master/docs/subsystems/approval.md)、[credentials](https://raw.githubusercontent.com/deepseek-ai/deepseek-harness/master/docs/subsystems/credentials.md)、[tool-execution-pipeline](https://raw.githubusercontent.com/deepseek-ai/deepseek-harness/master/docs/tool-execution-pipeline.md)、[config-catalog](https://raw.githubusercontent.com/deepseek-ai/deepseek-harness/master/docs/config-catalog.md)
- pi：[compaction](https://raw.githubusercontent.com/earendil-works/pi/main/packages/coding-agent/docs/compaction.md)、[models](https://raw.githubusercontent.com/earendil-works/pi/main/packages/coding-agent/docs/models.md)、[extensions](https://raw.githubusercontent.com/earendil-works/pi/main/packages/coding-agent/docs/extensions.md)、[作者的设计说明](https://mariozechner.at/posts/2025-11-30-pi-coding-agent/)
- Codex：[仓库](https://github.com/openai/codex)（`codex-rs/core/src/compact.rs`、`codex-rs/prompts/templates/compact/`）、[审批与安全](https://learn.chatgpt.com/docs/agent-approvals-security)、[高级配置](https://learn.chatgpt.com/docs/config-file/config-advanced)、[认证](https://learn.chatgpt.com/docs/auth)
