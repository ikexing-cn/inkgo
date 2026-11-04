// 扩展：talk。界面、动作和登记都在这里。
'use strict'

// ───────── 讨论 ─────────

function talkPage() {
  const talk = S.tv === 'one' ? S.talks.find((x) => x.id === S.tid) : null
  // 设定里已经有东西的书，接着往下聊，不再从一个想法开始。
  const begun = S.entries.length > 0
  const [opening, about] = begun ? [L.talkContinueTitle(), L.talkContinueAbout()] : [L.talkStartTitle(), L.talkStartAbout()]
  const starts = begun ? [L.talkNext1(), L.talkNext2(), L.talkNext3()] : [L.talkStart1(), L.talkStart2(), L.talkStart3()]
  return '<div class="editor">' +
    `<div class="toolbar"><div class="crumb">${t(L.talkAll())}</div>${icon('right', 'xs', 'faint')}<div class="crumb here ell">${t(talk ? talk.title : L.talkNew())}</div><div class="grow"></div>` +
    (talk ? `<div class="count-all">${t(L.talkCount(talk.msgs.length))}</div>` : '') +
    (S.paneOpen ? '' : `<button class="icon-btn muted" aria-label="${t(L.talkShowSettings())}" title="${t(L.talkShowSettingsDoc())}"${onClick(() => set({ paneOpen: true }))}>${svg('icon-md', PANEL + path('M15 5v14'))}</button>`) + '</div>' +
    (talk
      ? `<div class="talk big scroll" id="talk-page">${said(talk)}</div>`
      : '<div class="paper">' + blank('spark', opening, about,
        `<div class="starts">${starts.map((words) => `<div class="hit"${onClick(offer(words))}>${t(words)}</div>`).join('')}</div>`) + '</div>') +
    '<div class="composer big" data-k="composer-page"><div class="box send-box">' +
    `<textarea class="in" rows="2" placeholder="${t(L.talkInputHint())}" aria-label="${t(L.talkInputLabel())}"${onInput((v) => set({ draft: v }))}${onKey(sendKey)}>${t(S.draft)}</textarea>` +
    `<div class="send-line">${modelPick()}<div class="grow"></div><div class="faint">${keyLabel('Ctrl+Enter')}</div><div class="send hit${S.draft.trim() && !S.waiting ? '' : ' off'}" title="${t(L.talkSend(keyLabel('Ctrl+Enter')))}"${onClick(() => send())}>${ICON.send}</div></div>` +
    '</div></div></div>'
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
  const cards = fixed ? eventsOf(fixed).length : 0, owned = cards ? [[L.chatChapterCards(cards), cards * 45]] : []
  const own = { 起草: [[L.chatPartOutline(), 260], ...owned, [L.chatPartPrevOutlines(), 310], [L.chatPartPrevChapter(), 5060]], 改写: [[L.chatPartOutline(), 260], ...owned, [L.chatPartThisChapter(), 4300]] }[talk.origin] || []
  const parts = [[L.chatPartRules(), 1100], [S.crowded ? L.chatPartCatalog() : L.chatPartLoreCount(sent.length), S.crowded ? sent.length * 40 : sent.length * 260], ...own, ...(digested ? [[L.chatPartDigest(), 820]] : []), [L.chatPartLook(), (m.look || []).length * 640], [L.chatPartSaid(), 40]]
  parts.splice(parts.length - 1, 0, [L.chatPartBefore(), Math.max(Math.round(input * 0.97) - parts.reduce((n, [, x]) => n + x, 0), 0)])
  const head = S.crowded ? L.chatCrowded() : fixed ? L.chatAttachedAt(sent.length, chTitle(fixed)) : L.chatAttachedAll(sent.length)
  const named = (e) => (e.names.length ? e.names.join('、') + '：' : '')
  const changes = (e) => (fixed ? '' : changesOf(e).map((g) => `<div class="note">${t(fromWhere(g.key) + '：' + g.text)}</div>`).join(''))
  const rows = S.crowded ? sent.map((e) => `<div class="saw-row"><div class="fit">${t(label[e.place] + e.cat + (e.names[0] ? ' · ' + e.names[0] : '') + '：' + first(textAt(e, fixed)))}</div></div>`).join('')
    : sent.map((e) => `<div class="saw-row">${icon('tick', 'sm', 'green')}<div class="fit"><div>${t(label[e.place] + named(e) + (fixed ? textAt(e, fixed) : e.text) + (e.asked ? L.chatAnswered(e.asked) : ''))}</div>${changes(e)}</div></div>`).join('')
  return '<div class="reply-saw">' +
    `<div class="saw-title">${t(L.chatSentAbout(tokens(parts.reduce((n, [, x]) => n + x, 0))))}</div>${m.use.length > 1 ? `<div class="note">${t(L.chatRoundRequests(m.use.length))}</div>` : ''}<div class="note">${parts.filter(([, x]) => x).map(([name, x]) => t(name) + ' ' + tokens(x)).join(' · ')}</div>` +
    `<div class="note">${t(cached ? L.chatUsageCached(tokens(input), tokens(cached), tokens(output)) : L.chatUsage(tokens(input), tokens(output)))}</div>` +
    `<div class="saw-title">${t(head)}</div>${S.crowded ? '<div class="note">' + t(L.chatCatalogNote()) + '</div>' : ''}${ahead ? `<div class="note">${t(L.chatMoreChanges(ahead))}</div>` : ''}` +
    (sent.length ? rows : '<div class="note">' + t(L.chatLoreEmpty()) + '</div>') + '</div>'
}

// 回复里 AI 在正文里提的那一处：改哪一章、改哪里、多少字，作者定了没有。还没定的，点「在正文中查看」过去看。
function propCard(m) {
  const p = S.props.find((x) => x.id === m.prop), words = p ? p.paras.join('').length : m.words
  const state = S.fate[m.prop] ? FATE()[S.fate[m.prop]] || S.fate[m.prop] : !p ? L.chatFateGone() : p.writing ? L.chatWriting() : L.chatFateWaiting()
  return `<div class="prop-card"><div class="prop-card-head">${icon('pen', 'sm', p ? 'green' : 'muted')}<div class="fit ell">${t(L.chatProposeEdit(chTitle(m.key)))}</div>` +
    (p ? `<div class="hit link"${onClick(() => set((s) => go(s, m.key)))}>${t(L.chatViewInText())}</div>` : '') + `</div><div class="note">${t(whereWord(m.where))} · ${t(L.chatWords(words))} · ${t(state)}</div></div>`
}

// 讨论长了压缩过的地方：前面几轮整理成了一份摘要，原文还在上面。点开看摘要，只能看不能改。
function digest(talk) {
  const d = talk.digest, key = 'digest-' + talk.id, open = S.thoughts[key]
  return `<div class="digest"><div class="digest-head hit"${onClick(() => set((s) => ({ thoughts: { ...s.thoughts, [key]: !open } })))}>${t(L.chatDigestRounds(d.after / 2))}${icon(open ? 'down' : 'right', 'xs')}</div>` +
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
      rows(found.length ? [head(q ? L.commonFound(found.length) : L.chatRecent()), ...found.map(row)] : [empty(q ? L.commonNotFound() : L.chatEmpty())])
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

// 把选中的一段拿到右栏的讨论里。
function cite() {
  const pick = grab() || S.pick
  if (!pick) return
  const cur = chapterOf(S.ch)
  set({ ctx: null, pick: null, bar: null, cite: { key: cur.key, title: titleOf(cur), quote: pick.text }, paneOpen: true, focus: false, tab: '讨论', tv: S.tv === 'one' ? 'one' : 'new' })
  unselect()
}

// 一次回复查到了上限：查了 BOOK.limit 次还没查完，照查到的回答。
const capped = () => [{ me: true, at: stamp(), text: '全书写到井的地方，前后对得上吗' }, {
  me: false, at: stamp(), text: '已查到的几处里，井沿的刻痕前后一致：「第2章：井沿」写三道，「第85章：灯数」也数出三道。还有几处没来得及读，不能说全书都对得上。', capped: true, use: [[19100, 18420, 60], [27300, 19100, 70], [35800, 27300, 64], [44200, 35800, 66], [52900, 44200, 62], [61400, 52900, 71], [69800, 61400, 58], [78300, 69800, 66], [86900, 78300, 63], [96400, 86900, 520]],
  look: [['搜', '全书「井」', '9 处，在 5 章'], ...['1', '2', '46', '84', '85', '86', '87'].map((key) => ['读', '「' + chTitle(key) + '」全章', clip(chapterOf(key).paras.join(''), 40)]), ['设定', '「铜钥匙」', '一直挂在林晚脖子上，不知道开哪扇门。'], ['讨论', '「井」', '没有找到']].slice(0, BOOK.limit),
}]

contribute('PaneTabs', 'talk', { name: '讨论', label: () => L.paneTabChat(), draw: discussion, badge: () => 0, beside: false }, 2)

contribute('Pages', 'talk', { id: 'talk', label: () => L.pageTalk(), draw: talkPage, order: 0 }, 0)

contribute('SelectionMenus', 'talk', (p) => menuRow(L.editorQuote(), p ? cite : null, '', 'Ctrl+L'), 2)

// 讨论；cite 是从正文拿来的一段
// AI 正在回哪段讨论、到哪一步了：{ id, step }，step 是 pack 在压缩、think 在想、look 在查、reply 在写
// 哪几条回复的「思考过程」展开了
// 侧栏里收起来的几段讨论，和要不要把它们列出来
// 讨论页里哪条回复展开了「AI 参考内容」
// AI 在讨论里提议的修改，作者还没定的：{ id, key, talk, at, paras, writing }。at 是改哪里：{ kind: 'end' } 接在章末、{ kind: 'all' } 换掉全章、{ kind: 'swap', para, quote } 换掉一句
// 定了的提议：已采纳、已放弃、已被新的提议替换、正文已改动，提议已作废。讨论里那一条照它写
// 设定多到装不下，讨论只附目录
// 关闭了 AI：本机的所有作品都不显示 AI 功能，也不发送任何内容；offFrom 是关的时候停在的讨论页、检查页
contribute('State', 'talk', ({ of }) => ({
  talks: of('talks', []),
  tv: 'all',
  tid: null,
  tq: '',
  draft: '',
  attach: true,
  cite: null,
  waiting: null,
  thoughts: {},
  shelved: {},
  shelvedShown: false,
  told: {},
  crowded: false
}))
