# Agent Note: 正文里标出设定的名字

Status: implemented

## Problem

2026-10-19 受访的作者想在正文里点一个人名、地名，就看到这条设定。现在要看一条设定，得去右栏的列表里找，或者搜。

名字已经有了：设定的每一条带名字表，正文打字时用来补全（[名字补全](../../implemented/feature/2026-10-09-name-completion.md)）。

## Decision

- 正文里设定的名字下面画一道很淡的虚线，悬停提示「查看设定」，点了右栏打开这一条，光标留在正文里。
- 只标在用的设定（不标待确认、已删除的）。和别的记号（伏笔、检查的疑点、AI 写的字、搜到的字）重叠时让给它们。
- 设置「外观」里有「正文中标出设定的名字」，默认勾上，本机的所有作品一起管。
- 样子在原型：`prototype/write.js` 的 `pieces` 和 `seeName`，设置在 `prototype/dialogs.js`。

沿用原型的两项交互，仍待作者在实际写作中试用：

- 点名字是打开设定，还是照常把光标放在那里。现在是打开设定，光标不动；想在名字中间改字，要用方向键或者点名字旁边。另一种做法是按住 Ctrl 点才打开。
- 名字的虚线和伏笔的点线看起来像。

## Alternatives considered

- **按住 Ctrl 点才打开** — 最强的理由：点正文就是放光标，写作时最常做的事不被打断。没有用（暂时）：受访者说的是「点一下」，Ctrl 点没人会想到；名字只占正文里很少的字，点到名字中间想改字的时候不多。等作者在软件里试了再定。
- **悬停就浮出设定的内容** — 最强的理由：不用点，看完就走。没有用：写作时指针常停在正文上，经过名字就弹出东西会分心；右栏本来就是看设定的地方。

## Consequences

- desk/feedNames 只提供 PlaceBody 的名字和条目 ID；设定修订或正文改动后刷新。editor.NameLink 和命中的 rune 区间只存在编辑器显示层，不进入 Doc.Marks、正文文件或撤销记录。
- 绘制和点击共用同一套区间；伏笔、AI 标记、检查疑点、搜索结果、提议和选区有重叠时，整个名字让开。命中使用排版后的字形范围，支持折行，名字旁的留白仍可正常放光标。
- 点击调用 desk.seeName，在右栏显示对应设定，并保留正文光标和键盘；无须 AI。悬停显示「查看设定」，虚线使用现有编辑器绘图函数。
- 外观的勾选默认开启。本机所有作品共用，和颜色一起保存到 works.json 的 Shelf.NoNames（noNames 字段，沿用 Shelf 的严格解码和临时文件重命名写入）。旧 works.json 没有此字段时按 false 读取，即默认标出名字；默认值写盘省略该字段。不另存 appearance.json。shelf.TestShelfNameHighlights 检查旧格式默认值、与颜色共同持久化和错误类型拒绝读取。
- desk.TestNamesInTheProse 覆盖点击、继续打字、开关、重新启动和换书；editor.TestNameLinksYieldToAnnotations 和 TestNameLinksFollowEditedProse 覆盖重叠优先级、点击位置、临时性和正文变更。原有 TestNamesComplete、TestAddANameFromTheProse、TestRenameOffersToReplace 仍检查补全和名字表。
- 名字扫描缓存按正文版本和名字列表失效，避开每帧重复扫描；大量名字和长正文的初次扫描仍有成本。单击名字会打开设定，要在名字中间编辑仍按原型用方向键或点旁边，Ctrl 点的取舍尚待作者试用。
