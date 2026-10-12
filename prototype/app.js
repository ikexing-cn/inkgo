// 原型：整个窗口照一份状态画出来。点一下改状态，再整个画一遍，只动变了的地方。
// 不用任何库，也不用构建，双击 index.html 就能开。
//   book.js     书里的内容，都是假的
//   index.html  样式：颜色、字号、间距只写 desk/tokens.css 里的名字
//   app.js      这一份：状态、每个动作做什么、每一块怎么画
// 还点不了的：侧栏的「讨论」「作品分类」、新建章节、回收站、设置，
// 正文上面的查找、更多、专注写作，「章纲」和「本章摘要」。
// 改完跑 npm run verify-prototype，它在浏览器里把每个动作点一遍。
(() => {
  'use strict'

  // ───────── 状态 ─────────

  const chapterOf = (key) => BOOK.chapters.find((c) => c.key === key)
  const titleOf = (c) => (c.vol ? '第' + c.no + '章：' + c.t : c.t)

  // 刚打开的样子：停在最新一章，别的卷收着。
  function fresh() {
    const last = BOOK.chapters.find((c) => c.no === BOOK.now)
    return {
      dark: false, paneOpen: true, tab: '设定',
      ch: last.key, folds: Object.fromEntries(BOOK.volumes.map(([key]) => [key, key !== last.vol])),
      menu: false, cats: false, // 右栏里开着的菜单：「···」和分类下拉
      n: 100, // 新东西的编号，用一个加一个
      sv: 'list', sid: null, sq: '', form: null, // 设定：在哪一页、看哪一条、搜什么、正在填的表
      fv: 'list', fid: null, fq: '', renote: null, // 伏笔：同上，renote 是正在改的备注
      tv: 'all', tid: null, tq: '', draft: '', attach: true, waiting: false, // 讨论
      ctx: null, pick: null, bury: null, pay: null, // 正文上浮着的：右键菜单、选中的字、记伏笔、选回收哪条
      flash: null, // 「看原文」跳过去后要标出来的那句
      toast: '',
      ...structuredClone({ entries: BOOK.entries, log: BOOK.log, marks: BOOK.marks, talks: BOOK.talks }),
    }
  }

  let S = fresh()
  let sent = 0 // 作者发过几条消息，AI 按顺序回下一句
  let replying, fading

  // change 是要改的那几项，或者一个看着现在的状态算出它们的函数；返回空就什么都不变。
  function set(change) {
    const next = typeof change === 'function' ? change(S) : change
    if (!next) return
    S = { ...S, ...next }
    draw()
  }

  const SHUT = { menu: false, cats: false }

  // ───────── 动作 ─────────

  const clip = (text, n) => (text.length > n ? text.slice(0, n) + '…' : text)

  // 作者对设定做的事，记在修改记录最上面。
  const did = (s, line) => [{ id: 'l' + s.n, who: '你改的', model: '', at: '刚刚', lines: [line] }, ...s.log]

  function move(id, place) {
    set((s) => {
      const e = s.entries.find((x) => x.id === id)
      const entries = s.entries.map((x) => (x.id === id ? { ...x, place, about: undefined } : x))
      const left = entries.some((x) => x.place === 'open')
      return {
        entries, n: s.n + 1, log: did(s, '移到' + { body: '正文', dropped: '已删除' }[place] + '：' + clip(e.text, 24)),
        sid: e.place === 'open' ? s.sid : null, sv: s.sv === 'ideas' && !left ? 'list' : s.sv,
      }
    })
  }

  function save() {
    set((s) => {
      const f = s.form
      const text = f.text.trim()
      if (!text) return null
      const names = [f.name, ...f.names.split('、')].map((x) => x.trim()).filter(Boolean)
      if (!f.id) {
        const e = { id: 'e' + s.n, place: 'body', cat: f.cat, names, text }
        return { entries: [...s.entries, e], n: s.n + 1, form: null, cats: false, log: did(s, '新增到正文 · ' + f.cat + '：' + text) }
      }
      const old = s.entries.find((x) => x.id === f.id)
      const line = old.text !== text ? '改写为：' + text : '改归' + f.cat + '：' + clip(text, 24)
      return { entries: s.entries.map((x) => (x.id === f.id ? { ...x, names, text, cat: f.cat } : x)), n: s.n + 1, form: null, cats: false, log: did(s, line) }
    })
  }

  // AI 没整理成功的那一轮再来一次：这回成了，多出一条建议。
  function retry(id) {
    set((s) => ({
      n: s.n + 1,
      log: [{ id: 'l' + s.n, who: '后台整理', model: 'Opus 5.5 · Low', at: '刚刚', lines: ['新增到等你决定 · 物品：怀表停在七点一刻，一直没有修。'] }, ...s.log.filter((x) => x.id !== id)],
      entries: [...s.entries, { id: 'e' + s.n, place: 'open', cat: '物品', names: [], text: '怀表停在七点一刻，一直没有修。' }],
    }))
  }

  const mark = (id, change) => set((s) => ({ marks: s.marks.map((m) => (m.id === id ? { ...m, ...change(m) } : m)), renote: null }))

  function send(words) {
    const text = (words ?? S.draft).trim()
    if (!text || S.waiting) return
    const isNew = S.tv !== 'one'
    const id = isNew ? 't' + S.n : S.tid
    const add = (me, text) => (talk) => (talk.id === id ? { ...talk, msgs: [...talk.msgs, { me, text }] } : talk)
    const talks = (isNew ? [{ id, title: text, at: '今天', msgs: [] }, ...S.talks] : S.talks).map(add(true, text))
    const reply = BOOK.replies[sent++ % BOOK.replies.length]
    set({ talks, tid: id, tv: 'one', n: S.n + 1, draft: '', waiting: true })
    replying = setTimeout(() => set((s) => ({ waiting: false, talks: s.talks.map(add(false, reply)) })), 1200)
  }

  function say(words) {
    clearTimeout(fading)
    set({ toast: words })
    fading = setTimeout(() => set({ toast: '' }), 3000)
  }

  function restart() {
    clearTimeout(replying)
    clearTimeout(fading)
    sent = 0
    set({ ...fresh(), dark: S.dark })
  }

  // 打开一章：它所在的卷展开，上一章上面浮着的东西收掉。
  function go(s, key, flash) {
    const ch = chapterOf(key)
    return { ch: key, folds: { ...s.folds, [ch.vol]: false }, ctx: null, pick: null, bury: null, pay: null, flash: flash || null }
  }
  const goTo = (key, flash) => () => set((s) => go(s, key, flash))

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
    const [left, top] = spot(r.left, r.top)
    return { para: Number(para.dataset.para), start, end: start + text.length, text, left, top, bottom: top + r.height }
  }

  function openMenu(ev) {
    ev.preventDefault()
    const on = ev.target.closest('[data-mark]')
    const [x, y] = spot(ev.clientX, ev.clientY)
    set({ ctx: { x, y, mark: on ? on.dataset.mark : '' }, pick: grab(), bury: null, pay: null, flash: null })
  }

  const closeFloat = () => set({ ctx: null, bury: null, pay: null, pick: null })

  // 问「回收的是哪个伏笔」。id 是先选好的那条，从右键菜单来的没有。
  function askPay(id) {
    const pick = grab() || S.pick
    if (!pick) { say('先在正文里选中回收伏笔的那一段'); return }
    set({ ctx: null, bury: null, pick, pay: { q: '', id } })
  }

  function buryNow() {
    set((s) => {
      if (!s.pick) return null
      const ch = chapterOf(s.ch)
      const m = { id: 'm' + s.n, st: 'open', note: s.bury.note.trim(), ch: ch.no, key: ch.key, para: s.pick.para, quote: s.pick.text }
      return { marks: [...s.marks, m], n: s.n + 1, bury: null, pick: null }
    })
    getSelection().removeAllRanges()
  }

  function payNow(id) {
    if (!id) return
    set((s) => {
      if (!s.pick) return null
      const ch = chapterOf(s.ch)
      const pay = { ch: ch.no, key: ch.key, para: s.pick.para, quote: s.pick.text }
      return { marks: s.marks.map((m) => (m.id === id ? { ...m, st: 'done', pay } : m)), pay: null, pick: null, tab: '伏笔', fid: id, fv: 'list', renote: null, paneOpen: true, ...SHUT }
    })
    getSelection().removeAllRanges()
  }

  // 原句删掉了的伏笔，挂到新选中的句子上。
  function rehang(id) {
    const pick = grab()
    if (!pick) { say('先在正文里选中要挂上去的句子'); return }
    const ch = chapterOf(S.ch)
    mark(id, () => ({ st: 'open', ch: ch.no, key: ch.key, para: pick.para, quote: pick.text }))
    getSelection().removeAllRanges()
  }

  // ───────── 画：小零件 ─────────

  // 每画一遍，把这一遍用到的动作记在这里，元素上只留它的号码。
  let H = []
  const hook = (kind) => (fn) => (fn ? ` data-${kind}="${H.push(fn) - 1}"` : '')
  const onClick = hook('click'), onInput = hook('input'), onKey = hook('keydown'), onMenu = hook('menu')

  // 书里的字、作者打的字，放进页面前都过一遍。
  const t = (s) => String(s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]))

  const svg = (cls, inner, width = 1.8) => `<svg class="${cls}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="${width}" stroke-linecap="round" stroke-linejoin="round">${inner}</svg>`
  const path = (d) => `<path d="${d}"></path>`
  const PANEL = '<rect x="4" y="5" width="16" height="14" rx="2"></rect>'
  const ICON = {
    left: svg('icon-sm muted', path('M15 6l-6 6 6 6')),
    right: svg('icon-sm faint', path('M9 6l6 6-6 6')),
    down: svg('icon-sm muted', path('M6 9l6 6 6-6')),
    find: svg('icon-sm faint', '<circle cx="11" cy="11" r="7"></circle>' + path('M20 20l-3.5-3.5')),
    plus: svg('icon-sm', path('M12 5v14M5 12h14')),
    dots: '<svg class="icon-md" viewBox="0 0 24 24" fill="currentColor"><circle cx="5" cy="12" r="1.6"></circle><circle cx="12" cy="12" r="1.6"></circle><circle cx="19" cy="12" r="1.6"></circle></svg>',
    send: svg('icon-sm', path('M12 19V5M6 11l6-6 6 6')),
    check: svg('icon-sm', path('M5 12l5 5 9-10')),
    flag: svg('icon-sm', path('M6 20V4h10l-2 4 2 4H6')),
    link: svg('icon-sm', path('M10 14l4-4M8.5 11.5l-2 2a3 3 0 004 4l2-2M15.5 12.5l2-2a3 3 0 00-4-4l-2 2')),
    // 侧栏
    panelLeft: svg('icon-md', PANEL + path('M9 5v14')),
    backward: svg('icon-sm', path('M15 6l-6 6 6 6'), 2),
    forward: svg('icon-sm', path('M9 6l6 6-6 6'), 2),
    bookMore: svg('icon-sm faint', path('M6 9l6 6 6-6')),
    newFolder: svg('icon-sm', path('M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z') + path('M12 10.5v5M9.5 13h5')),
    closed: svg('icon-xs', path('M9 6l6 6-6 6'), 2),
    opened: svg('icon-xs', path('M6 9l6 6 6-6'), 2),
    file: svg('icon-sm', path('M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z') + path('M14 3v5h5')),
    bin: svg('icon-md', path('M5 7h14M10 7V5h4v2M7 7l1 12h8l1-12')),
    gear: svg('icon-md', path('M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z') + '<circle cx="12" cy="12" r="3"></circle>'),
    // 正文工具栏
    crumb: svg('icon-xs faint', path('M9 6l6 6-6 6'), 2),
    flagMd: svg('icon-md', path('M6 20V4h10l-2 4 2 4H6')),
    findMd: svg('icon-md', '<circle cx="11" cy="11" r="6"></circle>' + path('M20 20l-4.5-4.5')),
    more: svg('icon-md', path('M5 12h.01M12 12h.01M19 12h.01')),
    panelRight: svg('icon-md', PANEL + path('M15 5v14')),
    focus: svg('icon-md', path('M15 3h6v6M9 21H3v-6M21 3l-7 7M3 21l7-7')),
  }
  const DOT = '<div class="dot"></div>'
  const HAIR = '<div class="hair"></div>'

  // 输入框。change 拿到的是打完以后的字。
  const typed = (value, change, hint, label, more = '') => `<input class="in" value="${t(value)}" placeholder="${t(hint)}" aria-label="${t(label)}"${onInput(change)}${more}>`
  const line = (value, change, hint, label) => `<div class="box line">${typed(value, change, hint, label)}</div>`
  const field = (label, box) => `<div class="field"><div class="label">${t(label)}</div>${box}</div>`
  const button = (label, act, cls = '') => `<div class="btn hit ${cls}"${onClick(act)}>${t(label)}</div>`
  const buttons = (...bs) => `<div class="buttons">${bs.join('')}</div>`
  const where = (text, find) => `<div class="where"><div class="fit ell">${t(text)}</div><div class="see hit"${onClick(find)}>看原文</div></div>`
  // 按回车时做 ok，按 Esc 时做 no。拼音还没选完字的回车不算。
  const keys = (ok, no) => onKey((ev) => { if (ev.key === 'Enter' && !ev.isComposing) ok(); else if (ev.key === 'Escape' && no) no() })

  // 一段字，搜的词标出来。
  function marked(text, find) {
    if (!find) return t(text)
    return text.split(find).map(t).join(`<span class="found">${t(find)}</span>`)
  }
  // 从搜的词前面一点开始，免得一行截断后看不到它。
  function near(text, find) {
    const i = find ? text.indexOf(find) : -1
    return i > 8 ? '…' + text.slice(i - 6) : text
  }

  // 列表顶上：搜索、旁边的按钮（add）、「···」里的几项（more）。
  function tool(hint, key, add, more) {
    return '<div class="tool" data-k="tool">' +
      `<div class="find box">${ICON.find}${typed(S[key], (v) => set({ [key]: v }), hint, hint)}</div>` +
      (add ? `<div class="add hit"${onClick(add[1])}>${ICON.plus}${t(add[0])}</div>` : '') +
      (more ? `<div class="more hit${S.menu ? ' on' : ''}" title="更多"${onClick(() => set({ menu: !S.menu }))}>${ICON.dots}</div>` : '') +
      (more && S.menu ? `<div class="tool-menu pop">${more.map(([label, n, pick]) => `<div class="pop-row row"${onClick(pick)}><div class="grow">${t(label)}</div><div class="side">${n || ''}</div></div>`).join('')}</div>` : '') +
      '</div>'
  }
  // 详情页顶上：返回哪里，右边还能做什么。
  function back(label, goBack, actLabel, act) {
    return `<div class="back" data-k="back"><div class="back-to hit"${onClick(goBack)}>${ICON.left}<div class="ell">${t(label)}</div></div><div class="grow"></div>` +
      (act ? `<div class="hit"${onClick(act)}>${t(actLabel)}</div>` : '') + '</div>'
  }
  // 列表里的几种行。
  const rows = (list) => `<div class="rows scroll" data-k="rows">${list.join('')}</div>`
  const head = (label) => `<div class="head">${t(label)}</div>`
  const empty = (words, how) => `<div class="empty"><div>${t(words)}</div>${how ? `<div class="note">${t(how)}</div>` : ''}</div>`
  const todo = (label, n, open) => `<div class="todo row"${onClick(open)}>${DOT}<div class="grow">${t(label)}</div><div class="side">${n}</div>${ICON.right}</div>`
  const item = ({ name, about = '', find = '', dot, side, open }) => `<div class="item row"${onClick(open)}><div class="name ell">${t(name)}</div><div class="about ell">${marked(about, find)}</div>${dot ? DOT : ''}${side ? `<div class="side">${t(side)}</div>` : ''}${ICON.right}</div>`

  // ───────── 画：侧栏 ─────────

  function sidebar() {
    const nav = [['讨论', 'M5 5h14v10H10l-4 4v-4H5z'], ['正文', 'M6 4h11a1 1 0 011 1v15H7a2 2 0 01-2-2V5a1 1 0 011-1zM5 17a2 2 0 012-2h11'], ['作品分类', 'M4 12V4h8l8 8-8 8z']]
    const leaf = (c) => `<div class="tree-row hit${c.vol ? ' deep' : ''}${c.key === S.ch ? ' on' : ''}"${onClick(goTo(c.key))}>${ICON.file}<div class="fit ell">${t(titleOf(c))}</div><div class="count">${t(c.c)}</div></div>`
    const volume = ([key, title, count]) => `<div class="tree-row hit"${onClick(() => set((s) => ({ folds: { ...s.folds, [key]: !s.folds[key] } })))}>${S.folds[key] ? ICON.closed : ICON.opened}<div class="fit ell">${t(title)}</div><div class="count">${t(count)}</div></div>` +
      (S.folds[key] ? '' : BOOK.chapters.filter((c) => c.vol === key).map(leaf).join(''))
    return '<div class="sidebar">' +
      `<div class="sidebar-top"><div>${ICON.panelLeft}</div><div>${ICON.backward}</div><div>${ICON.forward}</div></div>` +
      `<div class="book"><div class="book-logo">雨</div><div class="book-names"><div class="book-name">雨城来信</div><div class="book-kind">网文</div></div>${ICON.bookMore}</div>` +
      `<div class="navs">${nav.map(([name, d]) => `<div class="nav${name === '正文' ? ' on' : ''}">${svg('icon-md', path(d))}<div class="grow">${name}</div></div>`).join('')}</div>` +
      `<div class="tree-head"><div>章节</div><div class="tree-head-buttons"><button aria-label="新建文件夹" title="新建文件夹">${ICON.newFolder}</button><button aria-label="新建章节" title="新建章节">${ICON.plus}</button></div></div>` +
      `<div class="tree">${BOOK.volumes.map(volume).join('')}${BOOK.chapters.filter((c) => !c.vol).map(leaf).join('')}</div>` +
      '<div class="grow"></div>' +
      `<div class="sidebar-end"><div class="bin">${ICON.bin}<div>回收站</div></div><div class="gear" title="设置">${ICON.gear}</div></div>` +
      '</div>'
  }

  // ───────── 画：正文 ─────────

  // 一段切成几块：里面的伏笔、正在看的那条、选中的字。重叠的只留先到的。
  function pieces(cur, i, text, focus) {
    const cuts = []
    const cut = (quote, para, cls, id) => {
      const at = para === undefined || para === i ? text.indexOf(quote) : -1
      if (at >= 0) cuts.push({ a: at, b: at + quote.length, cls, id })
    }
    if (S.pick && S.pick.para === i) cuts.push({ a: S.pick.start, b: S.pick.end, cls: 'sel', id: '' })
    if (S.flash && S.flash.key === cur.key) cut(S.flash.quote, undefined, 'sel', '')
    for (const m of S.marks) {
      const cls = m.id === focus ? 'fs on' : 'fs'
      if ((m.st === 'open' || m.st === 'done') && m.key === cur.key) cut(m.quote, m.para, cls, m.id)
      if (m.st === 'done' && m.pay && m.pay.key === cur.key) cut(m.pay.quote, m.pay.para, cls, m.id)
    }
    const kept = []
    for (const c of cuts) if (!kept.some((k) => c.a < k.b && k.a < c.b)) kept.push(c)
    kept.sort((x, y) => x.a - y.a)
    const span = (words, c) => `<span${c && c.id ? ` data-mark="${c.id}"` : ''}${c ? ` class="${c.cls}"` : ''}>${t(words)}</span>`
    let out = '', at = 0
    for (const c of kept) {
      if (c.a > at) out += span(text.slice(at, c.a))
      out += span(text.slice(c.a, c.b), c)
      at = c.b
    }
    return at < text.length ? out + span(text.slice(at)) : out
  }

  function editor(cur) {
    const volume = BOOK.volumes.find(([key]) => key === cur.vol)
    const here = S.marks.filter((m) => m.st === 'open' && m.key === cur.key).length
    const focus = S.paneOpen && S.tab === '伏笔' ? S.fid : null
    const idle = (label, title, icon) => `<button class="icon-btn" aria-label="${label}" title="${title}">${icon}</button>`
    return '<div class="editor">' +
      `<div class="toolbar${S.paneOpen ? '' : ' wide'}">` +
      (volume ? `<div class="crumb">${t(volume[1])}</div>${ICON.crumb}` : '') +
      `<div class="crumb here">${t(titleOf(cur))}</div><div class="count-all ell">本章 ${t(cur.c)} 字 · 全书 ${t(BOOK.words)}</div><div class="grow"></div><div class="tools">` +
      (!S.paneOpen && here ? `<button class="fs-btn" aria-label="本章伏笔"${onClick(() => set({ paneOpen: true, tab: '伏笔', fid: null, fv: 'list' }))}>${ICON.flagMd}<span>本章伏笔 ${here}</span></button>` : '') +
      idle('查找替换', '查找替换（Ctrl+F）', ICON.findMd) + idle('更多', '续写、起草、快照、导出和外观', ICON.more) +
      (S.paneOpen ? '' : `<button class="icon-btn" aria-label="展开右栏" title="展开右栏"${onClick(() => set({ paneOpen: true }))}>${ICON.panelRight}</button>`) +
      idle('专注写作', '专注写作', ICON.focus) + '</div></div>' +
      `<div class="paper"${onMenu(openMenu)}><div class="column"><div class="chapter-head"><div class="chapter-title">${t(cur.t)}</div><div class="chapter-more"><div>章纲 ▾</div><div>本章摘要 ▾</div></div></div>` +
      `<div class="prose">${cur.paras.map((text, i) => `<div data-para="${i}">${pieces(cur, i, text, focus)}</div>`).join('')}</div></div></div>` +
      '</div>'
  }

  // ───────── 画：右栏 · 设定 ─────────

  function setting() {
    const at = (place) => S.entries.filter((e) => e.place === place)
    const body = at('body'), open = at('open'), gone = at('dropped')
    const shown = S.entries.find((e) => e.id === S.sid)
    const row = (e, find = '') => item({ name: e.names[0] || e.text, about: e.names.length ? near(e.text, find) : '', find, dot: e.place === 'body' && open.some((o) => o.about === e.id), open: () => set({ sid: e.id }) })
    const idea = (o, about) => `<div class="idea">${about ? `<div class="idea-about">${t(about)}</div>` : ''}<div class="text">${t(o.text)}</div>` +
      buttons(button('忽略', () => move(o.id, 'dropped')), button('采纳', () => move(o.id, 'body'), 'primary')) + '</div>'

    if (S.form) return back(S.form.id ? shown.names[0] || shown.cat : '全部设定', () => set({ form: null, cats: false })) + form(S.form)

    if (shown) {
      const dropped = shown.place === 'dropped'
      const about = open.filter((o) => o.about === shown.id)
      const from = shown.src ? chapterOf(shown.src.key) : null
      const edit = () => set({ form: { id: shown.id, name: shown.names[0] || '', names: shown.names.slice(1).join('、'), text: shown.text, cat: shown.cat } })
      return back(dropped ? '已删除' : '全部设定', () => set({ sid: null }), dropped ? '恢复设定' : '删除设定', () => move(shown.id, dropped ? 'body' : 'dropped')) +
        '<div class="page scroll" data-k="entry">' +
        `<div class="title-line"><div class="title ell">${t(shown.names[0] || shown.cat)}</div>${shown.names.length > 1 ? `<div class="alias ell">别名 ${t(shown.names.slice(1).join('、'))}</div>` : ''}<div class="grow"></div>${button('修改', edit)}</div>` +
        `<div class="text${dropped ? ' dropped' : ''}">${t(shown.text)}</div>` +
        (from ? where('第 ' + from.no + ' 章“' + shown.src.quote + '”', goTo(from.key, shown.src)) : '') +
        (about.length ? HAIR + `<div class="ai-head">${DOT}AI 建议</div>` + about.map((o) => idea(o)).join('') : '') +
        '</div>'
    }

    if (S.sv === 'ideas') {
      const of = (o) => S.entries.find((e) => e.id === o.about)
      return back('全部设定', () => set({ sv: 'list' })) +
        `<div class="page scroll" data-k="ideas"><div class="title-line"><div class="title">等你决定</div><div class="grow"></div><div class="side">${open.length} 条 AI 建议</div></div>` +
        open.map((o, i) => (i ? HAIR : '') + idea(o, of(o) ? '关于 ' + of(o).names[0] : '新设定')).join('') + '</div>'
    }

    if (S.sv === 'dropped') return back('全部设定', () => set({ sv: 'list' })) + rows([head('已删除'), ...(gone.length ? gone.map((e) => row(e)) : [empty('没有删除过设定')])])

    if (S.sv === 'log') {
      const card = (c) => '<div class="card">' +
        `<div class="card-head"><div class="who">${t(c.who)}</div>${c.model ? `<div class="tiny">${t(c.model)}</div>` : ''}<div class="grow"></div><div class="tiny">${t(c.at)}</div></div>` +
        c.lines.map((l) => `<div class="said-line">${t(l)}</div>`).join('') +
        (c.extra ? `<div class="extra">${t(c.extra)}</div>` : '') +
        (c.fail ? `<div class="fail">${t(c.fail)}</div><div class="retry hit"${onClick(() => retry(c.id))}>重试</div>` : '') + '</div>'
      return back('全部设定', () => set({ sv: 'list' })) + `<div class="page log scroll" data-k="log"><div class="title">修改记录</div>${S.log.map(card).join('')}</div>`
    }

    const q = S.sq.trim()
    let list = []
    if (q) {
      const found = body.filter((e) => (e.names.join('、') + e.text).includes(q))
      list = found.length ? [head('找到 ' + found.length + ' 条'), ...found.map((e) => row(e, q))] : [empty('没有找到')]
    } else if (!body.length && !open.length) {
      list = [empty('还没有设定', '聊过、写过的会自动记到这里，也可以点「新增」自己记一条。')]
    } else {
      if (open.length) list.push(todo('等你决定', open.length, () => set({ sv: 'ideas' })))
      for (const cat of BOOK.cats) {
        const of = body.filter((e) => e.cat === cat)
        if (of.length) list.push(head(cat), ...of.map((e) => row(e)))
      }
    }
    return tool('搜索设定', 'sq', ['新增', () => set({ form: { id: null, name: '', names: '', text: '', cat: BOOK.cats[0] }, menu: false })],
      [['已删除', gone.length, () => set({ sv: 'dropped', menu: false })], ['修改记录', '', () => set({ sv: 'log', menu: false })]]) + rows(list)
  }

  // 新增和修改用同一张表。新增先填内容，修改先看名字。
  function form(f) {
    const type = (key) => (value) => set((s) => ({ form: { ...s.form, [key]: value } }))
    const name = field(f.id ? '名字' : '名字（选填）', line(f.name, type('name'), '', '名字'))
    const names = field(f.id ? '别名' : '别名（选填）', line(f.names, type('names'), '多个用顿号隔开', '别名'))
    const text = field('内容', `<div class="box area"><textarea class="in" rows="4" aria-label="内容"${onInput(type('text'))}>${t(f.text)}</textarea></div>`)
    const cat = (label) => `<div class="pop-row row${label === f.cat ? ' on' : ''}"${onClick(() => set((s) => ({ cats: false, form: { ...s.form, cat: label } })))}>${label}</div>`
    return '<div class="page scroll" data-k="form">' +
      (f.id ? name + names + text : '<div class="title-line"><div class="title">新增设定</div></div>' + text + name + names) +
      `<div class="field cat"><div class="label">分类</div><div class="box line hit"${onClick(() => set({ cats: !S.cats }))}>${t(f.cat)}<div class="grow"></div>${ICON.down}</div>` +
      (S.cats ? `<div class="cat-menu pop">${BOOK.cats.map(cat).join('')}</div>` : '') + '</div>' +
      buttons(button('取消', () => set({ form: null, cats: false })), button('保存', save, f.text.trim() ? 'primary' : 'primary off')) +
      '</div>'
  }

  // ───────── 画：右栏 · 伏笔 ─────────

  const nameOf = (m) => m.note || clip(m.quote, 14)

  function foreshadowing() {
    const of = (st) => S.marks.filter((m) => m.st === st)
    const groups = { lost: '原句已删', done: '已回收', dropped: '已放弃' }
    const row = (m, find = '') => item({ name: nameOf(m), about: find && !m.note.includes(find) ? near(m.quote, find) : '', find, side: m.key === S.ch ? '本章' : '第 ' + m.ch + ' 章', open: () => set({ fid: m.id, renote: null }) })
    const shown = S.marks.find((m) => m.id === S.fid)

    if (shown) {
      const st = shown.st, gap = BOOK.now - shown.ch
      const act = { open: ['放弃伏笔', () => ({ st: 'dropped' })], done: ['取消回收', () => ({ st: 'open', pay: null })], dropped: ['恢复伏笔', () => ({ st: 'open' })] }[st]
      const remove = () => set((s) => ({ marks: s.marks.filter((m) => m.id !== shown.id), fid: null, renote: null }))
      const saveNote = () => mark(shown.id, () => ({ note: S.renote.trim() || shown.note }))
      return back('全部伏笔', () => set({ fid: null, renote: null }), act ? act[0] : '删除伏笔', act ? () => mark(shown.id, act[1]) : remove) +
        '<div class="page scroll" data-k="mark">' +
        (S.renote !== null
          ? `<div class="title-line"><div class="box line fit">${typed(S.renote, (v) => set({ renote: v }), '备注', '改伏笔备注', keys(saveNote, () => set({ renote: null })) + ' autofocus')}</div>${button('保存', saveNote, 'primary')}</div>`
          : `<div class="title-line"><div class="title ell">${t(nameOf(shown))}</div><div class="grow"></div>${st === 'lost' ? '' : button('改备注', () => set({ renote: shown.note }))}</div>`) +
        where(st === 'lost' ? '第 ' + shown.ch + ' 章的原句已删除' : '第 ' + shown.ch + ' 章埋下' + (st === 'open' && gap > 0 ? '，已过 ' + gap + ' 章' : ''), goTo(shown.key)) +
        `<div class="quote${st === 'lost' ? ' gone' : ''}">${t(shown.quote)}</div>` + HAIR +
        (st === 'open' ? '<div class="state">未回收</div><div class="note">先在正文选中回收的段落</div>' + buttons(button('记为回收', () => askPay(shown.id), 'primary')) : '') +
        (st === 'done' ? where('第 ' + shown.pay.ch + ' 章回收，相隔 ' + Math.max(shown.pay.ch - shown.ch, 0) + ' 章', goTo(shown.pay.key)) + `<div class="quote">${t(shown.pay.quote)}</div>` : '') +
        (st === 'lost' ? '<div class="note">先在正文选中新的句子</div>' + buttons(button('换个原句', () => rehang(shown.id), 'primary')) : '') +
        (st === 'dropped' ? '<div class="state">已放弃</div>' : '') +
        '</div>'
    }

    if (S.fv !== 'list' && of(S.fv).length) return back('全部伏笔', () => set({ fv: 'list' })) + rows([head(groups[S.fv]), ...of(S.fv).map((m) => row(m))])

    const q = S.fq.trim()
    let list = []
    if (q) {
      const found = S.marks.filter((m) => (m.note + m.quote).includes(q))
      list = found.length ? [head('找到 ' + found.length + ' 条'), ...found.map((m) => row(m, q))] : [empty('没有找到')]
    } else if (!S.marks.length) {
      list = [empty('还没有伏笔', '在正文选中一句，右键「记为伏笔」')]
    } else {
      const lost = of('lost'), long = of('open').filter((m) => BOOK.now - m.ch >= BOOK.late), rest = of('open').filter((m) => BOOK.now - m.ch < BOOK.late)
      if (lost.length) list.push(todo(groups.lost, lost.length, () => set({ fv: 'lost' })))
      if (long.length) list.push(head('久未回收'), ...long.map((m) => row(m)))
      if (rest.length) list.push(head('未回收'), ...rest.map((m) => row(m)))
    }
    return tool('搜索伏笔', 'fq', null, ['done', 'dropped'].map((st) => [groups[st], of(st).length, () => set({ fv: st, menu: false })])) + rows(list)
  }

  // ───────── 画：右栏 · 讨论 ─────────

  function discussion() {
    if (S.tv === 'all') {
      const q = S.tq.trim()
      const found = S.talks.filter((talk) => !q || talk.title.includes(q) || talk.msgs.some((m) => m.text.includes(q)))
      const row = (talk) => item({ name: talk.title, side: talk.at, open: () => set({ tv: 'one', tid: talk.id, draft: '' }) })
      return tool('搜索讨论', 'tq', ['新讨论', () => set({ tv: 'new', tid: null, draft: '' })]) +
        rows(found.length ? [head(q ? '找到 ' + found.length + ' 条' : '最近'), ...found.map(row)] : [empty(q ? '没有找到' : '还没有讨论')])
    }
    const talk = S.tv === 'one' ? S.talks.find((x) => x.id === S.tid) : null
    const hint = talk ? '输入消息' : '想聊什么'
    return back('全部讨论', () => set({ tv: 'all' })) +
      (talk
        ? `<div class="talk scroll" id="talk">${talk.msgs.map((m) => `<div class="${m.me ? 'said' : 'reply'}">${t(m.text)}</div>`).join('')}${S.waiting ? '<div class="waiting">正在回复…</div>' : ''}</div>`
        : `<div class="asks" data-k="asks">${head('试试这样问')}${BOOK.asks.map((text) => `<div class="item row"${onClick(() => send(text))}><div class="name ell">${t(text)}</div><div class="grow"></div>${ICON.right}</div>`).join('')}</div>`) +
      '<div class="composer" data-k="composer">' +
      `<div class="box send-box">${typed(S.draft, (v) => set({ draft: v }), hint, hint, keys(() => send()))}<div class="send hit${S.draft.trim() && !S.waiting ? '' : ' off'}" title="发送"${onClick(() => send())}>${ICON.send}</div></div>` +
      `<div class="attach-line"><div class="attach hit"${onClick(() => set({ attach: !S.attach }))}><div class="check${S.attach ? ' on' : ''}">${S.attach ? ICON.check : ''}</div>附上本章内容</div><div class="grow"></div><div class="model">Opus 5.5</div></div>` +
      '</div>'
  }

  function pane() {
    const waiting = S.entries.filter((e) => e.place === 'open').length, lost = S.marks.filter((m) => m.st === 'lost').length
    const tab = (name, n) => `<div class="tab hit${name === S.tab ? ' on' : ''}"${onClick(() => set({ tab: name, ...SHUT }))}>${name}${n ? `<span class="badge">${n}</span>` : ''}</div>`
    return '<div id="pane">' +
      `<div class="tabs">${tab('设定', waiting)}${tab('讨论')}${tab('伏笔', lost)}<div class="grow"></div><button class="icon-btn" aria-label="收起右栏" title="收起右栏"${onClick(() => set({ paneOpen: false, ...SHUT }))}>${ICON.panelRight}</button></div>` +
      (S.tab === '设定' ? setting() : S.tab === '伏笔' ? foreshadowing() : discussion()) +
      // 菜单开着时，点别处先把它收起来。
      (S.menu || S.cats ? `<div class="veil"${onClick(() => set(SHUT))}></div>` : '') +
      '</div>'
  }

  // ───────── 画：正文上浮着的 ─────────

  function floats() {
    if (!S.ctx && !S.bury && !S.pay) return ''
    const win = document.getElementById('win'), side = document.querySelector('.sidebar'), right = document.getElementById('pane')
    // 贴着选中的字放：下面放不下就放上面，左右不出正文这一栏。
    const by = (wide, tall) => {
      const p = S.pick, edge = win.offsetWidth - (right ? right.offsetWidth : 0)
      const top = p.bottom + 6 + tall > win.offsetHeight - 8 && p.top - tall - 6 >= 44 ? p.top - tall - 6 : p.bottom + 6
      return `left:${Math.max(side.offsetWidth + 8, Math.min(p.left, edge - wide - 8))}px;top:${top}px`
    }
    const veil = `<div class="veil float-veil"${onClick(closeFloat)}${onMenu((ev) => { ev.preventDefault(); closeFloat() })}></div>`

    if (S.ctx) {
      const items = [['记为伏笔…', S.pick ? () => set({ ctx: null, bury: { note: '' } }) : null], ['记为回收…', S.pick ? () => askPay('') : null]]
      if (S.ctx.mark) items.push(['查看伏笔', () => set({ ctx: null, pick: null, tab: '伏笔', fid: S.ctx.mark, fv: 'list', renote: null, paneOpen: true, ...SHUT })])
      // 菜单宽 160，三行高 108，都留在窗口里。
      return veil + `<div class="ctx pop" data-k="ctx" style="left:${Math.min(S.ctx.x, win.offsetWidth - 168)}px;top:${Math.min(S.ctx.y, win.offsetHeight - 108)}px">` +
        items.map(([label, pick]) => `<div class="pop-row row${pick ? '' : ' off'}"${onClick(pick)}>${label}</div>`).join('') + '</div>'
    }

    if (S.bury) {
      return veil + `<div class="float bury" data-k="bury" style="${by(360, 132)}"><div class="float-head">${ICON.flag}记为伏笔，写句备注</div>` +
        `<div class="bury-box">${typed(S.bury.note, (v) => set({ bury: { note: v } }), '比如：小祥为什么突然改口', '伏笔备注', keys(buryNow, () => set({ bury: null, pick: null })) + ' autofocus')}</div>` +
        `<div class="float-end"><div class="ghost hit"${onClick(closeFloat)}>取消</div><div class="solid hit"${onClick(buryNow)}>记下</div></div></div>`
    }

    const q = S.pay.q.trim()
    const can = S.marks.filter((m) => (m.st === 'open' || m.st === 'done') && (!q || m.note.includes(q) || m.quote.includes(q)))
    const id = can.some((m) => m.id === S.pay.id) ? S.pay.id : can.length ? can[0].id : ''
    const ago = (m) => (BOOK.now - m.ch > 0 ? BOOK.now - m.ch + ' 章前埋下' : '本章埋下')
    return veil + `<div class="float pay" data-k="pay" style="${by(380, 340)}"><div class="float-head">${ICON.link}回收的是哪个伏笔</div>` +
      `<div class="pay-find">${typed(S.pay.q, (v) => set({ pay: { q: v, id } }), '搜备注或原句', '找伏笔', keys(() => payNow(id), () => set({ pay: null, pick: null })) + ' autofocus')}</div>` +
      (can.length ? '' : `<div class="pay-none">${q ? '没有叫这个的伏笔' : '还没有埋下的伏笔。先选中一句，右键「记为伏笔」。'}</div>`) +
      `<div class="pay-list scroll">${can.map((m) => `<div class="pay-row row${m.id === id ? ' on' : ''}"${onClick(() => set((s) => ({ pay: { ...s.pay, id: m.id } })))}><div class="pay-name ell">${t(nameOf(m))}</div><div class="pay-meta">第${m.ch}章 · ${ago(m)}</div></div>`).join('')}</div>` +
      `<div class="float-end"><div class="ghost hit"${onClick(closeFloat)}>取消</div><div class="solid hit${id ? '' : ' off'}" title="确定回收"${onClick(() => payNow(id))}>记为回收</div></div></div>`
  }

  // ───────── 画：整个窗口 ─────────

  const WINDOW_BUTTONS = '<div class="window-buttons">' + ['<path d="M0 5h10"></path>', '<rect x=".5" y=".5" width="9" height="9" rx="1"></rect>', '<path d="M.5.5l9 9M9.5.5l-9 9"></path>']
    .map((d) => `<div><svg class="icon-xs" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1">${d}</svg></div>`).join('') + '</div>'

  function view() {
    return '<div class="stage">' +
      `<div class="stage-bar">${button('重新开始', restart)}${button(S.dark ? '浅色' : '深色', () => set({ dark: !S.dark }))}<div class="stage-note">内容是假的，正文不能打字，AI 只回固定的几句。</div></div>` +
      `<div id="win" class="${S.dark ? 'dark' : 'light'}">` + WINDOW_BUTTONS + sidebar() + editor(chapterOf(S.ch)) + (S.paneOpen ? pane() : '') + floats() +
      (S.toast ? `<div id="toast">${t(S.toast)}</div>` : '') +
      '</div></div>'
  }

  // ───────── 把画好的换到页面上 ─────────

  const root = document.getElementById('app')
  let born = null // 这一遍新放进页面的、要把光标放进去的输入框

  // 有 id 或 data-k 的元素认名字，换了位置也还是它；别的按顺序对。
  const keyOf = (n) => (n.nodeType === 1 ? n.id || n.getAttribute('data-k') : null)

  // 让 old 变成 next 的样子：只改不一样的属性和字，输入框里正打着的字、正文里选中的字都不受打扰。
  function patch(old, next) {
    if (old.nodeType !== 1) { if (old.data !== next.data) old.data = next.data; return }
    for (const { name } of [...old.attributes]) if (!next.hasAttribute(name)) old.removeAttribute(name)
    for (const { name, value } of [...next.attributes]) if (old.getAttribute(name) !== value) old.setAttribute(name, value)
    if (old.nodeName === 'INPUT' || old.nodeName === 'TEXTAREA') { if (old.value !== next.value) old.value = next.value; return }
    children(old, next)
  }

  function children(parent, want) {
    const named = new Map()
    for (const c of parent.children) if (keyOf(c)) named.set(keyOf(c), c)
    let at = parent.firstChild
    for (const w of [...want.childNodes]) {
      const key = keyOf(w)
      let have = null
      if (key) { have = named.get(key) ?? null; named.delete(key) } else if (at && !keyOf(at) && at.nodeName === w.nodeName) have = at
      if (have && have.nodeName !== w.nodeName) have = null
      if (!have) {
        parent.insertBefore(w, at)
        if (w.nodeType === 1) born = (w.matches('[autofocus]') ? w : w.querySelector('[autofocus]')) || born
      } else {
        if (have === at) at = at.nextSibling; else parent.insertBefore(have, at)
        patch(have, w)
      }
    }
    while (at) { const gone = at; at = at.nextSibling; gone.remove() }
  }

  function draw() {
    H = []
    born = null
    const want = document.createElement('template')
    want.innerHTML = view()
    children(root, want.content)
    if (born) { born.focus(); born.setSelectionRange(born.value.length, born.value.length) }
    const talk = document.getElementById('talk') // 讨论总是停在最新一句
    if (talk) talk.scrollTop = talk.scrollHeight
  }

  // 页面上只挂这四个听的：事情出在哪个元素上，就照它身上的号码去找动作。
  for (const [event, kind] of [['click', 'click'], ['keydown', 'keydown'], ['contextmenu', 'menu']]) {
    root.addEventListener(event, (ev) => { const el = ev.target.closest(`[data-${kind}]`); if (el) H[el.dataset[kind]](ev) })
  }
  root.addEventListener('input', (ev) => { const el = ev.target.closest('[data-input]'); if (el) H[el.dataset.input](el.value) })

  draw()
})()
