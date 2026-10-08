# Agent Note: 模型层改用 Fantasy 流式调用，推理内容落盘，回复按 Markdown 渲染

Status: implemented

## Problem

M1 的模型客户端是自己写的三种协议的非流式请求。实际使用中暴露出三个问题：
- 看不到思考链：Chat Completions 只取 `content`，丢掉了 DeepSeek 的 `reasoning_content`；Anthropic 只取 `text` 块，丢掉了 `thinking`。
- 要等整段回复生成完才显示，高推理等级下要干等几十秒到几分钟。
- 回复是纯文本，模型输出的 Markdown 原样显示。

作者明确要求推理内容落盘，用于事后复盘；还要能选推理等级，参照 DeepSeek Harness 的 Default/Low/High/Max。后续的联网核实需要工具调用循环。在三种协议上自己补齐流式、推理字段和工具循环，等于重写现成的多提供商 SDK。

本篇取代 [M1 实现笔记](../feature/2026-10-07-m1-discussion.md)中"API 不流式，单次请求最长两分钟"的决定；该篇的上下文、预算、记账与凭据规则继续有效。模型看到的思路形式沿用[短编号与讨论规则](../feature/2026-10-08-model-facing-refs-and-talk-rules.md)。

## Decision

### 模型层

- `model.Client` 通过 `charm.land/fantasy`（v0.45.2）发请求，三种协议分别对应：`chat` 用 `providers/openaicompat`；`responses` 用 `providers/openai`，并对所有模型 ID 强制走 Responses API；`anthropic` 用 `providers/anthropic`，Anthropic SDK 会自己补 `/v1`，所以配置里 API 地址末尾的 `/v1` 要去掉。只有 `model` 包导入 Fantasy。
- 所有请求都走流式。`Completer.Complete(ctx, req, onDelta)` 逐段回调推理和正文增量（不回调空增量），结束后返回 `Reply{Text, Reasoning}`。结束原因是截断、内容过滤或错误时，以及流在结束事件前断开时，都返回错误，不把半截回复当成功。
- 模型目录（`ListModels`）仍用自己的 GET 请求，Fantasy 不负责目录。
- 单次请求的总时长由 context 限制在 15 分钟（最高推理等级可能先思考好几分钟），HTTP 客户端不再设固定超时；目录请求仍是 2 分钟。正文加推理超过 8 MiB 时中止。
- 请求允许的输出 token：讨论 32768、记账 32768、冲突检查 16384，推理也计入其中。上下文预算里给回复预留的 2048、4096、1024 不变。

### 凭据关卡

OpenAI 和 Anthropic 的 SDK 默认会从环境变量读取凭据、组织 ID 和自定义请求头（`OPENAI_API_KEY`、`ANTHROPIC_AUTH_TOKEN`、`OPENAI_CUSTOM_HEADERS` 等）。Anthropic SDK 在没有 key 时还会去找 `ant` 的登录配置。`model.guard` 包在 HTTP 传输层，对每个请求做三件事：
- 目标主机必须等于配置里 API 地址的主机；
- 请求头只放行白名单：`Accept`、`Content-Type`、`User-Agent`、`anthropic-version`、`anthropic-beta`、`Idempotency-Key` 和 `X-Stainless-*`；
- 认证头只填这个提供商自己的 key（`Authorization: Bearer` 或 `x-api-key`），没有 key 就不发认证头。

Anthropic 无 key 时传一个占位 key，只为阻止 SDK 去找凭据，这个占位 key 会被关卡剥掉。重定向仍然拒绝；错误只按 HTTP 状态码给中文原因，不显示响应体。

### 推理等级

| 等级 | `chat`（DeepSeek 等） | `responses`（OpenAI） | `anthropic` |
|---|---|---|---|
| Default | 不发送，由服务商决定（DeepSeek 默认 high） | 不发送 | 不发送 |
| Low / High | `reasoning_effort: low/high` | `reasoning.effort: low/high`，`summary: auto` | `output_config.effort`，自适应思考，显示摘要 |
| Max | `reasoning_effort: max` | `xhigh` | `max` |

- 讨论和整理分别设推理等级，存在 `work.yaml` 的 `discussion.effort`、`bookkeeping.effort`（空、`low`、`high`、`max`）。整理沿用讨论模型时，可以单独设一个更低的等级。
- 输入框下方的模型标签显示"模型 ID + 等级"，点开是原生菜单：`模型 ›` 按提供商列出目录里的模型，`推理等级 ›` 列四档并勾选当前项；选中后立即写回作品配置。设置页也提供同样的两个下拉框。

### 落盘与复盘

- 会话事件新增三个可省略字段：`model`（`提供商/模型`）、`effort`、`reasoning`。助手回复和记账事件都会记录；作者自己的发言不能带这三个字段。推理内容只用于回看，不回放给模型。
- schema 仍是 1：只新增了可省略字段，旧文件照常可读；新文件旧版程序读不了（严格解码会拒绝未知字段），但目前没有发布过的版本。
- `discussion.Engine` 的 `Talk`、`Book` 是 `Model{Completer, Name, Effort}`。`Progress(step, delta)` 先用一个空增量宣告步骤开始，再转发流式输出；步骤有 `reply`、`bookkeep`、`conflicts` 三种。

### 界面

- 一轮对话进行中，回复区按流式内容实时显示：推理阶段展开"正在思考"，正文开始后折叠，接着显示"正在把这一轮整理成思路卡片…"。工作线程写入带锁的 `live`，界面每帧读取。
- 已保存的回复上方有可折叠的"思考过程"，默认收起；底部时间后标出"模型 · 等级"。记账时的推理在"AI 看到了什么"面板里折叠显示。
- 回复用 goldmark（CommonMark，加表格和删除线扩展）解析成段落、标题、列表、引用、代码、分隔线、表格，再用 mygo 的 `RichText` 和行列布局画出来。链接只着色不跳转，原始 HTML 当文本显示。解析结果按文本缓存，上限 256 条。

## Alternatives considered

- **在自己的客户端上补齐三种流式格式** — 二进制最小（去掉调试信息 13.6 MB），没有预览版依赖，每个字节都可控。但三种 SSE 格式、各家推理字段（`reasoning_content`、`reasoning`、Responses 的 summary、Anthropic 的 thinking 和 signature），以及之后的工具调用循环，都要自己写、自己跟进协议变化。作者明确不想重复造轮子。
- **Genkit Go** — 提供商最全，包括 DeepSeek 和 Kimi 插件，有 Google 维护。但它是整套框架，带 flow、开发者界面和遥测，桌面写作工具只需要其中的模型调用层。
- **Eino** — 图编排、多 agent、中断与恢复都很完整。但概念和依赖远超需要，我们没有编排场景。
- **OpenAI 兼容和 Responses 走 Fantasy，Anthropic 直接用官方 SDK 的根包** — 去掉调试信息后能小约 23 MB，因为 Fantasy 的 Anthropic 包为了支持 Bedrock 和 Vertex，引入了 AWS SDK、Google Cloud、gRPC 和 OpenTelemetry；官方 SDK 的根包不带这些。但这样 Anthropic 就脱离了 Fantasy 统一的流式和工具调用循环，联网核实等工具要维护两条路径。
- **推理内容只显示、不落盘** — 会话文件小，格式不用改。但作者要靠它事后复盘 AI 当时为什么这样回答。

## Consequences

- **收益**：
  - 思考链、正文和整理过程都实时可见，推理内容随回复保存，可以回看；
  - 推理等级可以按用途、按作品选择；
  - 回复的列表、强调、引用和代码正常排版；
  - 以后加工具调用时，可以直接用 Fantasy 的 agent 循环；
  - SDK 从环境里读到的凭据不会发往配置以外的地址。
- **代价与已知上限**：
  - Windows/amd64、`CGO_ENABLED=0` 下，`cmd/poc` 去掉调试信息从 13.6 MB 变成 60.8 MB（带调试信息从 19.7 MB 变成 87 MB），其中约 23 MB 来自 Fantasy 的 Anthropic 包引入的 Bedrock 和 Vertex 依赖。体积成为问题时，先看上游能否把 Bedrock 和 Vertex 拆成独立包，再考虑上面第四个备选方案。
  - Fantasy 还在预览阶段（v0.45.x），API 可能变动；版本已在 `go.mod` 锁定，只有 `model` 包依赖它。
  - Responses 协议下，Fantasy 只给它识别为推理模型的 ID（gpt-5、o 系列）发送推理等级，其他模型的设置会被静默忽略。
  - Max 在 DeepSeek 以外的兼容服务上可能被拒绝，此时 400 错误会提示"推理等级或参数不受支持"。
  - 会话文件会随推理内容明显变大。
  - 标签和菜单只列出目录里的模型，目录外的模型 ID 仍在设置页填写。
  - 三种协议的真实服务没有在合成测试中验证，需要实际使用时确认。

## Verification

- `model/client_test.go` 用三种协议的 SSE 假服务器验证：推理和正文分开流出、各协议的推理等级参数、HTTP 错误不泄露密钥、半截流报错、取消与超时、拒绝重定向。`TestEnvironmentCredentialsNeverLeak` 设置环境变量里的 key、组织 ID 和自定义请求头后，断言它们都不会出现在请求里。
- `discussion/engine_test.go` 的 `TestTurnRecordsReasoningAndSteps` 验证推理内容、模型名和等级都落盘，步骤顺序为回复、整理。
- `workspace/workspace_test.go` 的 `TestModelOutputKeepsReasoningForReview` 验证新字段可以往返读写，作者发言不能带推理，未知等级被拒绝。
- `desk/app_test.go` 覆盖 Markdown 分块、流式显示和推理等级写回作品配置。
- `SHOT=<path> go test -run TestLayout ./cmd/poc` 额外输出 `-thought` 截图，展示展开的思考过程和 Markdown 回复。
- `GOOS=darwin|linux CGO_ENABLED=0 go build ./cmd/poc` 可以通过。
