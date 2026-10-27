# Agent Note: 阅读入口、页面权限和服务表单状态

Status: implemented

## Problem

[架构审查](../../proposed/architecture/2026-10-27-maintainable-project-review.md)指出缺少人工阅读入口、技能命令未按本地安装适配、页面持有多余 Store 权限，以及 App 状态归属不清。作者要求新建分支开始实施。

## Decision

- 根 README 连接现有规范和启动、保存、AI 三条调用链；AGENTS.md 只链接入口并将笔记判定细则交给技能。
- 笔记技能只列本地已安装命令，脚本直接用 Node 运行；技能和 verification、archiving 参考一致。局部直观修复不自动建笔记，已授权实施不重复确认。磁盘轮询笔记明确 Book 中间层与既有后台 Store 用户；后续获作者同意的轮询改造由该篇单独记录。
- outline、phone 的 Host 只提供 `WorkPath()`；导出仍保留实际需要的 Store。Book 类型注释明确集合的只读借用，包括嵌套值：编辑前复制，不把借用数据交给后台任务。版本仍由 Book 写方法维护。
- `TestDeskLeavesTheBooksContentToBook` 通过 `go list -deps -export` 取得构建文件与依赖导出数据，使用标准库 `go/types` 判断方法声明者是否为 `workspace.Store`；覆盖 desk 及其子包，只有 Book 包可直接使用受保护的内容读写方法。变量别名、嵌入 Store、方法值和方法表达式都按同一规则检查。
- `desk/serviceform.go` 的包内具体类型集中字段、模型行解析、加载和清理；保存、后台请求及 UI 绘制仍由 App 显式协调。展开状态属于 serviceForm，不属于 backupView。每次加载、关闭表单递增 generation，模型目录交回时核对该代次；即使重开同一个服务也不应用旧结果。
- 关闭表单清除未保存输入（包括 key）；`resetView` 在换书和回书架时关闭表单。模型行 key 跨关闭继续递增，避免复用旧控件状态。布局和文案不变；已保存的服务与 key 不受清理影响。
- 本批没有改讨论状态、轮询策略、全局时间参数或其余大文件，保留审查中的后续建议。

### 相关笔记审计

- [架构审查](../../proposed/architecture/2026-10-27-maintainable-project-review.md)：部分落地，保留其余建议，互链。
- [desk 状态](../../implemented/architecture/2026-10-22-desk-state.md)：部分细化，保留其书模型与线程分工，更新 Host 和服务表单事实并互链。
- [笔记校验](../../implemented/process/2026-10-07-agent-notes-gates.md)：本地安装范围不变，更新技能适配事实并互链。
- [DESIGN 分工](../../implemented/process/2026-10-26-design-md-only-shared-rules.md)、[规则只说一次](../../implemented/process/2026-10-15-each-rule-said-once.md)：沿用，不修改设计规则。
- [少看磁盘](../../implemented/architecture/2026-10-13-fewer-looks-at-the-disk.md)：本批只校正实现事实；后续作者同意去重与后台暂停后，方案在该篇落地。
- [加快检查](../../implemented/process/2026-10-13-fast-checks.md)：保留现有分片策略；测试时间参数未在这批迁移。
- rejected 中的灵感池方案及其他 feature 提案与本批无关；无被完全吸收或应归档的笔记。

## Alternatives considered

- **只把字段加一个共同前缀。** diff 小，但状态仍由各处随意装配；同时集中加载、模型行处理和清理才有实际阅读收益。
- **所有页面统一取完整 Store。** Host 签名相同，适配省事；但手机预览和大纲只需要路径，无须承担书内容读写权限。
- **继续用变量名匹配边界。** 检查快、代码短，却无法识别别名和方法值；使用 Go 标准库类型信息，仍只保留这一个专用守卫，不建立分析框架。

## Consequences

- 人工能从 README 找到职责与操作链，agent 的本地命令和记录门槛一致；没有新增文档生成流程或运行时抽象层。
- 两个只需路径的页面不持有 Store；书内容集合仍是借用，不通过每帧深复制强制只读。修改嵌套集合仍须遵守契约，类型守卫不会检测所有别名写入，也不做跨函数数据流分析。
- 服务表单加载、关闭和模型行处理有明确归属，旧目录不能覆盖新草稿。换书会丢弃尚未保存的服务表单输入；此前展开状态已经随 backupView 重置，但隐藏输入仍留在 App。
- 守卫每次需要 Go 构建导出数据和类型检查，比只扫源码慢；复用编译缓存，不引入包加载依赖或分析框架。
- 本地技能与上游有差异，更新时须保留本仓库适配。

## Testing

- 原有服务模型限制、新服务字段校验与保存/取消测试通过；取消测试同时检查未保存输入被清理。
- `TestCatalogBelongsToItsServiceForm` 用可控延迟的模型目录，覆盖正常交回、关闭、重开同一服务和真实换书；后续草稿不被旧目录覆盖。
- 内容边界测试检查真实 desk 包，并对别名、嵌入、方法值、方法表达式注入负例；合法 Book 调用及未缓存的 Store 正文读取不误报。
- `TestShotBackup` 前后截图中，服务表单、AI 设置、外观设置和讨论四张逐像素一致；备份设置的差异为测试临时路径和运行时间。已查看服务表单与备份差异图。
- `npm run verify`：10 项全部通过，101.1 秒；包括 Go 测试、Go vet、UI vet、笔记、原型走查和单独运行的帧耗时检查。README 相对链接全部可定位，`git diff --check` 通过。
