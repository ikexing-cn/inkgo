# AGENTS.md

写作软件：纯 Go（`CGO_ENABLED=0`），UI 基于 `github.com/egoist/mygo`。编辑器在 `editor/`，可执行入口在 `cmd/poc/`。

## 提交前

```sh
go vet ./...
go test ./...
npm run verify-notes
```

三条都绿才算完成。界面改动在 `cmd/poc` 的布局测试里渲染截图核对（`SHOT=<path> go test -run TestLayout ./cmd/poc`）。

## 界面设计以 Design artifact 为准

设计稿在 Design artifact <https://claude.ai/artifact/RAP7UK4pSfnGArETpVtPSj>，文件都在 `project/` 下：入口是 `Main.dc.html`（总览），另有 `Pending`（待处理）、`Settings`（设置）、`Consistency`（一致性检查）、`Titlebar`（标题栏）、`Foreshadow`（伏笔）、`Planning`（全书规划）、`Import`（导入稿件）、`Backup`（备份）、`Toast`（提示）各一份 `.dc.html`，`System`（设计规范）定字号、圆角、阴影、间距和常用组件，公共组件 `Sidebar`、`WindowControls`，画布排布在 `canvas.json`。它由原 Claude Design 项目 <https://claude.ai/design/p/b415efa2-67ae-4711-8873-eafbaebc192a> 转来，原项目只作存档，不再更新。

1. 新的界面功能都以这份设计为基准：布局、色值、字号、交互照它来；字号、圆角用 `desk/theme.go` 里和「设计规范」页同名的常量，不直接写数字；和现有代码冲突时先说出来，不要自己决定以哪边为准。
2. 设计稿里没有的界面功能，先和作者在这份 artifact 里把设计方向定下来，确认后再开始写代码。
3. 可以改这份 artifact：作者要求的修改，或作者确认后把设计稿和代码实现的偏差同步过去。改之前先读最新版本，改完告诉作者改了哪几页；不删页面，不动分享设置。

## 重要改动必须留笔记

1. 非平凡改动（改了行为、架构、跨文件契约、流程与工具链、测试策略、落盘/网络/配置格式）前，按 `.agents/skills/write-notes-like-deepseek/SKILL.md` 写或更新 `.agents/notes/` 里的笔记；机械性小改（样式、格式化、打标、不改行为的补丁）直接提交。
2. 写之前先检索 `.agents/notes/` 里的同主题旧笔记：有归属就地更新；新想法先放 `proposed/`，落地时随同代码改动转 `implemented/`；新方案彻底取代旧决策时，同批归档旧篇并在新篇标明取代关系。
3. 被放弃的方案先写它最强的理由，再解释为什么不用。
4. 代码和笔记同一次提交。

笔记校验脚本在 `scripts/`，由 Node 直接运行 TypeScript（Node ≥ 22.18），不装 tsx 或任何 npm 依赖。
