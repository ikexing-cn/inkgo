// 右栏：设定、伏笔、讨论三页。
'use strict'

// ───────── 动作 ─────────

// 作者对设定做的事，记在修改记录最上面。
const did = (s, said) => [{ id: 'l' + s.n, who: '手动修改', model: '', at: stamp(), lines: [said] }, ...s.log]
// 状态里存的是中文（记录的人、日期、位置、废弃的理由），显示时换成当前语言。
const WHO = () => ({ 手动修改: L.loreWhoManual(), 讨论: L.paneTabChat() })

const whereWord = (s) => ({ 接在章末: L.chatWhereEnd(), 换掉全章: L.chatWhereAll(), 换掉一句: L.chatWhereSwap() })[s] || s
const FATE = () => ({ 已被新的提议替换: L.chatFateReplaced(), 已采纳: L.chatFateAdopted(), '正文已改动，提议已作废': L.chatFateChanged(), 已放弃: L.hooksDropped(), '关闭 AI 时已丢弃': L.chatFateAiOff(), '作品关闭时已丢弃': L.chatFateBookClosed() })

function move(id, place) {
  set((s) => {
    const e = s.entries.find((x) => x.id === id)
    const entries = s.entries.map((x) => (x.id === id ? { ...x, place, about: undefined } : x))
    const left = entries.some((x) => x.place === 'open')
    return {
      entries, n: s.n + 1, log: did(s, L.loreMovedTo({ body: L.loreWhereBody(), dropped: L.loreDropped() }[place], clip(e.text, 24))),
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
    if (!f.id) { // 回答了的提问不再等作者，跟着答案留一行
      const e = { id: 'e' + s.n, place: 'body', cat: f.cat, names, text, ...(f.ask ? { asked: f.q } : {}) }
      const entries = [...s.entries.filter((x) => x.id !== f.ask), e]
      return { entries, n: s.n + 1, form: null, ...SHUT, log: did(s, L.loreAddedActive(f.cat, text)), sv: s.sv === 'ideas' && !entries.some((x) => x.place === 'open') ? 'list' : s.sv }
    }
    const old = s.entries.find((x) => x.id === f.id)
    const said = old.text !== text ? L.loreRewritten(text) : L.loreRecat(f.cat, clip(text, 24))
    // 名字改了，正文里还写着旧名的话，问一句要不要全书替换。
    const was = old.names[0] || '', n = was && names[0] && was !== names[0] ? s.chapters.reduce((sum, c) => sum + hits(c, was).length, 0) : 0
    return { entries: s.entries.map((x) => (x.id === f.id ? { ...x, names, text, cat: f.cat } : x)), n: s.n + 1, form: null, ...SHUT, log: did(s, said), named: n ? { was, now: names[0], n } : null }
  })
}
// 记下、改掉、删掉一条设定的一处变化：从哪一章起，那以后的全文。
function saveChange() {
  set((s) => {
    const f = s.form, text = f.text.trim(), e = s.entries.find((x) => x.id === f.id)
    if (!text) return null
    const changes = [...(e.changes || []).filter((g) => g.id !== f.gid && g.key !== f.key), { id: f.gid || 'g' + s.n, key: f.key, text }]
    return { entries: s.entries.map((x) => (x.id === f.id ? { ...x, changes } : x)), n: s.n + 1, form: null, ...SHUT, log: did(s, L.loreChangeLog(e.names[0] || e.cat, fromWhere(f.key), clip(text, 24))) }
  })
}
function dropChange() {
  set((s) => {
    const f = s.form, e = s.entries.find((x) => x.id === f.id), g = e.changes.find((x) => x.id === f.gid)
    return { entries: s.entries.map((x) => (x.id === f.id ? { ...x, changes: x.changes.filter((y) => y.id !== f.gid) } : x)), n: s.n + 1, form: null, ...SHUT, log: did(s, L.loreChangeRemoved(e.names[0] || e.cat, fromWhere(g.key))) }
  })
}
// 忽略 AI 建议的一处变化：它不是一条设定，不进已删除；点错了在提示条上撤销。
function dropIdea(o) {
  const was = { entries: S.entries, log: S.log }
  set((s) => {
    const e = s.entries.find((x) => x.id === o.about), entries = s.entries.filter((x) => x.id !== o.id)
    return { entries, n: s.n + 1, log: did(s, L.loreChangeRemoved(e.names[0] || e.cat, fromWhere(o.from))), sv: s.sv === 'ideas' && !entries.some((x) => x.place === 'open') ? 'list' : s.sv }
  })
  const id = say(L.loreIgnored(), '', 'ok', [[L.cmdUndo(), () => { unsay(id); set(was) }]])
}
// 采纳 AI 建议的一处变化：记到它说的那一条上，从它说的那一章起。
// 那一章拿这一条对照查出来的不一致，设定从那一章起改了，就不再成立，一起拿掉。点错了在提示条上撤销，两样一起回来。
function takeChange(o) {
  const was = { entries: S.entries, checks: S.checks, log: S.log }
  set((s) => {
    const e = s.entries.find((x) => x.id === o.about), changes = [...(e.changes || []).filter((g) => g.key !== o.from), { id: 'g' + s.n, key: o.from, text: o.text }]
    const entries = s.entries.filter((x) => x.id !== o.id).map((x) => (x.id === e.id ? { ...x, changes } : x))
    const checks = s.checks.filter((c) => !(c.kind === '不一致' && c.entry === e.id && c.key === o.from))
    return { entries, checks, n: s.n + 1, log: did(s, L.loreChangeLog(e.names[0] || e.cat, fromWhere(o.from), clip(o.text, 24))), sv: s.sv === 'ideas' && !entries.some((x) => x.place === 'open') ? 'list' : s.sv }
  })
  const e = S.entries.find((x) => x.id === o.about), id = say(L.loreNotedTo(e.names[0] || e.cat, fromWhere(o.from)), '', 'ok', [[L.cmdUndo(), () => { unsay(id); set(was) }]])
}
// 记一处变化的表：编辑框里先放这一条在那一章之前的样子，改完就是从那一章起的全文。
const changing = (e, g, key) => { const before = S.chapters[rank(key) - 1]; return { change: true, id: e.id, gid: g ? g.id : '', key, text: g ? g.text : before ? textAt(e, before.key) : e.text } }

// 打开一条设定的修改表。
// AI 的提问：以问号结尾的建议。
const asks = (e) => /[？?]$/.test(e.text.trim())
const editing = (e) => ({ id: e.id, name: e.names[0] || '', names: e.names.slice(1).join('、'), text: e.text, cat: e.cat })

const mark = (id, change) => set((s) => ({ marks: s.marks.map((m) => (m.id === id ? { ...m, ...change(m) } : m)), renote: null }))

// 原句删掉了的伏笔，挂到新选中的句子上。
function rehang(id) {
  const pick = grab()
  if (!pick) { say(L.hooksSelectFirst()); return }
  const ch = chapterOf(S.ch)
  mark(id, () => ({ st: 'open', ch: ch.no, key: ch.key, para: pick.para, quote: pick.text }))
  unselect()
}

// 点一句示例：填进输入框，由作者自己发出去。
const offer = (words) => () => { set({ draft: words }); const box = document.querySelector('.send-box .in'); if (box) box.focus() }

// 两处输入框都是 Ctrl+Enter（macOS 上 ⌘↩）发出去，回车只是换行：一段话常要分几行写。
const sendKey = (ev) => { if (ev.key === 'Enter' && (ev.ctrlKey || ev.metaKey) && !ev.isComposing) send() }

// 发一句话。右栏的讨论和「讨论」那一页用的是同一份。
function send() {
  const text = S.draft.trim()
  if (!text || S.waiting) return
  if (!S.uses[0].model.trim()) { say(L.chatNeedModel(), '', 'fail', [[L.commonOpenSettings(), () => set({ dialog: { kind: 'settings', tab: 'AI', edit: '' }, toasts: [], ...SHUT })]]); return }
  const isNew = S.tv !== 'one'
  const id = isNew ? 't' + S.n : S.tid
  const add = (msg) => (talk) => (talk.id === id ? { ...talk, msgs: [...talk.msgs, msg] } : talk)
  const talks = (isNew ? [{ id, title: text, at: '今天', msgs: [] }, ...S.talks] : S.talks).map(add({ me: true, at: stamp(), text, cite: S.cite }))
  set({ talks, tid: id, tv: 'one', n: S.n + 1, draft: '', cite: null })
  if (talks.find((x) => x.id === id).origin) write(id)
  else answer(id)
}

// 起草、改写：新开一段讨论，替作者说出第一句，在正文页的右栏打开它。这段讨论跟着开它的那一章：附上的正文、给的设定都按那一章。
function startTalk({ origin, key, title, text, cite, over }) {
  if (!S.uses[0].model.trim()) { say(L.chatNeedModel(), '', 'fail', [[L.commonOpenSettings(), () => set({ dialog: { kind: 'settings', tab: 'AI', edit: '' }, toasts: [], ...SHUT })]]); return }
  if (S.waiting) { say(L.chatBusyOther(), L.chatBusyOtherWhy({ 起草: L.chatActDraft(), 改写: L.chatActRewrite() }[origin]), 'fail'); return }
  const id = 't' + S.n
  set((s) => ({ talks: [{ id, title, at: '今天', origin, key, over, msgs: [{ me: true, at: stamp(), text, cite }] }, ...s.talks], n: s.n + 1, paneOpen: true, focus: false, tab: '讨论', tv: 'one', tid: id, draft: '', ...SHUT }))
  write(id)
}
// 在右栏打开一段讨论。
const openTalk = (id) => set({ page: 'write', paneOpen: true, focus: false, tab: '讨论', tv: 'one', tid: id, ...SHUT })

// 每次向模型发的请求，在 AI 记录最上面记一行：一轮查了几次就是几行。讨论里的按讨论是怎么开的算：起草、改写，或者讨论。
function logged(s, talk, use) {
  const feature = talk.origin || '讨论', where = talk.origin ? chTitle(talk.key) : talk.title
  const own = { 起草: [['章纲和卷纲', 260], ['本卷前面各章章纲', 310], ['上一章全文', 5060]], 改写: [['章纲和卷纲', 260], ['本章全文', 4300]] }[feature] || []
  const parts = (input) => { const fixed = [['规则', 1100], ['设定', 1820], ...own]; return [...fixed, ['之前的对话和查阅', Math.max(input - fixed.reduce((n, [, x]) => n + x, 0), 0)]] }
  return [...use.map((u) => [stamp(), feature, where, s.uses[0].model, s.uses[0].effort, parts(u[0]), u, '完成']).reverse(), ...s.records]
}

// 续写、检查也各记一行，用「续写和检查」那个模型。
function noted(s, feature, where, parts, use, end = '完成') {
  const u = s.uses[1].model ? s.uses[1] : s.uses[0]
  return [[stamp(), feature, where, u.model, s.uses[1].effort, parts, use, end], ...s.records]
}

// AI 回这段讨论一句：先想，再查（搜全书、读正文、查设定），再写；写完之前可以取消。
// 要记进设定的，AI 在这一轮里当场记下，回复上方列着记了什么，修改记录里也有一条。
// 压缩过的讨论，摘要后面又说了两轮以上，就算长了：先把这一句以前的压进摘要，再回。压完了再取消，摘要照样留着。
let replying = []
function answer(id) {
  const r = BOOK.replies[sent++ % BOOK.replies.length]
  const talk = S.talks.find((x) => x.id === id), fold = talk.digest && talk.msgs.length - 1 - talk.digest.after >= 4, pack = fold ? 800 : 0
  set({ waiting: { id, step: fold ? 'pack' : 'think' } })
  replying = [
    ...(fold ? [later(pack, () => set((s) => ({ waiting: { id, step: 'think' }, talks: s.talks.map((x) => (x.id === id ? { ...x, digest: { ...x.digest, after: x.msgs.length - 1 } } : x)) })))] : []),
    later(pack + 400, () => set({ waiting: { id, step: r.look.length ? 'look' : 'reply', look: r.look } })),
    later(pack + 800, () => set({ waiting: { id, step: 'reply', look: r.look } })),
    later(pack + 1200, () => set((s) => {
      const kept = r.keep ? { id: 'e' + s.n, place: 'open', cat: r.keep.cat, names: [], text: r.keep.text } : null
      const said = kept ? L.loreToConfirmLine(kept.cat, kept.text) : ''
      const msg = { me: false, at: stamp(), text: r.text, think: BOOK.think, look: kept ? [...r.look, ['记', said, kept.id]] : r.look, use: r.use, capped: r.capped }
      return {
        waiting: null, talks: s.talks.map((talk) => (talk.id === id ? { ...talk, msgs: [...talk.msgs, msg] } : talk)), records: logged(s, talk, r.use),
        ...(kept ? { n: s.n + 1, entries: [...s.entries, kept], log: [{ id: 'l' + s.n, who: '讨论', talk: id, at: stamp(), lines: [L.loreAddedTo(said)] }, ...s.log] } : {}),
      }
    }))]
}

// 起草、改写的讨论里 AI 回一句：先想、再查，然后在正文里提议，边写边出来，写完说一句。
// 作者在这段讨论里接着说，AI 再提议一次，换掉上一次还没定的那一处。
function write(id) {
  const talk = S.talks.find((x) => x.id === id), drafting = talk.origin === '起草'
  const list = drafting ? BOOK.drafts : BOOK.rewrites, r = list[Math.min(talk.msgs.filter((m) => !m.me).length, list.length - 1)]
  const at = drafting ? { kind: talk.over ? 'all' : 'end' } : { kind: 'swap', quote: talk.msgs[0].cite.quote }
  const where = { end: '接在章末', all: '换掉全章', swap: '换掉一句' }[at.kind]
  // 起草一段一段出来，改写的那一句分三截出来。
  const bits = drafting ? r.prop : [0, 1, 2].map((n) => r.prop.slice(Math.round((r.prop.length * n) / 3), Math.round((r.prop.length * (n + 1)) / 3)))
  const pid = 'p' + S.n, step = 600, end = 800 + step * (bits.length + 1)
  set({ waiting: { id, step: 'think', write: true }, n: S.n + 1 })
  replying = [
    later(400, () => set({ waiting: { id, step: r.look.length ? 'look' : 'propose', look: r.look, write: true } })),
    later(800, () => set((s) => {
      const old = s.props.filter((p) => p.talk === id), gone = ruled(s, old.map((p) => p.id), '已被新的提议替换')
      return { waiting: { id, step: 'propose', look: r.look, prop: pid, write: true }, fate: gone.fate, props: [...gone.props, { id: pid, key: talk.key, talk: id, at, paras: [], writing: true }] }
    })),
    ...bits.map((_, n) => later(800 + step * (n + 1), () => set((s) => ({ props: s.props.map((p) => (p.id === pid ? { ...p, paras: drafting ? bits.slice(0, n + 1) : [bits.slice(0, n + 1).join('')] } : p)) })))),
    later(end, () => set((s) => {
      const p = s.props.find((x) => x.id === pid)
      if (!p) return { waiting: null }
      const msg = { me: false, at: stamp(), text: r.text, think: BOOK.think, look: r.look, use: r.use, prop: pid, key: talk.key, where, words: p.paras.join('').length }
      // 写的时候作者改掉了要换的那一句：写完就作废。
      const lost = at.kind === 'swap' && !chapterOf(talk.key).paras.some((x) => x.includes(at.quote))
      const done = { props: s.props.map((x) => (x.id === pid ? { ...x, writing: false } : x)) }
      if (lost) later(0, () => say(L.chatProposalVoid(), L.chatProposalVoidWhy()))
      return { waiting: null, talks: s.talks.map((x) => (x.id === id ? { ...x, msgs: [...x.msgs, msg] } : x)), records: logged(s, talk, r.use), ...(lost ? ruled(s, [pid], '正文已改动，提议已作废') : done) }
    })),
  ]
}

// 少见的情况：普通的讨论里，AI 提议修改另一章。那一章没开着就在右边开一个标签页，不换过去。
function elsewhere() {
  const key = '86', quote = '档案室在电力局二楼，窗户朝北，白天也要开灯。', words = '档案室在电力局二楼，窗户朝北，白天也得开着灯。', pid = 'p' + S.n
  set((s) => ({
    n: s.n + 1, page: 'write', tabs: s.tabs.includes(key) ? s.tabs : [...s.tabs, key], paneOpen: true, focus: false, tab: '讨论', tv: 'one', tid: 't1', dialog: null,
    props: [...s.props, { id: pid, key, talk: 't1', at: { kind: 'swap', quote }, paras: [words], writing: false, done: 0 }],
    talks: s.talks.map((x) => (x.id !== 't1' ? x : { ...x, msgs: [...x.msgs, { me: true, at: stamp(), text: '第86章开头那句“白天也要开灯”有点硬，帮我顺一下' },
      { me: false, at: stamp(), text: '在「第86章：档案室」提议了一处：“也要开灯”换成“也得开着灯”，读起来更像随口说的。', look: [['读', '「第86章：档案室」', '档案室在电力局二楼，窗户朝北，白天也要开灯。……']], use: [[18800, 18420, 70]], prop: pid, key, where: '换掉一句', words: words.length }] })),
  }))
}

// 取消请求：没回完的那句不要了。正在正文里写的那一处提议跟着丢掉。
function stopReply() {
  for (const id of replying) { clearTimeout(id); timers.delete(id) }
  const w = S.waiting
  if (w && !w.write) sent-- // 没回成的那一句，下次接着用
  set((s) => ({ waiting: null, ...(w && w.prop ? ruled(s, [w.prop], '已放弃') : {}) }))
}

// 讨论里引用的那段正文，点一下去正文里找它。
function findCited(c) {
  const ch = chapterOf(c.key)
  if (!ch) { say(L.chatChapterGone(), '', 'fail'); return }
  const there = ch.paras.some((p) => p.includes(c.quote))
  set((s) => go(s, c.key, there ? { key: c.key, quote: c.quote } : null))
  if (!there) say(L.chatQuoteChanged())
}

// ───────── 画：三页都用的 ─────────

// 列表顶上：搜索、旁边的按钮（add）、「···」里的几项（more）。
function tool(hint, key, add, more) {
  const on = S.open === 'tool'
  return '<div class="tool" data-k="tool">' +
    `<div class="find box">${ICON.find}${typed(S[key], (v) => set({ [key]: v }), hint, hint)}</div>` +
    (add ? `<div class="add hit"${onClick(add[1])}>${ICON.plus}${t(add[0])}</div>` : '') +
    (more ? `<div class="more hit${on ? ' on' : ''}" title="${t(L.commonMore())}"${onClick(() => set({ open: on ? '' : 'tool' }))}>${ICON.dots}</div>` : '') +
    (more && on ? `<div class="tool-menu pop">${more.map(([label, n, pick]) => `<div class="pop-row row"${onClick(pick)}><div class="grow">${t(label)}</div><div class="side">${n || ''}</div></div>`).join('')}</div>` : '') +
    '</div>'
}
// 详情页顶上：返回哪里，右边还能做什么。
function back(label, goBack, actLabel, act) {
  return `<div class="back" data-k="back"><div class="back-to hit"${onClick(goBack)}>${ICON.left}<div class="ell">${t(label)}</div></div><div class="grow"></div>` +
    (act ? `<div class="hit link"${onClick(act)}>${t(actLabel)}</div>` : '') + '</div>'
}

// ───────── 画：设定 ─────────

function setting() {
  const at = (place) => S.entries.filter((e) => e.place === place)
  const body = at('body'), open = aiOn() ? at('open') : [], gone = at('dropped') // open 是 AI 的建议，AI 关着时不显示
  const shown = S.entries.find((e) => e.id === S.sid)
  // 正文页开着一章（或一卷）时，列表里每条写它在这一章时的样子；别处写最新的。是从某一章起变成这样的，右边写从哪起；这一章以后还会变的，写「之后有变化」。
  const here = S.page === 'write' ? S.ch || S.vol : ''
  const when = (e) => { const g = asOf(e, here); return g ? fromWhere(g.key) : here && changesOf(e).some((x) => rank(x.key) > rank(here)) ? L.loreLaterChanges() : '' }
  const row = (e, find = '') => { const text = textAt(e, here); return item({ name: e.names[0] || text, about: e.names.length ? near(text, find) : '', find, side: e.place === 'body' ? when(e) : '', dot: e.place === 'body' && open.some((o) => o.about === e.id), open: () => set({ sid: e.id }) }) }
  // AI 的提问不是设定，答案才是：不「采纳」，「回答」。
  // 问题原来带着名字的，答案也带着：第一个进名字，其余进别名，作者删了才算不要。
  const answer = (o) => () => set({ form: { id: null, name: (o.names || [])[0] || '', names: (o.names || []).slice(1).join('、'), text: '', cat: o.cat, ask: o.id, q: o.text }, ...SHUT })
  // AI 建议的一处变化（from），采纳就记到它说的那一条上。
  const idea = (o, about) => `<div class="idea">${about ? `<div class="idea-about">${t(about)}</div>` : ''}${o.from && !about ? `<div class="idea-about">${t(fromWhere(o.from))}</div>` : ''}<div class="text">${t(o.text)}</div>` +
    buttons(button(L.commonIgnore(), o.from ? () => dropIdea(o) : () => move(o.id, 'dropped')), asks(o) ? button(L.commonAnswer(), answer(o), 'primary') : button(L.commonAccept(), o.from ? () => takeChange(o) : () => move(o.id, 'body'), 'primary')) + '</div>'

  if (S.form) return back(S.form.id ? shown.names[0] || shown.cat : S.form.ask ? L.loreToConfirm() : L.loreAll(), () => set({ form: null, ...SHUT })) + (S.form.change ? changeForm(S.form) : form(S.form))

  if (shown) {
    const dropped = shown.place === 'dropped'
    const about = open.filter((o) => o.about === shown.id)
    const from = shown.src ? chapterOf(shown.src.key) : null
    return back(dropped ? L.loreDropped() : L.loreAll(), () => set({ sid: null }), dropped ? L.loreRestore() : L.loreDelete(), () => move(shown.id, dropped ? 'body' : 'dropped')) +
      '<div class="page scroll" data-k="entry">' +
      `<div class="title-line"><div class="title ell">${t(shown.names[0] || shown.cat)}</div>${shown.names.length > 1 ? `<div class="alias ell">${t(L.loreAliasesOf(shown.names.slice(1).join('、')))}</div>` : ''}<div class="grow"></div>${button(L.commonEdit(), () => set({ form: editing(shown) }))}</div>` +
      `<div class="text${dropped ? ' dropped' : ''}">${t(shown.text)}</div>` + (shown.asked ? `<div class="note">${t(L.loreAnswer(shown.asked))}</div>` : '') +
      (from ? where(L.loreSource(titleOf(from), shown.src.quote), goTo(from.key, shown.src), L.appViewOriginal()) : '') + (dropped ? '' : timeline(shown)) +
      (about.length ? HAIR + `<div class="ai-head">${DOT}${t(L.loreAiSuggest())}</div>` + about.map((o) => idea(o)).join('') : '') +
      '</div>'
  }

  if (S.sv === 'ideas') {
    const of = (o) => S.entries.find((e) => e.id === o.about)
    return back(L.loreAll(), () => set({ sv: 'list' })) +
      `<div class="page scroll" data-k="ideas"><div class="title-line"><div class="title">${t(L.loreToConfirm())}</div><div class="grow"></div><div class="side">${t(L.loreAiCount(String(open.length)))}</div></div>` +
      open.map((o, i) => (i ? HAIR : '') + idea(o, of(o) ? (o.from ? L.loreAboutFrom(of(o).names[0], fromWhere(o.from)) : L.loreAbout(of(o).names[0])) : L.loreNew())).join('') + '</div>'
  }

  if (S.sv === 'dropped') return back(L.loreAll(), () => set({ sv: 'list' })) + rows([head(L.loreDropped()), ...(gone.length ? gone.map((e) => row(e)) : [empty(L.loreNoneDropped())])])

  if (S.sv === 'log') {
    const log = aiOn() ? S.log : S.log.filter((c) => !c.talk) // 讨论时 AI 记下的那几条，AI 关着时不列
    const talk = (id) => S.talks.find((x) => x.id === id)
    const card = (c) => '<div class="card">' +
      `<div class="card-head"><div class="who">${t(WHO()[c.who] || c.who)}</div><div class="grow"></div><div class="tiny">${t(timeShown(c.at))}</div></div>` +
      c.lines.map((l) => `<div class="said-line">${t(l)}</div>`).join('') +
      (c.talk && talk(c.talk) ? where(L.loreNotedIn(talk(c.talk).title), () => set({ page: 'talk', tv: 'one', tid: c.talk, ...SHUT }), L.chatViewTalk()) : '') + '</div>'
    return back(L.loreAll(), () => set({ sv: 'list' })) + `<div class="page log scroll" data-k="log"><div class="title">${t(L.loreLog())}</div>${log.length ? log.map(card).join('') : `<div class="note">${aiOn() ? t(L.loreLogEmptyAi()) : t(L.loreLogEmpty())}</div>`}</div>`
  }

  const q = S.sq.trim()
  let list = []
  if (q) {
    const found = body.filter((e) => (e.names.join('、') + e.text).includes(q))
    list = found.length ? [head(L.commonFound(String(found.length))), ...found.map((e) => row(e, q))] : [empty(L.commonNotFound())]
  } else if (!body.length && !open.length) {
    list = [empty(L.loreEmpty(), aiOn() ? L.loreEmptyAi() : L.loreEmptyNoAi())]
  } else {
    if (open.length) list.push(todo(L.loreToConfirm(), open.length, () => set({ sv: 'ideas' })))
    for (const cat of S.cats) {
      const of = body.filter((e) => e.cat === cat)
      if (of.length) list.push(head(cat), ...of.map((e) => row(e)))
    }
  }
  const r = S.named
  if (r) list.unshift(`<div class="renamed" data-k="renamed"><div class="fit">${t(L.loreRenamedLeft(String(r.n), r.was))}</div><div class="hit link" title="${t(L.loreReplaceAllTip())}"${onClick(() => set((s) => ({ named: null, page: 'write', focus: false, find: chapterOf(s.ch) ? { q: r.was, to: r.now, all: true, at: 0 } : null })))}>${t(L.loreReplaceAll())}</div>` +
    `<button class="icon-btn muted" aria-label="${t(L.loreKeepOld())}" title="${t(L.loreKeepOldTip())}"${onClick(() => set({ named: null }))}>${icon('cross', 'sm')}</button></div>`)
  return tool(L.loreSearch(), 'sq', [L.commonNew(), () => set({ form: { id: null, name: '', names: '', text: '', cat: S.cats[0] }, ...SHUT })],
    [[L.loreDropped(), gone.length, () => set({ sv: 'dropped', ...SHUT })], [L.loreLog(), '', () => set({ sv: 'log', ...SHUT })], [L.cmdCategories(), '', () => set({ page: 'cats', ...SHUT })]]) + rows(list)
}

// 一条设定随章节的变化，接在原来的样子下面，照侧栏的先后排。正文页开着的那一章用的是哪一段，标「本章适用」；变化所在的章进了回收站，这处不起作用，写「所在章节已删除」。
// 开着一章（或一卷）时，下面有「从本章起有变化」：本章已经有一处的，就是改它。
function timeline(e) {
  const here = S.page === 'write' ? S.ch || S.vol : '', on = asOf(e, here), all = e.changes || []
  const gone = all.filter((g) => rank(g.key) < 0), live = changesOf(e)
  const mine = here && all.find((g) => g.key === here)
  const step = (label, text, g, cur) => `<div class="step${g && rank(g.key) < 0 ? ' gone' : ''}"><div class="step-head"><div class="fit ell">${t(label)}</div>${cur ? '<div class="tag">' + t(L.loreHere()) + '</div>' : ''}` +
    (g ? `<div class="hit link"${onClick(() => set({ form: changing(e, g, g.key), ...SHUT }))}>${t(L.commonEdit())}</div>` : '') + `</div><div class="step-text">${t(text)}</div></div>`
  const add = here && !mine ? `<div class="hit link add-change"${onClick(() => set({ form: changing(e, null, here), ...SHUT }))}>${t(S.vol ? L.loreChangeFromVolume() : L.loreChangeFromChapter())}</div>` : ''
  if (!all.length) return add ? HAIR + add : ''
  return HAIR + `<div class="ai-head"><div class="grow">${t(L.loreChanges())}</div>${add}</div>` + (here && !on ? '<div class="note">' + t(L.loreOriginalHere()) + '</div>' : '') +
    live.map((g) => step(fromWhere(g.key), g.text, g, on && on.id === g.id)).join('') + gone.map((g) => step(L.loreGoneInactive(), g.text, g, false)).join('')
}

// 记一处变化：从哪一章或哪一卷起（只能选书里有的），那以后这一条的全文。
function changeForm(f) {
  const e = S.entries.find((x) => x.id === f.id), nodes = []
  for (const c of S.chapters) {
    const v = S.volumes.find((x) => x[0] === c.vol)
    if (v && !nodes.some((n) => n.key === v[0])) nodes.push({ key: v[0], title: v[1] })
    nodes.push({ key: c.key, title: titleOf(c) })
  }
  const titles = nodes.map((n) => n.title), now = nodes.find((n) => n.key === f.key)
  const pick = (title) => set((s) => ({ form: { ...s.form, key: nodes[titles.indexOf(title)].key } }))
  const note = aiOn() ? L.loreChangeNoteAi() : L.loreChangeNoteNoAi()
  return '<div class="page scroll" data-k="change-form">' +
    `<div class="title-line"><div class="title">${t(e.names[0] || e.cat)} · ${t(fromWhere(f.key))}</div></div><div class="note">${note}</div>` +
    field(L.loreFromChapter(), choose('change-from', now ? now.title : L.loreGoneChapter(), titles, pick, L.loreFromChapter())) +
    field(L.loreContent(), area(f.text, (v) => set((s) => ({ form: { ...s.form, text: v } })), '', L.loreChangedText(), 5, ' autofocus')) +
    `<div class="buttons">${f.gid ? `<div class="hit link danger"${onClick(dropChange)}>${t(L.loreDropChange())}</div><div class="grow"></div>` : ''}${button(L.commonCancel(), () => set({ form: null, ...SHUT }))}${button(L.commonSave(), saveChange, f.text.trim() ? 'primary' : 'primary off')}</div>` +
    '</div>'
}

// 新增和修改用同一张表。新增先填内容，修改先看名字：光标就在那一格里。
function form(f) {
  const type = (key) => (value) => set((s) => ({ form: { ...s.form, [key]: value } }))
  const name = field(f.id ? L.loreName() : L.loreNameOptional(), line(f.name, type('name'), '', L.loreName(), f.id ? ' autofocus' : ''))
  const names = field(f.id ? L.loreAliases() : L.loreAliasesOptional(), line(f.names, type('names'), L.loreAliasHint(), L.loreAliases()))
  const text = field(L.loreContent(), area(f.text, type('text'), '', L.loreContent(), 4, f.id ? '' : ' autofocus'))
  const cat = (label) => `<div class="pop-row row${label === f.cat ? ' on' : ''}"${onClick(() => set((s) => ({ ...SHUT, form: { ...s.form, cat: label } })))}>${t(label)}</div>`
  return '<div class="page scroll" data-k="form">' +
    (f.id ? name + names + text : `<div class="title-line"><div class="title">${t(f.ask ? L.commonAnswer() : L.loreNewLore())}</div></div>` + (f.ask ? `<div class="note">${t(f.q)}</div>` : '') + text + name + names) +
    `<div class="field cat"><div class="label">${t(L.loreCategory())}</div><div class="box line hit"${onClick(() => set({ open: S.open === 'cats' ? '' : 'cats' }))}>${t(f.cat)}<div class="grow"></div>${ICON.down}</div>` +
    (S.open === 'cats' ? `<div class="cat-menu pop">${S.cats.map(cat).join('')}</div>` : '') + '</div>' +
    buttons(button(L.commonCancel(), () => set({ form: null, ...SHUT })), button(L.commonSave(), save, f.text.trim() ? 'primary' : 'primary off')) +
    '</div>'
}

// ───────── 画：伏笔 ─────────

const nameOf = (m) => m.note || clip(m.quote, 14)

function foreshadowing() {
  const of = (st) => S.marks.filter((m) => m.st === st)
  const groups = () => ({ lost: L.hooksLost(), done: L.hooksDone(), dropped: L.hooksDropped() })
  const row = (m, find = '') => item({ name: nameOf(m), about: find && !m.note.includes(find) ? near(m.quote, find) : '', find, side: m.key === S.ch ? L.commonThisChapter() : chTitle(m.key), open: () => set({ fid: m.id, renote: null }) })
  const shown = S.marks.find((m) => m.id === S.fid)

  // 正在选「选择要回收的伏笔」时，右栏摆出选中的那一个，看清楚再记。
  const paying = S.pay && S.marks.find((m) => m.id === payPick())
  if (paying) {
    const gap = BOOK.now - paying.ch
    return '<div class="page scroll" data-k="mark-paying">' + `<div class="title-line"><div class="title ell">${t(nameOf(paying))}</div></div>` +
      `<div class="where"><div class="fit ell">${t(gap > 0 && !paying.pay ? L.hooksPlantedGap(chTitle(paying.key), String(gap)) : L.hooksPlanted(chTitle(paying.key)))}</div></div><div class="quote">${t(paying.quote)}</div>` +
      (paying.pay ? `<div class="note">${t(L.hooksAlreadyPaid(chTitle(paying.pay.key)))}</div>` : '') + '</div>'
  }

  if (shown) {
    const st = shown.st, gap = BOOK.now - shown.ch
    const act = { open: [L.hooksDrop(), () => ({ st: 'dropped' })], done: [L.hooksCancelPayoff(), () => ({ st: 'open', pay: null })], dropped: [L.hooksRestore(), () => ({ st: 'open' })] }[st]
    const remove = () => set((s) => ({ marks: s.marks.filter((m) => m.id !== shown.id), fid: null, renote: null }))
    const saveNote = () => mark(shown.id, () => ({ note: S.renote.trim() || shown.note }))
    return back(L.hooksAll(), () => set({ fid: null, renote: null }), act ? act[0] : L.hooksDelete(), act ? () => mark(shown.id, act[1]) : remove) +
      '<div class="page scroll" data-k="mark">' +
      (S.renote !== null
        ? `<div class="title-line"><div class="box line fit">${typed(S.renote, (v) => set({ renote: v }), L.hooksNote(), L.hooksEditNote(), keys(saveNote, () => set({ renote: null })) + ' autofocus')}</div>${button(L.commonSave(), saveNote, 'primary')}</div>`
        : `<div class="title-line"><div class="title ell">${t(nameOf(shown))}</div><div class="grow"></div>${st === 'lost' ? '' : button(L.hooksEditNoteBtn(), () => set({ renote: shown.note }))}</div>`) +
      where(st === 'lost' ? L.hooksLostIn(chTitle(shown.key)) : st === 'open' && gap > 0 ? L.hooksPlantedGap(chTitle(shown.key), String(gap)) : L.hooksPlanted(chTitle(shown.key)), goTo(shown.key), L.appViewOriginal()) +
      `<div class="quote${st === 'lost' ? ' gone' : ''}">${t(shown.quote)}</div>` + HAIR +
      (st === 'open' ? '<div class="state">' + t(L.hooksUnpaid()) + '</div><div class="note">' + t(L.hooksSelectPayoff()) + '</div>' + buttons(button(L.hooksMarkPaid(), S.bar ? () => askPay(shown.id) : null, S.bar ? 'primary' : 'primary off')) : '') +
      (st === 'done' ? where(L.hooksPaidIn(chTitle(shown.pay.key), String(Math.max(shown.pay.ch - shown.ch, 0))), goTo(shown.pay.key), L.appViewOriginal()) + `<div class="quote">${t(shown.pay.quote)}</div>` : '') +
      (st === 'lost' ? '<div class="note">' + t(L.hooksSelectNew()) + '</div>' + buttons(button(L.hooksReplaceQuote(), S.bar ? () => rehang(shown.id) : null, S.bar ? 'primary' : 'primary off')) : '') +
      (st === 'dropped' ? '<div class="state">' + t(L.hooksDropped()) + '</div>' : '') +
      '</div>'
  }

  if (S.fv !== 'list') return back(L.hooksAll(), () => set({ fv: 'list' })) + rows([head(groups()[S.fv]), ...(of(S.fv).length ? of(S.fv).map((m) => row(m)) : [empty(L.hooksNoneHere())])])

  const q = S.fq.trim()
  let list = []
  if (q) {
    const found = S.marks.filter((m) => (m.note + m.quote).includes(q))
    list = found.length ? [head(L.commonFound(String(found.length))), ...found.map((m) => row(m, q))] : [empty(L.commonNotFound())]
  } else if (!S.marks.length) {
    list = [empty(L.hooksEmpty(), L.hooksEmptyHow())]
  } else {
    const lost = of('lost'), long = of('open').filter((m) => BOOK.now - m.ch >= S.late), rest = of('open').filter((m) => BOOK.now - m.ch < S.late)
    if (lost.length) list.push(todo(groups().lost, lost.length, () => set({ fv: 'lost' })))
    if (long.length) list.push(head(L.hooksLong()), ...long.map((m) => row(m)))
    if (rest.length) list.push(head(L.hooksUnpaid()), ...rest.map((m) => row(m)))
    if (!list.length) list.push(empty(L.hooksNoneOpen()))
  }
  return tool(L.hooksSearch(), 'fq', null, ['done', 'dropped'].map((st) => [groups()[st], of(st).length, () => set({ fv: st, ...SHUT })])) + rows(list)
}

// ───────── 画：讨论 ─────────

// 从正文拿来的一段：哪一章、哪句话。drop 有的话，右边多一个去掉它的叉。
const cited = (c, drop) => `<div class="cited hit" title="${t(L.chatLocate())}"${onClick(() => findCited(c))}><div class="fit"><div class="cited-from">《${t(c.title)}》</div><div class="note">${t(c.quote)}</div></div>${drop ? `<button class="icon-btn muted" aria-label="${t(L.chatRemoveCite())}" title="${t(L.chatRemoveCite())}"${onClick(drop)}>${icon('cross', 'sm')}</button>` : ''}</div>`

// 输入框下面选模型的那几个字，点开能换模型和思考程度。
function modelPick() {
  const use = S.uses[0], service = S.services.find((x) => x.name === use.service)
  const pick = (change) => () => set((s) => ({ ...SHUT, uses: s.uses.map((u, i) => (i ? u : { ...u, ...change })) }))
  return `<div class="anchor up"><div class="model hit" title="${t(L.chatCurrentModel())}"${onClick(() => set({ open: S.open === 'model' ? '' : 'model' }))}>${t(use.model || L.chatPickModel())}${icon('up', 'xs')}</div>` +
    (S.open === 'model' ? '<div class="pop model-pop" data-k="model-pop"><div class="pop-label">' + t(L.chatModel()) + '</div>' + (service ? service.models : []).map(({ id }) => `<div class="pop-row row${id === use.model ? ' on' : ''}"${onClick(pick({ model: id }))}>${t(id)}</div>`).join('') +
      menuRow(L.chatAiConfig(), () => set({ dialog: { kind: 'settings', tab: 'AI', edit: '' }, ...SHUT })) + HAIR + '<div class="pop-label">' + t(L.chatEffort()) + '</div>' +
      BOOK.efforts.map((e) => `<div class="pop-row row${e === use.effort ? ' on' : ''}"${onClick(pick({ effort: e }))}>${e}</div>`).join('') + '</div>' : '') + '</div>'
}

// AI 想的过程，折在回复上面；正在想的时候开着。
function thought(key, text, running, label = L.chatThought()) {
  const open = running || S.thoughts[key]
  return `<div class="thought"><div class="thought-head hit"${onClick(running ? null : () => set((s) => ({ thoughts: { ...s.thoughts, [key]: !open } })))}>${t(running ? L.chatThinking() : label)}${icon(open ? 'down' : 'right', 'xs')}</div>` +
    (open ? `<div class="thought-text">${t(text)}</div>` : '') + '</div>'
}

// 回复前 AI 查了什么、记了什么，折在回复上面，和「思考过程」一个样子；正在查的时候开着。
// 每一项点开能看查到的；记下的那一条点一下去设定里看它。
const LOOK = () => ({ 搜: (w) => L.chatLookSearched(w), 读: (w) => L.chatLookRead(w), 设定: (w) => L.chatLookLore(w), 纲: (w) => L.chatLookOutline(w), 讨论: (w) => L.chatLookChats(w), 记: (w) => L.chatLookNoted(w) })
const lookLine = ([act, what]) => LOOK()[act](what)
// 在右栏打开设定里的一条：等作者决定的在「AI 建议」里，别的在列表里选中它。
const showEntry = (entry) => set({ paneOpen: true, tab: '设定', form: null, ...SHUT, ...(entry.place === 'open' ? { sv: 'ideas', sid: null } : { sv: 'list', sid: entry.id }) })
function looked(key, look, running) {
  const open = running || S.thoughts[key]
  const row = ([act, what, got]) => {
    const entry = act === '记' && S.entries.find((e) => e.id === got)
    return entry ? where(lookLine([act, what]), () => showEntry(entry), L.chatViewLore())
      : `<div class="look-row"><div>${t(lookLine([act, what]))}</div>${got && act !== '记' ? `<div class="look-got">${t(got)}</div>` : ''}</div>`
  }
  return `<div class="thought"><div class="thought-head hit"${onClick(running ? null : () => set((s) => ({ thoughts: { ...s.thoughts, [key]: !open } })))}><div class="ell">${running ? t(L.chatLooking()) : t(look.map(lookLine).join(L.commonListSep()))}</div>${icon(open ? 'down' : 'right', 'xs')}</div>` +
    (open ? `<div class="thought-text">${look.map(row).join('')}</div>` : '') + '</div>'
}

// 「AI 参考内容」：这次发给 AI 的各部分各约多少 token，附了哪些设定，服务商算的实际用量。
// 查了东西的一轮前后发了几次请求，说的都是最后一次，写出回复的那次。
function outgoing(talk, m, i) {
  // 发出去的是整份设定：正文、已放弃、悬而未决都在，后两种写明是哪一种；有名字的条目，名字在内容前面；回答过的问题跟在答案后面。目录每条只有分类、第一个名字和第一句。
  // 起草、改写的讨论跟着一章：设定按那一章时的样子，后面的变化不给，只说还有几处。普通讨论给原来的样子，下面每处变化写「第几章起」；目录给最新的样子。
  const [input, cached, output] = m.use.at(-1), sent = ['body', 'dropped', 'open'].flatMap((p) => S.entries.filter((e) => e.place === p && !e.from))
  const label = { body: '', dropped: L.chatLabelDropped(), open: L.chatLabelOpen() }
  const first = (text) => clip(text.match(/^[^。！？；\n]*[！？]?/)[0], 60)
  const fixed = talk.origin ? talk.key : '', ahead = fixed ? sent.flatMap(changesOf).filter((g) => rank(g.key) > rank(fixed)).length : 0
  const digested = talk.digest && i >= talk.digest.after
  // 起草、改写那一章有卡片的，带上它们：卡片是这一章要写的事。
  const cards = fixed ? eventsOf(fixed).length : 0, owned = cards ? [[L.chatChapterCards(String(cards)), cards * 45]] : []
  const own = { 起草: [[L.chatPartOutline(), 260], ...owned, [L.chatPartPrevOutlines(), 310], [L.chatPartPrevChapter(), 5060]], 改写: [[L.chatPartOutline(), 260], ...owned, [L.chatPartThisChapter(), 4300]] }[talk.origin] || []
  const parts = [[L.chatPartRules(), 1100], [S.crowded ? L.chatPartCatalog() : L.chatPartLoreCount(String(sent.length)), S.crowded ? sent.length * 40 : sent.length * 260], ...own, ...(digested ? [[L.chatPartDigest(), 820]] : []), [L.chatPartLook(), (m.look || []).length * 640], [L.chatPartSaid(), 40]]
  parts.splice(parts.length - 1, 0, [L.chatPartBefore(), Math.max(Math.round(input * 0.97) - parts.reduce((n, [, x]) => n + x, 0), 0)])
  const head = S.crowded ? L.chatCrowded() : fixed ? L.chatAttachedAt(String(sent.length), chTitle(fixed)) : L.chatAttachedAll(String(sent.length))
  const named = (e) => (e.names.length ? e.names.join('、') + '：' : '')
  const changes = (e) => (fixed ? '' : changesOf(e).map((g) => `<div class="note">${t(fromWhere(g.key) + '：' + g.text)}</div>`).join(''))
  const rows = S.crowded ? sent.map((e) => `<div class="saw-row"><div class="fit">${t(label[e.place] + e.cat + (e.names[0] ? ' · ' + e.names[0] : '') + '：' + first(textAt(e, fixed)))}</div></div>`).join('')
    : sent.map((e) => `<div class="saw-row">${icon('tick', 'sm', 'green')}<div class="fit"><div>${t(label[e.place] + named(e) + (fixed ? textAt(e, fixed) : e.text) + (e.asked ? L.chatAnswered(e.asked) : ''))}</div>${changes(e)}</div></div>`).join('')
  return '<div class="reply-saw">' +
    `<div class="saw-title">${t(L.chatSentAbout(tokens(parts.reduce((n, [, x]) => n + x, 0))))}</div>${m.use.length > 1 ? `<div class="note">${t(L.chatRoundRequests(String(m.use.length)))}</div>` : ''}<div class="note">${parts.filter(([, x]) => x).map(([name, x]) => t(name) + ' ' + tokens(x)).join(' · ')}</div>` +
    `<div class="note">${t(cached ? L.chatUsageCached(tokens(input), tokens(cached), tokens(output)) : L.chatUsage(tokens(input), tokens(output)))}</div>` +
    `<div class="saw-title">${t(head)}</div>${S.crowded ? '<div class="note">' + t(L.chatCatalogNote()) + '</div>' : ''}${ahead ? `<div class="note">${t(L.chatMoreChanges(String(ahead)))}</div>` : ''}` +
    (sent.length ? rows : '<div class="note">' + t(L.chatLoreEmpty()) + '</div>') + '</div>'
}

// 回复里 AI 在正文里提的那一处：改哪一章、改哪里、多少字，作者定了没有。还没定的，点「在正文中查看」过去看。
function propCard(m) {
  const p = S.props.find((x) => x.id === m.prop), words = p ? p.paras.join('').length : m.words
  const state = S.fate[m.prop] ? FATE()[S.fate[m.prop]] || S.fate[m.prop] : !p ? L.chatFateGone() : p.writing ? L.chatWriting() : L.chatFateWaiting()
  return `<div class="prop-card"><div class="prop-card-head">${icon('pen', 'sm', p ? 'green' : 'muted')}<div class="fit ell">${t(L.chatProposeEdit(chTitle(m.key)))}</div>` +
    (p ? `<div class="hit link"${onClick(() => set((s) => go(s, m.key)))}>${t(L.chatViewInText())}</div>` : '') + `</div><div class="note">${t(whereWord(m.where))} · ${t(L.chatWords(String(words)))} · ${t(state)}</div></div>`
}

// 讨论长了压缩过的地方：前面几轮整理成了一份摘要，原文还在上面。点开看摘要，只能看不能改。
function digest(talk) {
  const d = talk.digest, key = 'digest-' + talk.id, open = S.thoughts[key]
  return `<div class="digest"><div class="digest-head hit"${onClick(() => set((s) => ({ thoughts: { ...s.thoughts, [key]: !open } })))}>${t(L.chatDigestRounds(String(d.after / 2)))}${icon(open ? 'down' : 'right', 'xs')}</div>` +
    (open ? `<div class="digest-text">${d.text.map(([name, said]) => `<div><div class="saw-title">${t(name)}</div><div>${said.split('\n').map(digestLine).join('')}</div></div>`).join('')}<div class="note">${t(L.chatDigestNote())}</div></div>` : '') + '</div>'
}

// 摘要的一行和回复里一样排：「- 」起头的是列表的一项，画成圆点。
function digestLine(line) {
  const item = line.match(/^[-*] (.*)$/)
  return `<div>${item ? '<span class="muted">• </span>' + named(item[1]) : named(line)}</div>`
}

// 摘要里提到设定，存的是设定的编号「[2]」（设定只增不删，编号一直是那一条的）。给作者看换成那一条的名字，
// 没有名字的取第一句的开头；点它在右栏打开那一条。
function named(text) {
  return text.split(/\[(\d+)\]/).map((part, i) => {
    if (i % 2 === 0) return part ? `<span>${t(part)}</span>` : ''
    const e = S.entries[part - 1]
    return e ? `「<span class="hit link"${onClick(() => showEntry(e))}>${t(e.names[0] || clip(e.text.match(/^[^。！？；\n]*/)[0], 16))}</span>」` : `<span>${t('[' + part + ']')}</span>`
  }).join('')
}

// 讨论里的一句。右栏和「讨论」那一页画的是同一个样子；下面那排小按钮只在最后一句和鼠标指着的那句上露出来。
function message(talk, m, i) {
  const key = talk.id + '-' + i, last = i === talk.msgs.length - 1, saw = S.told[key]
  const tool = (label, pic, act, on, keep = '') => `<button class="icon-btn ${on ? 'ink' : 'muted'}" aria-label="${label}" title="${label}"${keep}${onClick(act)}>${icon(pic)}</button>`
  const pull = tool(L.chatPull(), 'lines', () => set({ dialog: { kind: 'source', text: m.text, cat: S.cats.at(-1), picked: '' }, ...SHUT }), false, ' data-keep')
  if (m.me) {
    const left = last && !(S.waiting && S.waiting.id === talk.id)
    return `<div class="msg me${last ? ' last' : ''}">${m.cite ? cited(m.cite) : ''}<div class="said">${t(m.text)}</div>` +
      `<div class="reply-tools">${pull}<div class="faint">${t(timeShown(m.at))}</div></div>` +
      (left ? `<div class="unanswered">${icon('warn', 'sm')}<div>${t(L.chatUnanswered())}</div><div class="hit link${S.waiting ? ' off' : ''}"${onClick(S.waiting ? null : () => answer(talk.id))}>${t(L.chatRetry())}</div></div>` : '') + '</div>'
  }
  const copy = () => { if (navigator.clipboard) navigator.clipboard.writeText(m.text).catch(() => {}); say(L.chatCopied(), '', 'ok') }
  // 这一轮所有请求的合计，和按内置价格估的花费；表里没有价格的模型不写花费。
  const cost = m.use && price(S.uses[0].model, m.use), use = m.use ? ' · ' + tokens(m.use.reduce((n, [input, , output]) => n + input + output, 0)) + ' token' + (cost ? ' · ' + cost : '') : ''
  return `<div class="msg${last ? ' last' : ''}"><div class="reply-row"><div class="reply-face">${icon('spark')}</div><div class="fit">${m.think ? thought(key, m.think) : ''}${m.look && m.look.length ? looked('look-' + key, m.look) : ''}${m.prop ? propCard(m) : ''}<div class="reply" data-reply="${i}">${t(m.text)}</div>` +
    (m.capped ? '<div class="note">' + t(L.chatCapped()) + '</div>' : '') +
    `<div class="reply-tools"><div class="faint ell" title="${t(timeShown(m.at) + ' · ' + S.uses[0].model + use)}">${t(timeShown(m.at))} · ${t(S.uses[0].model)}${use}</div>${pull}${tool(L.chatCopyAll(), 'copy', copy)}${m.use ? tool(L.chatContextSent(), 'eye', () => set((s) => ({ told: { ...s.told, [key]: !saw } })), saw) : ''}</div>` +
    (saw && m.use ? outgoing(talk, m, i) : '') + '</div></div></div>'
}
// 一段讨论从头到尾，AI 正在回的话接在最后。压缩过的，在压缩的地方隔一行。
function said(talk) {
  const w = S.waiting && S.waiting.id === talk.id ? S.waiting : null
  return talk.msgs.map((m, i) => message(talk, m, i) + (talk.digest && i === talk.digest.after - 1 ? digest(talk) : '')).join('') +
    (w ? `<div class="msg last"><div class="reply-row"><div class="reply-face">${icon('spark')}</div><div class="fit">${w.step === 'pack' ? '' : thought('live', BOOK.think, w.step === 'think')}${w.look && w.look.length ? looked('live-look', w.look, w.step === 'look') : ''}` +
      (w.prop ? propCard({ prop: w.prop, key: talk.key, where: { end: '接在章末', all: '换掉全章', swap: '换掉一句' }[(S.props.find((p) => p.id === w.prop) || { at: { kind: 'end' } }).at.kind] }) : '') +
      `<div class="waiting">${icon('spin', 'sm')}<div class="fit">${t({ pack: L.chatPacking(), think: L.chatThinkingMore(), look: L.chatLookingMore(), reply: L.chatReplying(), propose: L.chatProposing() }[w.step])}</div><div class="hit link"${onClick(stopReply)}>${t(L.chatCancelRequest())}</div></div></div></div></div>` : '')
}

function discussion() {
  if (S.tv === 'all') {
    const q = S.tq.trim()
    const found = S.talks.filter((talk) => !q || talk.title.includes(q) || talk.msgs.some((m) => m.text.includes(q)))
    const row = (talk) => item({ name: talk.title, side: timeShown(talk.at), open: () => set({ tv: 'one', tid: talk.id, draft: '' }) })
    return tool(L.chatSearch(), 'tq', [L.chatNew(), () => set({ tv: 'new', tid: null, draft: '' })]) +
      rows(found.length ? [head(q ? L.commonFound(String(found.length)) : L.chatRecent()), ...found.map(row)] : [empty(q ? L.commonNotFound() : L.chatEmpty())])
  }
  const talk = S.tv === 'one' ? S.talks.find((x) => x.id === S.tid) : null
  const hint = talk ? L.chatTypeMessage() : L.chatTypeTopic()
  return back(L.chatAll(), () => set({ tv: 'all' })) +
    (talk
      ? `<div class="talk scroll" id="talk">${said(talk)}</div>`
      : `<div class="asks" data-k="asks">${head(L.chatExamples())}${BOOK.asks.map((text) => `<div class="item row"${onClick(offer(text))}><div class="name ell">${t(text)}</div><div class="grow"></div>${ICON.right}</div>`).join('')}</div>`) +
    '<div class="composer" data-k="composer">' +
    `<div class="box send-box">${S.cite ? cited(S.cite, () => set({ cite: null })) : ''}<div class="send-line"><textarea class="in" rows="1" placeholder="${t(L.chatSendHint(hint, keyLabel('Ctrl+Enter')))}" aria-label="${hint}"${onInput((v) => set({ draft: v }))}${onKey(sendKey)}>${t(S.draft)}</textarea><div class="send hit${S.draft.trim() && !S.waiting ? '' : ' off'}" title="${t(L.chatSendTip(keyLabel('Ctrl+Enter')))}"${onClick(() => send())}>${ICON.send}</div></div></div>` +
    // 起草、改写开的讨论跟着那一章，不随开着的章换，也不用选附不附。
    `<div class="attach-line">${talk && talk.origin ? `<div class="fixed ell" title="${t(L.chatFixedTip())}">${t(L.chatFollows(chTitle(talk.key)))}</div>` : tickbox(S.attach, S.vol ? L.chatAttachVolume() : L.chatAttachChapter(), () => set({ attach: !S.attach }), S.vol ? L.chatAttachVolumeTip() : L.chatAttachChapterTip())}<div class="grow"></div>${modelPick()}</div>` +
    '</div>'
}

// ───────── 画：手机预览 ─────────

// 这一章在读者手机上大概的样子。尺寸、边距、字号都是手机上的点（393 宽是常见的 iPhone）；
// 手机里一律写 em，1em 就是手机上的一个字，整个手机再按右栏的大小缩放。行距借用正文的三级行高。
const PHONES = { 窄屏: [360, 780], 标准: [393, 852], 宽屏: [430, 932] }
const SPACING = { 紧凑: 'loose', 适中: 'read', 宽松: 'paper' }
// 选项的字：状态里是中文，显示时换成当前语言；seg 的选项是 [状态值, 图标, 显示的字]。
const wordOpts = (keys, words) => keys.map((k) => [k, null, words()[k]])
const SIZE_WORDS = () => ({ 窄屏: L.previewNarrow(), 标准: L.previewStandard(), 宽屏: L.previewWide() })
const SPACING_WORDS = () => ({ 紧凑: L.previewTight(), 适中: L.previewMedium(), 宽松: L.previewLoose() })
const THEME_WORDS = () => ({ 白天: L.previewDay(), 夜间: L.previewNight(), 护眼: L.previewEyeCare() })
const MODE_WORDS = () => ({ 翻页: L.previewPaged(), 滚动: L.previewScroll() })
const LONG = 5 // 一段在手机上超过几行算长
let phoneAt = 0, phoneKey = '', phoneCaret = '', phoneScreens = 1, phoneLongAt = [] // 看到第几屏（从 0 起）、哪一章、光标上次在哪一屏、共几屏、长段各在第几屏
const phoneSet = (change) => set((s) => ({ phone: { ...s.phone, ...change } }))

// 翻一屏，只在这一章里翻。
function phoneFlip(d) {
  const to = phoneAt + d
  if (to >= 0 && to < phoneScreens) { phoneAt = to; draw() }
}
// 在预览里点的是那一段的第几个字后面：正文的光标放到同一处。
function phoneOff(ev) {
  const r = document.caretRangeFromPoint(ev.clientX, ev.clientY)
  return r && r.startContainer.parentElement === ev.target.closest('[data-phone]') ? r.startOffset : 0
}
// 跳到下一处长段所在的那一屏。
function phoneNextLong() {
  if (!phoneLongAt.length) return
  phoneAt = phoneLongAt.find((n) => n > phoneAt) ?? phoneLongAt[0]
  draw()
}

function phoneView() {
  const c = chapterOf(S.ch)
  if (S.vol || !c) return `<div class="phone-view" data-k="phone-view">${empty(L.previewOpenChapter())}</div>`
  const P = S.phone, [W, H] = PHONES[P.size], e = (px) => +(px / P.fs).toFixed(3) + 'em', flip = P.mode === '翻页'
  const on = S.open === 'phone-look'
  const step = (label, tip, ok, d) => `<div class="hit${ok ? '' : ' off'}" title="${tip}"${onClick(() => ok && phoneSet({ fs: P.fs + d }))}>${label}</div>`
  const look = '<div class="pop phone-pop" data-k="phone-pop">' +
    `<div class="pop-label">${t(L.previewScreen())}</div><div class="pop-seg">${seg(P.size, wordOpts(Object.keys(PHONES), SIZE_WORDS), (v) => phoneSet({ size: v }), L.previewScreen())}</div>` +
    `<div class="pop-label">${t(L.previewFont())}</div><div class="pop-seg"><div class="seg" aria-label="${t(L.previewFont())}">${step('A−', L.previewFontDown(), P.fs > 16, -1)}<div class="phone-fs">${P.fs}</div>${step('A+', L.previewFontUp(), P.fs < 24, 1)}</div></div>` +
    `<div class="pop-label">${t(L.previewLineSpacing())}</div><div class="pop-seg">${seg(P.lh, wordOpts(Object.keys(SPACING), SPACING_WORDS), (v) => phoneSet({ lh: v }), L.previewLineSpacing())}</div>` +
    `<div class="pop-label">${t(L.previewBackground())}</div><div class="pop-seg">${seg(P.theme, wordOpts(['白天', '夜间', '护眼'], THEME_WORDS), (v) => phoneSet({ theme: v }), L.previewBackground())}</div>` + HAIR +
    `<div class="phone-ticks">${tickbox(P.frame, L.previewFrame(), () => phoneSet({ frame: !P.frame }))}${tickbox(P.long, L.previewLongTick(String(LONG)), () => phoneSet({ long: !P.long }))}</div>` +
    `<div class="pop-buttons"><div class="hit link"${onClick(() => set({ phone: { ...PHONE, mode: P.mode } }))}>${t(L.previewReset())}</div></div></div>`
  const tool = `<div class="tool phone-tool" data-k="tool">${seg(P.mode, wordOpts(['翻页', '滚动'], MODE_WORDS), (v) => phoneSet({ mode: v }), L.previewViewMode())}<div class="grow"></div>` +
    `<div class="more hit phone-aa${on ? ' on' : ''}" title="${t(L.previewLayout())}"${onClick(() => set({ open: on ? '' : 'phone-look' }))}>Aa</div>${on ? look : ''}</div>`
  const title = titleOf(c).replace('：', ' ')
  const paras = c.paras.map((p, i) => `<p data-phone="${i}"${onClick((ev) => putCaret({ para: i, off: phoneOff(ev) }, true))}>${t(p)}</p>`).join('')
  const screen = `<div class="phone-screen ${{ 白天: 'light', 夜间: 'dark', 护眼: 'light eye' }[P.theme]}" style="width:${e(W)};height:${e(H)}">` +
    `<div class="phone-status" style="height:${e(44)};padding:0 ${e(28)}"><span>${BOOK.clock}</span><span class="phone-battery"></span></div>` +
    `<div class="phone-page${flip ? '' : ' rolled'}" style="height:${e(H - 86)};margin:${e(8)} ${e(20)} 0">` +
    `<div class="phone-text lh-${SPACING[P.lh]}"${flip ? ` style="column-width:${e(W - 40)};column-gap:${e(40)}"` : ''}><div class="phone-title">${t(title)}</div>${paras}<i class="phone-caret" hidden></i></div></div>` +
    `<div class="phone-foot" style="height:${e(34)};padding:0 ${e(20)}"><span class="ell">${t(title)}</span><span class="phone-at"></span></div></div>`
  return '<div class="phone-view" data-k="phone-view">' + tool +
    `<div class="phone-stage"><div class="phone dark${P.frame ? ' framed' : ''}">${screen}</div></div>` +
    (flip ? `<div class="phone-nav"><div class="hit link phone-back"${onClick(() => phoneFlip(-1))}>‹ ${t(L.previewPrev())}</div><div class="phone-count"></div><div class="hit link phone-next"${onClick(() => phoneFlip(1))}>${t(L.previewNext())} ›</div></div>`
      : '<div class="phone-nav"><div class="grow"></div><div class="phone-count"></div><div class="grow"></div></div>') +
    `<div class="phone-long"><div class="hit link"${onClick(phoneNextLong)}></div></div></div>`
}

// 画完以后量：手机缩放到放得下，正文分成几屏，每段几行，正文里选中的字和光标落在预览的哪儿。只改量出来的那几处，不重画。
function fitPhone() {
  const view = document.querySelector('.phone-view .phone')
  if (!view) return
  const root = view.closest('.phone-view'), stage = root.querySelector('.phone-stage'), page = root.querySelector('.phone-page'), text = root.querySelector('.phone-text')
  const flip = S.phone.mode === '翻页'
  view.style.transform = ''
  const k = Math.min(1, stage.clientWidth / view.offsetWidth, stage.clientHeight / view.offsetHeight)
  view.style.transform = `scale(${k})`
  text.style.transform = ''
  const step = flip ? page.clientWidth + parseFloat(getComputedStyle(text).columnGap) : page.clientHeight
  const base = text.getBoundingClientRect(), lh = parseFloat(getComputedStyle(text.querySelector('p') || text).lineHeight), kz = k * winZoom() // 量出来的长度除以 kz 是手机上的长度
  const at = (r) => Math.floor(((flip ? r.left - base.left : r.top - base.top) / kz + 1) / step) // 一处字落在第几屏
  const paras = [...text.querySelectorAll('p')]
  const all = [...text.children].filter((el) => !el.matches('.phone-caret')).flatMap((el) => [...el.getClientRects()])
  phoneScreens = Math.max(1, all.length ? Math.max(...all.map(at)) + 1 : 1)
  // 正文里选中的字在预览里也选中；没选字时，正文有光标，预览也画一道。
  const span = spanNow(), spot = ({ para, off }) => { const p = paras[para], n = p && p.firstChild; return n ? [n, Math.min(off, n.length)] : p ? [p, 0] : null }
  const from = span && spot(span[0]), to = span && spot(span[1]), pick = new Range(), caret = root.querySelector('.phone-caret')
  if (from && to) { pick.setStart(...from); pick.setEnd(...to) }
  CSS.highlights.set('phone-pick', new Highlight(...(from && to && !pick.collapsed ? [pick] : [])))
  const end = to && new Range()
  if (end) { end.setStart(...to); end.collapse(true) }
  const mark = end && (end.getClientRects()[0] || end.startContainer.getClientRects?.()[0])
  const where = mark ? String(at(mark)) : ''
  caret.hidden = !(mark && pick.collapsed && document.activeElement === proseBox())
  if (mark) Object.assign(caret.style, { left: (mark.left - base.left) / kz + 'px', top: (mark.top - base.top) / kz + 'px', height: mark.height / kz + 'px' })
  // 换了一章：从第一屏看起，光标在哪不算挪过。
  if (S.ch !== phoneKey) { phoneKey = S.ch; phoneCaret = where; phoneAt = 0 }
  // 翻页看时，光标到了另一屏，预览跟过去；作者在预览里往回翻着看时，光标还在同一屏里打字，不把它拉回来。滚动看时跟着正文滚。
  if (flip && mark && where !== phoneCaret) { phoneCaret = where; phoneAt = at(mark) }
  phoneAt = Math.min(Math.max(phoneAt, 0), phoneScreens - 1)
  if (flip) text.style.transform = `translateX(${-phoneAt * step}px)`; else syncPhone()
  phoneLongAt = []
  paras.forEach((p) => {
    const rects = [...p.getClientRects()], lines = Math.round(rects.reduce((h, r) => h + r.height, 0) / kz / lh)
    const long = S.phone.long && lines > LONG
    p.classList.toggle('long', long)
    if (long) { p.title = L.previewLinesTip(String(lines)); if (!phoneLongAt.includes(at(rects[0]))) phoneLongAt.push(at(rects[0])) } else p.removeAttribute('title')
  })
  const word = (el, s) => { if (el) el.textContent = s }
  word(root.querySelector('.phone-at'), flip ? `${phoneAt + 1}/${phoneScreens}` : '')
  word(root.querySelector('.phone-count'), flip ? L.previewScreenOf(String(phoneAt + 1), String(phoneScreens)) : L.previewAbout(String(phoneScreens)))
  root.querySelector('.phone-back')?.classList.toggle('off', phoneAt === 0)
  root.querySelector('.phone-next')?.classList.toggle('off', phoneAt === phoneScreens - 1)
  const longs = paras.filter((p) => p.classList.contains('long')).length
  word(root.querySelector('.phone-long .link'), longs ? (flip ? L.previewLongNext(String(longs), String(LONG)) : L.previewLong(String(longs), String(LONG))) : '')
}

// 滚动看时，预览跟着正文滚：正文滚到几成，就取窗口里几成高的那一处，在预览里放到同样几成高的地方。滚到顶、到底，两边一起到顶、到底。
function syncPhone() {
  const page = document.querySelector('.phone-page.rolled'), paper = document.querySelector('.editor .paper.scroll'), box = proseBox()
  if (!page || !paper || !box) return
  const max = paper.scrollHeight - paper.clientHeight, f = max > 0 ? paper.scrollTop / max : 0
  const pr = paper.getBoundingClientRect(), y = pr.top + f * pr.height, all = blocks(box), mine = page.querySelectorAll('p')
  const n = all.findIndex((el) => el.getBoundingClientRect().bottom >= y)
  if (n < 0 || !mine[n]) { page.scrollTop = page.scrollHeight; return }
  const r = all[n].getBoundingClientRect(), into = Math.min(Math.max((y - r.top) / (r.height || 1), 0), 1)
  page.scrollTop = n === 0 && y < r.top ? 0 : mine[n].offsetTop + into * mine[n].offsetHeight - f * page.clientHeight
}
document.addEventListener('scroll', (ev) => { if (ev.target.matches?.('.editor .paper')) syncPhone() }, true)
document.addEventListener('selectionchange', () => { if (!drawing) fitPhone() })

const TAB_WORDS = () => ({ 批注: L.paneTabNotes(), 设定: L.paneTabLore(), 讨论: L.paneTabChat(), 伏笔: L.paneTabHooks(), 预览: L.paneTabPreview() })

function pane() {
  // 标签上的数是那一页里等作者处理的几条：待确认的设定；原句已删、久未回收的伏笔（和待处理里的「伏笔」同一个数）。
  const waiting = aiOn() ? S.entries.filter((e) => e.place === 'open').length : 0, lost = S.marks.filter((m) => m.st === 'lost' || (m.st === 'open' && BOOK.now - m.ch >= S.late)).length
  // 停在讨论时关掉 AI，这里显示设定；停在哪一页记着，重新开启后回到讨论。
  const at = (S.tab === '讨论' && !aiOn()) || (S.tab === '批注' && S.page !== 'write') ? '设定' : S.tab
  const tab = (name, n) => `<div class="tab hit${name === at ? ' on' : ''}"${onClick(() => set({ tab: name, ...SHUT }))}>${t(TAB_WORDS()[name])}${n ? `<span class="badge">${n}</span>` : ''}</div>`
  const edge = `<div class="pane-edge" title="${t(L.paneResize())}"></div>`, wide = S.paneW ? ` style="width:${S.paneW}px"` : ''
  // 正文那一页有四页（AI 关着时没有讨论）；别的页只留设定。
  if (S.page !== 'write') return `<div id="pane"${wide}>${edge}<div class="tabs"><div class="tab on">${t(L.paneTabLore())}${waiting ? `<span class="badge">${waiting}</span>` : ''}</div></div>${setting()}</div>`
  // 批注不带数：几条就在页边，一眼看得到。
  return `<div id="pane" class="${at === '批注' ? 'margin-pane' : ''}"${wide}>${edge}` + `<div class="tabs">${tab('批注')}${tab('设定', waiting)}${aiOn() ? tab('讨论') : ''}${tab('伏笔', lost)}${tab('预览')}</div>` +
    (at === '批注' ? margin() : at === '设定' ? setting() : at === '伏笔' ? foreshadowing() : at === '预览' ? phoneView() : discussion()) + '</div>'
}

// ───────── 画：批注 ─────────

// 页边的批注：开着的这一章里，对照设定查出来的、在这里埋下或回收的伏笔、AI 从这一章看出来要改的设定。
// 每条记着它说的是第几段的第几个字起、几个字（这几个字找不到就不列），照这个顺序从上往下排。AI 关着时只有伏笔。
function notesOf(cur) {
  if (!cur) return []
  const list = []
  const find = (quote, para) => {
    const i = para !== undefined ? para : cur.paras.findIndex((p) => p.includes(quote)), off = i >= 0 ? (cur.paras[i] || '').indexOf(quote) : -1
    return off >= 0 ? { para: i, off, len: quote.length } : null
  }
  const put = (at, note) => { if (at) list.push({ ...at, ...note }) }
  // 名字有误的只有标题：标题已经说了应为什么。
  for (const c of aiOn() ? S.checks : []) {
    if (c.key !== cur.key) continue
    put(find(c.quote, c.para), { id: c.id, warn: true, kind: CHECK_KINDS()[c.kind] || c.kind, title: c.title,
      text: c.says ? L.notesSays(c.says) : c.as || '', open: () => set({ page: 'check', kind: '全部', cid: c.id, ...SHUT }), acts: checkActs(c) })
  }
  for (const m of S.marks) {
    const open = () => set({ tab: '伏笔', fid: m.id, fv: 'list', renote: null, ...SHUT })
    if ((m.st === 'open' || m.st === 'done') && m.key === cur.key) put(find(m.quote, m.para), { id: m.id, kind: m.st === 'open' ? L.notesKindHook() : L.notesKindHookPaid(), title: nameOf(m), text: '', open })
    if (m.st === 'done' && m.pay && m.pay.key === cur.key) put(find(m.pay.quote, m.pay.para), { id: m.id + '-pay', kind: L.notesKindPayoff(), title: nameOf(m), text: L.notesPlanted(chTitle(m.key)), open })
  }
  // AI 建议的一处变化：钉在这一章头一回写到那个人（那样东西）的地方。
  for (const o of aiOn() ? S.entries.filter((e) => e.place === 'open' && e.from === cur.key) : []) {
    const e = S.entries.find((x) => x.id === o.about), name = e && e.names.find((n) => cur.paras.some((p) => p.includes(n)))
    put(name ? find(name) : { para: 0, off: 0, len: 0 }, { id: o.id, warn: true, kind: L.notesKindIdea(), title: e ? L.notesLoreChanged(e.names[0]) : L.loreNew(), text: o.text, acts: [[L.commonIgnore(), () => dropIdea(o)], [L.commonAccept(), () => takeChange(o)]] })
  }
  return list.sort((a, b) => a.para - b.para || a.off - b.off)
}

const CHECK_KINDS = () => ({ 不一致: L.notesKindInconsistent(), 名字有误: L.notesKindName(), 新出现: L.notesKindNew() })

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
