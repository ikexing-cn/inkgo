# Agent Note: 大纲页：卡片、按章节和时间线

Status: implemented

## Problem

2026-10-19 作者转来一位受访的写网文的作者的意见：现在用作家助手，「无法结合大纲和正文做时间线的记录」，想要「方便的大纲整理，以及时间线排布」。

现在的 Inkgo 只有贴着节点存的章纲、卷纲（[写作页和章节](../../implemented/feature/2026-10-08-writing-page-and-chapters.md)）：

- **通览不了**：要看全书的结构得一章一章点开。
- **记不下故事的先后**：倒叙、插叙、多条线交替写的长篇，写的先后和故事里发生的先后不一样。章纲只按写的先后排，「这件事在故事里是什么时候」没有地方记。
- **没定写在哪的事没处放**：想到了一件事，还不知道写进哪一章，只能记在别处。

原型改过三版。第一、二版是一张表加一份事件列表，作者说「差强人意，还是不够方便」。之后让 Codex 调研了同类软件和别的行业的排期工具（Aeon Timeline、Plottr、Dabble、Novlr、Scrivener、甘特图、看板），报告在会话的草稿目录里，要点：

- 故事的先后和写的先后分开存，是 Aeon Timeline 的核心。
- 章节 × 情节线的格子（Dabble、Novlr、Plottr）一眼看得出哪条线断了几章。
- 没定的东西放进托盘（Novlr 的 Unassigned），先记下来再说。
- 时间可以只写相对的或干脆不写，左右的先后就是先后。

第三版照这个方向做，作者看过同意。之后又按作者的意见改了几处：

- 时间线从单向的列表改成泳道。
- 卡片停上去浮起来一点，点开在对话框里改。
- 章节一列和「新增情节线」滚动时不动。
- 「添加」做成和卡片一样宽的虚线框。
- 时间线上也能添加，「未写时间」点了能改。
- 搜不到时整页只留一句话。

## Decision

侧栏「正文」下面多一页「大纲」。界面规则在 DESIGN.md「大纲：写的先后和故事的先后」，样子在 `prototype/pages.js` 的「大纲」一节；这里只写数据和取舍。

### 数据

一部作品一份 `outline.json`，根对象为 `schema`（沿用 workspace 的版本 2）、`lines`、`cards`。两组数组的文件顺序就是显示顺序；空文件不存在时按空大纲读取。卡片和情节线是作品的正式数据，整部作品备份会包含它们。卡片：

- `id`
- `title`：发生了什么，必填。
- `when`：故事中的时间，作者写的一句话，不解析成日期。
- `about`：说明。
- `chapter`：写在哪一章，存章节节点的 ID（`.order` 里那个），空的是「未放进章节」。
- `line`：情节线的 ID，空的是不分情节线。

情节线：`id`、`name`，按显示的先后存。

卡片在文件里的先后就是故事的先后。

- 时间线按文件的先后排。
- 按章节那张表里，一格的卡片也按这个先后排。
- 把一张没放进章节的卡片放进一章（拖进格子，或在对话框里选章），它在故事先后里挪到那一章和前面各章的卡片后面。
- 新记的一张也排在那里；在时间线上某一列添加的，排在那一列后面。

章纲、卷纲照旧是节点旁边的 `.outline.txt`（2026-11-04 起和 `outline.json` 一起搬进 `ext/outline/`，章纲按节点 ID 存成 `<ID>.txt`，见[扩展模块](../architecture/2026-11-02-extension-modules.md)）。大纲页改的就是它们，和正文页标题下面的「章纲」是同一份。

### 章节变了

- **改名、挪位置**：卡片存的是节点 ID，跟着章走。
- **进回收站**：卡片不删，它那一章不在树里时，卡片显示在「未放进章节」，章恢复了就回去。这和设定的变化在所在章被删时不起作用是同一个做法（[设定随章节变化](../../implemented/feature/2026-10-16-setting-changes-from-a-chapter.md)）。待作者确认。
- **彻底删掉**：卡片的 `chapter` 清空。

### 不用 AI

大纲整个是作者手写的，AI 关着时照常显示。这一期不从正文里抽事件。讨论读卡片见[讨论读得到大纲](2026-10-19-talk-reads-the-outline.md)。

## Alternatives considered

- **卡片带一个真正的日期或日历** — 最强的理由：Aeon Timeline 能按日期算年龄、间隔，排序也不用作者管。没有用：网文的时间多是「二十六年前」「十月六日夜」「第二天」，硬要日期作者就得编一个；修仙、架空的历法也套不进公历。先后由作者拖着排，时间只是一句话。
- **一章一张卡片，章纲就是卡片** — 最强的理由：少一种东西，章纲和大纲是同一份。没有用：一章往往写几件事，分属几条线；倒叙的一章里有更早的事，一章一张就排不进时间线。章纲留着写这一章怎么写，卡片记发生了什么。
- **只做时间线，不做按章节的表** — 最强的理由：受访者说的是「时间线」。没有用：作者通览全书时想看的是每章写了哪几条线、哪条线断了几章，这是按章节那张表的事；时间线只回答故事里谁先谁后。两种看法是同一叠卡片，切换不丢东西。
- **事件列表（第一、二版）** — 最强的理由：一行一件事，最省地方，键盘就能排。没有用：作者看过两版，说不够方便、「单向流…不直观」；多条线同时推进时，列表看不出哪几件事是同一时候。
- **在格子里直接打字改卡片** — 最强的理由：少开一次对话框。没有用：作者试了原型，说不方便，要停上去有反馈、点开在对话框里改；对话框里也放得下章、情节线、时间、说明几项。

## Consequences

- 新卡片和情节线在 Store 锁内避开大纲中两种对象的所有已有 ID；时间戳和随机部分的格式不变，碰撞时重新生成。`workspace.TestOutlineIDsRetryCollisions` 固定重复候选，覆盖卡片与情节线之间及同类对象之间的碰撞。
- 写入持有 Store 锁，重新读取并严格解码旧文件，再用已有 atomicWrite 写临时文件、同步并重命名。坏 JSON、未知字段、重复 ID、空标题、未知情节线返回带 outline.json 路径的错误，已有坏文件不会被界面操作覆盖。workspace.Open 不校验辅助大纲；损坏时作品和正文照常打开，正文隐藏「故事中」。大纲页整页换成和「没有找到」一样的一句话：「大纲文件读不出来」、出错的行（`workspace.PlanError.Line`，JSON 本身断在哪一行；断在结尾写「文件在结尾处断了」）和「打开所在文件夹」；没有「重试」，大纲页开着时和正文一样每 `book.CheckEvery`（2 秒）重读一次，改好了自己回来。作者 2026-10-19 看了带「重试」的那版，说太丑。
- SaveCard、PlaceCard、DropCard、SaveTime、SaveLine、MoveLine、DeleteLine 共同维持原型 placeAt/placeCard/dropAt/saveTime 的规则。仅相邻且非空的同一时间合为一列；改列时间只改该列全部卡片。删除情节线保留卡片和章节，只清空 line。
- 挪章、改名不改卡片；进回收站仅由界面把失效的 chapter 显示为未放进章节，恢复后归位；PurgeChapter 在大纲可读取时清空该节点及其所有后代章节的卡片 chapter，大纲损坏时跳过这一步，仍清理回收站和章节快照，不覆盖坏文件。修好大纲后，找不到章节的卡片显示在未放进章节。待作者确认。
- 大纲页通过 mygo Drag/DragOver/Drop 拖动卡片；表头独立于纵向滚动区，左列补偿横向滚动。ScrollIntoView 展示新增和从正文跳转的卡片，边框强调约 1.6 秒。右栏默认收起，仅显示设定。
- 章纲和卷纲继续写节点旁的 .outline.txt；workspace.Outlines 一次读取全部 live 节点的纲，只打开这些纲文件，不读正文和标记。章时间使用该章卡片的故事顺序；卷时间照原型把卷内各章的卡片按章节顺序连接后取两端。UI 尺寸放在 desk/kit/theme.go。
- 两本示例书各有 3 条情节线、22 张卡片，含回溯、3 张未归章和 2 张无时间。
- workspace 的 TestOutlineCardOrderAndStorage、TestOutlineTimeColumnsAndDrops、TestOutlineChapterLifecycle、TestOutlineMalformedFile、TestOutlinesReadWithoutProse、TestPurgeChapterWithMalformedOutline、TestOutlineErrorSaysWhere 检查持久化、排序、生命周期、坏文件不挡正文和回收站且不被覆盖、只读纲；desk 的 TestTheOutlineByChapter、TestTheOutlineTimeline、TestOutlinePlotLines、TestOutlineSearch、TestTheChapterInTheStory、TestOutlineStaysAtTheEdges、TestOutlineTrashAndSettingsPane、TestOutlineReadError、TestOutlineVirtualScrolling、TestOutlineVirtualDrag 通过真实 UI 输入检查动作、错误页标题栏、虚拟区域搜索及跳转、新卡片显露、占位高度、表头与泳道对齐、跨未构建区域拖动；cmd/inkgo 的 TestLayoutOutline 保存两种视图、卡片对话框、搜索空页和错误页截图；cmd/sample 的 TestBooksAreWritten 检查示例数据。
- 每帧开始一次建立章到卡片、回溯、情节线计数和时间列索引，cardsOf 和 planBack 不重复遍历全书。按章节仅构建视口上下各一屏内的行，其余用相同高度的占位；时间线仅构建视口左右各一屏内的时间列，表头和泳道共用列范围。行高和泳道高度由全部卡片的排版尺寸计算并缓存，避免离屏卡片改变滚动高度；修改卡片、纲或搜索时刷新。跳转先按几何位置滚动再构建目标，随后 ScrollIntoView 精确定位；拖动时额外保留源行或源列。
- `desk/outline/plan_perf_test.go` 的 TestOutlineFrameCost 只画大纲页，用 300 章/600 张卡片和 1000 章/2000 张卡片两档，各 4 条线；每种看法预热后画 10 帧取中位数，上限 25 ms。ui.Tester 检查软件渲染和输入，不能证明真实窗口的 GPU 呈现及操作系统调度下的连续拖动流畅度。文件被作者手动重排时，故事先后跟随文件，不猜日期也不修正作者的顺序。
