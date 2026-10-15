# Agent Note: 输入框出来时光标放在原型放的地方

Status: implemented

取代 [章节摘要](2026-10-09-chapter-summaries.md) 里「新建时标题框留空、用灰色占位字」那条，和 [写作页与章节](2026-10-08-writing-page-and-chapters.md) 里「重命名时还不能把名字全选上」那条：两条的前提「mygo 的输入框没有全选的接口」不对。

## Problem

作者 2026-10-15 问起「这种基本操作都没有吗」，重新查了 mygo：`Element.SetTextSelection(start, end)` 和 `TextSelection()` 一直都有（`ui/textinput.go`，mygo 自己的查找条 `ui/editor/parts.go` 就用它全选查找词）。之前没查到就下了结论。

照原型的规则（`prototype/base.js` 的 `draw`：新出来的框光标放在末尾，带 `data-all` 的整个选上；`DESIGN.md`「就地输入」）逐个对了一遍出来时就有字的框，在测试里打一个字看落在哪：

- F2、右键「重命名」：光标在名字末尾，原型和 `DESIGN.md` 是全选。
- 新建章节、新建文件夹没有号可接时：框空着，灰色占位字「新章节」「新文件夹」；`DESIGN.md` 是框里有这几个字、全选。
- 点讨论的示例问题（讨论页的、右栏的）：字放进讨论框，光标在最前面，接着打的字插到句首。原型光标在末尾。讨论框早就在了，字是后放进去的，mygo 给焦点时不挪光标。
- 别的（作品重命名、改写要求、伏笔备注、每日目标、查找、设定表）出来时光标就在末尾，和原型一样。

## Decision

- 改名的框出来时把名字全选上：`writeView.titleAll`，`nameAgain`（F2、右键重命名）和没有号可接的新建（框里放「新章节」「新文件夹」）设上，框出来那一帧 `SetTextSelection(0, 字数)`，用一次就清掉。
- 示例问题放进讨论框以后，光标放到末尾：`focusTalk` 那里 `Focus().SetTextSelection(n, n)`。从正文摘句子到讨论（`focusTalk` 的另一个入口）也走这里，接着打的字在已有的字后面。
- 占位字留着：框被清空时还显示原来的名字，空着确定等于没改。

## Alternatives considered

- **新建时照旧留空加占位字，只改重命名**。最强的理由：效果一样（打字就是新名字，回车用默认名），少改一处，已有测试不用动。没有用：`DESIGN.md` 写的是框里有字、全选上，原型也是；空框和选中的字看起来不一样，作者会以为两处做的不是同一个东西。

## Consequences

**收益**

- 改名打字就整个换掉，和系统文件管理器、原型一样；点示例问题接着写，字在后面。
- `DESIGN.md`「就地输入」末尾那句「软件里重命名时还不能把名字全选上」删了。

**代价与已知上限**

- 选中要在框出来那一帧做，靠 `titleAll` 记一帧；以后别处要全选，同样要记一个一次性的标记。
- 只对了出来时就有字的框。以后新加，按原型的规则：默认末尾，原型带 `data-all` 的全选。

## Verification

- `TestF2NamesTheOpenChapter`：F2 后打「第二章：脚印」，标题就是它，不是接在原名后面。不改时失败。
- `TestANewChapterBetweenTwoTakesNoNumber`：没有号可接时框里是「新章节」。不改时失败。`TestNameANewChapter` 末尾：打「尾声」，标题就是「尾声」。
- `TestAStarterLeavesTheCaretAfterIt`：点「主角有个说不出口的秘密」再打「，她怕黑」，连在后面。不改时失败（字插到了句首）。
- `npm run verify-prototype`：「naming in the tree」本来就查 F2 全选；「a new talk on its page」加了一处，点示例问题后讨论框里是那句话、光标在末尾。
