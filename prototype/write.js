// 写作那一页的左边和中间：侧栏、工具栏、正文，和正文上浮着的东西。
'use strict'

// ───────── 动作：章节 ─────────

const CLEAR = { ctx: null, pick: null, bury: null, pay: null, bar: null, ai: null, box: '', tree: null, rename: null }

// 打开一章，或一卷的卷纲：已经有它的标签页就切过去，没有就在正看的那个右边开一个。它所在的卷展开，上一章上面浮着的东西收掉，记进打开过的里面。
// 换到别的章之前，没存的字先存了。
function go(s, key, flash) {
  const ch = s.chapters.find((c) => c.key === key), vol = !ch && s.volumes.some((v) => v[0] === key)
  if (!ch && !vol) return null // 已经删了
  const trail = s.trail[s.step] === key ? s.trail : [...s.trail.slice(0, s.step + 1), key]
  const at = s.tabs.indexOf(s.ch || s.vol), tabs = s.tabs.includes(key) ? s.tabs : [...s.tabs.slice(0, at + 1), key, ...s.tabs.slice(at + 1)]
  return { page: 'write', ch: ch ? key : '', vol: vol ? key : '', tabs, trail, step: s.trail === trail ? s.step : trail.length - 1, folds: { ...s.folds, [ch ? ch.vol : key]: false }, ...CLEAR, outside: '', flash: flash || null, ...(key === (s.ch || s.vol) ? {} : { saved: true }) }
}
// 关掉几个标签页。关的是正看的那个，就换到它右边的，没有就左边的；一个不剩时正文空着。
// 正看的那一章还有没存的字，先问存不存（sure 是已经问过了）。
const shutTabs = (gone, sure) => set((s) => {
  const tabs = s.tabs.filter((k) => !gone.includes(k)), on = s.ch || s.vol, at = s.tabs.indexOf(on)
  if (!sure && s.ch && !s.saved && gone.includes(on)) return { dialog: { kind: 'unsaved', gone }, tree: null }
  if (!gone.includes(on)) return { tabs, tree: null }
  const next = s.tabs.slice(at + 1).find((k) => tabs.includes(k)) || s.tabs.slice(0, at).reverse().find((k) => tabs.includes(k))
  return next ? { ...go({ ...s, tabs }, next), saved: true } : { tabs, ch: '', vol: '', step: s.trail.length, ...CLEAR, outside: '', saved: true } // step 停在最后一个之后：‹ 回到刚关掉的那一章
})
const goTo = (key, flash) => () => set((s) => go(s, key, flash))
// 顶栏左边的 ‹ ›：回到上一个、下一个打开过的章。
const stepBy = (d) => set((s) => {
  const to = go(s, s.trail[s.step + d])
  return to ? { ...to, trail: s.trail, step: s.step + d } : null
})

// 改一章的字：change 拿到原来的几段，交回新的几段。字数跟着变；检查过的结果从此算旧的，也还没存；撤销记录作废（要留一步好退回的用 oneStep）。
const retype = (s, key, change) => {
  const typed = (c) => { const paras = change(c.paras); return { ...c, paras, c: c.c && sizeOf(c) + paras.join('').length - c.paras.join('').length } }
  return { chapters: s.chapters.map((c) => (c.key === key ? typed(c) : c)), dirty: { ...s.dirty, [key]: true }, saved: false, undo: { ...s.undo, [key]: null } }
}
// 改之前先存一份快照。

function newChapter(into) {
  set((s) => {
    // 开着一章，就建在它后面；开着一卷的卷纲，或者是在树里一卷上右键建的（into 是那一卷），就建在这一卷的末尾。
    const home = typeof into === 'string' ? into : chapterOf(s.ch) ? '' : s.vol
    const cur = home ? s.chapters.filter((c) => c.vol === home).at(-1) : chapterOf(s.ch), key = 'n' + s.n
    // 卷里的章接着上一章的号：名字先空着，标题就是「第 N 章」，输入框里号后面等着写名字。卷外的叫「新章节」，全选上等着换。
    const vol = home || (cur ? cur.vol : ''), made = { key, no: cur ? cur.no + 1 : 1, vol, t: vol ? '' : L.sideNewChapterName(), c: '', paras: [] }
    const at = cur ? s.chapters.indexOf(cur) + 1 : s.chapters.length
    const chapters = [...s.chapters.slice(0, at), made, ...s.chapters.slice(at)]
    return { chapters, n: s.n + 1, never: { ...s.never, [key]: true }, ...go({ ...s, chapters }, key), rename: { key, t: vol ? titleOf(made) + '：' : made.t, all: !vol, offered: vol ? titleOf(made) + '：' : '' } }
  })
}
function newFolder() {
  set((s) => ({ volumes: [...s.volumes, ['v' + s.n, L.sideNewFolderName(), '']], n: s.n + 1, tree: null, rename: { key: 'v' + s.n, t: L.sideNewFolderName(), all: true } }))
}
// F2：给开着的这一章改名字。它在折起来的卷里、侧栏收着，都先摆出来。
function renameOpen() {
  set((s) => { const c = chapterOf(s.ch); return { ...CLEAR, ...SHUT, focus: false, folds: c.vol ? { ...s.folds, [c.vol]: false } : s.folds, rename: { key: c.key, t: titleOf(c), all: true } } })
}
// 名字定下来：按了回车，或者光标离开了输入框。空着就还是原来的。写了什么，标题就是什么：
// - 卷里的章写成「第 12 章：雨停」，号和名字拆开记，后面的章接着这个号；
// - 写的不是这个样子（「第一章：雨停」「楔子」），整个记成标题，前面不再加号；
// - 新建时框里给的「第 12 章：」后面又写了一个带号的标题，以后写的那个为准。
function renameNow() {
  set((s) => {
    if (!s.rename) return null
    const { key, t: title, offered } = s.rename, typed = title.trim().replace(/[：:]$/, '')
    const rest = offered && typed.startsWith(offered) ? typed.slice(offered.length).trim() : ''
    const name = /^第\s*[0-9零〇一二两三四五六七八九十百千]+\s*[章回节]/.test(rest) ? rest : typed
    if (!name) return { rename: null }
    const named = (c) => { const m = c.vol && /^第\s*(\d+)\s*章\s*[：:]?\s*(.*)$/.exec(name); return m ? { ...c, no: Number(m[1]), t: m[2], own: false } : { ...c, t: name, own: !!c.vol } }
    return { rename: null, chapters: s.chapters.map((c) => (c.key === key ? named(c) : c)), volumes: s.volumes.map((v) => (v[0] === key ? [key, name, v[2]] : v)) }
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
      bin: [{ id: 'b' + s.n, title, at: stamp(), volume: vol || null, chapters: gone }, ...s.bin],
      tabs: !to || left.includes(to) ? left : [...left, to],
      ...(to === on ? {} : chapters.some((c) => c.key === to) ? { ch: to, vol: '' } : { ch: '', vol: to, ...CLEAR }),
    }
  })
  say(L.sideMovedToTrash(title), '', 'ok')
}

// ───────── 动作：在正文里打字 ─────────
// 打字、删字由浏览器先改页面，这里把页面上的几段读回来；回车、粘贴、菜单里的剪切和粘贴自己算。原型不存盘，刷新就回到原样。

let typedAt = 0 // 上一回打字是什么时候：紧接着打的并成一步撤销

// 能打字的那一块正文；AI 正在续写、等着写改写要求的时候没有。
const proseBox = () => document.querySelector('.prose[contenteditable]')
// 正文里 AI 提议的字（绿的，和它后面的按钮）不是正文：读字、放光标都跳过它们。
const proposed = (n) => !!(n.parentElement && n.parentElement.closest('[data-prop]'))
const textOf = (el) => { let out = ''; const walk = document.createTreeWalker(el, NodeFilter.SHOW_TEXT); for (let n = walk.nextNode(); n; n = walk.nextNode()) if (!proposed(n)) out += n.data; return out }
const blocks = (box) => [...box.children].filter((e) => e.nodeName !== 'BR' && !e.hasAttribute('data-prop'))
const plain = (text) => text.replace(/\r\n?/g, '\n').replace(/\n+$/, '')
// 两串字（或两组段）开头有几个一样，除此以外结尾有几个一样。
function shared(a, b) {
  let p = 0, q = 0
  while (p < a.length && p < b.length && a[p] === b[p]) p++
  while (q < a.length - p && q < b.length - p && a[a.length - 1 - q] === b[b.length - 1 - q]) q++
  return [p, q]
}
// node 里的第 offset 个位置，是第几段的第几个字后面。
function placeOf(box, node, offset) {
  const all = blocks(box)
  const end = { para: Math.max(all.length - 1, 0), off: all.length ? textOf(all.at(-1)).length : 0 }
  if (node === box) return offset < all.length ? { para: offset, off: 0 } : end
  let top = node
  while (top.parentNode !== box) top = top.parentNode
  if (!all.includes(top)) return end // 落在接在章末的提议上
  const r = document.createRange()
  r.selectNodeContents(top)
  r.setEnd(node, offset)
  // 前面整个提议的字不算。
  let off = r.toString().length
  for (const el of top.querySelectorAll('[data-prop]')) { const x = document.createRange(); x.selectNodeContents(el); if (r.compareBoundaryPoints(Range.END_TO_END, x) >= 0) off -= el.textContent.length }
  return { para: Math.max(all.indexOf(top), 0), off }
}
// 选中的字的两头；没选中就是光标，两头一样。光标不在正文里时没有。box 是哪一块正文，导入书籍那一页给它自己的。
function spanNow(box = proseBox()) {
  const sel = getSelection()
  if (!box || !sel.rangeCount || !box.contains(sel.anchorNode) || !box.contains(sel.focusNode)) return null
  const r = sel.getRangeAt(0)
  return [placeOf(box, r.startContainer, r.startOffset), placeOf(box, r.endContainer, r.endOffset)]
}
// 把光标放到第几段的第几个字后面；show 是顺便把那一段挪到看得见的地方。
function putCaret({ para, off }, show, box = proseBox()) {
  const div = box && blocks(box)[para]
  if (!div) return
  const walk = document.createTreeWalker(div, NodeFilter.SHOW_TEXT), r = document.createRange()
  r.setStart(div, 0)
  let left = off
  for (let n = walk.nextNode(); n; n = walk.nextNode()) {
    if (proposed(n)) continue
    r.setStart(n, Math.min(left, n.data.length))
    if (left <= n.data.length) break
    left -= n.data.length
  }
  r.collapse(true)
  box.focus({ preventScroll: true })
  const sel = getSelection()
  sel.removeAllRanges()
  sel.addRange(r)
  if (show) div.scrollIntoView({ block: 'nearest' })
}

// 正文换成 paras：伏笔、检查出来的句子在第几段，跟着改。
function retext(s, paras) {
  const key = s.ch, old = s.chapters.find((c) => c.key === key).paras, [p, q] = shared(old, paras)
  // 前面没动的几段不用管，后面没动的几段跟着挪；中间改了的，看那句话现在落在哪一段。
  const seat = (x) => {
    if (x.key !== key || x.para === undefined || x.para < p) return x
    if (x.para >= old.length - q) return { ...x, para: x.para + paras.length - old.length }
    const to = paras.findIndex((text, i) => i >= p && i < paras.length - q && text.includes(x.quote))
    return to < 0 ? x : { ...x, para: to }
  }
  return { ...retype(s, key, () => paras), marks: s.marks.map((m) => (m.pay ? { ...seat(m), pay: seat(m.pay) } : seat(m))), checks: s.checks.map(seat), bar: null, pick: null, flash: null, ...unfit(s, key, old, paras) }
}
// 作者下的一步改动（替换、采纳续写或改写、起草）：改之前的样子记下来，撤销一次就退回去。
const oneStep = (s, key, change) => {
  const u = s.undo[key] || { back: [], fwd: [] }, old = s.chapters.find((c) => c.key === key).paras
  return { ...retype(s, key, change), undo: { ...s.undo, [key]: { back: [...u.back, old], fwd: [] } } }
}
// 作者改了正文：改之前的样子记下来好撤销。typing 是一个字一个字打的，紧接着的并成一步。
function typeOver(s, paras, typing) {
  const key = s.ch, u = s.undo[key] || { back: [], fwd: [] }, old = s.chapters.find((c) => c.key === key).paras
  const joined = typing && u.back.length && Date.now() - typedAt < 1000
  typedAt = typing ? Date.now() : 0
  return { ...retext(s, paras), undo: { ...s.undo, [key]: { back: joined ? u.back : [...u.back, old], fwd: [] } } }
}
// 撤销（d 是 -1）和重做（1）。光标放到改回来的那一处后面。
function unwrite(d) {
  const key = S.ch, u = S.undo[key], [from, to] = d < 0 ? ['back', 'fwd'] : ['fwd', 'back']
  if (!u || !u[from].length || !proseBox()) return
  const now = chapterOf(key).paras, paras = u[from].at(-1)
  typedAt = 0
  set((s) => ({ ...retext(s, paras), undo: { ...s.undo, [key]: { [from]: u[from].slice(0, -1), [to]: [...u[to], now] } } }))
  const q = shared(now, paras)[1], i = paras.length - 1 - q, a = paras[i] || '', same = shared(a, now[now.length - 1 - q] || '')[1]
  putCaret({ para: Math.max(i, 0), off: i < 0 ? 0 : a.length - same }, true)
}

// 浏览器替作者改了页面上的字：读回来。拼音还没选完字时先不读。
function wrote() {
  const box = proseBox(), cur = chapterOf(S.ch)
  if (!box || !cur || composing) return
  const at = spanNow(), read = blocks(box).map((e) => textOf(e).replace(/\n/g, ''))
  const paras = !read.length ? [box.textContent].filter(Boolean) : read.length === 1 && !read[0] ? [] : read
  if (paras.join('\n') === cur.paras.join('\n') && paras.length === cur.paras.length) return
  set((s) => typeOver(s, paras, true))
  if (at) putCaret(at[1])
}
// 把选中的换成 text，没选中就放在光标那里；text 里的换行另起一段。span 是事先记下的两头，没给就看现在的。
function putText(text, span = spanNow()) {
  const cur = chapterOf(S.ch)
  if (!span || !cur || !proseBox()) return
  const [a, b] = span, old = cur.paras.length ? cur.paras : [''], lines = text.split('\n')
  const paras = [...old.slice(0, a.para), ...((old[a.para] || '').slice(0, a.off) + text + (old[b.para] || '').slice(b.off)).split('\n'), ...old.slice(b.para + 1)]
  set((s) => typeOver(s, paras.length === 1 && !paras[0] ? [] : paras, false))
  putCaret({ para: a.para + lines.length - 1, off: (lines.length > 1 ? 0 : a.off) + lines.at(-1).length }, true)
}
// 在正文里按键：回车另起一段，Ctrl+Z 撤销，Ctrl+Y、Ctrl+Shift+Z 重做，Tab 不让光标跑出去。
function proseKey(ev) {
  const ctrl = ev.ctrlKey || ev.metaKey, k = ev.key.toLowerCase()
  const run = (act) => { ev.preventDefault(); act() }
  if (ev.isComposing) return
  if (ev.key === 'Enter') run(() => putText('\n'))
  else if (ev.key === 'Tab' && !S.ai) run(() => {})
  else if (ctrl && !ev.shiftKey && k === 'z') run(() => unwrite(-1))
  else if (ctrl && (k === 'y' || (ev.shiftKey && k === 'z'))) run(() => unwrite(1))
}
// 粘贴只要字，不要别处带来的样式。
function pasted(ev) {
  ev.preventDefault()
  putText(plain(ev.clipboardData.getData('text/plain')))
}

// ───────── 动作：正文里选中的字 ─────────

// 屏幕上的一点，换成从窗口左上角算起的位置。
function spot(x, y) {
  const box = document.getElementById('win').getBoundingClientRect(), z = winZoom()
  return [(x - box.left) / z, (y - box.top) / z]
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
  return { para: Number(para.dataset.para), start, end: start + text.length, text, left, top: y, bottom: y + r.height / winZoom() }
}
const unselect = () => getSelection().removeAllRanges()

// 松开鼠标时：选中了字，就在它上面浮出「AI 改写」「加入设定」（AI 关着时只有后一个）。
function picked(ev) {
  if (ev.button !== 0 || S.ai) return
  const pick = grab()
  if (pick || S.bar) set({ bar: pick })
}

function openMenu(ev) {
  ev.preventDefault()
  const on = ev.target.closest('[data-mark]')
  const [x, y] = spot(ev.clientX, ev.clientY)
  set({ ctx: { x, y, mark: on ? on.dataset.mark : '', span: spanNow() }, pick: grab(), bar: null, bury: null, pay: null, flash: null }) // span：菜单一开，选中的字换了画法，光标在哪要先记下
}

const closeFloat = () => set({ ctx: null, bury: null, pay: null, pick: null, tree: null })

// ───────── 动作：AI 在正文里写 ─────────

// Ctrl+S：马上存。AI 写的还没采纳，不在正文里，不用等它。
function saveNow() {
  set({ saved: true })
  say(L.statusSaved(), L.statusAutosave(), 'ok')
}

// 软件外面改了开着的这一章：留磁盘上的还是编辑器里的，另一边存成快照。
function settleOutside(disk) {
  const key = S.ch
  set((s) => ({ outside: '', ...snap(s, key, '外部改动'), ...(disk ? retype(s, key, (paras) => [...paras, BOOK.outside]) : {}) }))
  say(disk ? L.editorSettledDisk() : L.editorSettledKeep(), '', 'ok')
}
// 软件外面把开着的这一章删了、改名了：编辑器里的字存成一章新的。
function saveLost() {
  set((s) => {
    const c = chapterOf(s.ch)
    return { outside: '', chapters: [...s.chapters.filter((x) => x !== c), { ...c, vol: '' }] }
  })
  say(L.editorSavedNewTop(), '', 'ok')
}

// 让备份换一种没成功的样子。

// 平时不易遇到的情况：出错、作品在软件外被改动。在窗口上面那一排里点出来看。
function rare() {
  const here = (change) => () => {
    if (!chapterOf(S.ch)) { set(SHUT); say(L.editorOpenFirst()); return }
    set((s) => ({ page: 'write', focus: false, dialog: null, find: null, ...CLEAR, ...SHUT, ...change(s) }))
  }
  return [
    ['本章在软件外被修改', here(() => ({ outside: 'changed' }))],
    ['本章文件丢失', here(() => ({ outside: 'lost' }))],
    ...(aiOn() ? aiRare(here) : []),
    ['备份文件夹丢失', failBackup('dir')],
    ['备份文件夹不可写', failBackup('deny')],
    [S.narrow ? '窗口恢复原宽' : '窗口变窄', () => set((s) => ({ narrow: !s.narrow, ...SHUT }))],
    ['大纲文件读不出来', () => set({ page: 'plan', planBroken: true, planQ: '', dialog: null, ...SHUT })],
    ['添加的文件夹中没有作品', () => { set(SHUT); say(L.editorNoWork(), PATHS().stray, 'fail') }],
    // 扩展出错只停它自己：提示条写名字和错误原文，出在哪个回调记在扩展的账里。
    ['扩展出错', () => { set((s) => ({ ...SHUT, extErrors: { ...s.extErrors, phone: [{ at: stamp(), where: 'draw', message: 'draw failed' }, ...(s.extErrors.phone || [])] } })); say(L.extFailed(L.modPhone(), 'draw failed'), '', 'fail') }],
    ['另装的扩展起冲突', () => set({ rivals: true, dialog: { kind: 'settings', tab: '扩展', ext: 'vertical', edit: '' }, ...SHUT })],
    ['扩展设置读不出来', () => { set(SHUT); say(L.extFileUnreadable("invalid character '}' in ext.json"), '', 'fail') }],
  ]
}
// 出错的原因在状态里记的是中文（和它判断用的一样），显示时换成当前语言的字。
const WHYS = () => ({ '无法连接模型服务': L.editorWhyNoModel(), '请在设置中「AI」配置里选择续写和检查模型': L.editorWhyPickModel() })
const whyText = (v) => WHYS()[v] || v
// AI 出错的那几种：AI 关着时用不上，菜单里也不列。
function aiRare(here) {
  const why = '无法连接模型服务'
  return [
    ['本章检查失败', here((s) => ({ failed: { ...s.failed, [s.ch]: why } }))],
    ['未选择续写和检查模型', here((s) => ({ failed: { ...s.failed, [s.ch]: '请在设置中「AI」配置里选择续写和检查模型' } }))],
    ['本章过长，仅检查前文', here((s) => ({ part: { ...s.part, [s.ch]: { check: 5000 } }, never: { ...s.never, [s.ch]: false }, failed: { ...s.failed, [s.ch]: '' }, dirty: { ...s.dirty, [s.ch]: false } }))],
    ['讨论里提议修改另一章', () => { set(SHUT); elsewhere() }],
    ['回复超出最大输出', () => { set(SHUT); say(L.editorOutputTooLong(), L.editorOutputTooLongHint(), 'fail', [[L.commonOpenSettings(), () => set({ dialog: { kind: 'settings', tab: 'AI', edit: '' }, toasts: [], ...SHUT })]]) }],
    ['讨论查阅达到上限', here((s) => ({ paneOpen: true, tab: '讨论', tv: 'one', tid: 't1', talks: s.talks.map((talk) => (talk.id !== 't1' ? talk : { ...talk, msgs: [...talk.msgs, ...capped()] })) }))],
    ['设定较多，只附目录', here(() => ({ paneOpen: true, tab: '讨论', tv: 'one', tid: 't2', crowded: true, told: { 't2-1': true } }))],
    ['模型目录读取失败', () => { set(SHUT); const id = say(L.editorModelListFailed(), whyText(why), 'fail', [[L.commonRetry(), () => { unsay(id); fetchModels() }], [L.commonOpenSettings(), () => set({ dialog: { kind: 'settings', tab: 'AI', edit: '' }, toasts: [], ...SHUT })]]) }],
  ]
}

// ───────── 动作：查找替换、检查、待处理 ─────────

// 待处理里的每一条：哪一组、写什么、在哪一章、点了去哪。忽略掉的不算；哪个扩展关着（检查、伏笔、备份），它的那几条也不算。
function pending() {
  return contributions('PendingGroups').flatMap((of) => of()).filter((x) => !S.ignored.includes(x.id))
}

// ───────── 画：侧栏 ─────────

// 侧栏只列东西：顶上一行是作品名和新建，下面是章节树；讨论页列的是讨论。换页在标题栏正中，回收站和设置在「文件」菜单里，备份在底栏。
// 树里一行只有名字：卷带折起、展开的箭头，章没有图标；字数指针移上去才出来。
function sidebar() {
  // 改名字的那一行还是那一行：图标、缩进都不动，只是名字换成输入框。回车和光标离开都算定了，Esc 不改。
  // 回车定的是正开着的那一章，光标接着进正文。
  const named = (key) => () => { renameNow(); const box = key === S.ch && proseBox(); if (box) box.focus() }
  const naming = (key, pic, deep) => `<div class="tree-row naming${pic ? '' : ' leaf'}${deep ? ' deep' : ''}${key === S.ch ? ' on' : ''}" data-k="node-${key}">${pic}` +
    `${typed(S.rename.t, (v) => set((s) => (s.rename ? { rename: { ...s.rename, t: v } } : null)), '', L.sideChapterTitle(), keys(named(key), () => set({ rename: null })) + onBlur(renameNow) + ' autofocus' + (S.rename.all ? ' data-all' : ''))}</div>`
  const menu = (key, talk) => onMenu((ev) => { ev.preventDefault(); const [x, y] = spot(ev.clientX, ev.clientY); set({ tree: { key, x, y, talk } }) })
  const leaf = (c) => (S.rename && S.rename.key === c.key ? naming(c.key, '', c.vol)
    : `<div class="tree-row leaf hit${c.vol ? ' deep' : ''}${c.key === S.ch ? ' on' : ''}" draggable="true" data-k="node-${c.key}" data-node="${c.key}"${onClick(goTo(c.key))}${menu(c.key)}><div class="fit ell">${t(titleOf(c))}</div><div class="count">${t(L.commonNumber(sizeOf(c)))}</div></div>`)
  const volume = ([key, title, count]) => (S.rename && S.rename.key === key ? naming(key, icon(S.folds[key] ? 'right' : 'down', 'xs'))
    : `<div class="tree-row hit${key === S.vol ? ' on' : ''}" data-k="node-${key}" data-node="${key}"${onClick(() => set((s) => ({ folds: { ...s.folds, [key]: !s.folds[key] } })))}${menu(key)}>${icon(S.folds[key] ? 'right' : 'down', 'xs')}<div class="fit ell">${t(title)}</div><div class="count">${t(count)}</div></div>`) +
    (S.folds[key] ? '' : listed.filter((c) => c.vol === key).map(leaf).join(''))
  const listed = S.chapters.filter((c) => S.outside !== 'lost' || c.key !== S.ch) // 磁盘上找不到的那一章，树里也没有
  const book = S.shelf.find((b) => b.id === S.book)
  const others = S.shelf.filter((b) => b.id !== S.book && !b.gone)
  const open = (talk) => S.page === 'talk' && talk.id === S.tid && S.tv === 'one'
  const put = S.talks.filter((x) => S.shelved[x.id])
  const talkRow = (talk) => `<div class="tree-row hit${open(talk) ? ' on' : ''}${S.shelved[talk.id] ? ' put' : ''}"${onClick(() => set({ page: 'talk', tv: 'one', tid: talk.id, draft: '' }))}${menu(talk.id, true)}><div class="fit ell">${t(talk.title)}</div><div class="count">${t(timeShown(talk.at))}</div></div>`
  // 点作品名出来的菜单：这一部在最上，再是别的几部。
  const bookMenu = S.open !== 'book' ? '' : `<div class="pop"><div class="pop-label">${L.sideBooks()}</div><div class="pop-row row on"><div class="grow">${t(book.name)}</div><div class="side">${t(genreWord(book.kind))}</div></div>` + others.map((o) => menuRow(o.name, leave(() => openBook(o.id)), '', genreWord(o.kind))).join('') + HAIR +
    menuRow(L.sideNewBook(), leave(() => set({ page: 'shelf', dialog: { kind: 'newBook', name: '', type: BOOK.kinds[0] }, ...SHUT }))) + menuRow(L.sideAllBooks(), leave(() => set({ page: 'shelf', ...SHUT }))) + '</div>'
  // AI 正在回讨论时不能换作品，别的照常：回完或者取消了再换。
  const pickBook = onClick(S.waiting ? null : () => set({ open: S.open === 'book' ? '' : 'book' })), off = S.waiting ? ' off' : ''
  const talks = S.page === 'talk' // 讨论页列讨论，别的页都列章节
  // 顶上一行右边只一个加号：列章节时新建章节，列讨论时新对话。新建文件夹在「文件」菜单和右键菜单里。
  const add = talks ? `<button class="icon-btn muted" aria-label="${L.sideNewTalk()}" title="${L.sideNewTalk()}"${onClick(() => set({ page: 'talk', tv: 'new', tid: null, draft: '' }))}>${ICON.plus}</button>`
    : `<button class="icon-btn muted" aria-label="${L.cmdNewChapter()}" title="${L.sideNewChapterKey(keyLabel('Ctrl+N'))}"${onClick(newChapter)}>${ICON.plus}</button>`
  return '<div class="sidebar" data-k="sidebar">' +
    `<div class="sidebar-top"><div class="anchor wide"><div class="book hit${off}" title="${L.sideSwitchBook()}"${pickBook}><div class="book-logo">${t(book.name[0])}</div><div class="book-name">${t(book.name)}</div>${icon('down', 'sm', 'faint')}</div>` +
    bookMenu + `</div>${add}</div>` +
    (talks
      ? `<div class="tree scroll">${S.talks.filter((x) => !S.shelved[x.id] || S.shelvedShown || open(x)).map(talkRow).join('')}` +
        (put.length ? `<div class="tree-more hit"${onClick(() => set({ shelvedShown: !S.shelvedShown }))}>${S.shelvedShown ? L.sideHideFolded() : L.sideFoldedCount(put.length)}</div>` : '') + '</div>'
      : `<div class="tree scroll">${S.volumes.map(volume).join('')}${listed.filter((c) => !c.vol).map(leaf).join('')}</div>`) +
    '</div>'
}

// 侧栏里右键一章或一卷点出来的菜单；一段讨论、一个标签页的菜单也在这里。
function treeMenu() {
  if (S.tree.tab) {
    const key = S.tree.key, others = S.tabs.filter((k) => k !== key), right = S.tabs.slice(S.tabs.indexOf(key) + 1)
    return `<div class="veil float-veil"${onClick(closeFloat)}${onMenu((ev) => { ev.preventDefault(); closeFloat() })}></div>` +
      `<div class="ctx pop" data-k="tree-menu" style="left:${Math.min(S.tree.x, document.getElementById('win').offsetWidth - 208)}px;top:${S.tree.y}px">` +
      menuRow(L.commonClose(), () => shutTabs([key]), '', 'Ctrl+W') + menuRow(L.sideCloseOthers(), others.length ? () => shutTabs(others) : null) + menuRow(L.sideCloseRight(), right.length ? () => shutTabs(right) : null) + '</div>'
  }
  if (S.tree.talk) {
    const id = S.tree.key, flip = () => set((s) => ({ tree: null, shelved: { ...s.shelved, [id]: !s.shelved[id] } }))
    return `<div class="veil float-veil"${onClick(closeFloat)}${onMenu((ev) => { ev.preventDefault(); closeFloat() })}></div>` +
      `<div class="ctx pop" data-k="tree-menu" style="left:${S.tree.x}px;top:${S.tree.y}px">${menuRow(S.shelved[id] ? L.sideMoveBack() : L.sideCollapseTalk(), flip)}</div>`
  }
  const key = S.tree.key, vol = S.volumes.some((v) => v[0] === key), c = chapterOf(key)
  // 移到哪一卷收在「移到」的下一级里：卷再多，菜单也不跟着变长。into 是下一级开着。
  const spots = vol ? [] : [[L.sideTopLevel(), ''], ...S.volumes.map((v) => [v[1], v[0]])].filter(([, to]) => to !== c.vol)
  const win = document.getElementById('win')
  return `<div class="veil float-veil"${onClick(closeFloat)}${onMenu((ev) => { ev.preventDefault(); closeFloat() })}></div>` +
    `<div class="ctx pop${S.tree.x > win.offsetWidth - 208 - 208 ? ' flip' : ''}" data-k="tree-menu"${subOver(!!S.tree.into, (on) => set((s) => ({ tree: { ...s.tree, into: on } })))} style="left:${S.tree.x}px;top:${Math.min(S.tree.y, win.offsetHeight - Math.max(270, 178 + Math.min(spots.length, 10) * 30) - (vol ? 30 : 0))}px">` +
    (vol ? menuRow(L.sideEditVolumeOutline(), goTo(key)) : '') + // 点一卷只是折起、展开；它的卷纲从这里打开
    menuRow(L.cmdRename(), () => set({ tree: null, rename: { key, t: vol ? S.volumes.find((v) => v[0] === key)[1] : titleOf(c), all: true } })) + menuRow(L.cmdNewChapter(), () => newChapter(vol ? key : undefined)) + menuRow(L.cmdNewFolder(), newFolder) + HAIR +
    menuRow(L.sideMoveUp(), () => shift(key, -1)) + menuRow(L.sideMoveDown(), () => shift(key, 1)) +
    (spots.length ? subRow(L.sideMoveTo(), S.tree.into, () => set((s) => ({ tree: { ...s.tree, into: true } })), 'tree-into', spots.map(([label, to]) => menuRow(label, () => moveInto(key, to))).join('')) : '') + HAIR +
    menuRow(L.sideMoveToTrash(), () => toBin(key)) + '</div>'
}

// ───────── 正文标记 ─────────

// 正文标记（基础扩展 marks）：正文上留着、要跟着正文挪的标记都经这里。扩展登记一个来源，给出开着的这一章里的每一处：
// 哪一段（para，不知道就不写）的哪几个字（quote），画成什么样（cls，空的不画，只在页边有一条批注），
// id（右键点在上面时「查看」用，没有就空着），页边的批注（note，批注扩展把它们排在正文旁边）。
// 编辑器只画这里给的：正文标记关着时一处也不画，伏笔、批注、检查跟着停下；哪个来源的扩展关着，它的那些不画。
// 几处重叠时，按登记的先后留先到的。
// 不经这里的：选中的字、查到的字、AI 正在提的修改、设定的名字。它们随时算得出来，不用存、不用跟着正文挪，
// 由编辑器直接画，各自的扩展开着才画。
const marksIn = (cur) => cur && extOn('marks') ? contributions('MarginSources').flatMap((of) => of(cur)) : []

// ───────── 画：正文 ─────────

// 一段切成几块：要改写的、AI 提议换掉的、选中的、查到的、正文标记（伏笔、检查出来的、AI 写的）、设定的名字。重叠的只留先到的。
function pieces(cur, i, text, focus, found) {
  const cuts = []
  const cut = (quote, para, cls, id, then = '') => {
    const at = para === undefined || para === i ? text.indexOf(quote) : -1
    if (at >= 0) cuts.push({ a: at, b: at + quote.length, cls, id, then })
  }
  const a = S.ai && S.ai.pick && S.ai.pick.para === i ? S.ai : null
  if (a) cuts.push({ a: a.pick.start, b: a.pick.end, cls: 'sel', id: '' })
  // 换掉一句的提议：原句划掉，AI 写的绿色跟在后面。
  for (const p of propsOf(cur.key)) if (p.at.kind === 'swap') cut(p.at.quote, undefined, 'cut', '', `<span class="add"${mine(p)}>${t(p.paras.join(''))}</span>` + propActs(p))
  if (S.pick && S.pick.para === i) cuts.push({ a: S.pick.start, b: S.pick.end, cls: 'sel', id: '' })
  if (S.flash && S.flash.key === cur.key) cut(S.flash.quote, undefined, 'sel', '')
  found.forEach((h, n) => { if (h.para === i) cuts.push({ a: h.a, b: h.a + S.find.q.length, cls: n === S.find.at ? 'got on' : 'got', id: '' }) })
  for (const m of marksIn(cur)) if (m.cls) cut(m.quote, m.para, m.id && m.id === focus ? m.cls + ' on' : m.cls, m.id)
  // 设定的名字：一道很淡的虚线，点了在右栏打开那一条。排在最后，和别的记号重叠时让给它们。
  contributions('NameHighlights').forEach((of) => of(text, cuts))
  const kept = []
  for (const c of cuts) if (!kept.some((k) => c.a < k.b && k.a < c.b)) kept.push(c)
  kept.sort((x, y) => x.a - y.a)
  const span = (words, c) => `<span${c && c.id ? ` data-mark="${c.id}"` : ''}${c ? ` class="${c.cls}"` : ''}${c && c.entry ? ` title="${L.editorSeeEntry()}"` + onClick(seeName(c.entry)) : ''}>${t(words)}</span>` + ((c && c.then) || '')
  let out = '', at = 0
  for (const c of kept) {
    if (c.a > at) out += span(text.slice(at, c.a))
    out += span(text.slice(c.a, c.b), c)
    at = c.b
  }
  return at < text.length ? out + span(text.slice(at)) : out
}

// 「全部忽略」只是不再列在待处理里：检查页、批注、正文里的线都还在。点错了在提示条上撤销。
const ignoreAll = (shown) => () => {
  const was = S.ignored
  set((s) => ({ ignored: [...s.ignored, ...shown.map((x) => x.id)], ...SHUT }))
  const id = say(L.statusIgnored(shown.length), '', 'ok', [[L.cmdUndo(), () => { unsay(id); set({ ignored: was }) }]])
}
// 工具栏右边的「待处理 N」点开的那一块。
function todoPop(list) {
  const groupName = (g) => ({ 检查: L.pageCheck(), 伏笔: L.statusGroupMarks(), 备份: L.statusGroupBackup() })[g]
  const here = list.filter((x) => x.key === S.ch), shown = S.todoAll ? list : here
  const groups = ['检查', '伏笔', '备份'].map((g) => [g, shown.filter((x) => x.group === g)]).filter(([, of]) => of.length)
  const row = (x) => `<div class="todo-row row"${onClick(x.open)}><div class="fit"><div class="ell">${t(x.name)}</div><div class="note ell">${t(x.about)}${x.key === S.ch ? L.statusHereTag() : ''}</div></div>${ICON.right}</div>`
  return '<div class="pop todo-pop" data-k="todo-pop">' +
    `<div class="todo-head"><div class="todo-title">${L.statusPending()}</div><div class="grow"></div>${seg(S.todoAll ? '全部 ' + list.length : '本章 ' + here.length, [['全部 ' + list.length, '', L.statusAllCount(list.length)], ['本章 ' + here.length, '', L.statusHereCount(here.length)]], (v) => set({ todoAll: v.startsWith('全部') }), L.statusPendingScope())}</div>` +
    `<div class="todo-list scroll">${groups.length ? groups.map(([g, of]) => {
      const all = S.more[g] || of.length <= 3
      return `<div class="todo-group"><div class="dot grey"></div>${groupName(g)}<span class="faint">${of.length}</span></div>` + (all ? of : of.slice(0, 3)).map(row).join('') +
        (of.length > 3 ? `<div class="todo-rest hit"${onClick(() => set((s) => ({ more: { ...s.more, [g]: !all } })))}>${all ? L.commonCollapse() : L.toastMore(of.length - 3)}</div>` : '')
    }).join('') : `<div class="todo-rest">${L.statusNoneHere()}</div>`}</div>` +
    `<div class="todo-foot"><div class="hit link"${onClick(ignoreAll(shown))}>${L.statusIgnoreAll()}</div></div></div>`
}

// vol 是开着的那一卷：这时没有开着的章。
// 底栏右边的字数：选中的、本章或本卷的、全书的，设了每日目标还有今天的。点开设目标。
function wordsOf(cur, vol) {
  const inVol = vol && wordCount(vol[2] || S.chapters.filter((c) => c.vol === vol[0]).reduce((n, c) => n + sizeOf(c), 0))
  const count = (S.bar ? L.statusSelected(S.bar.text.length, 'character') : cur ? L.statusChapterWords(sizeOf(cur), 'character') : vol ? L.statusVolumeWords(inVol) : '') + (cur || vol ? ' · ' : '') + L.statusBookWords(wordCount(S.book === BOOK.shelf[0].id ? BOOK.words : S.chapters.reduce((n, c) => n + sizeOf(c), 0))) + (drawPoint('WordSuffix'))
  return `<div class="anchor up"><div class="words ell hit" title="${L.statusWordsTip('character')}"${onClick(() => set({ open: S.open === 'goal' ? '' : 'goal' }))}>${t(count)}</div>${drawPoint('WordPopovers')}</div>`
}

// 标题栏右边那几样：写作页才有。专注写作时也在，那时标题栏只剩它们和正中的章名。
function tools(cur) {
  const here = cur ? S.marks.filter((m) => m.st === 'open' && m.key === cur.key).length : 0
  const wide = !S.paneOpen || S.focus
  const list = pending()
  const tool = (label, title, pic, act, on, off) => `<button class="icon-btn${on ? ' on' : ''}${off ? ' off' : ''}" aria-label="${label}" title="${title}"${onClick(act)}>${icon(pic)}</button>`
  const flip = (name) => () => set({ open: S.open === name ? '' : name })
  // 时有时无的几样在左，查找和专注两个图标总在右边同一处：谁出来、谁走了，都挤不动它们。
  return '<div class="tools">' +
    (S.ai && S.ai.kind === 'ghosting' ? '<div class="count-all">' + L.editorContinuing() + '</div>' : S.ai && S.ai.kind === 'ghost' ? '<div class="count-all">' + L.editorGhostHint() + '</div>' : '') +
    drawPoint('ToolHints', cur, wide, here, tool, flip) +
    (list.length ? `<div class="anchor"><button class="todo-btn${S.open === 'todo' ? ' on' : ''}" aria-label="${L.statusPending()}"${onClick(flip('todo'))}>${DOT}<span>${L.statusPendingCount(list.length)}</span></button>${S.open === 'todo' ? todoPop(list) : ''}</div>` : '') +
    drawPoint('Tools', cur, wide, here, tool, flip) +
    '</div>'
}

// 标签页：标题栏下面那一行，开着的几章（或卷）从左到右，只有名字；正看的那个是墨色，下面一道墨线。
// 最左边是回到上一个、下一个打开过的章的两个箭头。没存的那个，关闭的叉换成一个圆点。专注写作时这一行收起。
// 这一行的底线和侧栏顶上、右栏标签那两行的底线是同一条。
function sheets() {
  const on = S.ch || S.vol, tabs = S.tabs.filter((k) => chapterOf(k) || S.volumes.some((x) => x[0] === k))
  if (S.focus) return ''
  const tab = (key) => {
    const c = chapterOf(key), v = S.volumes.find((x) => x[0] === key), home = c && S.volumes.find((x) => x[0] === c.vol)
    const name = c ? titleOf(c) : v[1], dirty = key === S.ch && !S.saved
    const menu = onMenu((ev) => { ev.preventDefault(); const [x, y] = spot(ev.clientX, ev.clientY); set({ tree: { key, x, y, tab: true } }) })
    return `<div class="sheet hit${key === on ? ' on' : ''}${dirty ? ' dirty' : ''}" data-tab="${key}" title="${t((home ? home[1] + ' › ' : '') + name + (dirty ? L.tabUnsaved() : ''))}"${onClick(goTo(key))}${menu}${onMid((ev) => { if (ev.button === 1) shutTabs([key]) })}>` +
      `${c ? '' : icon('folder', 'sm')}<div class="fit ell">${t(name)}</div><div class="sheet-dot"></div><div class="sheet-x" title="${L.tabClose()}" aria-label="${L.tabClose()}"${onClick(() => shutTabs([key]))}>${icon('cross', 'xs')}</div></div>`
  }
  return '<div class="sheet-row" data-k="sheet-row">' +
    `<div class="steps"><div class="hit${S.step > 0 ? '' : ' off'}" title="${L.tabPrevOpened()}"${onClick(() => stepBy(-1))}>${icon('left', 'sm')}</div><div class="hit${S.trail[S.step + 1] ? '' : ' off'}" title="${L.tabNextOpened()}"${onClick(() => stepBy(1))}>${icon('right', 'sm')}</div></div>` +
    `<div class="sheets" data-k="sheets">${tabs.map(tab).join('')}</div></div>`
}

// 一章太长、AI 只读了前面一段时，那一行字上的说明。

// 正文下面那一行：检查到哪了、什么时候查，右边是字数。AI 关着时只有字数。
function foot(cur, vol) {
  return `<div class="foot">${drawPoint('FooterItems', cur)}<div class="grow"></div>${drawPoint('FootStatus')}${wordsOf(cur, vol)}</div>`
}

// 软件外面动了开着的这一章时，正文上面的那一条。
function outsideBar() {
  if (S.outside === 'changed') return `<div class="notice-bar" data-k="outside">${icon('warn')}<div class="fit">${L.editorOutsideChanged()}</div>${button(L.editorUseDisk(), () => settleOutside(true))}${button(L.editorKeepCurrent(), () => settleOutside(false), 'primary')}</div>`
  if (S.outside === 'lost') return `<div class="notice-bar lost" data-k="outside">${icon('warn')}<div class="fit">${L.editorOutsideLost()}</div>${button(L.editorSaveAsNew(), saveLost, 'primary')}</div>`
  return ''
}

// 点开一卷：卷纲，和它里面的各章。每章带一句章纲，没有章纲的带正文的开头。
function folderPage([key, title]) {
  const inside = S.chapters.filter((c) => c.vol === key)
  const gist = (c) => (S.outlines[c.key] || '').trim() || c.paras.join(' ')
  return '<div class="paper scroll"><div class="column folder">' +
    `<div class="chapter-title">${t(title)}</div><div class="folder-label">${L.editorVolumeOutline()}</div>` +
    area(S.outlines[key] || '', (v) => set((s) => ({ outlines: { ...s.outlines, [key]: v } })), L.editorOutlineHint(), L.editorVolumeOutline(), 6) +
    `<div class="folder-count"><div class="grow">${L.editorItems(inside.length)}</div><div class="hit link"${onClick(newChapter)}>${L.editorNewHere()}</div></div>` +
    inside.map((c) => `<div class="folder-row hit"${onClick(goTo(c.key))}><div class="folder-name">${icon('page', 'md', 'muted')}<div class="fit ell">${t(titleOf(c))}</div><div class="count">${t(L.statusWords(sizeOf(c), 'character'))}</div></div>${gist(c) ? `<div class="gist">${t(gist(c))}</div>` : ''}</div>`).join('') +
    '</div></div>'
}

function editor(cur) {
  const vol = S.volumes.find((v) => v[0] === S.vol)
  if (vol) return '<div class="editor" data-k="editor">' + sheets() + folderPage(vol) + foot(null, vol) + '</div>'
  if (!cur) {
    return `<div class="editor" data-k="editor">${sheets()}<div class="paper">` + (S.chapters.length ? blank('book', L.editorPickChapter(), '')
      : blank('book', L.editorNoChapters(), L.editorNoChaptersHint(), buttons(button(L.cmdNewFolder(), newFolder), button(L.editorNewFirst(), newChapter, 'primary')))) + '</div>' + foot(null) + '</div>'
  }
  const focus = S.paneOpen && S.tab === '伏笔' ? S.fid : null
  const found = S.find && S.find.q && !S.find.all ? hits(cur, S.find.q) : []
  const ghostly = S.ai && S.ai.kind === 'ghost' ? `<span class="ghost-text">${t(S.ai.text)}</span>` : ''
  // 一段不剩时也留一个空段，光标才有地方放。空段里放一个换行，它才有一行高。
  // AI 提议接在章末、换掉全章的，绿色的几段接在后面，最后一段跟着按钮；换掉全章的，原文都划掉。换掉一句的在那一段里（pieces）。
  const whole = propsOf(cur.key).filter((p) => p.at.kind !== 'swap'), over = whole.some((p) => p.at.kind === 'all')
  const shown = cur.paras.length || whole.length ? cur.paras : ['']
  const paras = shown.map((text, i) => { const last = i === shown.length - 1 && ghostly; return `<div data-para="${i}"${over ? ' class="cut"' : ''}>${pieces(cur, i, text, focus, found) || (last ? '' : '<br>')}${last || ''}</div>` }).join('') +
    whole.map((p) => (p.paras.length ? p.paras : ['']).map((text, n, all) => `<div class="add"${mine(p)}>${t(text)}${n === all.length - 1 ? propActs(p) : ''}</div>`).join('')).join('')
  const hint = cur.paras.join('') || ghostly || whole.length ? '' : '<div class="prose-hint">' + L.editorStartHint() + '</div>'
  // 灰字等着采纳、等着写改写要求的时候不能打字；有 AI 的提议时照常写，改到提议换掉的那几句，提议就作废。
  const typing = S.ai ? '' : ` contenteditable="plaintext-only" spellcheck="false" aria-label="${L.editorBody()}"${onInput(wrote)}${onKey(proseKey)}${onPaste(pasted)}`
  return '<div class="editor" data-k="editor">' + sheets() + (S.find ? findBar(cur) : '') + outsideBar() +
    `<div class="paper scroll"${onMenu(openMenu)}${onUp(picked)}><div class="column chapter"><div class="chapter-head"><div class="chapter-title">${t(cur.t || titleOf(cur))}</div>${drawPoint('ChapterHeadItems', cur)}</div>` + aiBar(cur) +
    `<div class="prose-box">${hint}<div class="prose" data-k="prose"${typing}>${paras}</div></div></div></div>` + foot(cur) + '</div>'
}

// ───────── 画：正文上浮着的 ─────────

function floats() {
  if (S.tree) return treeMenu()
  if (!S.ctx && !S.bury && !S.pay && !S.bar) return ''
  const win = document.getElementById('win'), right = document.getElementById('pane')
  const leftEdge = barEdge()
  // 贴着选中的字放：下面放不下就放上面，左右不出正文这一栏。
  const by = (wide, tall, p) => {
    const edge = win.offsetWidth - (right ? right.offsetWidth : 0)
    const y = p.bottom + 6 + tall > win.offsetHeight - 8 && p.top - tall - 6 >= 44 ? p.top - tall - 6 : p.bottom + 6
    return `left:${Math.max(leftEdge, Math.min(p.left, edge - wide - 8))}px;top:${y}px`
  }
  const veiled = `<div class="veil float-veil"${onClick(closeFloat)}${onMenu((ev) => { ev.preventDefault(); closeFloat() })}></div>`

  if (S.ctx) {
    const p = S.pick
    // 剪贴板的四项。粘贴要浏览器肯让页面读剪贴板，不肯就提示按 Ctrl+V。AI 正在改的时候正文不能动，剪切、粘贴是灰的。
    const span = S.ctx.span
    const copy = () => { if (navigator.clipboard) navigator.clipboard.writeText(p.text).catch(() => {}); set({ ctx: null, pick: null }) }
    const cut = () => { copy(); putText('', span) }
    const paste = () => { set({ ctx: null, pick: null }); (navigator.clipboard ? navigator.clipboard.readText() : Promise.reject()).then((text) => putText(plain(text), span), () => say(L.editorNoClipboard(), L.editorPasteKey(keyLabel('Ctrl+V')))) }
    const all = () => { set({ ctx: null, pick: null }); const first = document.querySelector('[data-para]'); if (first) getSelection().selectAllChildren(first.parentElement) }
    const over = subOver(!!S.ctx.names, (on) => set((s) => ({ ctx: { ...s.ctx, names: on } })))
    // 菜单宽 200，最多十二行，都留在窗口里；旁边再开一列时也不出窗口。剪贴板那组是编辑器自己的，下面两组是扩展加的：
    // 续写、改写、引用、加入设定；伏笔的几项。扩展关着时它那几项不在，一组空了连同上面的分隔线一起不画。
    return veiled + `<div class="ctx pop${S.ctx.x > win.offsetWidth - 208 - 130 ? ' flip' : ''}" data-k="ctx"${over} style="left:${Math.min(S.ctx.x, win.offsetWidth - 208)}px;top:${Math.min(S.ctx.y, win.offsetHeight - 430)}px">` +
      [menuRow(L.commonCut(), p && span ? cut : null, '', 'Ctrl+X') + menuRow(L.commonCopy(), p ? copy : null, '', 'Ctrl+C') + menuRow(L.commonPaste(), span ? paste : null, '', 'Ctrl+V') + menuRow(L.commonSelectAll(), all, '', 'Ctrl+A'),
        drawPoint('SelectionMenus', p),
        drawPoint('HookMenus', p),
      ].filter(Boolean).join(HAIR) + '</div>'
  }

  // 已经浮着的记伏笔、回收框跟选区一起收起，不在这里另外看扩展开关。
  const extra = hookFloats(veiled, by)
  if (extra) return extra

  // 选中一段字以后：浮在它上面的两个按钮，AI 关着时一个。
  return `<div class="selbar pop" data-k="selbar" data-keep style="${barAt(S.bar)}">${aiOn() ? `<div class="hit" title="${L.editorRewriteSel()}"${onClick(askRewrite)}>${icon('pen', 'sm')}${L.editorAIRewrite()}</div>` : ''}<div class="hit" title="${L.editorAddSelTip()}"${onClick(take)}>${icon('keep', 'sm')}${L.editorAddToLore()}</div></div>`
}
// 浮着的东西左边不越过侧栏。
const barEdge = () => { const side = document.querySelector('.sidebar'); return (side ? side.offsetWidth : 0) + 8 }
const barAt = (p) => `left:${Math.max(barEdge(), p.left)}px;top:${Math.max(p.top - 40, 44)}px`
// 选中的字换了地方（专注写作、收起侧栏或右栏、拖宽右栏），浮条跟着它：每画一遍，照那几个字现在的位置放。
// 照段号和第几个字重新量，点了别处、选区没了也量得出。
function placeBar() {
  const bar = document.querySelector('.selbar'), para = bar && S.bar && document.querySelector(`.prose [data-para="${S.bar.para}"]`)
  if (!para) return
  const r = document.createRange(), walk = document.createTreeWalker(para, NodeFilter.SHOW_TEXT)
  let at = 0, from = null
  for (let n = walk.nextNode(); n; n = walk.nextNode()) {
    const to = at + n.data.length
    if (!from && S.bar.start <= to) { r.setStart(n, S.bar.start - at); from = n }
    if (from && S.bar.end <= to) { r.setEnd(n, S.bar.end - at); break }
    at = to
  }
  if (!from) return
  const box = r.getBoundingClientRect(), [left, top] = spot(box.left, box.top)
  bar.setAttribute('style', barAt({ left, top }))
}
