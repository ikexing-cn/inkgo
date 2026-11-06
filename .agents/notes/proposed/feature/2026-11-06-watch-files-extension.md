# Agent Note: 用文件事件发现软件外面的改动，做成一个扩展

Status: proposed

## Problem

正文页现在每 2 秒（`Book.Timing.CheckEvery`）扫一遍章节树、各目录的 `.order` 和当前章的时间戳，看软件外面有没有改稿、改名、删除和移动（`desk/save.go` 的 `watch`、`workspace` 的 `ChaptersAndStamp`）。[少看几次磁盘](../../implemented/architecture/2026-10-13-fewer-looks-at-the-disk.md)已经把它压到一次扫描，失焦时暂停；但千章的书一次仍要几十毫秒，外面的改动最慢 2 秒才看到。

[盘点](../architecture/2026-11-06-reinvented-wheels.md)第 23 条把这列为部分轮子，候选是 `fsnotify/fsnotify`。作者说：「这个感觉也是个有需要的，后续可能维护一个新的插件吧，先记一下笔记」。

## Proposal

做一个可开关的扩展，用文件系统事件告诉宿主「这本书的文件可能变了」，宿主收到就提前做一次现在的检查。

- 用 mygo 自带的 `mygo.WatchFiles`（v0.7.4 的 `watch.go`），不引 fsnotify。它能递归监视一个目录，`Ignore` 跳过不看的路径，`Debounce`、`MaxDelay` 把连着的几次改动合成一次报（默认 50 ms 和 500 ms），另一个文件改名盖过来（原子保存）报成一次写入，超过 `MaxEntries`（Windows、Linux 默认 8192）就停下并报 `ErrWatchLimit`。它的文档也说事件只当「该重读了」的提示。
- 盘点那篇第 23 条列的候选是 fsnotify，当时漏看了 mygo 的这个接口。

- 事件只是提示，不是结论。收到以后照旧走 `ChaptersAndStamp` 和写之前的时间戳核对，判断有没有改、改了什么，仍归宿主现在的代码。
- 轮询留着兜底，不删。网盘同步目录、网络盘、FAT 上的事件可能漏，[少看几次磁盘](../../implemented/architecture/2026-10-13-fewer-looks-at-the-disk.md)的备选里写过这一点。扩展开着时轮询可以放慢，放慢到多少施工时量。
- 自己写的文件也会触发事件，要认出来跳过，不然每次保存都多查一次。
- 扩展只往宿主开的一个点报「某路径变了」，不碰界面、不读正文。点叫什么、在哪一帧处理，照[扩展 API](../architecture/2026-11-03-extension-api.md)的规矩定。
- 和[增量云同步](2026-11-06-incremental-cloud-sync.md)有关：同步客户端从别的机器拉下来的改动，在本机看来就是「软件外面改了」，这个扩展能让它更快出现在界面上。

## Alternatives considered

- **用 `fsnotify/fsnotify`。** 最强的理由是用的人最多。没选：mygo 已经有合并改动、认原子保存、限制条目数的监视接口，再引 fsnotify 是多一份依赖，这几样还得自己补。
- **只靠现在的轮询。** 最强的理由是没有新依赖、行为在所有文件系统上一样。它会一直留着兜底；扩展只是让常见情况更快、更省。
- **用事件完全代替轮询。** 没选：事件会漏，漏了就再也发现不了，正确性不能押在它上面。

## Acceptance criteria

- 开着扩展，用别的编辑器改当前章，界面在 2 秒以内提示，不必等下一次轮询。
- 软件自己保存不会多触发一次检查。
- 关掉扩展或事件出错，行为和现在完全一样。
- 不新加依赖：只用 mygo 的 `WatchFiles`。
- 一本书超过 `MaxEntries` 时扩展自己停下，行为退回只靠轮询，不报错给作者。

## Risks

- 各平台的事件语义不一样：改名可能拆成删除加创建，编辑器的「另存再改名」会触发好几次。只当提示用，这些都不影响正确性，只影响多查几次。
- 监视大目录会占系统的监视名额（Linux 上每个目录占一个）。只监视书的目录，用 `Ignore` 跳过回收站、`.tmp-*` 这类不用看的。

## 相关笔记当场审计

- [少看几次磁盘](../../implemented/architecture/2026-10-13-fewer-looks-at-the-disk.md)：部分重叠，那篇在备选里暂不采用文件监听。本篇是以扩展的形式重提，轮询仍留着兜底，不推翻那篇的决定。
- [正文页和章节](../../implemented/feature/2026-10-08-writing-page-and-chapters.md)：无关，外部改动的提示和处理不变。
- [盘点](../architecture/2026-11-06-reinvented-wheels.md)：第 23 条由本篇接手。
- 没有要删或归档的笔记。
