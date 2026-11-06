# Agent Note: 上下文上限配在模型上，作品预算去掉，回复带回用量和工具调用

Status: implemented

## Problem

这是 [AI 基建](2026-10-15-ai-infrastructure.md)五步里的第一步 a：模型层。

- 每部作品有一个 `token_budget`，默认 16000，和用哪个模型无关。DeepSeek 的窗口是它的几十倍，默认值浪费容量；换成窗口小的模型，又可能超出。
- 讨论、续写改写起草、检查、摘要各从这个预算里扣掉一个写死的回复预留（2048、4096、1024），预留和请求实际允许的输出（讨论 32768）对不上。
- GoAI 每次都返回用量，`model` 包没有取。
- 后面几步要让讨论用工具，`model` 还不能发工具定义、读工具调用。

## Decision

### 内置模型表

模型上限和价格来自内置的 `model/models.json`，后台从 models.dev 更新本机缓存；具体格式、查找顺序与更新约束见[models.dev 后台拉取](2026-11-06-model-table-from-models-dev.md)。此决定取代本篇原来「随发布更新、不联网拉」的维护方式。本篇的模型配置、请求预算和用量约定继续成立。

`Builtin(id)` 同步读内存目录；找不到时用上下文 32000、最大输出 4096，压缩阈值从 80 开始，见[讨论长了压缩成摘要](2026-10-15-compacting-a-long-talk.md)。`model.Cost` 估算花费，记录方式见[AI 写的字等作者采纳，作品记下用量](../feature/2026-10-15-ai-text-waits-and-usage-is-kept.md)。

### 数值配在模型上

- `model.Provider.Models` 是 `[]model.Model{ID, Window, Output, CompactAt}`，存在 `providers.yaml` 里，键名 `id`、`context`、`max_output`、`compact_at`。
- 上下文上限和最大输出用 `writer/tokens` 包读写：读的时候认 `128K`、`1M`、`1.5m`、`8 k`（K 是一千）；写的时候整千写 K、整百万写 M，凑不成的写数字。
- 一个模型可以只写 ID，也可以缺几项，缺的从内置表补。作者现有的 `providers.yaml`（模型是字符串列表）因此照样能读。提案原说「不做兼容」，落地时发现补齐缺项本来就要做，只写 ID 是它的一种情况，不多花代码。
- 校验（`validateModels`），不合规的不存，按这个顺序报第一处：ID 不能为空、不能有前后空格；ID 不能重复；上下文上限和最大输出须为正整数；最大输出须小于上下文上限；压缩阈值在 50 到 95 之间。写了别的键报「模型只能写 id、context、max_output、compact_at 几项」。`Load` 原样给出这几句，不换成通用的 YAML 报错。
- 默认服务商：DeepSeek 是 `deepseek-flash`、`deepseek-v4-pro`；Kimi 是 `kimi-k3`（k2.5 已不在 Kimi 的模型列表里）；OpenAI、Anthropic 不预填模型。

### 请求按模型的上限装

- `inference.Model` 带着 `model.Model`。它的 `Complete` 把请求的 `MaxTokens` 压到模型的最大输出以内（没填的就填最大输出）。讨论、续写、起草、检查、摘要原来各自的输出数照旧，只是不能超过模型的。
- 可以发送的输入是 `Model.Input()`，即上下文上限减最大输出。四处原来「预算减预留」的地方都换成它，预留的常数删掉。四处共用一个拼装器是第 b 步。
- 超出时的报错和悬停说「…超出模型的上下文上限…请在设置中「AI」配置里调大上下文上限」；省略原因写「超出上下文上限」。

### 去掉作品预算

`Work.TokenBudget` 删掉，校验和「设置」里那一项也删掉。`work.yaml` 里旧的 `token_budget` 读到就丢掉：`reloadWork` 用一个内嵌 `Work` 的结构多收这一个键，不让严格解码报错。

### 用量和工具调用

- `model.Reply` 带 `Usage{Input, CacheRead, CacheWrite, Output, Reasoning}`，取流里最后一个非零的用量。GoAI 的输入数不含缓存命中和写入，`Usage.Input` 把三者加回去，是这次请求实际的输入；推理算在 `Output` 里，`Reasoning` 单列。
- 讨论回复的事件记 `workspace.Usage`，字段和 `model.Usage` 一样。
- `model.Request.Tools` 是工具定义（名字、说明、参数的 JSON Schema）。消息可以带 `Calls`（assistant）或 `CallID`（role `tool`，工具结果）。回复的 `Calls` 是模型这一步要调的工具，参数为空时补成 `{}`，不是 JSON 时报错。只发一步，不交给 GoAI 执行，多步循环由调用方掌握（第 c 步）。有工具调用时允许回复正文为空。
- 不支持工具的模型由服务商报错，原样告诉作者。

### 设置里的样子

照原型（`prototype/` 2026-10-15 已确认）：

- 服务商表单的「可用模型」是一张表，每行模型 ID、上下文上限、最大输出、压缩阈值 % 和移除按钮；下面「添加模型」（按保守值加一行）、「获取可用模型」（拉回来的模型按内置表填数）。行里填了表里有的 ID，三个数随之换成表里的。按上面的规则随填随查（2026-10-16 起；原来是按了保存才查、弹提示条）：哪一行不合规，表下面红字写出第一处，「保存提供商」「获取可用模型」等它改好；保存成功把各行写回规范写法。报错里模型 ID 和后面的中文之间空一格（"deepseek-flash 的压缩阈值须在 50% 到 95% 之间"）。
- 「本作品使用」三行（讨论、写作、检查用哪个模型）不再有保存按钮：选全了、和作品里存的不同、光标离开了模型输入框，就存下来；没选全时不提示，等作者选完。换服务商时模型填成它的第一个。

## Alternatives considered

- **作品预算保留，改成花费上限**：最强的理由是作者能按作品控制花费。没有用：用自己密钥的人懂这些配置，限用量就在模型上把上下文上限调小，同一个数没必要两处配。
- **联网拉模型目录（models.dev 或服务商接口）**：最强的理由是目录不会过时，新模型马上能用。当时因网络依赖、服务商接口缺少窗口与价格、测试成本而否掉；此维护决定由[models.dev 后台拉取](2026-11-06-model-table-from-models-dev.md)取代，保留内置 JSON 作为离线兜底。
- **配置严格按新格式，旧的字符串列表报错**（提案原来的说法）：最强的理由是只有一种写法，读的代码最简单。没有用：缺项从表里补本来就要写，只写 ID 是它的特例；作者手上已有配置，报错要他手改一遍文件。
- **「本作品使用」保留保存按钮**：最强的理由是作者明确知道什么时候存了。没有用：去掉预算以后只剩三个选择，和写作开关「勾上就存」一致；按钮还要单独查讨论模型填没填全，而发送时本来就会提示。

## Consequences

- **收益**：默认的 DeepSeek 一次能发近 62 万 token 的输入，原来是 16000 减预留；换了小窗口的模型，请求自动跟着缩，不会被服务端以超长拒绝。用量有了，第 e 步能显示、第 b 步能拿实际数和估算对比。工具调用备好了，第 c 步只写循环和工具。
- **代价**：内置表会过期，新模型和改价要发版；表外的模型按 32000 起步，作者要自己填。价格写在表里但这一步还没用上。窗口大以后一次请求可能很贵，第 e 步显示用量之前作者看不到（第 e 步已显示，见[AI 写的字等作者采纳，作品记下用量](../feature/2026-10-15-ai-text-waits-and-usage-is-kept.md)）。
- **还没做**：chat 协议的工具轮要把推理内容（DeepSeek 的 `reasoning_content`）发回去，GoAI 的 compat 层不带这个字段，第 c 步处理。
- **测试**：`go test ./model` 的 `TestBuiltinLimits`、`TestModelYAML`（写法、只写 ID、缺项补齐）、`TestModelValidation`（每条规则和 `Load` 的报错）、`TestToolCallsAndUsage`（三种协议各发一次带工具的请求，核对发出的工具定义、读回的调用和用量，以及坏请求被拒）；`go test ./workspace` 的 `TestTheOldBudgetIsIgnored`；`go test ./discussion` 核对 `MaxTokens` 被压到最大输出、回复事件带用量；`go test ./desk` 的 `TestModelLimitsInTheServiceForm`（每条校验、K/M 写回、移除和添加、填已知 ID 换数）和 `TestIncompleteFormsExplainInChinese`（没选全不存、选全才存）；`cmd/inkgo` 的布局测试多截一张服务商表单。

## Sources

- DeepSeek：[价格](https://api-docs.deepseek.com/quick_start/pricing/)
- Kimi：[价格](https://platform.kimi.ai/docs/pricing/chat)
- Claude：[模型](https://platform.claude.com/docs/en/about-claude/models/overview)、[价格](https://platform.claude.com/docs/en/about-claude/pricing)
- OpenAI：[模型](https://developers.openai.com/api/docs/models)、[价格](https://developers.openai.com/api/docs/pricing)
