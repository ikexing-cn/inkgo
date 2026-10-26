# Agent Note: 界面设计照 oil-ui 的做法

Status: implemented

## Problem

2026-10-24 照 [oil-ui](https://github.com/oil-oil/oil-ui) 改了一版原型，见[页边批注这一版界面](../feature/2026-10-24-margin-notes.md)。作者觉得这套做法不错，要装进项目。

原样装进来，有两处会自己动：

- SKILL.md 一加载就自动跑 `scripts/check_update.sh`：联网问 ui.oiloil.org 有没有新版，有就用 `npx` 从 GitHub 拉 oil-cli，覆盖 skill 目录里的文件。放在仓库里，就是仓库的文件被网上下载的代码改掉，提交前才看得到改了什么。
- 第一次用完，要跑 `scripts/recommend_once.py`，在回复末尾推荐付费的 oil-ui-pro。

## Decision

- 放在 `.agents/skills/oil-ui/`，和写笔记的 skill 在一处。`AGENTS.md`「界面」一节指过来：换界面的方向、做视觉评审照它的做法，和 `DESIGN.md` 说的不一样时以 `DESIGN.md` 为准。
- 来源：上游提交 `a66a177`（2026-10-23），版本 0.24.2，MIT，`LICENSE` 随附；字体带着它的 `OFL.txt`。
- 和上游不同的只有这些：
  - SKILL.md 去掉开头的 `allowed-tools`（四条，预先放行版本检查），去掉「开始前」一节（加载时自动跑的版本检查）和「完整版提示」一节。
  - 不带 `scripts/check_update.sh`、`check_update.py`、`recommend_once.py`，三份 README 和它们的配图、`tests/`、`.gitignore`。`assets/logo.png` 留着：对比页的生成器要把它嵌进页面，缺了就不生成。
  - 别的文件一字不改。
- 不自动更新。要升级就从上游拿一个新的提交，照上面重做一遍，在这里改提交号和版本。

## Alternatives considered

- **原样装，留着自动更新**。
  - 最强的理由：上游修了什么，下次用就有，不用人管。
  - 没有用：加载一个 skill 就联网、下载代码来跑、改仓库里的文件；项目里的东西要先看过再进来。
- **只装在本机的用户目录，不进仓库**。
  - 最强的理由：仓库不多 640K 第三方的东西，本机随时更新。
  - 没有用：作者要装进项目；Codex 这些别的 agent 也读 `.agents/`，做法跟着仓库走。
- **放在 `.claude/skills/oil-ui/`**。
  - 最强的理由：Claude Code 自己认得出，会列进 skill，可以直接调用。
  - 没有用：项目的 skill 约定放在 `.agents/skills/`，别的 agent 也找得到。代价写在下面。

## Consequences

- 换方向、做评审有一套写下来的做法（先看同类、几个方向给作者挑、独立评审只看截图、做减法）；每次审查多一份 640K 的第三方文档和脚本。
- 不会自己更新：上游改了，要有人去拿。
- Claude Code 不会把它列进 skill 列表，要靠 `AGENTS.md` 那一行指过去。
- `scripts/oiloil-image.mjs` 生图要登录 ui.oiloil.org 的服务；skill 自己写了只用任务授权过的服务，没授权就不用它。
