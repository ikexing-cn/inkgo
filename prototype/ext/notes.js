// 扩展：notes。界面、动作和登记都在这里。
'use strict'

// ───────── 画：批注 ─────────

// 页边的批注：开着的这一章里，正文标记带着批注的那几处（伏笔、检查、AI 从这一章看出来要改的设定，见 MarginSources 的登记）。
// 每条记着它说的是第几段的第几个字起、几个字（这几个字找不到就不列；没给字的排在最上面），照这个顺序从上往下排。
function notesOf(cur) {
  if (!cur) return []
  const find = (quote, para) => {
    if (!quote) return { para: 0, off: 0, len: 0 }
    const i = para !== undefined ? para : cur.paras.findIndex((p) => p.includes(quote)), off = i >= 0 ? (cur.paras[i] || '').indexOf(quote) : -1
    return off >= 0 ? { para: i, off, len: quote.length } : null
  }
  const list = []
  for (const m of marksIn(cur)) {
    const at = m.note && find(m.quote, m.para)
    if (at) list.push({ ...at, ...m.note })
  }
  return list.sort((a, b) => a.para - b.para || a.off - b.off)
}

function margin() {
  const cur = chapterOf(S.ch), list = notesOf(cur)
  if (!list.length) return `<div class="margin" data-k="margin"><div class="margin-none">${t(cur ? L.notesNone() : L.notesOpenChapter())}</div></div>`
  // 换一章，这一章的几条从上往下一条接一条出来。等作者处理的几条，就在批注上处理（和检查页、设定页的按钮同一份），最后一样是主要的。
  const act = ([label, go], i, all) => `<div class="hit link${i < all.length - 1 ? ' muted' : ''}${go ? '' : ' off'}"${onClick(go && (() => moving(go)))}>${label}</div>`
  const note = (n) => `<div class="mnote hit" data-k="note-${cur.key}-${n.id}" data-note="${n.id}" data-at="${n.para},${n.off},${n.len}" style="view-transition-name: note-${n.id}; view-transition-class: note"${onClick(n.open || (() => set({ tab: '设定', sv: 'ideas', sid: null, form: null, ...SHUT })))}>` +
    `<div class="mnote-kind">${n.warn ? DOT : ''}${t(n.kind)}</div><div class="mnote-title">${t(n.title)}</div>${n.text ? `<div class="mnote-text">${t(n.text)}</div>` : ''}` +
    (n.acts ? `<div class="mnote-acts">${n.acts.map(act).join('')}</div>` : '') + '</div>'
  return `<div class="margin" data-k="margin">${list.map(note).join('')}</div>`
}

// 批注挨着它说的那几个字：每画一遍、正文每滚一下，照那几个字现在的高度放，挤在一起的往下让。
function placeNotes() {
  const box = document.querySelector('.margin')
  if (!box) return
  const top = box.getBoundingClientRect().top, z = winZoom()
  let floor = -Infinity
  for (const n of box.querySelectorAll('.mnote')) {
    const r = spanAt(n.dataset.at), y = r ? (r.getBoundingClientRect().top - top) / z : floor
    const at = Math.max(Math.round(y) - 3, floor)
    n.style.top = at + 'px'
    floor = at + n.offsetHeight + 8
  }
}
// 第几段第几个字起几个字，在正文里的那一截；一个字也没有的，是那一段。
function spanAt(at) {
  const [para, off, len] = at.split(',').map(Number), p = document.querySelector(`.prose [data-para="${para}"]`)
  if (!p || !len) return p
  const r = document.createRange(), walk = document.createTreeWalker(p, NodeFilter.SHOW_TEXT)
  let n = 0, from = null
  for (let node = walk.nextNode(); node; node = walk.nextNode()) {
    if (proposed(node)) continue
    const to = n + node.data.length
    if (!from && off < to) { r.setStart(node, off - n); from = node }
    if (from && off + len <= to) { r.setEnd(node, off + len - n); return r }
    n = to
  }
  return from ? r : p
}
// 指针在一条批注上：正文里它说的那几个字垫上底色。指针在正文里画了线的那几个字上：它的批注也垫上底色。
document.addEventListener('mouseover', (ev) => {
  const el = ev.target.closest ? ev.target : null, line = el && el.closest('.prose .fs, .prose .doubt')
  const n = el && (el.closest('.mnote') || (line && [...document.querySelectorAll('.mnote')].find((x) => { const r = spanAt(x.dataset.at); return r instanceof Range && r.intersectsNode(line) })))
  for (const x of document.querySelectorAll('.mnote.lit')) if (x !== n) x.classList.remove('lit')
  if (n && line) n.classList.add('lit')
  if (!window.CSS || !CSS.highlights) return
  const r = n && spanAt(n.dataset.at)
  if (!r) return CSS.highlights.delete('note')
  const range = r instanceof Range ? r : (() => { const x = document.createRange(); x.selectNodeContents(r); return x })()
  CSS.highlights.set('note', new Highlight(range))
})
document.addEventListener('scroll', (ev) => { if (ev.target.classList && ev.target.classList.contains('paper')) placeNotes() }, true)

contribute('PaneTabs', 'notes', { name: '批注', label: () => L.paneTabNotes(), draw: margin, badge: () => 0, beside: false }, 0)

contribute('AfterDraw', 'notes', () => placeNotes(), 1) // 批注也是
