# Agent Note: 预算估计按字符计，不再按 UTF-8 字节

Status: implemented

## Problem

[M1 讨论](../feature/2026-10-07-m1-discussion.md)把 UTF-8 字节数当作 token 的保守上限：`Estimate(s) = len(s) + 16`。一个汉字占 3 个字节，于是被算成 3 个 token，而 DeepSeek 等常见 tokenizer 对汉字大约只用 0.6 到 1 个，高估了三到五倍。

作者用默认预算 16000 聊了四轮，共约 5000 字。后台记账判定"超出整理预算"，一条设定都没整理出来。讨论上下文用的是同一个估计，同样过早截掉了设定和旧对话。

本篇取代 M1 讨论中"字节数是保守预算估计"一句；该篇关于"估计不是精确计数""不接入供应商 tokenizer"的决定继续有效。

## Decision

- `discussion.Estimate`（2026-10-15 起是 `prompt.Estimate`，见[每次请求由同一个拼装器装](2026-10-15-one-budget-for-every-request.md)）按字符计：
  - 每 3 个 ASCII 字节算 1 个 token，英文常见是约 4 个字符一个 token，留了余量；
  - 其他字符每个算 1.5 个 token，高于已知 tokenizer 对汉字的用量；
  - 每段另加 16 个 token 的开销。
- 界面和笔记里仍只称它为估计，不报告为精确的 token 数。
- 2026-11-04 起估算跟书的语言走：英文书的 ASCII 每 4 个字符算 1 个 token（OpenAI 给的经验值），中文书仍按 3 个；其他字符都按 1.5 个，英文书里夹的中文和中文规则也照这样留余量。见[多语言](2026-11-02-i18n-on-mygo.md)「书的语言和界面语言分开」。
- 默认作品预算 16000 不变。2026-10-15 起作品预算去掉，按模型的上下文上限装（见[上下文上限配在模型上](2026-10-15-model-limits-and-usage.md)）。

## Alternatives considered

- **保留字节估计，把默认预算提高到 48000** — 估计永远不会低估，任何字节级 tokenizer 都放得下，改动只在一个默认值上。但已有作品的预算不会跟着变；预算数字和模型窗口的对应关系也会被扭曲，作者按模型窗口填预算时仍会被高估三倍。
- **汉字按 1 个 token 计** — 更接近 DeepSeek 和 GPT 系列的实际用量，上下文能多装一半。但有的 tokenizer 对生僻字或部分模型会超过 1 个；预算估计低了，请求会被服务端以超长拒绝，比少装几条设定更糟。
- **接入各家 tokenizer** — 最精确。但 M1 讨论里不接入的理由仍成立：三种协议和自定义端点未必提供 tokenizer。

## Consequences

- **收益**：同样的预算，中文对话和设定能多装约一倍。默认预算下，后台记账一批能处理约一万字的对话。
- **代价**：对 ASCII 为主的内容，估计比字节计数小三倍，不再是对任意字节级 tokenizer 都成立的上限；遇到对英文特别不友好的 tokenizer，仍可能被服务端以超长拒绝，届时由作者调低预算。

## Verification

`go test ./discussion` 的 `TestEstimateCountsCharacters` 固定了 1000 个汉字和 1500 个 ASCII 字节的估计值；预算相关的上下文测试按新估计通过。
