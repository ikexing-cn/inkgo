# Agent Note: 去掉章节摘要

Status: implemented

## Problem

[章节摘要与起草时的前情提要](../../archived/feature/2026-10-09-chapter-summaries.md)让 AI 在作者离开一章改过的正文时写一份摘要，起草时把前面各章的摘要当「前情提要」带上；讨论还有 `read_summary` 工具读它。

2026-10-16 作者说：「摘要和检查的形式我在思考，感觉现在很浪费token而且没办法直观看到用途。直接ai读原文不就行了」。

- **钱花在作者看不见的地方**：每离开一章改过的正文就调用一次模型，作者没点过什么；花了多少、摘要被谁用了，界面上都看不出。
- **用到它的地方只有两处**：起草时的前情提要，和讨论的 `read_summary`。起草要改成在讨论里写、AI 自己查书（[起草和改写交给讨论](../feature/2026-10-16-writing-through-the-talk.md)），讨论本来就能 `search_book`、`read_prose` 读原文。
- **当初的理由不成立了**：做摘要时，起草只放得下上一章结尾。现在内置模型的上下文都在一百万 token 左右（[上下文上限配在模型上](../architecture/2026-10-15-model-limits-and-usage.md)），上一章全文和本卷各章章纲一起带上也只要一两万 token。
- **同类软件没人这样做**：2026-10-16 查过的写作软件里，没有一家在作者背后自动写摘要；Sudowrite 起草时直接读前文原文，有长度上限。
- 摘要可能过时、可能只读了一半、作者改过又被正文甩在后面，这几种状态各要一套说明和测试，界面上又是一块折叠区。

## Decision

章节摘要整个没有了，五处一起去掉：

1. **离开一章不写摘要**：`desk/summary.go` 的队列、`leaveChapter`、`writing.Summarize`、`aiport.Pen.Summarize` 都删了。离开改过的一章，只有检查设成「写完一章后检查」时才发请求。
2. **标题下面只有章纲**：没有「本章摘要」折叠区，也不能手写摘要。
3. **起草不带前情提要**：`writing/writer.go` 里 `AIDraft` 的 `recap` 删了；起草对话框里没有「补齐摘要」和「起草时 AI 会读前面 N 章的摘要」。旧的请求记录里记着的 `recap` 部分（`workspace.PartRecap`）照旧显示成「前情提要」。
4. **讨论的工具只剩五个**：没有 `read_summary`。旧讨论里调用过它的那一步，照旧写「读了摘要「…」」。
5. **设置里没有「章节修改后更新摘要」**：`.writer/desk.json` 不再写 `no_auto_summary`。`desk.json` 本来就宽松地读，旧文件里留着这一项也照常打开。

连带的：

- **待处理**：摘要失败来的那几条随之没有（[待处理](../feature/2026-10-09-pending-list.md)）。
- **卷页每章下面的一行灰字**：原来有章纲写章纲、没有就写摘要（[走查修正](../feature/2026-10-09-walkthrough-fixes.md)的「卷页列出每章的要点」）。现在没有章纲时写正文的开头：`gistsIn` 取前 200 字，换行压成空格，最多显示两行；一个字也没有的不写。原型里先做了，作者 2026-10-16 同意。
- **AI 关着时的那份清单**（DESIGN.md「AI 可以整个关掉」）里删掉了「章节摘要」「摘要来的那几条」。
- **讨论压缩成摘要**（「以上 N 轮已整理成摘要」，[讨论长了压缩成摘要](../architecture/2026-10-15-compacting-a-long-talk.md)）和这个无关，留着。
- `workspace.ProseHash` 从 `summary.go` 挪到 `check.go`，检查结果还用它认正文改没改过。

### 已有的 `.summary.json`

- 不删：里面可能有作者亲手写的摘要，删了找不回来。
- 不再管：`workspace/chapters.go` 的 `companions()` 不认它，改名、移动、进回收站时它不跟着走，留在原处。`workspace/summary.go` 删了。
- 章节文件夹里多出的 `.summary.json` 不影响读章节：章节树只认 `.order` 和正文文件。

## Alternatives considered

- **留着摘要，只是不自动写，作者点了才写** — 最强的理由：几百章的长篇，前情提要用几千字就交代完前面的情节，比让 AI 每次去查便宜；作者也能拿它当自己的提纲。没有用：起草带上一章全文和本卷章纲、AI 需要时自己查，已经够接上前情；手写的摘要和章纲意思重叠，作者要提纲就写章纲；留着它就还要留折叠区、过时提示、`read_summary`，换来的只是一个作者不常点的按钮。
- **分层摘要（卷摘要、全书摘要）** — 几百章也放得下。没有用：要先有章摘要，问题一样；而且 AI 读到的是二手的概括，细节和原文对不上时作者更难察觉。
- **把已有的 `.summary.json` 删掉** — 章节文件夹干净，不留没人管的文件。没有用：删是不能撤回的，作者写的摘要会丢；留在那里不碍事。
- **让 `.summary.json` 照旧跟着章节改名、移动** — 改了名的章旁边不会剩下对不上名字的文件。没有用：跟着走就还要在 `companions()` 里认它、在回收站里恢复它，等于还在维护一个已经没有的功能。作者真想要回来，文件还在原处。

## Consequences

- 离开改过的一章不再在作者背后花钱。
- 几百章以后，起草只带上一章和本卷章纲，更早的伏笔要靠 AI 自己想到去查。查不查、查得准不准，现在没有评测；等起草在讨论里用过一阵，看实际写出来的东西。
- 作者的旧作品里改过名的章，旁边会剩下用旧名字的 `.summary.json`。软件没有发出去过，只在作者自己的测试作品里有。
- 提案里「改过一章再离开，AI 记录里没有新行」一条：落地时先做了这一篇，AI 记录（[AI 记录和完整请求](../feature/2026-10-16-ai-record-and-trace.md)）排在后面，所以现在由测试查「一次也没问模型」。先做这一篇，是因为它删掉一批调用模型的地方，AI 记录要挂的地方就少一些。
- 测试：
  - `desk/write_test.go` 的 `TestLeavingAChangedChapterAsksNothing`：检查设成手动时，改过一章再离开，模型一次也没被问到（有摘要时这里会排一份摘要）。`TestFolderShowsTheGists`：没有章纲的章，卷页显示正文的开头。
  - `workspace/chapters_test.go` 的 `TestAnOldSummaryStaysWhereItWas`：旁边有 `.summary.json` 的章改名、进回收站、再恢复，那个文件原样留在原处；作品重新打开，章节照常。
  - `desk/aiswitch_test.go` 的 `TestAIOffShowsNothingOfIt`：作品里有旧版本写的 `.summary.json`，关掉 AI 走一遍，文件原样还在。
  - `discussion` 的测试：讨论带五个工具；旧的 `read_summary` 那一步照旧有说明。
  - 删掉的：`workspace/summary_test.go`、`writing/summary_test.go`、`desk/summary_test.go`、`desk/shot_test.go` 的 `TestShotSummary`。
  - 原型在上一次提交里同样去掉了，`scripts/prototype/walk.ts` 里生成摘要的几步随之删掉。「AI turned off」仍查窗口里不能有「摘要」两个字：压缩的「整理成摘要」只在讨论里，AI 关着时本来就不显示。
- 取代[章节摘要与起草时的前情提要](../../archived/feature/2026-10-09-chapter-summaries.md)，那一篇已归档。
