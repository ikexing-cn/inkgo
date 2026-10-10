# Agent Note: 纯 Go 原生 UI，正文编辑器自己画

Status: implemented

## Problem

写作工具需要的不只是纯文本框：伏笔标记要跟着文字移动，人物/地名要在光标处弹补全，AI 预测要以灰字显示在光标后、按 Tab 才进正文，还要能做 diff 式审阅。mygo 自带的 `ui.TextArea` 只编辑纯文本，拿不到光标坐标，也画不了行内装饰。

编辑器已接入[正文页](../feature/2026-10-08-writing-page-and-chapters.md)。[AI 辅助写作](../feature/2026-10-08-ai-assisted-writing.md)改动了下文两处：灰字可以只在作者要时出现（正文页默认如此），正文页不开补全。

## Decision

- 全部代码是 Go，`CGO_ENABLED=0` 可编译；窗口与绘制用 `github.com/egoist/mygo` 的原生 UI（Windows 走 Direct3D 11，macOS 走 Metal，Linux 走 OpenGL），不用 WebView。
- 正文编辑器在 `editor/` 自己实现，不用 `ui.TextArea`：用 `HandleInput` 取按键、文字与输入法组字，`TextCaret` 告诉系统光标位置，`ui.Shape` 排字形后自己按宽度断行、自己画光标、选区、标记、灰字与补全弹层。
- 文档模型 `editor.Doc` 只存纯文本（rune 下标）加一组 `Mark`；标记在每次替换时按位置映射移动，在标记边上打字不扩大标记，标记内文字被删光时置 `Orphaned` 而不是静默删除。
- AI 预测只显示、不写入：`Predictor` 的结果作为灰字插在显示层，按 Tab 才作为一次可撤销的编辑写入，并打 `MarkAIAccepted` 标记留下来源（`Options.GhostNote` 给出标记的备注，正文页记模型名）。`Options.PredictOnDemand` 打开时，只在 Cmd+J 或 `RequestPrediction` 时预测，光标在哪儿都行；关着时停止输入、光标在段尾才预测。补全候选只来自传入的已确认实体，不调用模型；正文页不传实体，补全不出现。
- 改写建议和起草也在编辑器里：`Propose(a, b, text)` 把原文划掉、新文着色显示在原文后，文字一变就失效，`AcceptProposal` 一步替换并打 `MarkAIAccepted`；`BeginDraft` / `DraftAppend` / `EndDraft` 让 AI 流式写入，期间只读，结束时整段打 `MarkAIDraft`，连同被覆盖的原文算一步撤销。
- 标记有三种：伏笔（`MarkForeshadow`）、AI 采纳（`MarkAIAccepted`）、AI 起草（`MarkAIDraft`）。替换正好覆盖一个标记的文字时，标记跟着新文字走。AI 起草标记例外：作者在它里面改字，改动的部分脱离标记，前后剩下的各自留着。
- 正文页用到的能力：
  - `Options.Prose`：首行缩进两字只在显示层，粘贴时去掉行首空白，Tab 不插入"　　"；
  - `Placeholder`：空文档的提示；
  - `Menu`：调用方往右键菜单里追加项；
  - 查找替换：`Doc.Find`、`SetFound` 高亮、`Replace`、`ReplaceAll`。连续的替换在撤销记录里合成一步（`undoStep.joined`）。

## Alternatives considered

- **业务用 Go，正文编辑器单独一个 WebView 窗口内嵌 CodeMirror 6** — 装饰层、补全弹窗、行内灰字、并排 diff 都有成熟实现，几天就能做出来。但用户要求纯 Go、不走 Web；且三平台 WebView 行为不一致，mygo 近期提交还在修 WebView 崩溃。
- **直接用 `ui.TextArea`** — 撤销、输入法、长文本性能都现成。但它是纯文本，没有光标坐标和行内装饰，三个核心功能都做不了。
- **换 Wails、Tauri 或 Electron** — 生态比只有几周历史的 mygo 成熟得多。但它们都把界面交给 Web 技术，与纯 Go 的要求冲突。

## Consequences

- **收益**：单个 Go 二进制（去掉调试信息约 15 MB，见[模型层用 GoAI](2026-10-08-model-layer-on-goai.md)），交叉编译 Windows/macOS/Linux 不需要 C 工具链；编辑器每个行为都在自己代码里，可用 `ui.Tester` 无窗口测试（输入法组字、Tab 采纳与撤销、补全、点击定位）。
- **代价与已知上限**：中文断行、光标、选区、撤销、滚动、无障碍都要自己维护；目前还没有无障碍文本暴露，超长章节每次编辑会重新断全部行。mygo 处于 0.2.x、基本单人维护，API 变动或停更时需要重访这个决定。2026-10-10 从 v0.2.15 升到 v0.3.7 时就变了两处：元素从 `*ui.Element` 改成值 `ui.Element`；`*ui.Context` 只在画一帧的时候有效，帧外调用（输入回调里读写剪贴板、打开链接）不报错也不生效，所以编辑器另存一份 `c.Services()` 来读写剪贴板，`Invalidate` 仍可在任何时候调用。Linux 上回车/退格是否以按键事件到达、fcitx5/ibus 输入法兼容性尚未在真机验证。

## Verification

`go test ./...` 覆盖 `editor/doc_test.go`（标记映射、断行避头尾、补全匹配、撤销合并、全部替换一次撤销、`Prose` 不存缩进）、`editor/ui_test.go`（无窗口驱动真实界面）与 `cmd/poc/main_test.go`（编辑器占满窗口）。`GOOS=darwin|linux CGO_ENABLED=0 go build ./cmd/poc` 可通过。
