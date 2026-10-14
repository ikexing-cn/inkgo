// 右栏：设定、伏笔、讨论三页。
'use strict'

// ───────── 动作 ─────────

// 作者对设定做的事，记在修改记录最上面。
const did = (s, said) => [{ id: 'l' + s.n, who: '手动修改', model: '', at: '刚刚', lines: [said] }, ...s.log]

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
      return { entries: [...s.entries, e], n: s.n + 1, form: null, ...SHUT, log: did(s, '新增到正文 · ' + f.cat + '：' + text) }
    }
    const old = s.entries.find((x) => x.id === f.id)
    const said = old.text !== text ? '改写为：' + text : '改归' + f.cat + '：' + clip(text, 24)
    // 名字改了，正文里还写着旧名的话，问一句要不要全书替换。
    const was = old.names[0] || '', n = was && names[0] && was !== names[0] ? s.chapters.reduce((sum, c) => sum + hits(c, was).length, 0) : 0
    return { entries: s.entries.map((x) => (x.id === f.id ? { ...x, names, text, cat: f.cat } : x)), n: s.n + 1, form: null, ...SHUT, log: did(s, said), named: n ? { was, now: names[0], n } : null }
  })
}
// 打开一条设定的修改表。
const editing = (e) => ({ id: e.id, name: e.names[0] || '', names: e.names.slice(1).join('、'), text: e.text, cat: e.cat })

// AI 没整理成功的那一轮再来一次：这回成了，多出一条建议。
function retry(id) {
  const toast = say('正在重新整理…', '', 'busy', [['取消', () => { clearTimeout(timer); timers.delete(timer); unsay(toast) }]])
  const timer = aiLater(1200, () => {
    unsay(toast)
    set((s) => ({
      n: s.n + 1,
      log: [{ id: 'l' + s.n, who: '后台整理', redo: true, model: 'Opus 5.5 · Low', at: '刚刚', lines: ['新增到待确认 · 物品：怀表停在七点一刻，一直没有修。'] }, ...s.log.filter((x) => x.id !== id)],
      entries: [...s.entries, { id: 'e' + s.n, place: 'open', cat: '物品', names: [], text: '怀表停在七点一刻，一直没有修。' }],
    }))
  })
}

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

// 发一句话。右栏的讨论和「讨论」那一页用的是同一份。
function send() {
  const text = S.draft.trim()
  if (!text || S.waiting) return
  if (!S.uses[0].model.trim()) { say('请先配置讨论模型', '', 'fail', [['打开设置', () => set({ dialog: { kind: 'settings', tab: 'AI', edit: '' }, toasts: [], ...SHUT })]]); return }
  const isNew = S.tv !== 'one'
  const id = isNew ? 't' + S.n : S.tid
  const add = (msg) => (talk) => (talk.id === id ? { ...talk, msgs: [...talk.msgs, msg] } : talk)
  const talks = (isNew ? [{ id, title: text, at: '今天', msgs: [] }, ...S.talks] : S.talks).map(add({ me: true, text, cite: S.cite }))
  set({ talks, tid: id, tv: 'one', n: S.n + 1, draft: '', cite: null })
  answer(id)
}

// AI 回这段讨论一句：先想，再写；写完之前可以取消。
let replying = []
function answer(id) {
  const text = BOOK.replies[sent++ % BOOK.replies.length]
  const add = (talk) => (talk.id === id ? { ...talk, msgs: [...talk.msgs, { me: false, text, think: BOOK.think }] } : talk)
  set({ waiting: { id, step: 'think' } })
  replying = [later(600, () => set({ waiting: { id, step: 'reply' } })), later(1200, () => { set((s) => ({ waiting: null, talks: s.talks.map(add) })); tidy() })]
}
// 每聊完一轮，AI 在后台把这一轮整理进设定，修改记录里多一条。
function tidy() {
  set({ tidying: true })
  aiLater(1500, () => set((s) => ({ tidying: false, n: s.n + 1, log: [{ id: 'l' + s.n, who: '后台整理', model: 'Opus 5.5 · Low', at: '刚刚', lines: [], think: BOOK.tidy }, ...s.log] })))
}
function stopReply() {
  for (const id of replying) { clearTimeout(id); timers.delete(id) }
  sent-- // 没回成的那一句，下次接着用
  set({ waiting: null })
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
  const row = (e, find = '') => item({ name: e.names[0] || e.text, about: e.names.length ? near(e.text, find) : '', find, dot: e.place === 'body' && open.some((o) => o.about === e.id), open: () => set({ sid: e.id }) })
  const idea = (o, about) => `<div class="idea">${about ? `<div class="idea-about">${t(about)}</div>` : ''}<div class="text">${t(o.text)}</div>` +
    buttons(button('忽略', () => move(o.id, 'dropped')), button('采纳', () => move(o.id, 'body'), 'primary')) + '</div>'

  if (S.form) return back(S.form.id ? shown.names[0] || shown.cat : '全部设定', () => set({ form: null, ...SHUT })) + form(S.form)

  if (shown) {
    const dropped = shown.place === 'dropped'
    const about = open.filter((o) => o.about === shown.id)
    const from = shown.src ? chapterOf(shown.src.key) : null
    return back(dropped ? '已删除' : '全部设定', () => set({ sid: null }), dropped ? '恢复设定' : '删除设定', () => move(shown.id, dropped ? 'body' : 'dropped')) +
      '<div class="page scroll" data-k="entry">' +
      `<div class="title-line"><div class="title ell">${t(shown.names[0] || shown.cat)}</div>${shown.names.length > 1 ? `<div class="alias ell">别名 ${t(shown.names.slice(1).join('、'))}</div>` : ''}<div class="grow"></div>${button('修改', () => set({ form: editing(shown) }))}</div>` +
      `<div class="text${dropped ? ' dropped' : ''}">${t(shown.text)}</div>` +
      (from ? where(chapterName(from) + '“' + shown.src.quote + '”', goTo(from.key, shown.src)) : '') +
      (about.length ? HAIR + `<div class="ai-head">${DOT}AI 建议</div>` + about.map((o) => idea(o)).join('') : '') +
      '</div>'
  }

  if (S.sv === 'ideas') {
    const of = (o) => S.entries.find((e) => e.id === o.about)
    return back('全部设定', () => set({ sv: 'list' })) +
      `<div class="page scroll" data-k="ideas"><div class="title-line"><div class="title">待确认</div><div class="grow"></div><div class="side">${open.length} 条 AI 建议</div></div>` +
      open.map((o, i) => (i ? HAIR : '') + idea(o, of(o) ? '关于 ' + of(o).names[0] : '新设定')).join('') + '</div>'
  }

  if (S.sv === 'dropped') return back('全部设定', () => set({ sv: 'list' })) + rows([head('已删除'), ...(gone.length ? gone.map((e) => row(e)) : [empty('没有已删除的设定')])])

  if (S.sv === 'log') {
    const log = aiOn() ? S.log : S.log.filter((c) => c.who !== '后台整理') // AI 整理的那几条，AI 关着时不列
    const card = (c) => '<div class="card">' +
      `<div class="card-head"><div class="who">${t(c.who)}${c.redo ? '（重试）' : ''}</div>${c.model ? `<div class="tiny">${t(c.model)}</div>` : ''}<div class="grow"></div><div class="tiny">${t(c.at)}</div></div>` +
      (c.lines.length || c.fail ? c.lines.map((l) => `<div class="said-line">${t(l)}</div>`).join('') : '<div class="extra">无需修改</div>') +
      (c.extra ? `<div class="extra">${t(c.extra)}</div>` : '') + (c.think ? thought(c.id, c.think, false, '整理思考过程') : '') +
      (c.fail ? `<div class="fail">${t(c.fail)}</div><div class="retry hit link"${onClick(() => retry(c.id))}>重试</div>` : '') + '</div>'
    return back('全部设定', () => set({ sv: 'list' })) + `<div class="page log scroll" data-k="log"><div class="title">修改记录</div>${log.length ? log.map(card).join('') : `<div class="note">${aiOn() ? '暂无修改记录。讨论后，AI 会自动整理设定。' : '暂无修改记录。'}</div>`}</div>`
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

// 新增和修改用同一张表。新增先填内容，修改先看名字。
function form(f) {
  const type = (key) => (value) => set((s) => ({ form: { ...s.form, [key]: value } }))
  const name = field(f.id ? '名字' : '名字（选填）', line(f.name, type('name'), '', '名字'))
  const names = field(f.id ? '别名' : '别名（选填）', line(f.names, type('names'), '多个别名用顿号分隔', '别名'))
  const text = field('内容', area(f.text, type('text'), '', '内容'))
  const cat = (label) => `<div class="pop-row row${label === f.cat ? ' on' : ''}"${onClick(() => set((s) => ({ ...SHUT, form: { ...s.form, cat: label } })))}>${t(label)}</div>`
  return '<div class="page scroll" data-k="form">' +
    (f.id ? name + names + text : '<div class="title-line"><div class="title">新增设定</div></div>' + text + name + names) +
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
  const row = (m, find = '') => item({ name: nameOf(m), about: find && !m.note.includes(find) ? near(m.quote, find) : '', find, side: m.key === S.ch ? '本章' : '第 ' + m.ch + ' 章', open: () => set({ fid: m.id, renote: null }) })
  const shown = S.marks.find((m) => m.id === S.fid)

  // 正在选「选择要回收的伏笔」时，右栏摆出选中的那一个，看清楚再记。
  const paying = S.pay && S.marks.find((m) => m.id === payPick())
  if (paying) {
    const gap = BOOK.now - paying.ch
    return '<div class="page scroll" data-k="mark-paying">' + `<div class="title-line"><div class="title ell">${t(nameOf(paying))}</div></div>` +
      `<div class="where"><div class="fit ell">第 ${paying.ch} 章埋下${gap > 0 && !paying.pay ? '，已过 ' + gap + ' 章' : ''}</div></div><div class="quote">${t(paying.quote)}</div>` +
      (paying.pay ? `<div class="note">第 ${paying.pay.ch} 章已有一处回收</div>` : '') + '</div>'
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
      where(st === 'lost' ? '第 ' + shown.ch + ' 章的原句已删除' : '第 ' + shown.ch + ' 章埋下' + (st === 'open' && gap > 0 ? '，已过 ' + gap + ' 章' : ''), goTo(shown.key)) +
      `<div class="quote${st === 'lost' ? ' gone' : ''}">${t(shown.quote)}</div>` + HAIR +
      (st === 'open' ? '<div class="state">未回收</div><div class="note">请先在正文中选中回收的段落</div>' + buttons(button('记为回收', () => askPay(shown.id), 'primary')) : '') +
      (st === 'done' ? where('第 ' + shown.pay.ch + ' 章回收，相隔 ' + Math.max(shown.pay.ch - shown.ch, 0) + ' 章', goTo(shown.pay.key)) + `<div class="quote">${t(shown.pay.quote)}</div>` : '') +
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
    (S.open === 'model' ? '<div class="pop model-pop" data-k="model-pop"><div class="pop-label">模型</div>' + (service ? service.models : []).map((m) => `<div class="pop-row row${m === use.model ? ' on' : ''}"${onClick(pick({ model: m }))}>${t(m)}</div>`).join('') +
      menuRow('模型设置…', () => set({ dialog: { kind: 'settings', tab: 'AI', edit: '' }, ...SHUT })) + HAIR + '<div class="pop-label">思考程度</div>' +
      BOOK.efforts.map((e) => `<div class="pop-row row${e === use.effort ? ' on' : ''}"${onClick(pick({ effort: e }))}>${e}</div>`).join('') + '</div>' : '') + '</div>'
}

// AI 想的过程，折在回复上面；正在想的时候开着。
function thought(key, text, running, label = '思考过程') {
  const open = running || S.thoughts[key]
  return `<div class="thought"><div class="thought-head hit"${onClick(running ? null : () => set((s) => ({ thoughts: { ...s.thoughts, [key]: !open } })))}>${running ? '正在思考' : label}${icon(open ? 'down' : 'right', 'xs')}</div>` +
    (open ? `<div class="thought-text">${t(text)}</div>` : '') + '</div>'
}

// 讨论里的一句。右栏和「讨论」那一页画的是同一个样子；下面那排小按钮只在最后一句和鼠标指着的那句上露出来。
function message(talk, m, i) {
  const key = talk.id + '-' + i, last = i === talk.msgs.length - 1, saw = S.told[key]
  const tool = (label, pic, act, on) => `<button class="icon-btn ${on ? 'ink' : 'muted'}" aria-label="${label}" title="${label}"${onClick(act)}>${icon(pic)}</button>`
  const pull = tool('摘取原文', 'lines', () => set({ dialog: { kind: 'source', text: m.text, cat: S.cats.at(-1), picked: '' }, ...SHUT }))
  if (m.me) {
    const left = last && !(S.waiting && S.waiting.id === talk.id)
    return `<div class="msg me${last ? ' last' : ''}">${m.cite ? cited(m.cite) : ''}<div class="said">${t(m.text)}</div>` +
      `<div class="reply-tools">${pull}<div class="faint">${t(talk.at)}</div></div>` +
      (left ? `<div class="unanswered">${icon('warn', 'sm')}<div>该消息尚未收到回复</div><div class="hit link"${onClick(() => answer(talk.id))}>重试回复</div></div>` : '') + '</div>'
  }
  const copy = () => { if (navigator.clipboard) navigator.clipboard.writeText(m.text).catch(() => {}); say('已复制') }
  const taken = S.entries.filter((e) => e.place === 'body')
  return `<div class="msg${last ? ' last' : ''}"><div class="reply-row"><div class="reply-face">${icon('spark')}</div><div class="fit">${m.think ? thought(key, m.think) : ''}<div class="reply" data-reply="${i}">${t(m.text)}</div>` +
    `<div class="reply-tools"><div class="faint">${t(talk.at)} · ${t(S.uses[0].model)}</div>${pull}${tool('复制全文', 'copy', copy)}${tool('AI 参考内容', 'eye', () => set((s) => ({ told: { ...s.told, [key]: !saw } })), saw)}</div>` +
    (saw ? `<div class="reply-saw"><div class="saw-title">回复时附带 ${taken.length} 条设定，因预算不足省略 0 条</div>` +
      (taken.length ? taken.map((e) => `<div class="saw-row">${icon('tick', 'sm', 'green')}<div class="fit">${t(e.text)}</div></div>`).join('') : '<div class="note">仅参考了最近几轮对话，设定文档暂无内容。</div>') + '</div>' : '') +
    '</div></div></div>'
}
// 一段讨论从头到尾，AI 正在回的话接在最后。
function said(talk) {
  const w = S.waiting && S.waiting.id === talk.id ? S.waiting : null
  return talk.msgs.map((m, i) => message(talk, m, i)).join('') +
    (w ? `<div class="msg last"><div class="reply-row"><div class="reply-face">${icon('spark')}</div><div class="fit">${thought('live', BOOK.think, w.step === 'think')}` +
      `<div class="waiting">${icon('spin', 'sm')}<div class="fit">${w.step === 'think' ? '正在思考…' : '正在回复…'}</div><div class="hit link"${onClick(stopReply)}>取消请求</div></div></div></div></div>` : '')
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
    `<div class="box send-box">${S.cite ? cited(S.cite, () => set({ cite: null })) : ''}<div class="send-line">${typed(S.draft, (v) => set({ draft: v }), hint, hint, keys(() => send()))}<div class="send hit${S.draft.trim() && !S.waiting ? '' : ' off'}" title="发送"${onClick(() => send())}>${ICON.send}</div></div></div>` +
    `<div class="attach-line">${tickbox(S.attach, S.vol ? '附上本卷内容' : '附上本章内容', () => set({ attach: !S.attach }), S.vol ? '将卷纲一并发送给 AI' : '将章纲和光标前约 2000 字一并发送给 AI')}<div class="grow"></div>${modelPick()}</div>` +
    '</div>'
}

function pane() {
  const waiting = aiOn() ? S.entries.filter((e) => e.place === 'open').length : 0, lost = S.marks.filter((m) => m.st === 'lost').length
  const tab = (name, n) => `<div class="tab hit${name === S.tab ? ' on' : ''}"${onClick(() => set({ tab: name, ...SHUT }))}>${name}${n ? `<span class="badge">${n}</span>` : ''}</div>`
  // 后台整理的记号：正在整理时一个呼吸的点，上一回没成功时一个黄点；点开是修改记录。
  const failed = aiOn() && !S.tidying && (S.log.find((c) => c.who === '后台整理') || {}).fail, sign = S.tidying ? '正在整理设定' : '上次整理失败，点击查看'
  const dot = S.tidying || failed ? `<button class="icon-btn tidy${failed ? ' warn' : ''}" aria-label="${sign}" title="${sign}"${onClick(() => set({ paneOpen: true, tab: '设定', sv: 'log', sid: null, form: null, ...SHUT }))}><div class="dot"></div></button>` : ''
  const edge = '<div class="pane-edge" title="调整右栏宽度"></div>', wide = S.paneW ? ` style="width:${S.paneW}px"` : ''
  const fold = `<div class="grow"></div>${dot}<button class="icon-btn" aria-label="收起右栏" title="收起右栏"${onClick(() => set({ paneOpen: false, ...SHUT }))}>${svg('icon-md', PANEL + path('M15 5v14'))}</button>`
  // 正文那一页有三页（AI 关着时没有讨论）；别的页只留设定。
  if (S.page !== 'write') return `<div id="pane"${wide}>${edge}<div class="tabs"><div class="tab on">设定${waiting ? `<span class="badge">${waiting}</span>` : ''}</div>${fold}</div>${setting()}</div>`
  return `<div id="pane"${wide}>${edge}` + `<div class="tabs">${tab('设定', waiting)}${aiOn() ? tab('讨论') : ''}${tab('伏笔', lost)}${fold}</div>` +
    (S.tab === '设定' ? setting() : S.tab === '伏笔' ? foreshadowing() : discussion()) + '</div>'
}
