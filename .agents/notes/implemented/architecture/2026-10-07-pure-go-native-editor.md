# Agent Note: 纯 Go 原生 UI，正文编辑器自己画

Status: implemented

## Problem

写作工具需要的不只是纯文本框：伏笔标记要跟着文字移动，人物/地名要在光标处弹补全，AI 预测要以灰字显示在光标后、按 Tab 才进正文，还要能做 diff 式审阅。mygo 自带的 `ui.TextArea` 只编辑纯文本，拿不到光标坐标，也画不了行内装饰。

编辑器已接入[正文页](../feature/2026-10-08-writing-page-and-chapters.md)。[AI 辅助写作](../../proposed/feature/2026-10-08-ai-assisted-writing.md)计划再改动下文两处：灰字改为默认按快捷键出现、v1 关闭补全；落地前下文仍是现状。

## Decision

- 全部代码是 Go，`CGO_ENABLED=0` 可编译；窗口与绘制用 `github.com/egoist/mygo` 的原生 UI（Windows 走 Direct3D 11，macOS 走 Metal，Linux 走 OpenGL），不用 WebView。
- 正文编辑器在 `editor/` 自己实现，不用 `ui.TextArea`：用 `HandleInput` 取按键、文字与输入法组字，`TextCaret` 告诉系统光标位置，`ui.Shape` 排字形后自己按宽度断行、自己画光标、选区、标记、灰字与补全弹层。
- 文档模型 `editor.Doc` 只存纯文本（rune 下标）加一组 `Mark`；标记在每次替换时按位置映射移动，在标记边上打字不扩大标记，标记内文字被删光时置 `Orphaned` 而不是静默删除。
- AI 预测只显示、不写入：`Predictor` 的结果作为灰字插在显示层，按 Tab 才作为一次可撤销的编辑写入，并打 `MarkAIAccepted` 标记留下来源。补全候选只来自传入的已确认实体，不调用模型。
- 标记有三种：伏笔（`MarkForeshadow`）、AI 采纳（`MarkAIAccepted`）、AI 起草（`MarkAIDraft`）。替换正好覆盖一个标记的文字时，标记跟着新文字走。
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
- **代价与已知上限**：中文断行、光标、选区、撤销、滚动、无障碍都要自己维护；目前还没有无障碍文本暴露，超长章节每次编辑会重新断全部行。mygo 处于 0.2.x、基本单人维护，API 变动或停更时需要重访这个决定。Linux 上回车/退格是否以按键事件到达、fcitx5/ibus 输入法兼容性尚未在真机验证。

## Verification

`go test ./...` 覆盖 `editor/doc_test.go`（标记映射、断行避头尾、补全匹配、撤销合并、全部替换一次撤销、`Prose` 不存缩进）、`editor/ui_test.go`（无窗口驱动真实界面）与 `cmd/poc/main_test.go`（编辑器占满窗口）。`GOOS=darwin|linux CGO_ENABLED=0 go build ./cmd/poc` 可通过。
