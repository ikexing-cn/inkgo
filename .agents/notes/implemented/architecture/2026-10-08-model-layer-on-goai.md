# Agent Note: 模型层用 GoAI 流式调用，推理内容落盘，回复按 Markdown 渲染

Status: implemented

## Problem

M1 的模型客户端是自己写的三种协议的非流式请求。实际使用中暴露出三个问题：
- 看不到思考链：Chat Completions 只取 `content`，丢掉了 DeepSeek 的 `reasoning_content`；Anthropic 只取 `text` 块，丢掉了 `thinking`。
- 要等整段回复生成完才显示，高推理等级下要干等几十秒到几分钟。
- 回复是纯文本，模型输出的 Markdown 原样显示。

作者明确要求推理内容落盘，用于事后复盘；还要能选推理等级，参照 DeepSeek Harness 的 Default/Low/High/Max。后续的联网核实需要工具调用循环。作者不想在三种协议上自己补齐流式、推理字段和工具循环，要用现成的多提供商库。

第一次选用的 Fantasy 让程序体积从 13.6 MB 涨到 60.8 MB（Windows/amd64，去掉调试信息），而且它走 Responses 协议时会丢掉 DeepSeek 的推理内容，所以改用 GoAI。Fantasy 的落地只存在于提交 `bcbb8b2`，这里不再单独保留笔记，测量结果和否决理由都写在下文"被否方案"里。

本篇取代 [M1 实现笔记](../feature/2026-10-07-m1-discussion.md)中"API 不流式，单次请求最长两分钟"的决定；该篇的上下文、预算、记账与凭据规则继续有效。模型看到的思路形式沿用[短编号与讨论规则](../feature/2026-10-08-model-facing-refs-and-talk-rules.md)。

## Decision

### 模型层

- `model.Client` 通过 `github.com/zendev-sh/goai`（MIT 许可，核心只依赖标准库）的 `StreamText` 发请求，三种协议分别对应：
  - `chat` 用 `provider/compat`，`reasoning_effort` 等未知参数原样写进请求体；
  - `responses` 用 `provider/openai`，这个包默认就走 Responses API；
  - `anthropic` 用 `provider/anthropic`，它会自己补 `/v1/messages`，所以配置里 API 地址末尾的 `/v1` 要去掉。

  只有 `model` 包导入 GoAI。
- **Responses 推理补丁**：DeepSeek 的 Responses API 用 `response.reasoning_text.delta` 发送完整推理（OpenAI SDK 里的 `ResponseReasoningTextDeltaEvent`），GoAI v0.10.6 只认摘要事件。补丁已提交上游 [zendev-sh/goai#183](https://github.com/zendev-sh/goai/pull/183)；合并前，`go.mod` 用 `replace` 指向作者 fork 上的 `fix/responses-reasoning-text` 分支（`3b8f181`）。上游发版后去掉 `replace`。
- 所有请求都走流式。`Completer.Complete(ctx, req, onDelta)` 逐段回调推理和正文增量（不回调空增量），结束后返回 `Reply{Text, Reasoning}`。GoAI 把模型停下的原因放在 `step_finish` 块上，最后的 `finish` 块可能不带原因。所以：
  - 截断、内容过滤或错误都返回错误；
  - 没有出现过带原因的结束块，说明流是半途断开的，也返回错误；
  - 不把半截回复当成功。
- 请求出错、尚未开始回复时，GoAI 默认重试 2 次；测试里通过包内字段关掉重试。
- 模型目录（`ListModels`）仍用自己的 GET 请求，GoAI 不负责目录。
- 单次请求的总时长由 context 限制在 15 分钟（最高推理等级可能先思考好几分钟），HTTP 客户端不设固定超时；目录请求仍是 2 分钟。正文加推理超过 8 MiB 时中止。
- 请求允许的输出 token：讨论 32768、记账 32768、冲突检查 16384，推理也计入其中。上下文预算里给回复预留的 2048、4096、1024 不变。

### 凭据关卡

GoAI 在没传 key 时会读 `OPENAI_API_KEY`、`ANTHROPIC_API_KEY`，使用默认地址时还会读 `OPENAI_BASE_URL`。所以：
- 没有 key 的提供商也传一个占位 key，阻止它回退到环境变量；Anthropic 另外加上 `WithSkipEnvResolve`。
- `model.guard` 包在 HTTP 传输层，对每个请求做三件事：
  - 目标主机必须等于配置里 API 地址的主机；
  - 请求头只放行白名单：`Accept`、`Content-Type`、`User-Agent`、`anthropic-version`、`anthropic-beta`、`Idempotency-Key` 和 `X-Stainless-*`（GoAI 自己只设置其中几种，外加认证头）；
  - 认证头只填这个提供商自己的 key，没有 key 就不发；占位 key 在这里被剥掉。
- 重定向仍然拒绝；错误只按 HTTP 状态码给中文原因（从 `goai.APIError.StatusCode` 取），不显示响应体。

### 推理等级

| 等级 | `chat`（DeepSeek 等） | `responses` | `anthropic` |
|---|---|---|---|
| Default | 不发送，由服务商决定（DeepSeek 默认 high） | 不发送 | 不发送 |
| Low / High / Max | `reasoning_effort: low/high/max` | `reasoning.effort: low/high/max`，`summary: auto` | `output_config.effort`，加 `thinking: {type: adaptive, display: summarized}` |

- 讨论和整理分别设推理等级，存在 `work.yaml` 的 `discussion.effort`、`bookkeeping.effort`（空、`low`、`high`、`max`）。整理沿用讨论模型时，可以单独设一个更低的等级。
- 输入框下方的模型标签显示"模型 ID + 等级"，点开是原生菜单：`模型 ›` 按提供商列出目录里的模型，`推理等级 ›` 列四档并勾选当前项；选中后立即写回作品配置。设置页也提供同样的两个下拉框。

### 落盘与复盘

- 会话事件新增三个可省略字段：`model`（`提供商/模型`）、`effort`、`reasoning`。助手回复和记账事件都会记录；作者自己的发言不能带这三个字段。推理内容只用于回看，不回放给模型。
- schema 仍是 1：只新增了可省略字段，旧文件照常可读；新文件旧版程序读不了（严格解码会拒绝未知字段），但目前没有发布过的版本。
- `discussion.Engine` 的 `Talk`、`Book` 是 `Model{Completer, Name, Effort}`。`Progress(step, delta)` 先用一个空增量宣告步骤开始，再转发流式输出；步骤有 `reply`、`bookkeep`、`conflicts` 三种。

### 界面

- 一轮对话进行中，回复区按流式内容实时显示：推理阶段展开"正在思考"，正文开始后折叠，接着显示"正在把这一轮整理成思路卡片…"。工作线程写入带锁的 `live`，界面每帧读取。
- 已保存的回复上方有可折叠的"思考过程"，默认收起；底部时间后标出"模型 · 等级"。记账时的推理在"AI 看到了什么"面板里折叠显示。
- 回复用 goldmark（CommonMark，加表格和删除线扩展）解析成段落、标题、列表、引用、代码、分隔线、表格，再用 mygo 的 `RichText` 和行列布局画出来。链接只着色不跳转，原始 HTML 当文本显示。解析结果按文本缓存，上限 256 条。相邻的文字块怎样合成一个可选中的文本、流式时怎样冻结排版，见[回复跨段选中与流式排版](../../implemented/feature/2026-10-08-reply-selection-and-smooth-streaming.md)。

### 选依赖的约束

mygo 在初始化时调用 `bind`，用反射按名字查找方法。只要程序里有这种调用（标准库的 `text/template` 也会），Go 链接器就会保留所有可达类型的全部导出方法，方法级的死代码消除对整个程序失效。所以，类型和方法表庞大的依赖会被整体链接进来，包括生成式 SDK（官方 openai-go、anthropic-sdk-go）和云厂商 SDK。以后加依赖之前，先用"依赖 + mygo"编一个去掉调试信息的探针量体积，并用 `go build -ldflags=-dumpdep` 查 `<ReflectMethod>`。

## Alternatives considered

- **Fantasy（`charm.land/fantasy`，曾在 `bcbb8b2` 落地）** — 由 Charm 维护，统一的流式回调和 agent 循环最完整，能区分推理、正文、工具输入和工具结果。否决原因有三：
  - **体积**：去掉调试信息后程序 60.8 MB。Fantasy 的 Anthropic 包为了支持 Bedrock 和 Vertex，引入了 AWS SDK、Google Cloud、gRPC 和 OpenTelemetry，经由 `net/rpc` 和 `x/net/trace` 又带进了 `text/template`；它的 OpenAI 部分包着生成的 openai-go，在 mygo 的反射之下被整体保留。
  - **会读环境凭据**：它包的官方 SDK 默认会读取环境变量里的凭据、组织 ID 和自定义请求头，必须在传输层拦截。
  - **Responses 协议有缺陷**：只认推理摘要，而且只给 gpt-5、o 系列这些名字的模型发推理等级。DeepSeek 走 Responses 时既没有推理内容，等级设置也被静默丢弃。
- **在自己的客户端上补齐三种流式格式** — 依赖最少，每个字节可控。但三种 SSE 格式、各家推理字段，以及之后的工具调用循环，都要自己写、自己跟进协议变化。作者明确不想重复造轮子。
- **其他轻量库**：
  - [go-ai-sdk](https://pkg.go.dev/github.com/azrtydxb/go-ai-sdk)：零依赖，但 `reasoning_text` 只在内部的 Codex 传输层里处理，Anthropic 也没有思考配置。
  - [go-llm-sdk](https://pkg.go.dev/github.com/BackendStack21/go-llm-sdk)：零依赖、代码量小，但没有 Responses 的 `reasoning_text`，也没有 Anthropic 的 effort。
  - [pgEdge](https://github.com/pgEdge/pgedge-go-llm-lib)：零依赖，但完全不处理推理内容。
  
  这三个都要补的东西比 GoAI 多。
- **包着官方 SDK 的库**：[llm-go](https://github.com/XiaoConstantine/llm-go) 和 [any-llm-go](https://blog.mozilla.ai/run-openai-claude-mistral-llamafile-and-more-from-one-interface-now-in-go/) 功能齐全，但它们依赖官方生成式 SDK，加上 AWS 或 genai，会遇到和 Fantasy 一样的体积问题。
- **Genkit Go、Eino** — 前者提供商最全，后者编排能力最强。但都是整套框架，桌面写作工具只需要模型调用层。
- **推理内容只显示、不落盘** — 会话文件小，格式不用改。但作者要靠它事后复盘 AI 当时为什么这样回答。

## Consequences

- **收益**：
  - 思考链、正文和整理过程都实时可见，推理内容随回复保存，可以回看；
  - 推理等级可以按用途、按作品选择，三种协议都会真正发送；
  - 回复的列表、强调、引用和代码正常排版；
  - Windows/amd64、`CGO_ENABLED=0` 下，`cmd/poc` 去掉调试信息是 15.2 MB（改造前 13.6 MB，用 Fantasy 时 60.8 MB），带调试信息 21.8 MB，整个项目共 22 个模块；
  - 以后加工具调用时，可以直接用 GoAI 的工具循环；
  - SDK 从环境里读到的凭据不会发往配置以外的地址。
- **代价与已知上限**：
  - 在 #183 合并发版之前，依赖的是 fork 上的一个提交；上游如果改了实现方式，去掉 `replace` 时要重新验证 Responses 推理。
  - GoAI 还在 0.x 版本，API 可能变动；只有 `model` 包依赖它。
  - Max 在 OpenAI 的 Responses 或其他兼容服务上可能不被接受，此时 400 错误会提示"推理等级或参数不受支持"。
  - 会话文件会随推理内容明显变大。
  - 标签和菜单只列出目录里的模型，目录外的模型 ID 仍在设置页填写。
  - DeepSeek 两种协议已经用真实服务验证过（2026-10-08，`deepseek-flash`、High，推理分别流出 38 段和 45 段）；OpenAI 和 Anthropic 的真实服务还没有验证。

## Verification

- `model/client_test.go` 用三种协议的 SSE 假服务器验证：推理和正文分开流出、各协议的推理等级参数、HTTP 错误不泄露密钥、截断和半截流报错、取消与超时、拒绝重定向。`TestEnvironmentCredentialsNeverLeak` 设置环境变量里的 key、组织 ID 和自定义请求头后，断言它们都不会出现在请求里。
- `discussion/engine_test.go` 的 `TestTurnRecordsReasoningAndSteps` 验证推理内容、模型名和等级都落盘，步骤顺序为回复、整理。
- `workspace/workspace_test.go` 的 `TestModelOutputKeepsReasoningForReview` 验证新字段可以往返读写，作者发言不能带推理，未知等级被拒绝。
- `desk/app_test.go` 覆盖 Markdown 分块、流式显示和推理等级写回作品配置。
- `SHOT=<path> go test -run TestLayout ./cmd/poc` 额外输出 `-thought` 截图，展示展开的思考过程和 Markdown 回复。
- `GOOS=darwin|linux CGO_ENABLED=0 go build ./cmd/poc` 可以通过；`go list -deps ./cmd/poc` 里没有 `text/template`、`google.golang.org` 或 AWS 的包。
