// 写作那一页的左边和中间：侧栏、工具栏、正文，和正文上浮着的东西。
'use strict'

// ───────── 动作：章节 ─────────

const CLEAR = { ctx: null, pick: null, bury: null, pay: null, bar: null, ai: null, box: '', tree: null, rename: null }

// 打开一章或一卷：已经有它的标签页就切过去，没有就在正看的那个右边开一个。它所在的卷展开，上一章上面浮着的东西收掉，记进打开过的里面。
function go(s, key, flash) {
  const ch = s.chapters.find((c) => c.key === key), vol = !ch && s.volumes.some((v) => v[0] === key)
  if (!ch && !vol) return null // 已经删了
  const trail = s.trail[s.step] === key ? s.trail : [...s.trail.slice(0, s.step + 1), key]
  const at = s.tabs.indexOf(s.ch || s.vol), tabs = s.tabs.includes(key) ? s.tabs : [...s.tabs.slice(0, at + 1), key, ...s.tabs.slice(at + 1)]
  return { page: 'write', ch: ch ? key : '', vol: vol ? key : '', tabs, trail, step: s.trail === trail ? s.step : trail.length - 1, folds: { ...s.folds, [ch ? ch.vol : key]: false }, ...CLEAR, outside: '', flash: flash || null }
}
// 关掉几个标签页。关的是正看的那个，就换到它右边的，没有就左边的；一个不剩时正文空着。没存的字关之前已经存了。
const shutTabs = (gone) => set((s) => {
  const tabs = s.tabs.filter((k) => !gone.includes(k)), on = s.ch || s.vol, at = s.tabs.indexOf(on)
  if (!gone.includes(on)) return { tabs, tree: null }
  const next = s.tabs.slice(at + 1).find((k) => tabs.includes(k)) || s.tabs.slice(0, at).reverse().find((k) => tabs.includes(k))
  return next ? { ...go({ ...s, tabs }, next), saved: true } : { tabs, ch: '', vol: '', step: s.trail.length, ...CLEAR, outside: '', saved: true } // step 停在最后一个之后：‹ 回到刚关掉的那一章
})
const goTo = (key, flash) => () => set((s) => go(s, key, flash))
// 侧栏顶上的 ‹ ›：回到上一个、下一个打开过的章。
const stepBy = (d) => set((s) => {
  const to = go(s, s.trail[s.step + d])
  return to ? { ...to, trail: s.trail, step: s.step + d } : null
})

// 改一章的字：change 拿到原来的几段，交回新的几段。检查过的结果、写好的摘要从此算旧的，也还没存。
const retype = (s, key, change) => {
  const sum = s.summaries[key]
  return {
    chapters: s.chapters.map((c) => (c.key === key ? { ...c, paras: change(c.paras) } : c)), dirty: { ...s.dirty, [key]: true }, saved: false,
    summaries: sum && sum.text ? { ...s.summaries, [key]: { ...sum, stale: true } } : s.summaries,
  }
}
// 改之前先存一份快照。
const snap = (s, key, kind, ai) => ({ snaps: { ...s.snaps, [key]: [{ id: 's' + s.n, at: '刚刚', kind, ai, paras: chapterOf(key).paras }, ...(s.snaps[key] || [])] }, n: s.n + 1 })

function newChapter() {
  set((s) => {
    // 开着一章，就建在它后面；开着一卷，就建在这一卷的末尾。
    const cur = chapterOf(s.ch) || (s.vol ? s.chapters.filter((c) => c.vol === s.vol).at(-1) : null), key = 'n' + s.n
    const made = { key, no: cur ? cur.no + 1 : 1, vol: cur ? cur.vol : s.vol, t: '新章节', c: '', paras: [] }
    const at = cur ? s.chapters.indexOf(cur) + 1 : s.chapters.length
    const chapters = [...s.chapters.slice(0, at), made, ...s.chapters.slice(at)]
    return { chapters, n: s.n + 1, never: { ...s.never, [key]: true }, ...go({ ...s, chapters }, key), rename: { key, t: made.t } }
  })
}
function newFolder() {
  set((s) => ({ volumes: [...s.volumes, ['v' + s.n, '新文件夹', '']], n: s.n + 1, tree: null, rename: { key: 'v' + s.n, t: '新文件夹' } }))
}
function renameNow() {
  set((s) => {
    const { key, t: title } = s.rename, name = title.trim()
    if (!name) return { rename: null }
    return { rename: null, chapters: s.chapters.map((c) => (c.key === key ? { ...c, t: name } : c)), volumes: s.volumes.map((v) => (v[0] === key ? [key, name, v[2]] : v)) }
  })
}
// 上移、下移：和同一层里前一个、后一个换位置。
function shift(key, d) {
  set((s) => {
    const swap = (list, same, at) => {
      const peers = list.filter(same), other = peers[peers.indexOf(list[at]) + d]
      if (!other) return list
      const out = [...list], j = list.indexOf(other)
      out[at] = other; out[j] = list[at]
      return out
    }
    const v = s.volumes.findIndex((x) => x[0] === key), c = s.chapters.findIndex((x) => x.key === key)
    return { tree: null, volumes: v >= 0 ? swap(s.volumes, () => true, v) : s.volumes, chapters: c >= 0 ? swap(s.chapters, (x) => x.vol === s.chapters[c].vol, c) : s.chapters }
  })
}
const moveInto = (key, vol) => set((s) => {
  const c = chapterOf(key)
  return { tree: null, chapters: [...s.chapters.filter((x) => x !== c), { ...c, vol }], folds: { ...s.folds, [vol]: false } }
})
// 章节树里拖一行：放到一卷上就进这一卷的末尾，放到一章上就排到它前面。卷不能放进卷里。
function dropOn(key, target) {
  set((s) => {
    const c = s.chapters.find((x) => x.key === key), into = s.volumes.some((v) => v[0] === target), before = s.chapters.find((x) => x.key === target)
    if (!c || c === before) return null
    const rest = s.chapters.filter((x) => x !== c)
    if (into) return { chapters: [...rest, { ...c, vol: target }], folds: { ...s.folds, [target]: false } }
    if (!before) return null
    const at = rest.indexOf(before)
    return { chapters: [...rest.slice(0, at), { ...c, vol: before.vol }, ...rest.slice(at)] }
  })
}
// 移到回收站：一卷连同里面的章一起走。开着的那章没了，就换一章开。
function toBin(key) {
  const vol = S.volumes.find((v) => v[0] === key)
  const gone = S.chapters.filter((c) => (vol ? c.vol === key : c.key === key))
  const title = vol ? vol[1] : titleOf(gone[0])
  set((s) => {
    const chapters = s.chapters.filter((c) => !gone.includes(c))
    // 进了回收站的，标签页也关掉。正看的那一章没了：换到旁边的标签页；没有别的标签页，就打开树里挨着它的一章。
    const left = s.tabs.filter((k) => k !== key && !gone.some((c) => c.key === k)), on = s.ch || s.vol
    const near = gone.some((c) => c.key === s.ch) && chapters.length ? chapters[Math.min(s.chapters.indexOf(gone[0]), chapters.length - 1)].key : ''
    const to = left.includes(on) ? on : left[Math.min(s.tabs.indexOf(on), left.length - 1)] || near
    return {
      chapters, volumes: s.volumes.filter((v) => v !== vol), n: s.n + 1, tree: null,
      bin: [{ id: 'b' + s.n, title, at: '刚刚', volume: vol || null, chapters: gone }, ...s.bin],
      tabs: !to || left.includes(to) ? left : [...left, to],
      ...(to === on ? {} : chapters.some((c) => c.key === to) ? { ch: to, vol: '' } : { ch: '', vol: to, ...CLEAR }),
    }
  })
  say('「' + title + '」已移到回收站')
}

// ───────── 动作：正文里选中的字 ─────────

// 屏幕上的一点，换成从窗口左上角算起的位置。
function spot(x, y) {
  const box = document.getElementById('win').getBoundingClientRect()
  return [x - box.left, y - box.top]
}

// 正文里选中的字：在第几段、从哪到哪、在窗口的什么位置。跨段的不算。
function grab() {
  const sel = getSelection()
  if (!sel || sel.isCollapsed || !sel.rangeCount) return null
  const range = sel.getRangeAt(0)
  const within = (n) => { const e = n.nodeType === 1 ? n : n.parentElement; return e ? e.closest('[data-para]') : null }
  const para = within(range.startContainer)
  const text = range.toString()
  if (!para || para !== within(range.endContainer) || !text.trim()) return null
  const before = document.createRange()
  before.selectNodeContents(para)
  before.setEnd(range.startContainer, range.startOffset)
  const start = before.toString().length
  const r = range.getBoundingClientRect()
  const [left, y] = spot(r.left, r.top)
  return { para: Number(para.dataset.para), start, end: start + text.length, text, left, top: y, bottom: y + r.height }
}
const unselect = () => getSelection().removeAllRanges()

// 松开鼠标时：选中了字，就在它上面浮出「AI 改写」「加入设定」。
function picked(ev) {
  if (ev.button !== 0 || S.ai) return
  const pick = grab()
  if (pick || S.bar) set({ bar: pick })
}

function openMenu(ev) {
  ev.preventDefault()
  const on = ev.target.closest('[data-mark]')
  const [x, y] = spot(ev.clientX, ev.clientY)
  set({ ctx: { x, y, mark: on ? on.dataset.mark : '' }, pick: grab(), bar: null, bury: null, pay: null, flash: null })
}

const closeFloat = () => set({ ctx: null, bury: null, pay: null, pick: null, tree: null })

// 选回收哪个伏笔时列出来的那些，和眼下选中的那一个：没点过就是头一个。
const payable = () => { const q = S.pay.q.trim(); return S.marks.filter((m) => (m.st === 'open' || m.st === 'done') && (!q || m.note.includes(q) || m.quote.includes(q))) }
const payPick = () => { const can = payable(); return can.some((m) => m.id === S.pay.id) ? S.pay.id : can.length ? can[0].id : '' }

// 问「选择要回收的伏笔」。id 是先选好的那条，从右键菜单来的没有。
function askPay(id) {
  const pick = grab() || S.pick
  if (!pick) { say('请先在正文中选中回收伏笔的段落'); return }
  set({ ctx: null, bury: null, bar: null, pick, pay: { q: '', id } })
}

function buryNow() {
  set((s) => {
    if (!s.pick) return null
    const ch = chapterOf(s.ch)
    const m = { id: 'm' + s.n, st: 'open', note: s.bury.note.trim(), ch: ch.no, key: ch.key, para: s.pick.para, quote: s.pick.text }
    return { marks: [...s.marks, m], n: s.n + 1, bury: null, pick: null }
  })
  unselect()
}

function payNow(id) {
  if (!id) return
  set((s) => {
    if (!s.pick) return null
    const ch = chapterOf(s.ch)
    const pay = { ch: ch.no, key: ch.key, para: s.pick.para, quote: s.pick.text }
    return { marks: s.marks.map((m) => (m.id === id ? { ...m, st: 'done', pay } : m)), pay: null, pick: null, tab: '伏笔', fid: id, fv: 'list', renote: null, paneOpen: true, ...SHUT }
  })
  unselect()
}

// 把选中的一段拿到右栏的讨论里。
function cite() {
  const pick = grab() || S.pick
  if (!pick) return
  const cur = chapterOf(S.ch)
  set({ ctx: null, pick: null, bar: null, cite: { key: cur.key, title: titleOf(cur), quote: pick.text }, paneOpen: true, focus: false, tab: '讨论', tv: S.tv === 'one' ? 'one' : 'new' })
  unselect()
}

// 把选中的一段记成设定：先问一句记成什么。
function take() {
  const pick = grab() || S.pick
  if (!pick) return
  set({ ctx: null, pick: null, bar: null, dialog: { kind: 'take', text: pick.text, cat: S.cats[0], src: { key: S.ch, quote: pick.text } } })
  unselect()
}

// ───────── 动作：AI 在正文里写 ─────────

const writer = () => { const u = S.uses[2].model ? S.uses[2] : S.uses[0]; return [u.service + '/' + u.model, S.uses[2].effort] }

function continueText() {
  if (!chapterOf(S.ch) || S.ai) return
  set({ ctx: null, pick: null, bar: null, ...SHUT, ai: { kind: 'ghosting' } })
  later(700, () => set((s) => (s.ai && s.ai.kind === 'ghosting' ? { ai: { kind: 'ghost', text: BOOK.ghost } } : null)))
}
// 续写出的那一句：采纳就接在最后一段后面。
function ghost(keep) {
  set((s) => {
    if (!s.ai || s.ai.kind !== 'ghost') return null
    return { ai: null, ...(keep ? retype(s, s.ch, (paras) => (paras.length ? [...paras.slice(0, -1), paras.at(-1) + s.ai.text] : [s.ai.text])) : {}) }
  })
}

function askRewrite() {
  const pick = grab() || S.pick
  if (!pick) return
  set({ ctx: null, pick: null, bar: null, ai: { kind: 'ask', pick, want: '' } })
  unselect()
}
function rewrite() {
  set((s) => ({ ai: { ...s.ai, kind: 'rewriting' } }))
  later(1200, () => set((s) => (s.ai && s.ai.kind === 'rewriting' ? { ai: { ...s.ai, kind: 'diff', text: BOOK.rewrite } } : null)))
}
function takeRewrite() {
  set((s) => {
    const { pick, text } = s.ai
    return { ai: null, ...snap(s, s.ch, 'AI 改动前'), ...retype(s, s.ch, (paras) => paras.map((p, i) => (i === pick.para ? p.slice(0, pick.start) + text + p.slice(pick.end) : p))) }
  })
  say('已采纳改写', '可撤销恢复原文，原版本已存入快照', 'ok')
}

function draftNow() {
  const [model, effort] = writer(), d = S.dialog, key = S.ch
  if (d.goal.trim() && !/^[1-9]\d*$/.test(d.goal.trim())) { say('目标字数须为正整数，或留空', '', 'fail'); return }
  set((s) => ({
    dialog: null, ...SHUT, bar: null,
    ...snap(s, key, 'AI 改动前', { head: '按章纲起草 · ' + model + ' · ' + effort + (d.goal.trim() ? ' · 约 ' + d.goal.trim() + ' 字' : ''), saw: 'AI 参考了：设定 ' + s.entries.filter((e) => e.place === 'body').length + ' 条、章纲、本章前文' }),
    ...retype(s, key, (paras) => (d.how === '覆盖已有内容' ? [] : paras)),
    ai: { kind: 'drafting', key, n: 0, wrote: 0 },
  }))
  set((s) => ({ chapters: s.chapters.map((c) => (c.key === key ? { ...c, aiFrom: c.paras.length } : c)) }))
  const next = () => later(800, () => {
    const a = S.ai
    if (!a || a.kind !== 'drafting') return
    const words = BOOK.drafted[a.n]
    if (!words) { set({ ai: null }); say('起草完成，共 ' + a.wrote + ' 字', '可撤销整段撤回，起草前的版本已存入快照', 'ok'); return }
    set((s) => ({ ...retype(s, key, (paras) => [...paras, words]), ai: { ...a, n: a.n + 1, wrote: a.wrote + words.length } }))
    next()
  })
  next()
}
function stopDraft() {
  set({ ai: null })
  say('已停止起草，已写部分保留；可撤销整段撤回')
}

// 让 AI 给一章写摘要。
function recap(key) {
  set((s) => ({ summaries: { ...s.summaries, [key]: { ...(s.summaries[key] || { text: '' }), busy: true } } }))
  later(1200, () => set((s) => ({ summaries: { ...s.summaries, [key]: { text: key === String(BOOK.now) ? BOOK.recap : '本章摘要：' + clip(chapterOf(key).paras.join(''), 40), by: 'AI 生成 · Opus 5.5 · 刚刚' } } })))
}

// Ctrl+S：马上存。AI 正在改的时候，等它改完再存。
function saveNow() {
  if (S.ai && (S.ai.kind === 'drafting' || S.ai.kind === 'rewriting')) { say('AI 正在修改本章，完成后自动保存'); return }
  set({ saved: true })
  say('已保存', '停笔片刻也会自动保存')
}

// 软件外面改了开着的这一章：留磁盘上的还是编辑器里的，另一边存成快照。
function settleOutside(disk) {
  const key = S.ch
  set((s) => ({ outside: '', ...snap(s, key, '外部改动'), ...(disk ? retype(s, key, (paras) => [...paras, BOOK.outside]) : {}) }))
  say(disk ? '已采用磁盘版本，编辑器原内容已存为快照' : '已保留编辑器版本，磁盘内容已存为快照')
}
// 软件外面把开着的这一章删了、改名了：编辑器里的字存成一章新的。
function saveLost() {
  set((s) => {
    const c = chapterOf(s.ch)
    return { outside: '', chapters: [...s.chapters.filter((x) => x !== c), { ...c, vol: '' }] }
  })
  say('已存为新章节，位于最上层末尾')
}

// 让备份换一种没成功的样子。
const failBackup = (why) => () => set((s) => ({ ...SHUT, backup: { ...s.backup, state: 'fail', why, at: s.backup.at || '昨天 21:40', dir: s.backup.dir || 'E:\\写作备份', hush: false } }))

// 平时不易遇到的情况：出错、作品在软件外被改动。在窗口上面那一排里点出来看。
function rare() {
  const here = (change) => () => {
    if (!chapterOf(S.ch)) { set(SHUT); say('请先打开一章'); return }
    set((s) => ({ page: 'write', focus: false, dialog: null, find: null, ...CLEAR, ...SHUT, ...change(s) }))
  }
  const why = '无法连接模型服务'
  return [
    ['本章在软件外被修改', here(() => ({ outside: 'changed' }))],
    ['本章文件丢失', here(() => ({ outside: 'lost' }))],
    ['本章检查失败', here((s) => ({ failed: { ...s.failed, [s.ch]: why } }))],
    ['本章摘要生成失败', here((s) => ({ box: '摘要', summaries: { ...s.summaries, [s.ch]: { ...(s.summaries[s.ch] || { text: '' }), busy: false, fail: why } } }))],
    ['AI 改写出错', () => { set(SHUT); say('改写失败', '模型未返回改写结果', 'fail') }],
    ['设定整理失败', () => set((s) => ({ ...SHUT, n: s.n + 1, log: [{ id: 'l' + s.n, who: '后台整理', model: 'Opus 5.5 · Low', at: '刚刚', lines: [], fail: '最近几轮整理失败：无法连接 AI' }, ...s.log] }))],
    ['模型目录读取失败', () => { set(SHUT); const id = say('读取模型目录失败', why, 'fail', [['重试', () => { unsay(id); fetchModels() }], ['打开设置', () => set({ dialog: { kind: 'settings', tab: '模型', edit: '' }, toasts: [], ...SHUT })]]) }],
    ['备份文件夹丢失', failBackup('dir')],
    ['备份文件夹不可写', failBackup('deny')],
    ['添加的文件夹中没有作品', () => { set(SHUT); say('该文件夹中没有作品（缺少 work.yaml）', 'D:\\下载\\新建文件夹', 'fail') }],
  ]
}

// ───────── 动作：查找替换、检查、待处理 ─────────

// 一章里找到的每一处：在第几段、从哪开始。
function hits(c, q) {
  const out = []
  if (q) c.paras.forEach((p, para) => { for (let a = p.indexOf(q); a >= 0; a = p.indexOf(q, a + q.length)) out.push({ para, a }) })
  return out
}
const findStep = (d) => set((s) => { const n = hits(chapterOf(s.ch), s.find.q).length; return n ? { find: { ...s.find, at: (s.find.at + d + n) % n } } : null })
function swapOne() {
  set((s) => {
    const { q, to, at } = s.find, all = hits(chapterOf(s.ch), q), hit = all[at]
    if (!hit) return null
    // 换掉的是最后一处，就回到第一处。
    return { ...retype(s, s.ch, (paras) => paras.map((p, i) => (i === hit.para ? p.slice(0, hit.a) + to + p.slice(hit.a + q.length) : p))), find: { ...s.find, at: at < all.length - 1 ? at : 0 } }
  })
}
function swapAll() {
  const { q, to } = S.find, n = hits(chapterOf(S.ch), q).length
  if (!n) return
  set((s) => retype(s, s.ch, (paras) => paras.map((p) => p.split(q).join(to))))
  say('已替换 ' + n + ' 处，可撤销')
}
function swapBook() {
  const { q, to } = S.dialog, had = S.chapters.filter((c) => hits(c, q).length)
  const n = had.reduce((sum, c) => sum + hits(c, q).length, 0)
  set((s) => {
    let next = { ...s, dialog: null }
    for (const c of had) next = { ...next, ...snap(next, c.key, '全书替换前'), ...retype(next, c.key, (paras) => paras.map((p) => p.split(q).join(to))) }
    return next
  })
  say('已在 ' + had.length + ' 章中替换 ' + n + ' 处，各章替换前的版本已存入快照')
}

const counted = (c) => Number(sizeOf(c).replace(/,/g, ''))
const chapterName = (c) => (c.vol ? '第' + c.no + '章' : c.t)

// 查一章，或者排着队一章一章查。查完的不再算没查过、改过、没查成。
function checkNow(...keys) {
  set({ checking: keys[0], queue: keys.slice(1) })
  const next = () => later(900, () => {
    const key = S.checking
    if (!key) return // 中途停下了
    set((s) => ({ checking: s.queue[0] || false, queue: s.queue.slice(1), dirty: { ...s.dirty, [key]: false }, never: { ...s.never, [key]: false }, failed: { ...s.failed, [key]: '' } }))
    if (S.checking) next()
  })
  next()
}
// 全书里该查的几章：够字数，而且没查过、改过或者上次没查成。
const due = () => S.chapters.filter((c) => counted(c) >= 200 && (S.never[c.key] || S.dirty[c.key] || S.failed[c.key]))

// 待处理里的每一条：哪一组、写什么、在哪一章、点了去哪。忽略掉的不算。
function pending() {
  const list = []
  for (const c of S.checks) {
    const ch = chapterOf(c.key)
    list.push({ id: c.id, group: '设定不一致', name: c.kind === '新出现' ? '「' + c.title + '」尚未加入设定' : c.title, about: ch ? chapterName(ch) : '', key: c.key, open: () => set({ page: 'check', kind: '全部', cid: c.id, ...SHUT }) })
  }
  for (const m of S.marks) {
    const gap = BOOK.now - m.ch
    const open = () => set({ paneOpen: true, focus: false, tab: '伏笔', fid: m.id, fv: 'list', renote: null, ...SHUT })
    if (m.st === 'lost') list.push({ id: m.id, group: '伏笔', name: '「' + nameOf(m) + '」原句已删', about: '第' + m.ch + '章', key: m.key, open })
    if (m.st === 'open' && gap >= S.late) list.push({ id: m.id, group: '伏笔', name: '「' + nameOf(m) + '」' + gap + ' 章未回收', about: '第' + m.ch + '章 埋下', key: m.key, open })
  }
  if (S.backup.state === 'fail') list.push({ id: 'backup', group: '备份', name: '备份失败，' + backupWhy()[0], about: '点击重试 · 上次备份 ' + S.backup.at, key: '', open: backupNow })
  return list.filter((x) => !S.ignored.includes(x.id))
}

// ───────── 画：侧栏 ─────────

function sidebar() {
  const nav = [['讨论', 'chat', 'talk'], ['正文', 'book', 'write'], ['检查', 'checked', 'check'], ['作品分类', 'tag', 'cats']]
  const to = { talk: () => set({ page: 'talk', ...(S.tv === 'all' ? { tid: S.talks[0] ? S.talks[0].id : null, tv: S.talks[0] ? 'one' : 'new' } : {}), ...SHUT }), write: () => set({ page: 'write', ...SHUT }), check: () => set({ page: 'check', ...SHUT }), cats: () => set({ page: 'cats', ...SHUT }) }
  const naming = (key) => `<div class="tree-row naming" data-k="naming">${typed(S.rename.t, (v) => set({ rename: { key, t: v } }), '章节标题', '章节标题', keys(renameNow, () => set({ rename: null })) + ' autofocus')}</div>`
  const menu = (key, talk) => onMenu((ev) => { ev.preventDefault(); const [x, y] = spot(ev.clientX, ev.clientY); set({ tree: { key, x, y, talk } }) })
  const leaf = (c) => (S.rename && S.rename.key === c.key ? naming(c.key)
    : `<div class="tree-row hit${c.vol ? ' deep' : ''}${c.key === S.ch ? ' on' : ''}" draggable="true" data-node="${c.key}"${onClick(goTo(c.key))}${menu(c.key)}>${icon('page', 'sm')}<div class="fit ell">${t(titleOf(c))}</div><div class="count">${t(sizeOf(c))}</div></div>`)
  const volume = ([key, title, count]) => (S.rename && S.rename.key === key ? naming(key)
    : `<div class="tree-row hit${key === S.vol ? ' on' : ''}" data-node="${key}"${onClick(() => set((s) => (s.vol === key ? { folds: { ...s.folds, [key]: !s.folds[key] } } : go(s, key))))}${menu(key)}>${icon(S.folds[key] ? 'right' : 'down', 'xs')}<div class="fit ell">${t(title)}</div><div class="count">${t(count)}</div></div>`) +
    (S.folds[key] ? '' : listed.filter((c) => c.vol === key).map(leaf).join(''))
  const listed = S.chapters.filter((c) => S.outside !== 'lost' || c.key !== S.ch) // 磁盘上找不到的那一章，树里也没有
  const book = S.shelf.find((b) => b.id === S.book)
  const others = S.shelf.filter((b) => b.id !== S.book && !b.gone)
  const open = (talk) => S.page === 'talk' && talk.id === S.tid && S.tv === 'one'
  const put = S.talks.filter((x) => S.shelved[x.id])
  const talkRow = (talk) => `<div class="tree-row hit${open(talk) ? ' on' : ''}${S.shelved[talk.id] ? ' put' : ''}"${onClick(() => set({ page: 'talk', tv: 'one', tid: talk.id, draft: '' }))}${menu(talk.id, true)}><div class="fit ell">${t(talk.title)}</div><div class="count">${t(talk.at)}</div></div>`
  const b = S.backup
  const backupLine = { ok: '已备份 · ' + b.at, busy: '正在备份…', fail: '备份失败', none: '尚未备份' }[b.state]

  return '<div class="sidebar">' +
    `<div class="sidebar-top"><div>${svg('icon-md', PANEL + path('M9 5v14'))}</div><div class="hit${S.step > 0 ? '' : ' off'}" title="上一个打开的章节"${onClick(() => stepBy(-1))}>${icon('left', 'sm')}</div><div class="hit${S.trail[S.step + 1] ? '' : ' off'}" title="下一个打开的章节"${onClick(() => stepBy(1))}>${icon('right', 'sm')}</div></div>` +
    `<div class="anchor wide"><div class="book hit" title="切换作品"${onClick(() => set({ open: S.open === 'book' ? '' : 'book' }))}><div class="book-logo">${t(book.name[0])}</div><div class="book-names"><div class="book-name">${t(book.name)}</div><div class="book-kind">${t(book.kind)}</div></div>${icon('down', 'sm', 'faint')}</div>` +
    (S.open === 'book' ? '<div class="pop">' + (others.length ? '<div class="pop-label">切换作品</div>' + others.map((o) => menuRow(o.name, () => openBook(o.id), '', o.kind)).join('') + HAIR : '') +
      menuRow('新建作品…', () => set({ page: 'shelf', dialog: { kind: 'newBook', name: '', type: BOOK.kinds[0] }, ...SHUT })) + menuRow('全部作品…', () => set({ page: 'shelf', ...SHUT })) + menuRow('快捷键…', () => set({ dialog: { kind: 'keys' }, ...SHUT }), '', 'Ctrl+/') + '</div>' : '') + '</div>' +
    `<div class="navs">${nav.map(([name, pic, page]) => `<div class="nav hit${S.page === page ? ' on' : ''}"${onClick(to[page])}>${icon(pic)}<div class="grow">${name}</div>${page === 'check' && S.checks.length ? `<div class="count">${S.checks.length}</div>` : ''}</div>`).join('')}</div>` +
    (S.page === 'write'
      ? `<div class="tree-head"><div>章节</div><div class="tree-head-buttons"><button aria-label="新建文件夹" title="新建文件夹"${onClick(newFolder)}>${svg('icon-sm', SHAPE.folder + path('M12 10.5v5M9.5 13h5'))}</button><button aria-label="新建章节" title="新建章节（Ctrl+N）"${onClick(newChapter)}>${ICON.plus}</button></div></div>` +
        `<div class="tree scroll">${S.volumes.map(volume).join('')}${listed.filter((c) => !c.vol).map(leaf).join('')}</div>`
      : `<div class="tree-head"><div>总讨论</div><div class="tree-head-buttons"><button aria-label="新对话" title="新对话"${onClick(() => set({ page: 'talk', tv: 'new', tid: null, draft: '' }))}>${ICON.plus}</button></div></div>` +
        `<div class="tree scroll">${S.talks.filter((x) => !S.shelved[x.id] || S.shelvedShown || open(x)).map(talkRow).join('')}` +
        (put.length ? `<div class="tree-more hit"${onClick(() => set({ shelvedShown: !S.shelvedShown }))}>${S.shelvedShown ? '不显示收起的对话' : '已收起 ' + put.length + ' 段对话'}</div>` : '') + '</div>') +
    `<div class="anchor up wide"><div class="backup-line hit"${onClick(() => set({ open: S.open === 'backup' ? '' : 'backup' }))}><div class="dot${b.state === 'ok' || b.state === 'busy' ? ' grey' : ''}"></div><div class="fit ell">${t(backupLine)}</div></div>${S.open === 'backup' ? backupPop() : ''}</div>` +
    `<div class="sidebar-end"><div class="bin hit"${onClick(() => set({ dialog: { kind: 'bin', sure: '' }, ...SHUT }))}>${icon('bin')}<div>回收站</div></div><div class="gear hit" title="设置"${onClick(() => set({ dialog: { kind: 'settings', tab: '模型', edit: '' }, ...SHUT }))}>${icon('gear')}</div></div>` +
    '</div>'
}

// 侧栏里右键一章或一卷点出来的菜单；一段讨论、一个标签页的菜单也在这里。
function treeMenu() {
  if (S.tree.tab) {
    const key = S.tree.key, others = S.tabs.filter((k) => k !== key), right = S.tabs.slice(S.tabs.indexOf(key) + 1)
    return `<div class="veil float-veil"${onClick(closeFloat)}${onMenu((ev) => { ev.preventDefault(); closeFloat() })}></div>` +
      `<div class="ctx pop" data-k="tree-menu" style="left:${Math.min(S.tree.x, document.getElementById('win').offsetWidth - 208)}px;top:${S.tree.y}px">` +
      menuRow('关闭', () => shutTabs([key]), '', 'Ctrl+W') + menuRow('关闭其他', others.length ? () => shutTabs(others) : null) + menuRow('关闭右侧', right.length ? () => shutTabs(right) : null) + '</div>'
  }
  if (S.tree.talk) {
    const id = S.tree.key, flip = () => set((s) => ({ tree: null, shelved: { ...s.shelved, [id]: !s.shelved[id] } }))
    return `<div class="veil float-veil"${onClick(closeFloat)}${onMenu((ev) => { ev.preventDefault(); closeFloat() })}></div>` +
      `<div class="ctx pop" data-k="tree-menu" style="left:${S.tree.x}px;top:${S.tree.y}px">${menuRow(S.shelved[id] ? '移回列表' : '收起这段对话', flip)}</div>`
  }
  const key = S.tree.key, vol = S.volumes.some((v) => v[0] === key), c = chapterOf(key)
  const spots = vol ? [] : [['最上层', ''], ...S.volumes.map((v) => [v[1], v[0]])].filter(([, to]) => to !== c.vol)
  const win = document.getElementById('win')
  return `<div class="veil float-veil"${onClick(closeFloat)}${onMenu((ev) => { ev.preventDefault(); closeFloat() })}></div>` +
    `<div class="ctx pop" data-k="tree-menu" style="left:${S.tree.x}px;top:${Math.min(S.tree.y, win.offsetHeight - 240 - (spots.length ? 26 + spots.length * 30 : 0))}px">` +
    menuRow('重命名', () => set({ tree: null, rename: { key, t: vol ? S.volumes.find((v) => v[0] === key)[1] : c.t } })) + menuRow('新建章节', newChapter) + menuRow('新建文件夹', newFolder) + HAIR +
    menuRow('上移', () => shift(key, -1)) + menuRow('下移', () => shift(key, 1)) +
    (spots.length ? '<div class="pop-label">移到</div>' + spots.map(([label, to]) => menuRow(label, () => moveInto(key, to))).join('') : '') + HAIR +
    menuRow('移到回收站', () => toBin(key)) + '</div>'
}

// ───────── 画：正文 ─────────

// 一段切成几块：改写的、选中的、查到的、伏笔、检查出来的。重叠的只留先到的。
function pieces(cur, i, text, focus, found) {
  const cuts = []
  const cut = (quote, para, cls, id) => {
    const at = para === undefined || para === i ? text.indexOf(quote) : -1
    if (at >= 0) cuts.push({ a: at, b: at + quote.length, cls, id })
  }
  const a = S.ai && S.ai.pick && S.ai.pick.para === i ? S.ai : null
  if (a) cuts.push({ a: a.pick.start, b: a.pick.end, cls: a.kind === 'diff' ? 'cut' : 'sel', id: '', then: a.kind === 'diff' ? `<span class="add">${t(a.text)}</span>` : '' })
  if (S.pick && S.pick.para === i) cuts.push({ a: S.pick.start, b: S.pick.end, cls: 'sel', id: '' })
  if (S.flash && S.flash.key === cur.key) cut(S.flash.quote, undefined, 'sel', '')
  found.forEach((h, n) => { if (h.para === i) cuts.push({ a: h.a, b: h.a + S.find.q.length, cls: n === S.find.at ? 'got on' : 'got', id: '' }) })
  for (const m of S.marks) {
    const cls = m.id === focus ? 'fs on' : 'fs'
    if ((m.st === 'open' || m.st === 'done') && m.key === cur.key) cut(m.quote, m.para, cls, m.id)
    if (m.st === 'done' && m.pay && m.pay.key === cur.key) cut(m.pay.quote, m.pay.para, cls, m.id)
  }
  for (const c of S.checks) if (c.key === cur.key) cut(c.quote, c.para, 'doubt', '')
  const kept = []
  for (const c of cuts) if (!kept.some((k) => c.a < k.b && k.a < c.b)) kept.push(c)
  kept.sort((x, y) => x.a - y.a)
  const span = (words, c) => `<span${c && c.id ? ` data-mark="${c.id}"` : ''}${c ? ` class="${c.cls}"` : ''}>${t(words)}</span>` + ((c && c.then) || '')
  let out = '', at = 0
  for (const c of kept) {
    if (c.a > at) out += span(text.slice(at, c.a))
    out += span(text.slice(c.a, c.b), c)
    at = c.b
  }
  return at < text.length ? out + span(text.slice(at)) : out
}

// 标题下面那一行：AI 正在做什么，或者等作者说怎么改。
function aiBar() {
  const a = S.ai, [model, effort] = writer()
  if (!a || a.kind === 'ghost' || a.kind === 'ghosting') return ''
  const drop = () => set({ ai: null })
  if (a.kind === 'ask') return `<div class="ai-bar" data-k="ai-bar">${icon('pen', 'md', 'muted')}<div class="box line fit">${typed(a.want, (v) => set((s) => ({ ai: { ...s.ai, want: v } })), '改写要求，例如：放慢节奏，多写她的犹豫', '改写要求', keys(rewrite, drop) + ' autofocus')}</div>${button('改写', rewrite, 'primary')}<button class="icon-btn muted" aria-label="关闭改写" title="关闭改写"${onClick(drop)}>${icon('cross')}</button></div>`
  if (a.kind === 'rewriting') return `<div class="ai-bar" data-k="ai-bar">${icon('pen', 'md', 'muted')}<div class="fit ell muted">AI 正在改写选中内容 · ${t(model)}</div>${button('取消', drop)}</div>`
  if (a.kind === 'diff') return `<div class="ai-bar" data-k="ai-bar">${icon('pen', 'md', 'green')}<div class="fit ell muted">红色删除线为原文，绿色为改写。</div>${button('放弃', drop, 'soft')}<div class="hit muted"${onClick(rewrite)}>重新改写</div>${button('采纳', takeRewrite, 'primary')}</div>`
  return `<div class="ai-bar" data-k="ai-bar">${icon('leaf', 'md', 'muted')}<div class="fit ell muted">AI 正在按章纲起草 · 已写 ${a.wrote} 字 · ${t(model)} · ${t(effort)}</div>${button('停止', stopDraft, 'stop')}</div>`
}

// 标题下面的「章纲」和「本章摘要」，点开是一个能写字的框。
function chapterMore(cur) {
  const outline = S.outlines[cur.key] || '', sum = S.summaries[cur.key]
  const arrow = (name) => (S.box === name ? ' ▴' : ' ▾')
  const flip = (name) => onClick(() => set({ box: S.box === name ? '' : name }))
  const has = sum && sum.text, old = has && sum.stale, warn = sum && !sum.busy && (old || sum.fail)
  const state = sum && sum.busy ? ' · 正在生成' : !has ? ' · 暂无' : old ? ' · 可能过时' : ''
  const about = sum && sum.busy ? 'AI 正在阅读本章并生成摘要…' : sum && sum.fail ? '上次生成失败：' + sum.fail
    : has ? sum.by + (old ? ' · 正文已修改' : '') : S.recap ? '正文满 200 字后，离开本章时自动生成；也可手动填写。' : '自动生成已关闭，可点击“生成摘要”，也可手动填写。'
  const short = counted(cur) < 200
  let box = ''
  if (S.box === '章纲') box = area(outline, (v) => set((s) => ({ outlines: { ...s.outlines, [cur.key]: v } })), '写下这里要发生的事，作为提纲', '章纲', 3)
  if (S.box === '摘要') {
    box = area(sum ? sum.text : '', (v) => set((s) => ({ summaries: { ...s.summaries, [cur.key]: { text: v, by: '手动填写' } } })), '本章发生的事。起草后续章节时，AI 会参考各章摘要', '摘要', 3) +
      `<div class="box-foot"><div class="fit${warn ? ' amber' : ''}">${t(about)}</div>` +
      `<div class="hit see${short || (sum && sum.busy) ? ' off' : ''}" title="${short ? '正文满 200 字后才需要摘要' : '由 AI 阅读本章并生成摘要'}"${onClick(short || (sum && sum.busy) ? null : () => recap(cur.key))}>${sum && sum.text ? '重新生成' : '生成摘要'}</div></div>`
  }
  return `<div class="chapter-more"><div class="hit${S.box === '章纲' ? ' on' : ''}"${flip('章纲')}>章纲${outline.trim() ? '' : ' · 空'}${arrow('章纲')}</div><div class="hit${S.box === '摘要' ? ' on' : ''}${warn ? ' amber' : ''}"${old ? ' title="摘要可能与正文不一致"' : ''}${flip('摘要')}>本章摘要${state}${arrow('摘要')}</div></div>` +
    (box ? `<div class="chapter-box" data-k="chapter-box">${box}</div>` : '')
}

function findBar(cur) {
  const f = S.find, type = (key) => (v) => set((s) => ({ find: { ...s.find, [key]: v, at: key === 'q' ? 0 : s.find.at } }))
  const here = cur ? hits(cur, f.q) : []
  const had = S.chapters.map((c) => [c, hits(c, f.q)]).filter(([, h]) => h.length)
  const total = had.reduce((sum, [, h]) => sum + h.length, 0)
  const count = !f.q ? '' : f.all ? (total ? '全书 ' + had.length + ' 章、共 ' + total + ' 处' : '全书未找到') : here.length ? f.at + 1 + ' / ' + here.length : '未找到'
  const shut = () => set({ find: null })
  const snip = (c, h) => { const p = c.paras[h.para]; return (h.a > 6 ? '…' : '') + p.slice(Math.max(h.a - 6, 0), h.a + f.q.length + 14) }
  return '<div class="find-bar" data-k="find-bar">' +
    `<div class="find-line"><div class="box line fit">${icon('find', 'sm', 'faint')}${typed(f.q, type('q'), '查找', '查找', keys(() => findStep(1), shut) + ' autofocus')}</div><div class="side find-count">${t(count)}</div>` +
    (f.all ? '' : `<button class="icon-btn muted" aria-label="上一个" title="上一个"${onClick(() => findStep(-1))}>${icon('up')}</button><button class="icon-btn muted" aria-label="下一个" title="下一个"${onClick(() => findStep(1))}>${icon('down')}</button>`) +
    `<div class="chip hit${f.all ? ' on' : ''}" title="在全部章节中查找"${onClick(() => set((s) => ({ find: { ...s.find, all: !s.find.all, at: 0 } })))}>全书</div><button class="icon-btn muted" aria-label="关闭查找" title="关闭查找"${onClick(shut)}>${icon('cross')}</button></div>` +
    `<div class="find-line"><div class="box line fit">${typed(f.to, type('to'), '替换为', '替换为', keys(swapOne, shut))}</div>` +
    (f.all ? button('全书替换', total ? () => set({ dialog: { kind: 'swap', q: f.q, to: f.to, chapters: had.length, total } }) : null, total ? '' : 'off')
      : button('替换', here.length ? swapOne : null, here.length ? '' : 'off') + button('全部替换', here.length ? swapAll : null, here.length ? '' : 'off')) + '</div>' +
    (f.all && total ? `<div class="find-all scroll">${had.map(([c, h]) => `<div class="head">${t(titleOf(c))} · ${h.length} 处</div>` + h.map((x) => `<div class="item row"${onClick(() => set((s) => ({ ...go(s, c.key), find: { ...s.find, all: false, at: hits(c, f.q).indexOf(x) } })))}><div class="about ell">${marked(snip(c, x), f.q)}</div></div>`).join('')).join('')}</div>` : '') +
    '</div>'
}

// 工具栏右边的「待处理 N」点开的那一块。
function todoPop(list) {
  const here = list.filter((x) => x.key === S.ch), shown = S.todoAll ? list : here
  const groups = ['设定不一致', '伏笔', '备份'].map((g) => [g, shown.filter((x) => x.group === g)]).filter(([, of]) => of.length)
  const row = (x) => `<div class="todo-row row"${onClick(x.open)}><div class="fit"><div class="ell">${t(x.name)}</div><div class="note ell">${t(x.about)}${x.key === S.ch ? ' · 本章' : ''}</div></div>${ICON.right}</div>`
  return '<div class="pop todo-pop" data-k="todo-pop">' +
    `<div class="todo-head"><div class="todo-title">待处理</div><div class="grow"></div>${seg(S.todoAll ? '全部 ' + list.length : '本章 ' + here.length, ['全部 ' + list.length, '本章 ' + here.length], (v) => set({ todoAll: v.startsWith('全部') }), '待处理范围')}</div>` +
    `<div class="todo-list scroll">${groups.length ? groups.map(([g, of]) => {
      const all = S.more[g] || of.length <= 3
      return `<div class="todo-group"><div class="dot grey"></div>${g}<span class="faint">${of.length}</span></div>` + (all ? of : of.slice(0, 3)).map(row).join('') +
        (of.length > 3 ? `<div class="todo-rest hit"${onClick(() => set((s) => ({ more: { ...s.more, [g]: !all } })))}>${all ? '收起' : '还有 ' + (of.length - 3) + ' 条'}</div>` : '')
    }).join('') : '<div class="todo-rest">本章没有待处理项</div>'}</div>` +
    `<div class="todo-foot"><div class="hit muted"${onClick(() => set((s) => ({ ignored: [...s.ignored, ...shown.map((x) => x.id)], ...SHUT })))}>全部忽略</div></div></div>`
}

// vol 是开着的那一卷：这时没有开着的章。
// 底栏右边的字数：选中的、本章或本卷的、全书的，设了每日目标还有今天的。点开设目标。
function wordsOf(cur, vol) {
  const inVol = vol && (vol[2] ? vol[2].replace('万', ' 万字') : S.chapters.filter((c) => c.vol === vol[0]).reduce((n, c) => n + counted(c), 0) + ' 字')
  const today = BOOK.days[0][1].toLocaleString('en-US')
  const count = (S.bar ? '选中 ' + S.bar.text.length + ' 字' : cur ? '本章 ' + sizeOf(cur) + ' 字' : vol ? '本卷 ' + inVol : '') + (cur || vol ? ' · ' : '') + '全书 ' + (S.book === BOOK.shelf[0].id ? BOOK.words : S.chapters.reduce((n, c) => n + counted(c), 0) + ' 字') + (S.goal.trim() ? ' · 今天 ' + today + ' / ' + S.goal.trim() : '')
  const goal = () => '<div class="pop goal-pop" data-k="goal-pop"><div class="pop-label">最近 7 天</div>' + BOOK.days.map(([day, n]) => `<div class="goal-day"><div class="grow">${day}</div><div>${n ? '+' + n.toLocaleString('en-US') : '0'} 字</div></div>`).join('') + HAIR +
    field('每日目标', line(S.goal, (v) => set({ goal: v }), '比如 2000', '每日目标字数')) + '<div class="note">设置后，字数旁显示今日字数和目标。留空则不设。</div></div>'
  return `<div class="anchor up"><div class="words ell hit" title="字数。点击设置每日目标"${onClick(() => set({ open: S.open === 'goal' ? '' : 'goal' }))}>${t(count)}</div>${S.open === 'goal' ? goal() : ''}</div>`
}

function toolbar(cur, vol) {
  const here = cur ? S.marks.filter((m) => m.st === 'open' && m.key === cur.key).length : 0
  const wide = !S.paneOpen || S.focus
  const list = pending()
  const tool = (label, title, pic, act, on) => `<button class="icon-btn${on ? ' on' : ''}" aria-label="${label}" title="${title}"${onClick(act)}>${icon(pic)}</button>`
  const flip = (name) => () => set({ open: S.open === name ? '' : name })
  const more = () => {
    const outline = cur && (S.outlines[cur.key] || '').trim()
    return '<div class="pop more-pop" data-k="more-pop">' + menuRow('续写一句', cur ? continueText : null, 'spark', 'Ctrl+J') +
      menuRow('起草本章', outline ? () => set({ dialog: { kind: 'draft', goal: '', how: '接在已有内容之后' }, ...SHUT }) : null, 'page', outline ? '' : '请先填写章纲') + HAIR +
      menuRow('快照', cur ? () => set({ dialog: { kind: 'snaps', id: '', diff: false }, ...SHUT }) : null, 'clock') + menuRow('导出…', () => set({ dialog: { kind: 'export', range: cur ? '本章' : '全书', indent: true }, ...SHUT }), 'save') + HAIR +
      `<div class="pop-label">外观</div><div class="pop-seg">${seg(S.look, [['跟随系统', 'screen'], ['浅色', 'sun'], ['深色', 'moon']], (v) => set({ look: v }), '外观')}</div></div>`
  }
  // 标签页：开着的几章从左到右。没存的那个，关闭的叉换成一个圆点；别的不写「已保存」。
  const on = S.ch || S.vol
  const tab = (key) => {
    const c = chapterOf(key), v = S.volumes.find((x) => x[0] === key), home = c && S.volumes.find((x) => x[0] === c.vol)
    const name = c ? titleOf(c) : v[1], dirty = key === S.ch && !S.saved
    const menu = onMenu((ev) => { ev.preventDefault(); const [x, y] = spot(ev.clientX, ev.clientY); set({ tree: { key, x, y, tab: true } }) })
    return `<div class="sheet hit${key === on ? ' on' : ''}${dirty ? ' dirty' : ''}" data-tab="${key}" title="${t((home ? home[1] + ' › ' : '') + name + (dirty ? ' · 未保存' : ''))}"${onClick(goTo(key))}${menu}${onMid((ev) => { if (ev.button === 1) shutTabs([key]) })}>` +
      `<div class="fit ell">${t(name)}</div><div class="sheet-dot"></div><div class="sheet-x" title="关闭标签页" aria-label="关闭标签页"${onClick(() => shutTabs([key]))}>${icon('cross', 'xs')}</div></div>`
  }
  const tabs = S.tabs.filter((k) => chapterOf(k) || S.volumes.some((x) => x[0] === k))
  return `<div class="toolbar${wide ? ' wide' : ''}${tabs.length ? ' tabbed' : ''}">` +
    (tabs.length ? `<div class="sheets" data-k="sheets">${tabs.map(tab).join('')}</div>` : '<div class="crumb here ell">正文</div>') +
    (S.ai && S.ai.kind === 'ghosting' ? '<div class="count-all">续写中…</div>' : S.ai && S.ai.kind === 'ghost' ? '<div class="count-all">Tab 采纳 · Esc 放弃</div>' : '') + '<div class="grow"></div><div class="tools">' +
    (wide && here && !S.focus ? `<button class="fs-btn" aria-label="本章伏笔"${onClick(() => set({ paneOpen: true, tab: '伏笔', fid: null, fv: 'list' }))}>${icon('flag')}<span>本章伏笔 ${here}</span></button>` : '') +
    (cur ? tool('查找替换', '查找替换（Ctrl+F）', 'find', () => set({ find: S.find ? null : { q: '', to: '', all: false, at: 0 } }), !!S.find) : '') +
    `<div class="anchor">${tool('更多', '续写、起草、快照、导出和外观', 'dots', flip('more'), S.open === 'more')}${S.open === 'more' ? more() : ''}</div>` +
    (S.paneOpen || S.focus ? '' : `<button class="icon-btn" aria-label="展开右栏" title="展开右栏"${onClick(() => set({ paneOpen: true }))}>${svg('icon-md', PANEL + path('M15 5v14'))}</button>`) +
    tool(S.focus ? '退出专注' : '专注写作', S.focus ? '退出专注，恢复章节树和右栏（F11）' : '仅显示正文，收起章节树和右栏（F11）', 'focus', () => set({ focus: !S.focus, ...SHUT }), S.focus) +
    (S.backup.state === 'fail' && !S.backup.hush ? `<div class="anchor"><button class="warn-btn" aria-label="备份提示"${onClick(flip('chip'))}>${DOT}<span>备份失败 · ${backupWhy()[0]}</span></button>${S.open === 'chip' ? backupPop() : ''}</div>` : '') +
    (list.length ? `<div class="anchor"><button class="todo-btn${S.open === 'todo' ? ' on' : ''}" aria-label="待处理"${onClick(flip('todo'))}>${DOT}<span>待处理 ${list.length}</span></button>${S.open === 'todo' ? todoPop(list) : ''}</div>` : '') +
    '</div></div>'
}

// 正文下面那一行：检查到哪了、什么时候查，右边是字数。
function foot(cur, vol) {
  if (!cur) return `<div class="foot"><div class="grow"></div>${wordsOf(null, vol)}</div>`
  const n = S.checks.filter((c) => c.key === cur.key).length, dirty = S.dirty[cur.key], name = cur.vol ? '第' + cur.no + '章' : '本章'
  const seeAll = () => set({ page: 'check', kind: '全部', cid: S.checks.find((c) => c.key === cur.key).id })
  // 这一章查到哪了，和现在能不能点「立即检查」。
  let state = name + '检查完成，未发现不一致', can = false
  if (S.checking === cur.key) state = '正在检查' + name + '…'
  else if (S.queue.includes(cur.key)) state = name + '等待检查'
  else if (S.failed[cur.key]) { state = '检查失败：' + S.failed[cur.key]; can = true }
  else if (!S.entries.some((e) => e.place === 'body')) state = '设定为空，无可对照内容'
  else if (counted(cur) < 200) state = '满 200 字后可检查'
  else if (S.never[cur.key]) { state = name + '尚未检查'; can = true }
  else if (n && dirty) { state = name + '有 ' + n + ' 处待处理，正文已再次修改'; can = true }
  else if (n) state = name + '检查完成，有 ' + n + ' 处待处理'
  else if (dirty) { state = name + '已修改，尚未重新检查'; can = true }
  const found = n && !S.checking && !S.failed[cur.key] && !S.never[cur.key] && counted(cur) >= 200
  const when = [['写完一章后检查', '推荐，每章调用一次 AI'], ['手动检查', '平时不调用 AI'], ['边写边检查', '停笔即检查，调用最多']]
  return '<div class="foot">' + (found ? `<div class="foot-state ell hit ink" title="前往「检查」页逐条查看"${onClick(seeAll)}>${t(state)}</div>` : `<div class="foot-state ell">${t(state)}</div>`) +
    (can ? `<div class="hit ink under" title="调用一次 AI，对照设定检查本章"${onClick(() => checkNow(cur.key))}>立即检查</div>` : '') + '<div class="faint">·</div>' +
    `<div class="anchor up left"><div class="hit" title="检查时机"${onClick(() => set({ open: S.open === 'when' ? '' : 'when' }))}>${t(S.when)} ▴</div>${S.open === 'when' ? `<div class="pop when-pop" data-k="when-pop"><div class="pop-title">检查时机</div>${radios(S.when, when, (v) => set({ when: v, ...SHUT }))}</div>` : ''}</div>` +
    `<div class="grow"></div>${wordsOf(cur)}</div>`
}

// 软件外面动了开着的这一章时，正文上面的那一条。
function outsideBar() {
  if (S.outside === 'changed') return `<div class="notice-bar" data-k="outside">${icon('warn')}<div class="fit">本章文件已被其他程序修改。请选择保留的版本，另一版本将存为快照。</div>${button('使用磁盘版本', () => settleOutside(true))}${button('保留当前版本', () => settleOutside(false), 'primary')}</div>`
  if (S.outside === 'lost') return `<div class="notice-bar lost" data-k="outside">${icon('warn')}<div class="fit">磁盘上已找不到本章文件：在软件外改名、移动或删除，均视为删除。编辑器中的内容仍保留。</div>${button('存为新章节', saveLost, 'primary')}</div>`
  return ''
}

// 点开一卷：卷纲，和它里面的各章。每章带一句章纲，没有章纲的带摘要。
function folderPage([key, title]) {
  const inside = S.chapters.filter((c) => c.vol === key)
  const gist = (c) => (S.outlines[c.key] || '').trim() || (S.summaries[c.key] || {}).text || ''
  return '<div class="paper scroll"><div class="column folder">' +
    `<div class="chapter-title">${t(title)}</div><div class="folder-label">卷纲</div>` +
    area(S.outlines[key] || '', (v) => set((s) => ({ outlines: { ...s.outlines, [key]: v } })), '写下这里要发生的事，作为提纲', '卷纲', 6) +
    `<div class="folder-count"><div class="grow">${inside.length} 项</div><div class="hit ink"${onClick(newChapter)}>在此新建章节</div></div>` +
    inside.map((c) => `<div class="folder-row hit"${onClick(goTo(c.key))}><div class="folder-name">${icon('page', 'md', 'muted')}<div class="fit ell">${t(titleOf(c))}</div><div class="count">${t(sizeOf(c))} 字</div></div>${gist(c) ? `<div class="gist">${t(gist(c))}</div>` : ''}</div>`).join('') +
    '</div></div>'
}

function editor(cur) {
  const vol = S.volumes.find((v) => v[0] === S.vol)
  if (vol) return '<div class="editor">' + toolbar(null, vol) + folderPage(vol) + foot(null, vol) + '</div>'
  if (!cur) {
    return `<div class="editor">${toolbar(null)}<div class="paper">` + (S.chapters.length ? blank('book', '从左侧选择一章开始写作', '')
      : blank('book', '暂无章节', '每章是作品文件夹 chapters 中的一个纯文本文件，可用其他软件打开。', buttons(button('新建第一章', newChapter, 'primary'), button('新建文件夹', newFolder)))) + '</div>' + foot(null) + '</div>'
  }
  const focus = S.paneOpen && S.tab === '伏笔' ? S.fid : null
  const found = S.find && S.find.q && !S.find.all ? hits(cur, S.find.q) : []
  const ghostly = S.ai && S.ai.kind === 'ghost' ? `<span class="ghost-text">${t(S.ai.text)}</span>` : ''
  const paras = cur.paras.map((text, i) => `<div data-para="${i}"${cur.aiFrom !== undefined && i >= cur.aiFrom ? ' class="ai"' : ''}>${pieces(cur, i, text, focus, found)}${i === cur.paras.length - 1 ? ghostly : ''}</div>`).join('')
  return '<div class="editor">' + toolbar(cur) + (S.find ? findBar(cur) : '') + outsideBar() +
    `<div class="paper scroll"${onMenu(openMenu)}${onUp(picked)}><div class="column"><div class="chapter-head"><div class="chapter-title">${t(cur.t)}</div>${chapterMore(cur)}</div>` + aiBar() +
    `<div class="prose">${paras || (ghostly ? `<div>${ghostly}</div>` : '<div class="faint">在此开始写作…</div>')}</div></div></div>` + foot(cur) + '</div>'
}

// ───────── 画：正文上浮着的 ─────────

function floats() {
  if (S.tree) return treeMenu()
  if (!S.ctx && !S.bury && !S.pay && !S.bar) return ''
  const win = document.getElementById('win'), side = document.querySelector('.sidebar'), right = document.getElementById('pane')
  const leftEdge = (side ? side.offsetWidth : 0) + 8
  // 贴着选中的字放：下面放不下就放上面，左右不出正文这一栏。
  const by = (wide, tall, p) => {
    const edge = win.offsetWidth - (right ? right.offsetWidth : 0)
    const y = p.bottom + 6 + tall > win.offsetHeight - 8 && p.top - tall - 6 >= 44 ? p.top - tall - 6 : p.bottom + 6
    return `left:${Math.max(leftEdge, Math.min(p.left, edge - wide - 8))}px;top:${y}px`
  }
  const veiled = `<div class="veil float-veil"${onClick(closeFloat)}${onMenu((ev) => { ev.preventDefault(); closeFloat() })}></div>`

  if (S.ctx) {
    const p = S.pick, name = p ? p.text.trim() : ''
    // 选中的是一个短名字，就可以直接记进设定里管名字的几类。
    const addName = (cat) => () => {
      set((s) => ({ ctx: null, pick: null, entries: [...s.entries, { id: 'e' + s.n, place: 'body', cat, names: [name], text: name }], n: s.n + 1, log: did(s, '新增到正文 · ' + cat + '：' + name) }))
      unselect()
      say('「' + name + '」已加入设定的' + cat + '，输入时可自动补全')
    }
    const names = !name || name.length > 12 ? menuRow('加入名字表', null)
      : S.entries.some((e) => e.place !== 'dropped' && e.names.includes(name)) ? menuRow('已在名字表中', null)
      : `<div class="anchor wide"><div class="pop-row row${S.ctx.names ? ' on' : ''}"${onClick(() => set((s) => ({ ctx: { ...s.ctx, names: true } })))}><div class="grow">加入名字表</div>${ICON.right}</div>` +
        (S.ctx.names ? `<div class="pop side-pop" data-k="ctx-names">${S.cats.filter((c) => BOOK.nameCats.includes(c)).map((c) => menuRow(c, addName(c))).join('')}</div>` : '') + '</div>'
    // 剪贴板的四项：复制和全选是真的；原型的正文不能改，剪切和粘贴只提示一句。
    const fixed = () => { set({ ctx: null, pick: null }); say('原型的正文不能修改', '软件中此步骤会改动正文') }
    const copy = () => { if (navigator.clipboard) navigator.clipboard.writeText(p.text).catch(() => {}); set({ ctx: null, pick: null }) }
    const all = () => { set({ ctx: null, pick: null }); const first = document.querySelector('[data-para]'); if (first) getSelection().selectAllChildren(first.parentElement) }
    // 指针移到「加入名字表」上，旁边那一列就打开；移到别的行上，它收起。
    const over = (ev) => {
      const row = ev.target.closest('.pop-row'), on = !!ev.target.closest('.anchor')
      if (row && on !== !!S.ctx.names) set((s) => ({ ctx: { ...s.ctx, names: on } }))
    }
    // 菜单宽 200，最多十二行，都留在窗口里；旁边再开一列时也不出窗口。
    return veiled + `<div class="ctx pop${S.ctx.x > win.offsetWidth - 208 - 130 ? ' flip' : ''}" data-k="ctx"${onOver(over)} style="left:${Math.min(S.ctx.x, win.offsetWidth - 208)}px;top:${Math.min(S.ctx.y, win.offsetHeight - 430)}px">` +
      menuRow('剪切', p ? fixed : null, '', 'Ctrl+X') + menuRow('复制', p ? copy : null, '', 'Ctrl+C') + menuRow('粘贴', fixed, '', 'Ctrl+V') + menuRow('全选', all, '', 'Ctrl+A') + HAIR +
      menuRow('续写一句', continueText, '', 'Ctrl+J') + menuRow('AI 改写…', p ? askRewrite : null) + menuRow('引用到讨论', p ? cite : null, '', 'Ctrl+L') + menuRow('加入设定', p ? take : null) + names + HAIR +
      menuRow('记为伏笔…', p ? () => set({ ctx: null, bury: { note: '' } }) : null) + menuRow('记为回收…', p ? () => askPay('') : null) +
      (S.ctx.mark ? menuRow('查看伏笔', () => set({ ctx: null, pick: null, tab: '伏笔', fid: S.ctx.mark, fv: 'list', renote: null, paneOpen: true, focus: false, ...SHUT })) : '') + '</div>'
  }

  if (S.bury) {
    return veiled + `<div class="float bury" data-k="bury" style="${by(360, 132, S.pick)}"><div class="float-head">${icon('flag', 'sm')}记为伏笔，填写备注</div>` +
      `<div class="bury-box">${typed(S.bury.note, (v) => set({ bury: { note: v } }), '例如：小祥为什么突然改口', '伏笔备注', keys(buryNow, () => set({ bury: null, pick: null })) + ' autofocus')}</div>` +
      `<div class="float-end"><div class="ghost hit"${onClick(closeFloat)}>取消</div><div class="solid hit"${onClick(buryNow)}>确定</div></div></div>`
  }

  if (S.pay) {
    const q = S.pay.q.trim()
    const can = payable(), id = payPick()
    const meta = (m) => (m.pay ? '第' + m.ch + '章埋下，第' + m.pay.ch + '章回收' : '第' + m.ch + '章 · ' + (BOOK.now - m.ch > 0 ? BOOK.now - m.ch + ' 章前埋下' : '本章埋下'))
    return veiled + `<div class="float pay" data-k="pay" style="${by(380, 340, S.pick)}"><div class="float-head">${icon('link', 'sm')}选择要回收的伏笔</div>` +
      `<div class="pay-find">${typed(S.pay.q, (v) => set({ pay: { q: v, id } }), '搜索备注或原句', '查找伏笔', keys(() => payNow(id), () => set({ pay: null, pick: null })) + ' autofocus')}</div>` +
      (can.length ? '' : `<div class="pay-none">${q ? '没有匹配的伏笔' : '暂无已埋下的伏笔。请先选中一句，右键选择「记为伏笔」。'}</div>`) +
      `<div class="pay-list scroll">${can.map((m) => `<div class="pay-row row${m.id === id ? ' on' : ''}"${onClick(() => set((s) => ({ pay: { ...s.pay, id: m.id } })))}><div class="pay-name ell">${t(nameOf(m))}</div><div class="pay-meta">${meta(m)}</div></div>`).join('')}</div>` +
      `<div class="float-end"><div class="ghost hit"${onClick(closeFloat)}>取消</div><div class="solid hit${id ? '' : ' off'}" title="确定回收"${onClick(() => payNow(id))}>记为回收</div></div></div>`
  }

  // 选中一段字以后：浮在它上面的两个按钮。
  return `<div class="selbar pop" data-k="selbar" style="left:${Math.max(leftEdge, S.bar.left)}px;top:${Math.max(S.bar.top - 40, 44)}px"><div class="hit" title="由 AI 改写选中内容"${onClick(askRewrite)}>${icon('pen', 'sm')}AI 改写</div><div class="hit" title="将选中的原文加入设定"${onClick(take)}>${icon('keep', 'sm')}加入设定</div></div>`
}
