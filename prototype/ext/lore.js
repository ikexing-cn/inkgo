// 扩展：lore。界面、动作和登记都在这里。
'use strict'

// 作者对设定做的事，记在修改记录最上面。
const did = (s, said) => [{ id: 'l' + s.n, who: '手动修改', model: '', at: stamp(), lines: [said] }, ...s.log]
// 状态里存的是中文（记录的人、日期、位置、废弃的理由），显示时换成当前语言。
const WHO = () => ({ 手动修改: L.loreWhoManual(), 讨论: L.paneTabChat() })

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
      `<div class="page scroll" data-k="ideas"><div class="title-line"><div class="title">${t(L.loreToConfirm())}</div><div class="grow"></div><div class="side">${t(L.loreAiCount(open.length))}</div></div>` +
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
    list = found.length ? [head(L.commonFound(found.length)), ...found.map((e) => row(e, q))] : [empty(L.commonNotFound())]
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
  if (r) list.unshift(`<div class="renamed" data-k="renamed"><div class="fit">${t(L.loreRenamedLeft(r.n, r.was))}</div><div class="hit link" title="${t(L.loreReplaceAllTip())}"${onClick(() => set((s) => ({ named: null, page: 'write', focus: false, find: chapterOf(s.ch) ? { q: r.was, to: r.now, all: true, at: 0 } : null })))}>${t(L.loreReplaceAll())}</div>` +
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

// 加入设定：选中的字先放进框里，可以改成自己的说法。
function takeDialog(d) {
  const type = (key) => (v) => set((s) => ({ dialog: { ...s.dialog, [key]: v } }))
  const add = () => {
    const text = d.text.trim()
    if (!text) return
    set((s) => ({ dialog: null, ...SHUT, n: s.n + 1, entries: [...s.entries, { id: 'e' + s.n, place: 'body', cat: d.cat, names: [], text, src: d.src }], log: did(s, L.dialogLogEntry(d.cat, text)) }))
    say(L.dialogSettingAdded(), '', 'ok')
  }
  return dialog('small', dialogHead(L.dialogAddSetting(), L.dialogTakeAbout()) +
    field(L.dialogSettingText(), area(d.text, type('text'), '', L.dialogSettingText(), 4)) + field(L.dialogSettingCat(), choose('take-cat', d.cat, S.cats, type('cat'), L.dialogSettingCat())) +
    buttons(button(L.commonCancel(), shut), button(L.dialogAdd(), add, d.text.trim() ? 'primary' : 'primary off')))
}

// 起草在一段新的讨论里，用讨论模型。这里说清楚 AI 预先带上什么、大约多少 token；别的它要用时自己查。不预先报价。

// 把选中的一段记成设定：先问一句记成什么。
function take() {
  const pick = grab() || S.pick
  if (!pick) return
  set({ ctx: null, pick: null, bar: null, dialog: { kind: 'take', text: pick.text, cat: S.cats[0], src: { key: S.ch, quote: pick.text } } })
  unselect()
}

// 设定：AI 从这一章看出来要改的设定，钉在这一章头一回写到那个人（那样东西）的地方，正文上不画。
contribute('MarginSources', 'lore', (cur) => (aiOn() ? S.entries.filter((e) => e.place === 'open' && e.from === cur.key) : []).map((o) => {
  const e = S.entries.find((x) => x.id === o.about), name = e && e.names.find((n) => cur.paras.some((p) => p.includes(n)))
  return { quote: name || '', cls: '', id: '', note: { id: o.id, warn: true, kind: L.notesKindIdea(), title: e ? L.notesLoreChanged(e.names[0]) : L.loreNew(), text: o.text, acts: [[L.commonIgnore(), () => dropIdea(o)], [L.commonAccept(), () => takeChange(o)]] } }
}), 3)

contribute('PaneTabs', 'lore', { name: '设定', label: () => L.paneTabLore(), draw: setting, badge: () => aiOn() ? S.entries.filter((e) => e.place === 'open').length : 0, beside: true }, 1)

contribute('BesideItems', 'lore', () => setting(), 0)

contribute('Dialogs', 'lore', { kind: 'take', draw: takeDialog }, 0)

contribute('ExportContents', 'lore', '设定', 1)

contribute('SelectionMenus', 'lore', (p) => loreSelectionMenu(p), 3)

contribute('State', 'lore', ({ of }) => ({
  entries: of('entries', []),
  log: of('log', []),
  sv: 'list',
  sid: null,
  sq: '',
  form: null,
  named: null
}))

function loreSelectionMenu(p) {
  const name = p ? p.text.trim() : ''
    // 选中的是一个短名字，就可以直接记进设定里管名字的几类。
    const addName = (cat) => () => {
      set((s) => ({ ctx: null, pick: null, entries: [...s.entries, { id: 'e' + s.n, place: 'body', cat, names: [name], text: name }], n: s.n + 1, log: did(s, '新增到正文 · ' + cat + '：' + name) }))
      unselect()
      say(L.editorAddedToLore(name, cat), '', 'ok')
    }
    const names = !name || name.length > 12 ? menuRow(L.editorAddName(), null)
      : S.entries.some((e) => e.place !== 'dropped' && e.names.includes(name)) ? menuRow(L.editorAlreadyNamed(), null)
      : subRow(L.editorAddName(), S.ctx.names, () => set((s) => ({ ctx: { ...s.ctx, names: true } })), 'ctx-names', S.cats.filter((c) => BOOK.nameCats.includes(c)).map((c) => menuRow(c, addName(c))).join(''))
  return menuRow(L.editorAddToLore(), p ? take : null) + names
}

// 设定：在哪一页、看哪一条、搜什么、正在填的表

// 设定里改了名字，正文里还留着旧名：{ was, now, n }

// 摘取原文：从一句话里拖选一段，照原话记成设定。
function sourceDialog(d) {
  const read = () => {
    const sel = getSelection(), box = document.querySelector('.source-text')
    const picked = sel && !sel.isCollapsed && box.contains(sel.anchorNode) && box.contains(sel.focusNode) ? sel.toString().trim() : ''
    if (picked !== d.picked) set((s) => ({ dialog: { ...s.dialog, picked } }))
  }
  const add = () => {
    if (!d.picked) return
    set((s) => ({ dialog: null, ...SHUT, n: s.n + 1, entries: [...s.entries, { id: 'e' + s.n, place: 'body', cat: d.cat, names: [], text: d.picked }], log: did(s, L.dialogLogEntry(d.cat, d.picked)) }))
    say(L.dialogSettingAdded(), '', 'ok')
  }
  return dialog('source', dialogHead(L.dialogSourceTitle(), L.dialogSourceAbout()) +
    `<div class="source-text scroll"${onUp(read)}>${t(d.text)}</div>` +
    `<div class="source-line">${choose('source-cat', d.cat, S.cats, (v) => set((s) => ({ dialog: { ...s.dialog, cat: v } })), L.dialogSourceCat())}<div class="${d.picked ? 'ink' : 'faint'}">${t(d.picked ? L.dialogPicked(d.picked.length) : L.dialogNotPicked())}</div><div class="grow"></div>` +
    `<div class="hit link"${onClick(shut)}>${t(L.dialogSourceCollapse())}</div>${button(L.dialogAddSetting(), add, d.picked ? 'primary' : 'primary off')}</div>`)
}

contribute('Dialogs', 'lore', { kind: 'source', draw: sourceDialog }, 0)

// 一条设定在某一章时的样子：那一章和它前面最后一处变化的全文；一处也没有就是原来的。key 空着是最新的样子。
const changesOf = (e) => (e.changes || []).filter((g) => rank(g.key) >= 0).sort((a, b) => rank(a.key) - rank(b.key))
const asOf = (e, key) => {
  const at = key ? rank(key) : Infinity, live = changesOf(e).filter((g) => rank(g.key) <= at)
  return live.length ? live.at(-1) : null
}
const textAt = (e, key) => (asOf(e, key) || e).text
// 指一处变化从哪起：「第87章起」「第三卷：停电起」，那一章不在了写「所在章节已删除」。
const fromWhere = (key) => { const c = chapterOf(key), v = S.volumes.find((x) => x[0] === key); return c ? L.appFrom(numbered(c) ? '第' + c.no + '章' : titleOf(c)) : v ? L.appFrom(v[1]) : L.appChapterGone() }
