# AGENTS.md

写作软件：纯 Go（`CGO_ENABLED=0`），UI 基于 `github.com/egoist/mygo`。编辑器在 `editor/`，可执行入口在 `cmd/poc/`。

## 提交前

```sh
go vet ./...
go test ./...
npm run verify-notes
```

三条都绿才算完成。界面改动在 `cmd/poc` 的布局测试里渲染截图核对（`SHOT=<path> go test -run TestLayout ./cmd/poc`）。

## 重要改动必须留笔记

1. 非平凡改动（改了行为、架构、跨文件契约、流程与工具链、测试策略、落盘/网络/配置格式）前，按 `.agents/skills/write-notes-like-deepseek/SKILL.md` 写或更新 `.agents/notes/` 里的笔记；机械性小改（样式、格式化、打标、不改行为的补丁）直接提交。
2. 写之前先检索 `.agents/notes/` 里的同主题旧笔记：有归属就地更新；新想法先放 `proposed/`，落地时随同代码改动转 `implemented/`；新方案彻底取代旧决策时，同批归档旧篇并在新篇标明取代关系。
3. 被放弃的方案先写它最强的理由，再解释为什么不用。
4. 代码和笔记同一次提交。

笔记校验脚本在 `scripts/`，由 Node 直接运行 TypeScript（Node ≥ 22.18），不装 tsx 或任何 npm 依赖。
