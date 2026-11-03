# Inkgo

写长篇小说的桌面软件，支持正文编辑、章纲与故事大纲、设定、讨论和 AI 辅助写作。产品用纯 Go 编写（`CGO_ENABLED=0`），原生界面基于 mygo。

## 运行

Go 版本见 [go.mod](go.mod)。验证脚本需要 Node ≥ 22.18，无须安装 npm 依赖；原型走查需要本机有 Edge 或 Chrome。

```sh
go run ./cmd/inkgo
go run ./cmd/inkgo -work <作品目录>
```

`go run ./cmd/sample <目录>` 可以生成两本有内容的测试作品。直接打开 [prototype/index.html](prototype/index.html) 可以查看交互原型，原型使用假数据，不保存编辑。

```sh
npm run verify
```

全套验证和截图用法见 [AGENTS.md](AGENTS.md)，各检查的编排在 [scripts/verify.ts](scripts/verify.ts)。

## 代码地图

| 位置 | 职责 |
| --- | --- |
| [cmd/inkgo](cmd/inkgo/main.go) | 装配应用、窗口生命周期、系统对话框和窗口回调 |
| [desk](desk/doc.go) | 界面、导航、用户操作与后台结果的协调；包内文件分工见入口注释 |
| [desk/book](desk/book/book.go) | 界面线程看到的书内容、版本与写后刷新 |
| [desk/outline](desk/outline/doc.go)、[phone](ext/phone/doc.go)、[export](ext/export/doc.go) | 独立页面或面板，outline 经 Host，phone 与 export 经扩展宿主取得能力 |
| [desk/kit](desk/kit/doc.go) | 调色板、尺寸和共用界面零件 |
| [editor](editor/editor.go) | 文本编辑、撤销、光标、标记与排版；不依赖作品或 AI 实现 |
| [workspace](workspace/store.go) | 作品类型与规则、文件读写、校验和外部修改保护 |
| [aiport](aiport/aiport.go)、[ai](ai/ai.go) | 界面使用的 AI 契约，以及装配、开关和请求记录 |
| [discussion](discussion/engine.go)、[writing](writing/writer.go) | 讨论与工具调用，以及续写、正文检查 |
| [inference](inference/model.go)、[prompt](prompt/prompt.go)、[model](model/client.go) | 共用推理规则、上下文预算、模型配置及网络协议 |
| [backup](ext/backup/module.go)、[export](ext/export/module.go)、[shelf](shelf/shelf.go)、[prefs](prefs/prefs.go) | 备份、导出、作品库与本机设置、每本作品的界面偏好 |
| [internal](internal)、[tokens](tokens/tokens.go) | 有具体用途的小工具、测试辅助和 token 估算 |

主要依赖方向：入口装配 `desk` 和 `ai`；`desk` 只通过 `aiport` 使用 AI；`ai` 组合 `discussion`、`writing` 和 `model`。界面和 AI 都使用 `workspace`，底层不反向依赖界面。对应边界由各包的依赖测试保护。

## 从一个操作读起

- **启动和换书**：[main](cmd/inkgo/main.go) → `desk.New/Open/attach`（[app.go](desk/app.go)）→ `workspace.Open`。
- **打字和保存**：[editor](editor/editor.go) → [save.go](desk/save.go) 的停笔保存 → [opened.go](desk/opened.go) 的当前章与冲突状态 → `Book.SaveChapterOver` → [workspace/prose.go](workspace/prose.go) 的写前版本比对。
- **外部改稿检查**：[main](cmd/inkgo/main.go) 的窗口活动通知 → [save.go](desk/save.go) 的前台轮询 → `Book.PollChapter` → [workspace/tree.go](workspace/tree.go) 同次读取树和当前章时间戳。后台暂停周期检查，返回立即检查；自动保存照常。
- **讨论和 AI 提议**：[talkrun.go](desk/talkrun.go) 的 `send/say` → `aiport.Talk` → [ai.go](ai/ai.go) → [discussion/engine.go](discussion/engine.go)。流式结果交回所属讨论轮次，正文提议的采纳在 [propose.go](desk/propose.go)。

阅读状态时区分三处：Store 是磁盘上的作品；Book 是界面线程最近读到的书内容；`openChapter` 是编辑器里尚未保存的字和外部冲突。后台任务使用 Store，交回界面后刷新 Book；集合访问的借用规则写在 Book 的类型注释中。

## 规范和设计依据

- [AGENTS.md](AGENTS.md)：开发流程和验证要求。
- [DESIGN.md](DESIGN.md)：通用界面规范；具体界面看 [prototype](prototype/index.html)。
- [.agents/notes](.agents/notes)：架构取舍和功能决定；`implemented` 是现行决定，`proposed` 尚未落地，`rejected` 记录被否方案，`archived` 是冻结历史。
- [.agents/skills](.agents/skills)：按任务启用的工作方法，不必为普通代码修改通读所有技能。

重要的状态和线程分工见 [desk 状态笔记](.agents/notes/implemented/architecture/2026-10-22-desk-state.md)，磁盘格式的取舍见 [文件存储笔记](.agents/notes/implemented/architecture/2026-10-08-m1-storage-without-index.md)。
