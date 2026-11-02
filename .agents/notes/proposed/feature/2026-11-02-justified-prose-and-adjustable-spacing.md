# Agent Note: 两端对齐与可调的缩进、段距

Status: proposed

## Problem

正文的排版现在是写死的：首行缩进两字（`editor/layout.go` 的 `indent = 2 * f.Size`），段距半个字（`paraGap := f.Size * 0.5`），行末不对齐。分页预览（`editor/pagination.go`）已有 `Indent`、`ParagraphGap` 参数，但作者在软件里调不了。印刷的小说是两端对齐的，作者看预览和导出前的样子时，缺的就是这一项。

mygo 0.7.2 起在 `ui.TextShape` 里有 `Justify`（中文按字间撑开，末行不撑）和 `Indent`，用 `ui.ShapeRichTextWith` 排版；同一版还给 `richtext` 加了 `Theme.FirstLineIndent`、`Theme.ParagraphSpacing`。2026-11-02 升到 mygo 0.7.4 时评估了这些，结论是只有两端对齐是真缺口，缩进和段距不必等 mygo。

## Proposal

先做一个小实验再定范围：

1. 用 `ShapeRichTextWith(Justify: true)` 排几段带引号、省略号、破折号的中文，看避头尾是否正确（mygo 源码里没有找到专门的避头尾规则，这是最大的未知）、撑开后好不好看，截图给作者看。
2. 作者认可后，按 `AGENTS.md` 的规矩先在 `prototype/` 里做出来：预览里的两端对齐开关，设置里的首行缩进（0、1、2 字）和段距。
3. 再写代码。两端对齐先只用在分页预览、手机预览这类只读视图；正文编辑器是否跟进，等实验结果再定。
4. 缩进和段距做成作者的设置，存进 `prefs`，默认值保持现状（2 字、0.5 字），不改变现有作品的样子。

## Alternatives considered

- **整个换成 mygo 的断行（`ShapeRichTextWith` / `TextLayout.Lines`）** — 最强的理由是少维护一套 `breakLines`，还能顺带得到两端对齐。没选：`breakLines` 手写了避头尾和英文按词断；编辑器的光标、选区、AI 提议的插入文字、输入法组字和预测灰字都建立在自己的行和字形索引上，换掉牵连很大。
- **改用 mygo 的 `richtext` 编辑器** — 它自带 `FirstLineIndent`、`ParagraphSpacing`。已在[回复和摘要的 Markdown 改用 mygo 的 ui/markdown](../../implemented/architecture/2026-10-19-markdown-on-mygo.md)里权衡过：它不给光标和选区的屏幕位置，不能控制滚动，不能画插在正文里的提议文字，0.7.4 的说明里也没有补上这些。
- **在自己的 `breakLines` 里做两端对齐** — 最强的理由是不引入新的排版路径，避头尾原样保留。没排除，是实验的对照项：若 mygo 的避头尾不够好，就在自己的断行结果上按行加字间距。代价是要自己处理字间距与光标、选区位置的对应。

## Acceptance criteria

- 实验截图里，行首不出现句号、逗号、右引号等，行末不出现左引号，且两端对齐不出现明显的疏密不均。
- 原型里能切换两端对齐，能调缩进和段距，`npm run verify-prototype` 的步骤跟着改。
- 默认设置下，现有作品的正文、预览、导出的版面与改前一致。
- 缩进、段距的设置重启后保留。

## Risks

- mygo 的中文避头尾不足，两端对齐只能在自己的断行上做，工作量比预想大。
- 预览和正文编辑器的断行不一致，作者会看到行尾位置对不上；因此先限定在只读视图，并在界面上说明。
- 字间距随行变化，分页高度不变，但每行的字形位置变了，手机预览的分页缓存要按新的设置失效。
