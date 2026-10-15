# Agent Note: AI 基建：模型按窗口配上限，讨论用工具查全书、当场记设定，长了就压缩

Status: implemented

## Problem

路线走到第 2 步 M2（[完整产品提案](../../proposed/feature/2026-10-07-idea-pool-discussion.md)），原定做 embedding 检索。作者说长篇「绝对撑不住」，不用再测，先把 AI 相关的基建做好，并要求先调研现在的 agent harness 怎么做上下文管理、工具、权限和自带密钥（BYOK）。

当时的 AI 层（[模型层](2026-10-08-model-layer-on-goai.md)、[把会发送的部分拆出去](2026-10-14-ai-out-of-the-core.md)、[设定文档与后台记账](../feature/2026-10-08-setting-document-and-background-bookkeeper.md)）有这些问题：

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

## Decision

作者 2026-10-15 逐条定了做法，界面先一次性做进 `prototype/`，同日确认。代码分五步，每步单独提交、带测试，各有一篇笔记写细节：

- **a 模型层**：上下文上限、最大输出、压缩阈值配在模型上，内置一张模型表；作品预算去掉；回复带回用量和工具调用。见[上下文上限配在模型上](2026-10-15-model-limits-and-usage.md)。「记账模型」（`Work.Bookkeeping`）已随 c 删掉。
- **b 共用的上下文拼装器**：讨论、续写改写起草、检查、摘要用同一个拼装器按模型的窗口装，记下每次发了哪些部分。见[每次请求由同一个拼装器装](2026-10-15-one-budget-for-every-request.md)。讨论设定只发目录时，「AI 参考内容」那句「需要时 AI 会查全文」随 c 加上。
- **c 讨论用工具，去掉记账员**：讨论自己搜、读全书，当场记设定。见[讨论用工具查全书、当场记设定](../feature/2026-10-15-talk-looks-up-and-notes-the-setting.md)。
- **d 压缩**：讨论发全部还没压缩的对话，快装不下时把最早的几轮整理成摘要，原话留在日志里。见[讨论长了压缩成摘要](2026-10-15-compacting-a-long-talk.md)。
- **e AI 写的字要采纳，显示用量**：起草和改写一样先作为提议，点「采纳」才进正文；每条讨论回复写用了多少 token，作品记下每个模型的用量和估算花费，设置里能看。见[AI 写的字等作者采纳，作品记下用量](../feature/2026-10-15-ai-text-waits-and-usage-is-kept.md)。

### 先放着的

- **额外请求头和查询参数**：接中转网关（Azure 等）时要用，Codex 有这一项。值可以引用环境变量或存进钥匙串。等作者真要接网关再定要不要做。不要密钥的本地模型同样等到那时。
- **联网**：路线第 7 步的联网核实和联网搜索工具移出 M2，以后单独讨论。调研时想到的方向（默认关、每次讨论第一次联网时问、网页内容标成外来资料、不从网页内容里记设定）到时再定。
- **按意思相近来找**：如果搜全书按词搜不到、意思相近的又找不着，再在工具里加向量检索，不另开一条路。

## Alternatives considered

- **向量检索（原 M2 方案）**：最强的理由是能按意思找，不怕换说法；原提案为它预留了缓存目录和「向量」用途。没有用：要另配一个 embedding 服务商（原提案记着 DeepSeek、Anthropic 官方据了解不提供），纯 Go、不用 CGO 又难以带一个本地模型；索引要跟着正文修改更新；小说里要找的多是名字和事件，设定条目本来就有 `Names`，按词搜够准；三个 harness 都不用它。留一条后路：按词搜不够时把它加成一个工具。

各步自己的备选写在各步的笔记里。

## Consequences

- **收益**：
  - 请求的大小跟着模型走，不再有作品预算要手填；讨论能查全书、记得住压缩前的话。
  - 每次请求发了什么、用了多少、大约花了多少，作者都看得到。
  - AI 写的字一律要作者采纳才进正文。
- **代价和风险**：
  - **估算和实际不符**：拼装器仍按估算装，可能和服务商的实际计数差得多。现在回复下面有实际用量，可以拿来对比；差太多就调整 `Estimate` 或预留空间。连 DeepSeek 时两边都差：改写一次，记录里各部分估算合计 3592，服务商算的输入 1649，估多了一倍多（`Estimate` 按每个汉字 1.5 个 token 算）；讨论一轮查了四次，「AI 参考内容」写本次发送约 6.8K，五次请求的输入平均约 8K。不查书的讨论连着三轮，估算 5174、5866、6621，实际输入 3014、3326、3678，估多了约 0.7 倍。讨论那一边，一轮里装得下多少只扣查到的结果（`discussion/engine.go` 的 `looked`），模型前几步的话和调用参数没扣，发回推理的协议还要加上推理。2026-10-15 第二轮起讨论按服务商的计数定压不压、查到的还装不装得下；写作类请求和设定发整份还是目录仍按估算（见[AI 基建第二轮](2026-10-15-ai-second-pass.md)）。
  - **「AI 参考内容」里两个数说的不是一回事**：「本次发送约 N」是最后一次请求的估算，「实际用量」是这一轮所有请求的合计，查了几次就差几倍，作者看了会以为估算差得离谱。原型也是这样写的；要不要改说法待作者定。第二轮在原型里改成只写最后一次请求，注明这一轮共几次，等作者确认。
  - **改动面大**：五步改了 `model`、`prompt`、`discussion`、`writing`、`ai`、`desk`、`editor`、`workspace` 和十几篇笔记。分步提交，每一步单独跑通，`npm run verify` 全绿。
  - 五步都只用假模型和假服务测过；2026-10-15 又连真实服务走了一遍，见下。
- **测试**：见各步的笔记。

### 连真实服务走了一遍

2026-10-15 作者下班后，用操作电脑的工具点真实的软件：样书（`go run ./cmd/sample` 的「雨城修灯人」），讨论和写作都用 DeepSeek 的 deepseek-flash。配置是作者那份的副本，DeepSeek 写的是 `responses` 协议；为了试压缩，把 deepseek-flash 的上下文上限改成 14K、最大输出 4K。一共 19 次请求，约 $0.035。

- **通了的**：讨论搜全书、查设定、读正文，找出了样书第六章埋的矛盾和一处错字，记下一条设定；回复下面的 token 数、「AI 参考内容」、查阅的折叠行；起草先作为提议、放弃以后正文和磁盘不变、采纳时存快照打标记、一次撤销；改写的删除线和采纳；设置里「本作品用量」和 `usage.json` 对得上。
- **当场修了的**：
  - 采纳以后 Ctrl+Z 没反应，见[AI 写的字等作者采纳](../feature/2026-10-15-ai-text-waits-and-usage-is-kept.md)；
  - `note_setting` 拒绝的原因读起来自相矛盾，同一句话记两条出处，见[讨论用工具查全书、当场记设定](../feature/2026-10-15-talk-looks-up-and-notes-the-setting.md)；
  - 回复被截断的那次请求没记用量，见[模型层](2026-10-08-model-layer-on-goai.md)；
  - AI 记录里「本章前文 N 字」「上一章结尾 N 字」连换行一起数，和界面上的字数对不上（453 对 441），改成和界面一样用 `workspace.Count`，见[AI 辅助写作](../feature/2026-10-08-ai-assisted-writing.md)。
- **待作者定的**：「停止」和「采纳」在同一个位置；放弃改写以后键盘去哪里；「AI 参考内容」两个数的说法；讨论不知道有哪些章；估算的偏差（上面两条）。作者当天逐条定了，见[AI 基建第二轮](2026-10-15-ai-second-pass.md)。
- **压缩**：上下文 14K 时一轮只发约 6.8K，阈值调到 50% 后第 6 轮压了，见[讨论长了压缩成摘要](2026-10-15-compacting-a-long-talk.md)。压完的第一次回复被截断：最大输出调成了 4K，推理把它用完，提示只说「模型回复被截断」，作者看不出是输出上限太小；第二轮改成要模型的最大输出，用满了提示写出它、叫作者去设置里调大。
- **没试到的**：界面上「以上 N 轮已整理成摘要」那一行在看得见的地方以上，操作工具滚不动对话，没看到；`chat` 协议、Kimi、Anthropic。

## Sources

- DeepSeek Harness：[architecture](https://raw.githubusercontent.com/deepseek-ai/deepseek-harness/master/docs/architecture.md)、[compaction](https://raw.githubusercontent.com/deepseek-ai/deepseek-harness/master/docs/subsystems/compaction.md)、[approval](https://raw.githubusercontent.com/deepseek-ai/deepseek-harness/master/docs/subsystems/approval.md)、[credentials](https://raw.githubusercontent.com/deepseek-ai/deepseek-harness/master/docs/subsystems/credentials.md)、[tool-execution-pipeline](https://raw.githubusercontent.com/deepseek-ai/deepseek-harness/master/docs/tool-execution-pipeline.md)、[config-catalog](https://raw.githubusercontent.com/deepseek-ai/deepseek-harness/master/docs/config-catalog.md)
- pi：[compaction](https://raw.githubusercontent.com/earendil-works/pi/main/packages/coding-agent/docs/compaction.md)、[models](https://raw.githubusercontent.com/earendil-works/pi/main/packages/coding-agent/docs/models.md)、[extensions](https://raw.githubusercontent.com/earendil-works/pi/main/packages/coding-agent/docs/extensions.md)、[作者的设计说明](https://mariozechner.at/posts/2025-11-30-pi-coding-agent/)
- Codex：[仓库](https://github.com/openai/codex)（`codex-rs/core/src/compact.rs`、`codex-rs/prompts/templates/compact/`）、[审批与安全](https://learn.chatgpt.com/docs/agent-approvals-security)、[高级配置](https://learn.chatgpt.com/docs/config-file/config-advanced)、[认证](https://learn.chatgpt.com/docs/auth)
