# Agent Note: 用 Agent Notes 约束非平凡改动，校验脚本由 Node 原生运行

Status: implemented

技能的本地适配与记录门槛见[阅读入口、页面权限和服务表单状态](../architecture/2026-10-27-reading-entry-and-owned-state.md)。

## Problem

本项目主要由 AI 编写。每个新会话只看得见代码，看不见当初的取舍（例如为什么不用 WebView、为什么自己画编辑器），容易把否掉的路重走一遍，或为局部需求打穿边界。

## Decision

- 采用 [write-notes-like-deepseek](https://github.com/czm15053/write-notes-like-deepseek)（commit `8f3b404`）的笔记方法：非平凡改动在 `.agents/notes/{proposed,implemented,rejected,archived}/{class}/yyyy-mm-dd-topic.md` 留笔记，规则入口是根目录 `AGENTS.md`。
- Skill 的本地适配副本在 `.agents/skills/write-notes-like-deepseek/`（`SKILL.md`、`references/`、`templates/`），校验脚本副本在 `scripts/`；只收录校验与归档脚本，上游的看板、DSH 导入、素材导出脚本不收录，技能只列实际安装的入口。
- 看笔记进展用本地写的 `npm run notes-board`（`scripts/notes-board.ts` + `notes-board.html`），不用上游看板：只读、只绑 127.0.0.1，每次笔记文件或 HEAD 变了才重读磁盘和一遍 `git log -M`（改名跟着走，提案挪到 implemented 也接得上提交记录），页面每 2 秒问一次。它只看 `--root` 指的那份检出，别的 worktree 里没合进来的笔记看不到。不接进 `npm run verify`。
- 脚本由 Node 直接运行 `.ts`（Node ≥ 22.18 的类型剥离），`package.json` 只有 scripts，没有任何依赖；`npm run verify-notes` 串跑目录、格式、归档封印三道检查。
- 提交前同时通过 `go vet ./...`、`go test ./...`、`npm run verify-notes`。2026-10-13 起由 `npm run verify` 一条命令同时跑，原型的走查也在里面，见[全套检查从四分半到半分钟](2026-10-13-fast-checks.md)。

## Alternatives considered

- **`npx skills add` / `npx tsx` 按上游原样安装** — 最省事，和上游文档一字不差。但每次校验都要联网拉 tsx，结果随 npm 上的版本漂移；Node 24 已能直接跑这些脚本，tsx 只是多一层供应链。
- **把校验脚本改写成 Go** — 产品是纯 Go，工具链统一后不必装 Node。但要重写并维护约 600 行与上游同步的逻辑；Node 只是开发机工具，不进产品二进制。等确实要去掉 Node 时再重访。
- **只在 `AGENTS.md` 写规矩，不要脚本** — 零工具链。但散文约定对 agent 约束力弱，格式和目录会慢慢走样；机械门禁是这套方法的核心。

## Consequences

- **收益**：决策与被否方案留在仓库里，和代码同一次提交；格式、目录、归档封印由脚本非零退出码兜底。
- **代价与已知上限**：开发机需要 Node ≥ 22.18；上游更新不会自动同步，需要手动比对 commit 后再复制。上游仓库没有 LICENSE 文件（README 标注 MIT），若要公开分发这些副本，先确认授权。
