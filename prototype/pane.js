// 右栏：设定、伏笔、讨论三页。
'use strict'

// ───────── 动作 ─────────

// 作者对设定做的事，记在修改记录最上面。
const did = (s, said) => [{ id: 'l' + s.n, who: '手动修改', model: '', at: stamp(), lines: [said] }, ...s.log]

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
    if (!f.id) { // 回答了的提问不再等作者，跟着答案留一行
      const e = { id: 'e' + s.n, place: 'body', cat: f.cat, names, text, ...(f.ask ? { asked: f.q } : {}) }
      const entries = [...s.entries.filter((x) => x.id !== f.ask), e]
      return { entries, n: s.n + 1, form: null, ...SHUT, log: did(s, '新增到正文 · ' + f.cat + '：' + text), sv: s.sv === 'ideas' && !entries.some((x) => x.place === 'open') ? 'list' : s.sv }
    }
    const old = s.entries.find((x) => x.id === f.id)
    const said = old.text !== text ? '改写为：' + text : '改归' + f.cat + '：' + clip(text, 24)
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
    return { entries: s.entries.map((x) => (x.id === f.id ? { ...x, changes } : x)), n: s.n + 1, form: null, ...SHUT, log: did(s, (e.names[0] || e.cat) + ' · ' + fromWhere(f.key) + '：' + clip(text, 24)) }
  })
}
function dropChange() {
  set((s) => {
    const f = s.form, e = s.entries.find((x) => x.id === f.id), g = e.changes.find((x) => x.id === f.gid)
    return { entries: s.entries.map((x) => (x.id === f.id ? { ...x, changes: x.changes.filter((y) => y.id !== f.gid) } : x)), n: s.n + 1, form: null, ...SHUT, log: did(s, '删除变化 · ' + (e.names[0] || e.cat) + ' · ' + fromWhere(g.key)) }
  })
}
// 采纳 AI 建议的一处变化：记到它说的那一条上，从它说的那一章起。
function takeChange(o) {
  set((s) => {
    const e = s.entries.find((x) => x.id === o.about), changes = [...(e.changes || []).filter((g) => g.key !== o.from), { id: 'g' + s.n, key: o.from, text: o.text }]
    const entries = s.entries.filter((x) => x.id !== o.id).map((x) => (x.id === e.id ? { ...x, changes } : x))
    return { entries, n: s.n + 1, log: did(s, (e.names[0] || e.cat) + ' · ' + fromWhere(o.from) + '：' + clip(o.text, 24)), sv: s.sv === 'ideas' && !entries.some((x) => x.place === 'open') ? 'list' : s.sv }
  })
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
  if (!pick) { say('请先在正文中选中要关联的句子'); return }
  const ch = chapterOf(S.ch)
  mark(id, () => ({ st: 'open', ch: ch.no, key: ch.key, para: pick.para, quote: pick.text }))
  unselect()
}

// 点一句示例：填进输入框，由作者自己发出去。
const offer = (words) => () => { set({ draft: words }); const box = document.querySelector('.send-box .in'); if (box) box.focus() }

// 两处输入框都是 Ctrl+Enter 发出去，回车只是换行：一段话常要分几行写。
const sendKey = (ev) => { if (ev.key === 'Enter' && ev.ctrlKey && !ev.isComposing) send() }

// 发一句话。右栏的讨论和「讨论」那一页用的是同一份。
function send() {
  const text = S.draft.trim()
  if (!text || S.waiting) return
  if (!S.uses[0].model.trim()) { say('请先配置讨论模型', '', 'fail', [['打开设置', () => set({ dialog: { kind: 'settings', tab: 'AI', edit: '' }, toasts: [], ...SHUT })]]); return }
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
  if (!S.uses[0].model.trim()) { say('请先配置讨论模型', '', 'fail', [['打开设置', () => set({ dialog: { kind: 'settings', tab: 'AI', edit: '' }, toasts: [], ...SHUT })]]); return }
  if (S.waiting) { say('AI 正在回复别的讨论', '等它回完或取消后再' + origin, 'fail'); return }
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
      const said = kept ? '待确认 · ' + kept.cat + '：' + kept.text : ''
      const msg = { me: false, at: stamp(), text: r.text, think: BOOK.think, look: kept ? [...r.look, ['记', said, kept.id]] : r.look, use: r.use, capped: r.capped }
      return {
        waiting: null, talks: s.talks.map((talk) => (talk.id === id ? { ...talk, msgs: [...talk.msgs, msg] } : talk)), records: logged(s, talk, r.use),
        ...(kept ? { n: s.n + 1, entries: [...s.entries, kept], log: [{ id: 'l' + s.n, who: '讨论', talk: id, at: stamp(), lines: ['新增到' + said] }, ...s.log] } : {}),
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
      const done = { props: s.props.map((x) => (x.id === pid ? { ...x, writing: false, done: Date.now() } : x)) }
      if (lost) later(0, () => say('AI 提议已作废', '写的时候，它要换掉的原文改动了'))
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
  if (!ch) { say('该章已不存在', '', 'fail'); return }
  const there = ch.paras.some((p) => p.includes(c.quote))
  set((s) => go(s, c.key, there ? { key: c.key, quote: c.quote } : null))
  if (!there) say('该段原文已修改，在本章中未找到')
}

// ───────── 画：三页都用的 ─────────

// 列表顶上：搜索、旁边的按钮（add）、「···」里的几项（more）。
function tool(hint, key, add, more) {
  const on = S.open === 'tool'
  return '<div class="tool" data-k="tool">' +
    `<div class="find box">${ICON.find}${typed(S[key], (v) => set({ [key]: v }), hint, hint)}</div>` +
    (add ? `<div class="add hit"${onClick(add[1])}>${ICON.plus}${t(add[0])}</div>` : '') +
    (more ? `<div class="more hit${on ? ' on' : ''}" title="更多"${onClick(() => set({ open: on ? '' : 'tool' }))}>${ICON.dots}</div>` : '') +
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
  const when = (e) => { const g = asOf(e, here); return g ? fromWhere(g.key) : here && changesOf(e).some((x) => rank(x.key) > rank(here)) ? '之后有变化' : '' }
  const row = (e, find = '') => { const text = textAt(e, here); return item({ name: e.names[0] || text, about: e.names.length ? near(text, find) : '', find, side: e.place === 'body' ? when(e) : '', dot: e.place === 'body' && open.some((o) => o.about === e.id), open: () => set({ sid: e.id }) }) }
  // AI 的提问不是设定，答案才是：不「采纳」，「回答」。
  const answer = (o) => () => set({ form: { id: null, name: '', names: '', text: '', cat: o.cat, ask: o.id, q: o.text }, ...SHUT })
  // AI 建议的一处变化（from），采纳就记到它说的那一条上。
  const idea = (o, about) => `<div class="idea">${about ? `<div class="idea-about">${t(about)}</div>` : ''}${o.from && !about ? `<div class="idea-about">${t(fromWhere(o.from))}</div>` : ''}<div class="text">${t(o.text)}</div>` +
    buttons(button('忽略', () => move(o.id, 'dropped')), asks(o) ? button('回答', answer(o), 'primary') : button('采纳', o.from ? () => takeChange(o) : () => move(o.id, 'body'), 'primary')) + '</div>'

  if (S.form) return back(S.form.id ? shown.names[0] || shown.cat : S.form.ask ? '待确认' : '全部设定', () => set({ form: null, ...SHUT })) + (S.form.change ? changeForm(S.form) : form(S.form))

  if (shown) {
    const dropped = shown.place === 'dropped'
    const about = open.filter((o) => o.about === shown.id)
    const from = shown.src ? chapterOf(shown.src.key) : null
    return back(dropped ? '已删除' : '全部设定', () => set({ sid: null }), dropped ? '恢复设定' : '删除设定', () => move(shown.id, dropped ? 'body' : 'dropped')) +
      '<div class="page scroll" data-k="entry">' +
      `<div class="title-line"><div class="title ell">${t(shown.names[0] || shown.cat)}</div>${shown.names.length > 1 ? `<div class="alias ell">别名 ${t(shown.names.slice(1).join('、'))}</div>` : ''}<div class="grow"></div>${button('修改', () => set({ form: editing(shown) }))}</div>` +
      `<div class="text${dropped ? ' dropped' : ''}">${t(shown.text)}</div>` + (shown.asked ? `<div class="note">回答：${t(shown.asked)}</div>` : '') +
      (from ? where('「' + titleOf(from) + '」“' + shown.src.quote + '”', goTo(from.key, shown.src)) : '') + (dropped ? '' : timeline(shown)) +
      (about.length ? HAIR + `<div class="ai-head">${DOT}AI 建议</div>` + about.map((o) => idea(o)).join('') : '') +
      '</div>'
  }

  if (S.sv === 'ideas') {
    const of = (o) => S.entries.find((e) => e.id === o.about)
    return back('全部设定', () => set({ sv: 'list' })) +
      `<div class="page scroll" data-k="ideas"><div class="title-line"><div class="title">待确认</div><div class="grow"></div><div class="side">${open.length} 条 AI 建议</div></div>` +
      open.map((o, i) => (i ? HAIR : '') + idea(o, of(o) ? '关于 ' + of(o).names[0] + (o.from ? ' · ' + fromWhere(o.from) : '') : '新设定')).join('') + '</div>'
  }

  if (S.sv === 'dropped') return back('全部设定', () => set({ sv: 'list' })) + rows([head('已删除'), ...(gone.length ? gone.map((e) => row(e)) : [empty('没有已删除的设定')])])

  if (S.sv === 'log') {
    const log = aiOn() ? S.log : S.log.filter((c) => !c.talk) // 讨论时 AI 记下的那几条，AI 关着时不列
    const talk = (id) => S.talks.find((x) => x.id === id)
    const card = (c) => '<div class="card">' +
      `<div class="card-head"><div class="who">${t(c.who)}</div><div class="grow"></div><div class="tiny">${t(c.at)}</div></div>` +
      c.lines.map((l) => `<div class="said-line">${t(l)}</div>`).join('') +
      (c.talk && talk(c.talk) ? where('记在「' + talk(c.talk).title + '」', () => set({ page: 'talk', tv: 'one', tid: c.talk, ...SHUT }), '查看讨论') : '') + '</div>'
    return back('全部设定', () => set({ sv: 'list' })) + `<div class="page log scroll" data-k="log"><div class="title">修改记录</div>${log.length ? log.map(card).join('') : `<div class="note">${aiOn() ? '暂无修改记录。讨论中 AI 记下的也列在这里。' : '暂无修改记录。'}</div>`}</div>`
  }

  const q = S.sq.trim()
  let list = []
  if (q) {
    const found = body.filter((e) => (e.names.join('、') + e.text).includes(q))
    list = found.length ? [head('找到 ' + found.length + ' 条'), ...found.map((e) => row(e, q))] : [empty('未找到')]
  } else if (!body.length && !open.length) {
    list = [empty('暂无设定', aiOn() ? '讨论和正文中的设定会自动收录于此，也可点击「新增」手动添加。' : '可点击「新增」添加，或从正文中选中文字加入。')]
  } else {
    if (open.length) list.push(todo('待确认', open.length, () => set({ sv: 'ideas' })))
    for (const cat of S.cats) {
      const of = body.filter((e) => e.cat === cat)
      if (of.length) list.push(head(cat), ...of.map((e) => row(e)))
    }
  }
  const r = S.named
  if (r) list.unshift(`<div class="renamed" data-k="renamed"><div class="fit">正文中仍有 ${r.n} 处「${t(r.was)}」</div><div class="hit link" title="将旧名替换为新名"${onClick(() => set((s) => ({ named: null, page: 'write', focus: false, find: chapterOf(s.ch) ? { q: r.was, to: r.now, all: true, at: 0 } : null })))}>全书替换</div>` +
    `<button class="icon-btn muted" aria-label="保留旧名" title="不替换"${onClick(() => set({ named: null }))}>${icon('cross', 'sm')}</button></div>`)
  return tool('搜索设定', 'sq', ['新增', () => set({ form: { id: null, name: '', names: '', text: '', cat: S.cats[0] }, ...SHUT })],
    [['已删除', gone.length, () => set({ sv: 'dropped', ...SHUT })], ['修改记录', '', () => set({ sv: 'log', ...SHUT })]]) + rows(list)
}

// 一条设定随章节的变化，接在原来的样子下面，照侧栏的先后排。正文页开着的那一章用的是哪一段，标「本章适用」；变化所在的章进了回收站，这处不起作用，写「所在章节已删除」。
// 开着一章（或一卷）时，下面有「从本章起有变化」：本章已经有一处的，就是改它。
function timeline(e) {
  const here = S.page === 'write' ? S.ch || S.vol : '', on = asOf(e, here), all = e.changes || []
  const gone = all.filter((g) => rank(g.key) < 0), live = changesOf(e)
  const mine = here && all.find((g) => g.key === here)
  const step = (label, text, g, cur) => `<div class="step${g && rank(g.key) < 0 ? ' gone' : ''}"><div class="step-head"><div class="fit ell">${t(label)}</div>${cur ? '<div class="tag">本章适用</div>' : ''}` +
    (g ? `<div class="hit link"${onClick(() => set({ form: changing(e, g, g.key), ...SHUT }))}>修改</div>` : '') + `</div><div class="step-text">${t(text)}</div></div>`
  const add = here && !mine ? `<div class="hit link add-change"${onClick(() => set({ form: changing(e, null, here), ...SHUT }))}>从${S.vol ? '本卷' : '本章'}起有变化</div>` : ''
  if (!all.length) return add ? HAIR + add : ''
  return HAIR + `<div class="ai-head"><div class="grow">随章节的变化</div>${add}</div>` + (here && !on ? '<div class="note">本章适用上面原来的样子。</div>' : '') +
    live.map((g) => step(fromWhere(g.key), g.text, g, on && on.id === g.id)).join('') + gone.map((g) => step('所在章节已删除 · 不起作用', g.text, g, false)).join('')
}

// 记一处变化：从哪一章起（只能选书里有的章），那以后这一条的全文。
function changeForm(f) {
  const e = S.entries.find((x) => x.id === f.id), titles = S.chapters.map(titleOf), now = chapterOf(f.key)
  const pick = (title) => set((s) => ({ form: { ...s.form, key: S.chapters[titles.indexOf(title)].key } }))
  return '<div class="page scroll" data-k="change-form">' +
    `<div class="title-line"><div class="title">${t(e.names[0] || e.cat)} · ${t(fromWhere(f.key))}</div></div><div class="note">从这一章起，这一条就是下面这段话；起草、改写这一章及以后的章时，AI 看到的是它。</div>` +
    field('从哪一章起', choose('change-from', now ? titleOf(now) : '所在章节已删除', titles, pick, '从哪一章起')) +
    field('内容', area(f.text, (v) => set((s) => ({ form: { ...s.form, text: v } })), '', '变化后的内容', 5, ' autofocus')) +
    `<div class="buttons">${f.gid ? `<div class="hit link danger"${onClick(dropChange)}>删除这处变化</div><div class="grow"></div>` : ''}${button('取消', () => set({ form: null, ...SHUT }))}${button('保存', saveChange, f.text.trim() ? 'primary' : 'primary off')}</div>` +
    '</div>'
}

// 新增和修改用同一张表。新增先填内容，修改先看名字：光标就在那一格里。
function form(f) {
  const type = (key) => (value) => set((s) => ({ form: { ...s.form, [key]: value } }))
  const name = field(f.id ? '名字' : '名字（选填）', line(f.name, type('name'), '', '名字', f.id ? ' autofocus' : ''))
  const names = field(f.id ? '别名' : '别名（选填）', line(f.names, type('names'), '多个别名用顿号分隔', '别名'))
  const text = field('内容', area(f.text, type('text'), '', '内容', 4, f.id ? '' : ' autofocus'))
  const cat = (label) => `<div class="pop-row row${label === f.cat ? ' on' : ''}"${onClick(() => set((s) => ({ ...SHUT, form: { ...s.form, cat: label } })))}>${t(label)}</div>`
  return '<div class="page scroll" data-k="form">' +
    (f.id ? name + names + text : `<div class="title-line"><div class="title">${f.ask ? '回答' : '新增设定'}</div></div>` + (f.ask ? `<div class="note">${t(f.q)}</div>` : '') + text + name + names) +
    `<div class="field cat"><div class="label">分类</div><div class="box line hit"${onClick(() => set({ open: S.open === 'cats' ? '' : 'cats' }))}>${t(f.cat)}<div class="grow"></div>${ICON.down}</div>` +
    (S.open === 'cats' ? `<div class="cat-menu pop">${S.cats.map(cat).join('')}</div>` : '') + '</div>' +
    buttons(button('取消', () => set({ form: null, ...SHUT })), button('保存', save, f.text.trim() ? 'primary' : 'primary off')) +
    '</div>'
}

// ───────── 画：伏笔 ─────────

const nameOf = (m) => m.note || clip(m.quote, 14)

function foreshadowing() {
  const of = (st) => S.marks.filter((m) => m.st === st)
  const groups = { lost: '原句已删', done: '已回收', dropped: '已放弃' }
  const row = (m, find = '') => item({ name: nameOf(m), about: find && !m.note.includes(find) ? near(m.quote, find) : '', find, side: m.key === S.ch ? '本章' : chTitle(m.key), open: () => set({ fid: m.id, renote: null }) })
  const shown = S.marks.find((m) => m.id === S.fid)

  // 正在选「选择要回收的伏笔」时，右栏摆出选中的那一个，看清楚再记。
  const paying = S.pay && S.marks.find((m) => m.id === payPick())
  if (paying) {
    const gap = BOOK.now - paying.ch
    return '<div class="page scroll" data-k="mark-paying">' + `<div class="title-line"><div class="title ell">${t(nameOf(paying))}</div></div>` +
      `<div class="where"><div class="fit ell">埋在「${t(chTitle(paying.key))}」${gap > 0 && !paying.pay ? '，已过 ' + gap + ' 章' : ''}</div></div><div class="quote">${t(paying.quote)}</div>` +
      (paying.pay ? `<div class="note">「${t(chTitle(paying.pay.key))}」里已有一处回收</div>` : '') + '</div>'
  }

  if (shown) {
    const st = shown.st, gap = BOOK.now - shown.ch
    const act = { open: ['放弃伏笔', () => ({ st: 'dropped' })], done: ['取消回收', () => ({ st: 'open', pay: null })], dropped: ['恢复伏笔', () => ({ st: 'open' })] }[st]
    const remove = () => set((s) => ({ marks: s.marks.filter((m) => m.id !== shown.id), fid: null, renote: null }))
    const saveNote = () => mark(shown.id, () => ({ note: S.renote.trim() || shown.note }))
    return back('全部伏笔', () => set({ fid: null, renote: null }), act ? act[0] : '删除伏笔', act ? () => mark(shown.id, act[1]) : remove) +
      '<div class="page scroll" data-k="mark">' +
      (S.renote !== null
        ? `<div class="title-line"><div class="box line fit">${typed(S.renote, (v) => set({ renote: v }), '备注', '修改伏笔备注', keys(saveNote, () => set({ renote: null })) + ' autofocus')}</div>${button('保存', saveNote, 'primary')}</div>`
        : `<div class="title-line"><div class="title ell">${t(nameOf(shown))}</div><div class="grow"></div>${st === 'lost' ? '' : button('修改备注', () => set({ renote: shown.note }))}</div>`) +
      where(st === 'lost' ? '「' + chTitle(shown.key) + '」里的原句已删除' : '埋在「' + chTitle(shown.key) + '」' + (st === 'open' && gap > 0 ? '，已过 ' + gap + ' 章' : ''), goTo(shown.key)) +
      `<div class="quote${st === 'lost' ? ' gone' : ''}">${t(shown.quote)}</div>` + HAIR +
      (st === 'open' ? '<div class="state">未回收</div><div class="note">请先在正文中选中回收的段落</div>' + buttons(button('记为回收', () => askPay(shown.id), 'primary')) : '') +
      (st === 'done' ? where('回收在「' + chTitle(shown.pay.key) + '」，相隔 ' + Math.max(shown.pay.ch - shown.ch, 0) + ' 章', goTo(shown.pay.key)) + `<div class="quote">${t(shown.pay.quote)}</div>` : '') +
      (st === 'lost' ? '<div class="note">请先在正文中选中新的句子</div>' + buttons(button('更换原句', () => rehang(shown.id), 'primary')) : '') +
      (st === 'dropped' ? '<div class="state">已放弃</div>' : '') +
      '</div>'
  }

  if (S.fv !== 'list') return back('全部伏笔', () => set({ fv: 'list' })) + rows([head(groups[S.fv]), ...(of(S.fv).length ? of(S.fv).map((m) => row(m)) : [empty('此处暂无伏笔')])])

  const q = S.fq.trim()
  let list = []
  if (q) {
    const found = S.marks.filter((m) => (m.note + m.quote).includes(q))
    list = found.length ? [head('找到 ' + found.length + ' 条'), ...found.map((m) => row(m, q))] : [empty('未找到')]
  } else if (!S.marks.length) {
    list = [empty('暂无伏笔', '在正文中选中一句，右键选择「记为伏笔」')]
  } else {
    const lost = of('lost'), long = of('open').filter((m) => BOOK.now - m.ch >= S.late), rest = of('open').filter((m) => BOOK.now - m.ch < S.late)
    if (lost.length) list.push(todo(groups.lost, lost.length, () => set({ fv: 'lost' })))
    if (long.length) list.push(head('久未回收'), ...long.map((m) => row(m)))
    if (rest.length) list.push(head('未回收'), ...rest.map((m) => row(m)))
    if (!list.length) list.push(empty('没有未回收的伏笔'))
  }
  return tool('搜索伏笔', 'fq', null, ['done', 'dropped'].map((st) => [groups[st], of(st).length, () => set({ fv: st, ...SHUT })])) + rows(list)
}

// ───────── 画：讨论 ─────────

// 从正文拿来的一段：哪一章、哪句话。drop 有的话，右边多一个去掉它的叉。
const cited = (c, drop) => `<div class="cited hit" title="在正文中定位此段"${onClick(() => findCited(c))}><div class="fit"><div class="cited-from">《${t(c.title)}》</div><div class="note">${t(c.quote)}</div></div>${drop ? `<button class="icon-btn muted" aria-label="移除此段引用" title="移除此段引用"${onClick(drop)}>${icon('cross', 'sm')}</button>` : ''}</div>`

// 输入框下面选模型的那几个字，点开能换模型和思考程度。
function modelPick() {
  const use = S.uses[0], service = S.services.find((x) => x.name === use.service)
  const pick = (change) => () => set((s) => ({ ...SHUT, uses: s.uses.map((u, i) => (i ? u : { ...u, ...change })) }))
  return `<div class="anchor up"><div class="model hit" title="当前模型"${onClick(() => set({ open: S.open === 'model' ? '' : 'model' }))}>${t(use.model || '选择讨论模型')}${icon('up', 'xs')}</div>` +
    (S.open === 'model' ? '<div class="pop model-pop" data-k="model-pop"><div class="pop-label">模型</div>' + (service ? service.models : []).map(({ id }) => `<div class="pop-row row${id === use.model ? ' on' : ''}"${onClick(pick({ model: id }))}>${t(id)}</div>`).join('') +
      menuRow('AI 配置…', () => set({ dialog: { kind: 'settings', tab: 'AI', edit: '' }, ...SHUT })) + HAIR + '<div class="pop-label">思考程度</div>' +
      BOOK.efforts.map((e) => `<div class="pop-row row${e === use.effort ? ' on' : ''}"${onClick(pick({ effort: e }))}>${e}</div>`).join('') + '</div>' : '') + '</div>'
}

// AI 想的过程，折在回复上面；正在想的时候开着。
function thought(key, text, running, label = '思考过程') {
  const open = running || S.thoughts[key]
  return `<div class="thought"><div class="thought-head hit"${onClick(running ? null : () => set((s) => ({ thoughts: { ...s.thoughts, [key]: !open } })))}>${running ? '正在思考' : label}${icon(open ? 'down' : 'right', 'xs')}</div>` +
    (open ? `<div class="thought-text">${t(text)}</div>` : '') + '</div>'
}

// 回复前 AI 查了什么、记了什么，折在回复上面，和「思考过程」一个样子；正在查的时候开着。
// 每一项点开能看查到的；记下的那一条点一下去设定里看它。
const LOOK = { 搜: '搜了', 读: '读了', 设定: '查了设定', 讨论: '搜了讨论', 记: '记下：' }
const lookLine = ([act, what]) => LOOK[act] + what
// 在右栏打开设定里的一条：等作者决定的在「AI 建议」里，别的在列表里选中它。
const showEntry = (entry) => set({ paneOpen: true, tab: '设定', form: null, ...SHUT, ...(entry.place === 'open' ? { sv: 'ideas', sid: null } : { sv: 'list', sid: entry.id }) })
function looked(key, look, running) {
  const open = running || S.thoughts[key]
  const row = ([act, what, got]) => {
    const entry = act === '记' && S.entries.find((e) => e.id === got)
    return entry ? where(lookLine([act, what]), () => showEntry(entry), '查看设定')
      : `<div class="look-row"><div>${t(lookLine([act, what]))}</div>${got && act !== '记' ? `<div class="look-got">${t(got)}</div>` : ''}</div>`
  }
  return `<div class="thought"><div class="thought-head hit"${onClick(running ? null : () => set((s) => ({ thoughts: { ...s.thoughts, [key]: !open } })))}><div class="ell">${running ? '正在查阅' : t(look.map(lookLine).join('，'))}</div>${icon(open ? 'down' : 'right', 'xs')}</div>` +
    (open ? `<div class="thought-text">${look.map(row).join('')}</div>` : '') + '</div>'
}

// 「AI 参考内容」：这次发给 AI 的各部分各约多少 token，附了哪些设定，服务商算的实际用量。
// 查了东西的一轮前后发了几次请求，说的都是最后一次，写出回复的那次。
function outgoing(talk, m, i) {
  // 发出去的是整份设定：正文、已放弃、悬而未决都在，后两种写明是哪一种；有名字的条目，名字在内容前面；回答过的问题跟在答案后面。目录每条只有分类、第一个名字和第一句。
  // 起草、改写的讨论跟着一章：设定按那一章时的样子，后面的变化不给，只说还有几处。普通讨论给最新的样子，每处变化写「第几章起」。
  const [input, cached, output] = m.use.at(-1), sent = ['body', 'dropped', 'open'].flatMap((p) => S.entries.filter((e) => e.place === p && !e.from))
  const label = { body: '', dropped: '已放弃：', open: '悬而未决：' }
  const first = (text) => clip(text.match(/^[^。！？；\n]*[！？]?/)[0], 60)
  const fixed = talk.origin ? talk.key : '', ahead = fixed ? sent.flatMap(changesOf).filter((g) => rank(g.key) > rank(fixed)).length : 0
  const digested = talk.digest && i >= talk.digest.after
  const own = { 起草: [['章纲和卷纲', 260], ['本卷前面各章章纲', 310], ['上一章全文', 5060]], 改写: [['章纲和卷纲', 260], ['本章全文', 4300]] }[talk.origin] || []
  const parts = [['规则', 1100], [S.crowded ? '设定目录' : '设定 ' + sent.length + ' 条', S.crowded ? sent.length * 40 : sent.length * 260], ...own, ...(digested ? [['讨论摘要', 820]] : []), ['查阅结果', (m.look || []).length * 640], ['本次发言', 40]]
  parts.splice(parts.length - 1, 0, ['之前的对话', Math.max(Math.round(input * 0.97) - parts.reduce((n, [, x]) => n + x, 0), 0)])
  const head = S.crowded ? '设定较多，本次只附目录' : fixed ? '附带 ' + sent.length + ' 条设定，按「' + chTitle(fixed) + '」时的样子' : '附带全部 ' + sent.length + ' 条设定'
  const named = (e) => (e.names.length ? e.names.join('、') + '：' : '')
  const changes = (e) => (fixed ? '' : changesOf(e).map((g) => `<div class="note">${t(fromWhere(g.key) + '：' + g.text)}</div>`).join(''))
  const rows = S.crowded ? sent.map((e) => `<div class="saw-row"><div class="fit">${t(label[e.place] + e.cat + (e.names[0] ? ' · ' + e.names[0] : '') + '：' + first(textAt(e, fixed)))}</div></div>`).join('')
    : sent.map((e) => `<div class="saw-row">${icon('tick', 'sm', 'green')}<div class="fit"><div>${t(label[e.place] + named(e) + textAt(e, fixed) + (e.asked ? '（回答：' + e.asked + '）' : ''))}</div>${changes(e)}</div></div>`).join('')
  return '<div class="reply-saw">' +
    `<div class="saw-title">本次发送约 ${tokens(parts.reduce((n, [, x]) => n + x, 0))} token</div>${m.use.length > 1 ? `<div class="note">这一轮共 ${m.use.length} 次请求，这是最后一次。</div>` : ''}<div class="note">${parts.filter(([, x]) => x).map(([name, x]) => t(name) + ' ' + tokens(x)).join(' · ')}</div>` +
    `<div class="note">实际用量：输入 ${tokens(input)}${cached ? '，其中缓存命中 ' + tokens(cached) : ''}；输出 ${tokens(output)}</div>` +
    `<div class="saw-title">${t(head)}</div>${S.crowded ? '<div class="note">目录只有分类、名字和第一句；需要时 AI 会查全文。</div>' : ''}${ahead ? `<div class="note">后面还有 ${ahead} 处变化，未列出。</div>` : ''}` +
    (sent.length ? rows : '<div class="note">设定文档暂无内容。</div>') + '</div>'
}

// 回复里 AI 在正文里提的那一处：改哪一章、改哪里、多少字，作者定了没有。还没定的，点「在正文中查看」过去看。
function propCard(m) {
  const p = S.props.find((x) => x.id === m.prop), words = p ? p.paras.join('').length : m.words
  const state = S.fate[m.prop] || (!p ? '已丢弃' : p.writing ? '正在写…' : '等你决定')
  return `<div class="prop-card"><div class="prop-card-head">${icon('pen', 'sm', p ? 'green' : 'muted')}<div class="fit ell">提议修改「${t(chTitle(m.key))}」</div>` +
    (p ? `<div class="hit link"${onClick(() => set((s) => go(s, m.key)))}>在正文中查看</div>` : '') + `</div><div class="note">${t(m.where)} · ${words} 字 · ${t(state)}</div></div>`
}

// 讨论长了压缩过的地方：前面几轮整理成了一份摘要，原文还在上面。点开看摘要，只能看不能改。
function digest(talk) {
  const d = talk.digest, key = 'digest-' + talk.id, open = S.thoughts[key]
  return `<div class="digest"><div class="digest-head hit"${onClick(() => set((s) => ({ thoughts: { ...s.thoughts, [key]: !open } })))}>以上 ${d.after / 2} 轮已整理成摘要${icon(open ? 'down' : 'right', 'xs')}</div>` +
    (open ? `<div class="digest-text">${d.text.map(([name, said]) => `<div><div class="saw-title">${t(name)}</div><div>${said.split('\n').map(digestLine).join('')}</div></div>`).join('')}<div class="note">摘要由 AI 写成，有误可在对话中指出。</div></div>` : '') + '</div>'
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
  const pull = tool('摘取原文', 'lines', () => set({ dialog: { kind: 'source', text: m.text, cat: S.cats.at(-1), picked: '' }, ...SHUT }), false, ' data-keep')
  if (m.me) {
    const left = last && !(S.waiting && S.waiting.id === talk.id)
    return `<div class="msg me${last ? ' last' : ''}">${m.cite ? cited(m.cite) : ''}<div class="said">${t(m.text)}</div>` +
      `<div class="reply-tools">${pull}<div class="faint">${t(m.at)}</div></div>` +
      (left ? `<div class="unanswered">${icon('warn', 'sm')}<div>该消息尚未收到回复</div><div class="hit link${S.waiting ? ' off' : ''}"${onClick(S.waiting ? null : () => answer(talk.id))}>重试回复</div></div>` : '') + '</div>'
  }
  const copy = () => { if (navigator.clipboard) navigator.clipboard.writeText(m.text).catch(() => {}); say('已复制', '', 'ok') }
  // 这一轮所有请求的合计，和按内置价格估的花费；表里没有价格的模型不写花费。
  const cost = m.use && price(S.uses[0].model, m.use), use = m.use ? ' · ' + tokens(m.use.reduce((n, [input, , output]) => n + input + output, 0)) + ' token' + (cost ? ' · ' + cost : '') : ''
  return `<div class="msg${last ? ' last' : ''}"><div class="reply-row"><div class="reply-face">${icon('spark')}</div><div class="fit">${m.think ? thought(key, m.think) : ''}${m.look && m.look.length ? looked('look-' + key, m.look) : ''}${m.prop ? propCard(m) : ''}<div class="reply" data-reply="${i}">${t(m.text)}</div>` +
    (m.capped ? '<div class="note">已达到本次查阅上限，以上回答仅依据已查到的内容。</div>' : '') +
    `<div class="reply-tools"><div class="faint">${t(m.at)} · ${t(S.uses[0].model)}${use}</div>${pull}${tool('复制全文', 'copy', copy)}${m.use ? tool('AI 参考内容', 'eye', () => set((s) => ({ told: { ...s.told, [key]: !saw } })), saw) : ''}</div>` +
    (saw && m.use ? outgoing(talk, m, i) : '') + '</div></div></div>'
}
// 一段讨论从头到尾，AI 正在回的话接在最后。压缩过的，在压缩的地方隔一行。
function said(talk) {
  const w = S.waiting && S.waiting.id === talk.id ? S.waiting : null
  return talk.msgs.map((m, i) => message(talk, m, i) + (talk.digest && i === talk.digest.after - 1 ? digest(talk) : '')).join('') +
    (w ? `<div class="msg last"><div class="reply-row"><div class="reply-face">${icon('spark')}</div><div class="fit">${w.step === 'pack' ? '' : thought('live', BOOK.think, w.step === 'think')}${w.look && w.look.length ? looked('live-look', w.look, w.step === 'look') : ''}` +
      (w.prop ? propCard({ prop: w.prop, key: talk.key, where: { end: '接在章末', all: '换掉全章', swap: '换掉一句' }[(S.props.find((p) => p.id === w.prop) || { at: { kind: 'end' } }).at.kind] }) : '') +
      `<div class="waiting">${icon('spin', 'sm')}<div class="fit">${{ pack: '正在压缩上下文…', think: '正在思考…', look: '正在查阅…', reply: '正在回复…', propose: '正在正文中提议修改…' }[w.step]}</div><div class="hit link"${onClick(stopReply)}>取消请求</div></div></div></div></div>` : '')
}

function discussion() {
  if (S.tv === 'all') {
    const q = S.tq.trim()
    const found = S.talks.filter((talk) => !q || talk.title.includes(q) || talk.msgs.some((m) => m.text.includes(q)))
    const row = (talk) => item({ name: talk.title, side: talk.at, open: () => set({ tv: 'one', tid: talk.id, draft: '' }) })
    return tool('搜索讨论', 'tq', ['新讨论', () => set({ tv: 'new', tid: null, draft: '' })]) +
      rows(found.length ? [head(q ? '找到 ' + found.length + ' 条' : '最近'), ...found.map(row)] : [empty(q ? '未找到' : '暂无讨论')])
  }
  const talk = S.tv === 'one' ? S.talks.find((x) => x.id === S.tid) : null
  const hint = talk ? '输入消息' : '输入想讨论的内容'
  return back('全部讨论', () => set({ tv: 'all' })) +
    (talk
      ? `<div class="talk scroll" id="talk">${said(talk)}</div>`
      : `<div class="asks" data-k="asks">${head('示例问题')}${BOOK.asks.map((text) => `<div class="item row"${onClick(offer(text))}><div class="name ell">${t(text)}</div><div class="grow"></div>${ICON.right}</div>`).join('')}</div>`) +
    '<div class="composer" data-k="composer">' +
    `<div class="box send-box">${S.cite ? cited(S.cite, () => set({ cite: null })) : ''}<div class="send-line"><textarea class="in" rows="1" placeholder="${hint}，Ctrl+Enter 发送" aria-label="${hint}"${onInput((v) => set({ draft: v }))}${onKey(sendKey)}>${t(S.draft)}</textarea><div class="send hit${S.draft.trim() && !S.waiting ? '' : ' off'}" title="发送（Ctrl+Enter）"${onClick(() => send())}>${ICON.send}</div></div></div>` +
    // 起草、改写开的讨论跟着那一章，不随开着的章换，也不用选附不附。
    `<div class="attach-line">${talk && talk.origin ? `<div class="fixed ell" title="这段讨论附上的正文和设定都按这一章">跟着「${t(chTitle(talk.key))}」</div>` : tickbox(S.attach, S.vol ? '附上本卷内容' : '附上本章内容', () => set({ attach: !S.attach }), S.vol ? '将卷纲一并发送给 AI' : '将章纲和光标前约 2000 字一并发送给 AI')}<div class="grow"></div>${modelPick()}</div>` +
    '</div>'
}

function pane() {
  const waiting = aiOn() ? S.entries.filter((e) => e.place === 'open').length : 0, lost = S.marks.filter((m) => m.st === 'lost').length
  // 停在讨论时关掉 AI，这里显示设定；停在哪一页记着，重新开启后回到讨论。
  const at = S.tab === '讨论' && !aiOn() ? '设定' : S.tab
  const tab = (name, n) => `<div class="tab hit${name === at ? ' on' : ''}"${onClick(() => set({ tab: name, ...SHUT }))}>${name}${n ? `<span class="badge">${n}</span>` : ''}</div>`
  const edge = '<div class="pane-edge" title="调整右栏宽度"></div>', wide = S.paneW ? ` style="width:${S.paneW}px"` : ''
  const fold = `<div class="grow"></div><button class="icon-btn" aria-label="收起右栏" title="收起右栏"${onClick(() => set({ paneOpen: false, ...SHUT }))}>${svg('icon-md', PANEL + path('M15 5v14'))}</button>`
  // 正文那一页有三页（AI 关着时没有讨论）；别的页只留设定。
  if (S.page !== 'write') return `<div id="pane"${wide}>${edge}<div class="tabs"><div class="tab on">设定${waiting ? `<span class="badge">${waiting}</span>` : ''}</div>${fold}</div>${setting()}</div>`
  return `<div id="pane"${wide}>${edge}` + `<div class="tabs">${tab('设定', waiting)}${aiOn() ? tab('讨论') : ''}${tab('伏笔', lost)}${fold}</div>` +
    (at === '设定' ? setting() : at === '伏笔' ? foreshadowing() : discussion()) + '</div>'
}
