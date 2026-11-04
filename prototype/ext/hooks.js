// 扩展：hooks。界面、动作和登记都在这里。
'use strict'

const mark = (id, change) => set((s) => ({ marks: s.marks.map((m) => (m.id === id ? { ...m, ...change(m) } : m)), renote: null }))

// 原句删掉了的伏笔，挂到新选中的句子上。
function rehang(id) {
  const pick = grab()
  if (!pick) { say(L.hooksSelectFirst()); return }
  const ch = chapterOf(S.ch)
  mark(id, () => ({ st: 'open', ch: ch.no, key: ch.key, para: pick.para, quote: pick.text }))
  unselect()
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
      `<div class="where"><div class="fit ell">${t(gap > 0 && !paying.pay ? L.hooksPlantedGap(chTitle(paying.key), gap) : L.hooksPlanted(chTitle(paying.key)))}</div></div><div class="quote">${t(paying.quote)}</div>` +
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
      where(st === 'lost' ? L.hooksLostIn(chTitle(shown.key)) : st === 'open' && gap > 0 ? L.hooksPlantedGap(chTitle(shown.key), gap) : L.hooksPlanted(chTitle(shown.key)), goTo(shown.key), L.appViewOriginal()) +
      `<div class="quote${st === 'lost' ? ' gone' : ''}">${t(shown.quote)}</div>` + HAIR +
      (st === 'open' ? '<div class="state">' + t(L.hooksUnpaid()) + '</div><div class="note">' + t(L.hooksSelectPayoff()) + '</div>' + buttons(button(L.hooksMarkPaid(), S.bar ? () => askPay(shown.id) : null, S.bar ? 'primary' : 'primary off')) : '') +
      (st === 'done' ? where(L.hooksPaidIn(chTitle(shown.pay.key), Math.max(shown.pay.ch - shown.ch, 0)), goTo(shown.pay.key), L.appViewOriginal()) + `<div class="quote">${t(shown.pay.quote)}</div>` : '') +
      (st === 'lost' ? '<div class="note">' + t(L.hooksSelectNew()) + '</div>' + buttons(button(L.hooksReplaceQuote(), S.bar ? () => rehang(shown.id) : null, S.bar ? 'primary' : 'primary off')) : '') +
      (st === 'dropped' ? '<div class="state">' + t(L.hooksDropped()) + '</div>' : '') +
      '</div>'
  }

  if (S.fv !== 'list') return back(L.hooksAll(), () => set({ fv: 'list' })) + rows([head(groups()[S.fv]), ...(of(S.fv).length ? of(S.fv).map((m) => row(m)) : [empty(L.hooksNoneHere())])])

  const q = S.fq.trim()
  let list = []
  if (q) {
    const found = S.marks.filter((m) => (m.note + m.quote).includes(q))
    list = found.length ? [head(L.commonFound(found.length)), ...found.map((m) => row(m, q))] : [empty(L.commonNotFound())]
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

// 「提醒」里伏笔那一段：埋下后过多少章还没回收算久，下面一句说本作品现在有几个。
function lateRows() {
  const lateNow = S.marks.filter((m) => m.st === 'open' && BOOK.now - m.ch >= S.late).length
  const step = (by) => () => set((s) => ({ late: Math.max(5, s.late + by) }))
  return `<div class="prefs-row"><div class="fit">${t(L.settingsLateAfter())}</div><div class="stepper"><div class="hit" title="${t(L.settingsLateLess())}"${onClick(step(-5))}>−</div><div class="stepper-n">${t(L.settingsLateChapters(S.late))}</div><div class="hit" title="${t(L.settingsLateMore())}"${onClick(step(5))}>+</div></div></div>` +
    `<div class="note">${t(L.settingsLateNow(lateNow))}</div>`
}

// 选回收哪个伏笔时列出来的那些，和眼下选中的那一个：没点过就是头一个。
// 能在这一章回收的：这一章和它前面埋下的。后面才埋的还没有，不列。
const payable = () => { const q = S.pay.q.trim(), here = chapterOf(S.ch).no; return S.marks.filter((m) => (m.st === 'open' || m.st === 'done') && m.ch <= here && (!q || m.note.includes(q) || m.quote.includes(q))) }
const payPick = () => { const can = payable(); return can.some((m) => m.id === S.pay.id) ? S.pay.id : can.length ? can[0].id : '' }

// 问「选择要回收的伏笔」。id 是先选好的那条，从右键菜单来的没有。
function askPay(id) {
  const pick = grab() || S.pick
  if (!pick) { say(L.editorSelectFirst()); return }
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

// 伏笔：埋下的那几个字，回收了的再加回收的那几个字。
contribute('MarginSources', 'hooks', (cur) => S.marks.flatMap((m) => {
  const open = () => set({ tab: '伏笔', fid: m.id, fv: 'list', renote: null, ...SHUT })
  const out = []
  if ((m.st === 'open' || m.st === 'done') && m.key === cur.key) out.push({ quote: m.quote, para: m.para, cls: 'fs', id: m.id, note: { id: m.id, kind: m.st === 'open' ? L.notesKindHook() : L.notesKindHookPaid(), title: nameOf(m), text: '', open } })
  if (m.st === 'done' && m.pay && m.pay.key === cur.key) out.push({ quote: m.pay.quote, para: m.pay.para, cls: 'fs', id: m.id, note: { id: m.id + '-pay', kind: L.notesKindPayoff(), title: nameOf(m), text: L.notesPlanted(chTitle(m.key)), open } })
  return out
}), 0)

contribute('SettingsSections', 'hooks', () => inBook() ? `<div class="ext-prefs">${lateRows()}</div>` : '', 0)

contribute('PaneTabs', 'hooks', { name: '伏笔', label: () => L.paneTabHooks(), draw: foreshadowing, badge: () => S.marks.filter((m) => m.st === 'lost' || (m.st === 'open' && BOOK.now - m.ch >= S.late)).length, beside: false }, 3)

contribute('PendingGroups', 'hooks', () => {
  const list = []
  for (const m of S.marks) {
    const gap = BOOK.now - m.ch
    const open = () => set({ paneOpen: true, focus: false, tab: '伏笔', fid: m.id, fv: 'list', renote: null, ...SHUT })
    if (m.st === 'lost') list.push({ id: m.id, group: '伏笔', name: L.statusQuoteGone(nameOf(m)), about: chTitle(m.key), key: m.key, open })
    if (m.st === 'open' && gap >= S.late) list.push({ id: m.id, group: '伏笔', name: L.statusUnpaid(nameOf(m), gap), about: L.statusPlantedIn(chTitle(m.key)), key: m.key, open })
  }
  return list
}, 1)

contribute('ToolHints', 'hooks', (cur, wide, here, tool, flip) => wide && here && !S.focus ? `<button class="fs-btn" aria-label="${L.tabMarksHere()}"${onClick(() => set({ paneOpen: true, tab: '伏笔', fid: null, fv: 'list' }))}>${icon('flag')}<span>${L.tabMarksCount(here)}</span></button>` : '', 0)

contribute('HookMenus', 'hooks', (p) => menuRow(L.editorMarkForeshadow(), p ? () => set({ ctx: null, bury: { note: '' } }) : null) + menuRow(L.editorMarkPay(), p ? () => askPay('') : null) +
          (S.ctx.mark ? menuRow(L.editorSeeMark(), () => set({ ctx: null, pick: null, tab: '伏笔', fid: S.ctx.mark, fv: 'list', renote: null, paneOpen: true, focus: false, ...SHUT })) : ''), 0)

// 伏笔：同上，renote 是正在改的备注
contribute('State', 'hooks', ({ of }) => ({
  marks: of('marks', []),
  fv: 'list',
  fid: null,
  fq: '',
  renote: null,
  bury: null,
  pay: null,
  late: BOOK.late
}))

function hookFloats(veiled, by) {
  if (S.bury) {
    return veiled + `<div class="float bury" data-k="bury" style="${by(360, 132, S.pick)}"><div class="float-head">${icon('flag', 'sm')}${L.editorBuryTitle()}</div>` +
      `<div class="bury-box">${typed(S.bury.note, (v) => set({ bury: { note: v } }), L.editorBuryHint(), L.editorBuryLabel(), keys(buryNow, () => set({ bury: null, pick: null })) + ' autofocus')}</div>` +
      `<div class="float-end">${button(L.commonCancel(), closeFloat)}${button(L.commonConfirm(), buryNow, 'primary')}</div></div>`
  }

  if (S.pay) {
    const q = S.pay.q.trim()
    const can = payable(), id = payPick()
    const here = chapterOf(S.ch).no // 「N 章前」从回收的这一章往前数
    const meta = (m) => (m.key === S.ch ? (m.pay ? L.editorMetaHerePaid() : L.editorMetaHere()) : m.pay ? L.editorMetaPaid(chTitle(m.key)) : L.editorMetaBefore(chTitle(m.key), Math.max(here - m.ch, 0)))
    return veiled + `<div class="float pay" data-k="pay" style="${by(380, 340, S.pick)}"><div class="float-head">${icon('link', 'sm')}${L.editorPayTitle()}</div>` +
      `<div class="pay-find">${typed(S.pay.q, (v) => set({ pay: { q: v, id } }), L.editorPayHint(), L.editorPayLabel(), keys(() => payNow(id), () => set({ pay: null, pick: null })) + ' autofocus')}</div>` +
      (can.length ? '' : `<div class="pay-none">${q ? L.editorNoMatch() : L.editorNoMarks()}</div>`) +
      `<div class="pay-list scroll">${can.map((m) => `<div class="pay-row row${m.id === id ? ' on' : ''}"${onClick(() => set((s) => ({ pay: { ...s.pay, id: m.id } })))}><div class="pay-name ell">${t(nameOf(m))}</div><div class="pay-meta ell">${t(meta(m))}</div></div>`).join('')}</div>` +
      `<div class="float-end">${button(L.commonCancel(), closeFloat)}<div class="btn hit primary${id ? '' : ' off'}" title="${L.editorPayTip()}"${onClick(() => payNow(id))}>${L.editorPayButton()}</div></div></div>`
  }

  return ''
}


contribute('SettingsPages', 'hooks', { name: '提醒', order: 20 }, 20)
