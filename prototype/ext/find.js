// 扩展：find。界面、动作和登记都在这里。
'use strict'

function swapDialog(d) {
  return dialog('ask', dialogHead(L.dialogSwapTitle(), L.dialogSwapAbout(d.chapters, d.total, d.q, d.to ? L.dialogSwapTo(d.to) : L.dialogSwapNone())) +
    buttons(button(L.commonCancel(), shut), button(L.dialogSwapConfirm(), swapBook, 'primary')))
}

// 关标签页时那一章还有没存的字：存了再关、不存就关（回到存着的那一份）、或者不关。
// 三个按钮照系统问「要不要保存」的顺序排：Windows 是保存、不保存、取消；macOS 是「不保存」单独在最左，取消、保存靠右。

// 一章里找到的每一处：在第几段、从哪开始。
function hits(c, q) {
  const out = []
  if (q) c.paras.forEach((p, para) => { for (let a = p.indexOf(q); a >= 0; a = p.indexOf(q, a + q.length)) out.push({ para, a }) })
  return out
}
// 打开查找；开着时光标回到查找框，框里的字全选上，接着打就换掉。正文里选中了一段之内的字，就拿它去找：选中一个名字按 Ctrl+F，就是找它出现的各处。
function openFind() {
  const pick = grab()
  set((s) => {
    const was = s.find || { q: '', to: '', all: false, at: 0 }
    if (!pick) return { find: was, ...SHUT }
    const at = hits(chapterOf(s.ch), pick.text).findIndex((h) => h.para === pick.para && h.a === pick.start)
    return { find: { ...was, q: pick.text, at: Math.max(at, 0) }, ...SHUT }
  })
  const box = document.querySelector('.find-bar .in')
  if (box) { box.focus(); box.select() }
}
const findStep = (d) => set((s) => { const n = hits(chapterOf(s.ch), s.find.q).length; return n ? { find: { ...s.find, at: (s.find.at + d + n) % n } } : null })
function swapOne() {
  set((s) => {
    const { q, to, at } = s.find, all = hits(chapterOf(s.ch), q), hit = all[at]
    if (!hit) return null
    // 换掉的是最后一处，就回到第一处。
    return { ...oneStep(s, s.ch, (paras) => paras.map((p, i) => (i === hit.para ? p.slice(0, hit.a) + to + p.slice(hit.a + q.length) : p))), find: { ...s.find, at: at < all.length - 1 ? at : 0 } }
  })
}
function swapAll() {
  const { q, to } = S.find, n = hits(chapterOf(S.ch), q).length
  if (!n) return
  set((s) => oneStep(s, s.ch, (paras) => paras.map((p) => p.split(q).join(to))))
  say(L.findReplaced(n), '', 'ok')
}
function swapBook() {
  const { q, to } = S.dialog, had = S.chapters.filter((c) => hits(c, q).length)
  const n = had.reduce((sum, c) => sum + hits(c, q).length, 0)
  set((s) => {
    let next = { ...s, dialog: null }
    for (const c of had) next = { ...next, ...snap(next, c.key, '全书替换前'), ...retype(next, c.key, (paras) => paras.map((p) => p.split(q).join(to))) }
    return next
  })
  say(L.findReplacedBook(had.length, n), '', 'ok')
}

function findBar(cur) {
  const f = S.find, type = (key) => (v) => set((s) => ({ find: { ...s.find, [key]: v, at: key === 'q' ? 0 : s.find.at } }))
  const here = cur ? hits(cur, f.q) : []
  const had = S.chapters.map((c) => [c, hits(c, f.q)]).filter(([, h]) => h.length)
  const total = had.reduce((sum, [, h]) => sum + h.length, 0)
  const count = !f.q ? '' : f.all ? (total ? L.findBookCount(had.length, total) : L.findNoneBook()) : here.length ? f.at + 1 + ' / ' + here.length : L.findNone()
  const shut = () => set({ find: null })
  const snip = (c, h) => { const p = c.paras[h.para]; return (h.a > 6 ? '…' : '') + p.slice(Math.max(h.a - 6, 0), h.a + f.q.length + 14) }
  return '<div class="find-bar" data-k="find-bar">' +
    `<div class="find-line"><div class="box line fit">${icon('find', 'sm', 'faint')}${typed(f.q, type('q'), L.findBox(), L.findBox(), keys(() => findStep(1), shut) + ' autofocus')}</div><div class="side find-count">${t(count)}</div>` +
    (f.all ? '' : `<button class="icon-btn muted" aria-label="${L.findPrev()}" title="${L.findPrev()}"${onClick(() => findStep(-1))}>${icon('up')}</button><button class="icon-btn muted" aria-label="${L.findNext()}" title="${L.findNext()}"${onClick(() => findStep(1))}>${icon('down')}</button>`) +
    `<div class="chip hit${f.all ? ' on' : ''}" title="${L.findAllTip()}"${onClick(() => set((s) => ({ find: { ...s.find, all: !s.find.all, at: 0 } })))}>${L.findBook()}</div><button class="icon-btn muted" aria-label="${L.findClose()}" title="${L.findClose()}"${onClick(shut)}>${icon('cross')}</button></div>` +
    `<div class="find-line"><div class="box line fit">${typed(f.to, type('to'), L.findReplaceWith(), L.findReplaceWith(), keys(swapOne, shut))}</div>` +
    (f.all ? button(L.findReplaceBook(), total ? () => set({ dialog: { kind: 'swap', q: f.q, to: f.to, chapters: had.length, total } }) : null, total ? '' : 'off')
      : button(L.findReplace(), here.length ? swapOne : null, here.length ? '' : 'off') + button(L.findReplaceAll(), here.length ? swapAll : null, here.length ? '' : 'off')) + '</div>' +
    (f.all && total ? `<div class="find-all scroll">${had.map(([c, h]) => `<div class="head">${t(L.findChapterHead(titleOf(c), h.length))}</div>` + h.map((x) => `<div class="item row"${onClick(() => set((s) => ({ ...go(s, c.key), find: { ...s.find, all: false, at: hits(c, f.q).indexOf(x) } })))}><div class="about ell">${marked(snip(c, x), f.q)}</div></div>`).join('')).join('')}</div>` : '') +
    '</div>'
}

contribute('Dialogs', 'find', { kind: 'swap', draw: swapDialog }, 0)

contribute('EditActions', 'find', (cur, inBook, writing, then, ask, outline) => menuRow(L.cmdFind(), cur ? openFind : null, '', 'Ctrl+F'), 0)

contribute('Tools', 'find', (cur, wide, here, tool, flip) => tool(L.cmdFind(), L.tabFindTip(keyLabel('Ctrl+F')), 'find', cur ? () => (S.find ? set({ find: null }) : openFind()) : null, !!S.find, !cur), 0)

// 查找替换：{ q, to, all, at }
contribute('State', 'find', () => ({
  find: null
}))


contribute('WritingKeys', 'find', { key: 'f', act: () => openFind() }, 0)
