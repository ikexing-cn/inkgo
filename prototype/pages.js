// 正文以外的几页：讨论、大纲、检查、作品分类，和书架。
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

// ───────── 大纲 ─────────

// 大纲是一叠卡片，一件事一张。S.events 的先后就是故事里发生的先后，作者自己排；
// key 是写在哪一章、line 是哪条情节线：两样都有的落在「按章节」那张表里的一格，没有章的放在下面「未放进章节」。
const eventsOf = (key) => S.events.filter((e) => e.key === key)
// 一章在故事里是什么时候：它的卡片里第一个写了时间的到最后一个。
const whenOf = (evs) => { const w = [...new Set(evs.map((e) => e.when).filter(Boolean))]; return w.length > 1 ? w[0] + ' — ' + w.at(-1) : w[0] || '' }
// 一章里有比前面各章更早的事（回忆、倒叙）：它最早的那张，排在前面各章最晚的那张之前。
function goesBack(c) {
  const at = (e) => S.events.indexOf(e), before = S.chapters.slice(0, S.chapters.indexOf(c)).flatMap((x) => eventsOf(x.key)).map(at)
  const mine = eventsOf(c.key).map(at)
  return mine.length > 0 && before.length > 0 && Math.min(...mine) < Math.max(...before)
}
const gistOf = (c) => (S.outlines[c.key] || '').trim()
const lineOf = (id) => S.lines.find((l) => l.id === id)
const found = (e, q) => !q || [e.title, e.about, e.when, e.key ? chTitle(e.key) : ''].some((x) => x.includes(q))

// 新的一张排在时间线上哪里：给了 after 就排它后面；放进一章的，排在这一章和前面各章的卡片后面；都没有就排在最后。
function placeAt(events, chapters, key, after) {
  if (after) return events.findIndex((e) => e.id === after) + 1
  if (!key) return events.length
  const no = (k) => chapters.findIndex((c) => c.key === k), mine = no(key)
  return events.reduce((at, e, i) => (e.key && no(e.key) <= mine ? i + 1 : at), 0)
}
// 拖到另一格：换章、换情节线。从「未放进章节」拖出来的，在时间线上也排到那一章的位置。
function placeCard(id, key, line) {
  set((s) => {
    const e = s.events.find((x) => x.id === id)
    if (!e || (e.key === key && (line === null || e.line === line))) return null
    const moved = { ...e, key, line: line === null ? e.line : line }, rest = s.events.filter((x) => x !== e)
    const at = !e.key && key ? placeAt(rest, s.chapters, key, null) : s.events.indexOf(e)
    return { events: [...rest.slice(0, at), moved, ...rest.slice(at)] }
  })
}
// 一张卡片在弹窗里改：点一张卡片改它，点格子里的「＋」新记一张。弹窗里改的是一份草稿，保存了才算。
// 新记的一张带着它那一格的章、情节线、时间；after 是在时间线上排到哪一张后面。
const formOf = (e) => ({ title: e.title, when: e.when, about: e.about, key: e.key, line: e.line })
function openCard(id) {
  const e = S.events.find((x) => x.id === id)
  if (e) set({ dialog: { kind: 'card', id, form: formOf(e), n: 0 }, planOutline: null, planLine: null, ...SHUT })
}
const newCard = (key, line, after = null, when = '') => set({ dialog: { kind: 'card', id: null, form: { title: '', when, about: '', key, line }, after, n: 0 }, planOutline: null, planLine: null, ...SHUT })
// 存下弹窗里的这一张。more 是「继续添加」：存下以后接着记下一张，章、情节线、时间照旧，排在刚记的这张后面。
function saveCard(more) {
  set((s) => {
    const d = s.dialog, f = { ...d.form, title: d.form.title.trim(), when: d.form.when.trim() }
    if (d.id) {
      const e = s.events.find((x) => x.id === d.id), rest = s.events.filter((x) => x !== e)
      const at = !e.key && f.key ? placeAt(rest, s.chapters, f.key, null) : s.events.indexOf(e) // 刚放进一章的，在时间线上排到那一章的位置
      return { events: [...rest.slice(0, at), { ...e, ...f }, ...rest.slice(at)], dialog: null }
    }
    const id = 'ev' + s.n, at = placeAt(s.events, s.chapters, f.key, d.after && s.events.some((e) => e.id === d.after) ? d.after : null)
    return { events: [...s.events.slice(0, at), { id, ...f }, ...s.events.slice(at)], n: s.n + 1, planSee: id,
      dialog: more ? { ...d, form: { ...f, title: '', about: '' }, after: id, n: d.n + 1 } : null }
  })
  if (S.planSee) seeCard(S.planSee)
}
// 把一张卡片挪到看得见的地方，标出来一会儿。
function seeCard(id) {
  const card = document.querySelector(`[data-card="${id}"]`)
  if (card) card.scrollIntoView({ block: 'nearest', inline: 'nearest' })
  later(1600, () => set((s) => (s.planSee === id ? { planSee: null } : null)))
}
function cardDialog(d) {
  const f = d.form, ok = f.title.trim() !== ''
  const type = (k) => (v) => set((s) => ({ dialog: { ...s.dialog, form: { ...s.dialog.form, [k]: v } } }))
  const save = () => (ok ? saveCard(false) : null)
  const drop = () => { set((s) => ({ events: s.events.filter((x) => x.id !== d.id), dialog: null })); say(L.planCardDeleted(f.title), '', 'ok') }
  const chs = [L.planNoChapter(), ...S.chapters.map(titleOf)], lines = [L.planNoLine(), ...S.lines.map((l) => l.name)]
  const pickCh = (v) => type('key')(v === chs[0] ? '' : S.chapters.find((c) => titleOf(c) === v).key)
  const pickLine = (v) => type('line')(v === lines[0] ? '' : S.lines.find((l) => l.name === v).id)
  return dialog('small', dialogHead(d.id ? L.planCardEdit() : L.planCardNew(), d.id ? '' : d.n ? L.planCardAdded(S.events.find((e) => e.id === d.after).title) : L.planCardIntro()) +
    // 「继续添加」以后换一个新的输入框，光标回到它里面。
    `<div class="field" data-k="card-title-${d.n}"><div class="label">${t(L.planCardWhat())}</div>${line(f.title, type('title'), L.planCardWhatHint(), L.planCardEvent(), keys(save) + ' autofocus')}</div>` +
    `<div class="pair">${field(L.planCardChapter(), choose('card-ch', f.key ? titleOf(chapterOf(f.key)) : chs[0], chs, pickCh, L.planCardChapter()))}${field(L.planLine(), choose('card-line', lineOf(f.line) ? lineOf(f.line).name : lines[0], lines, pickLine, L.planLine()))}</div>` +
    field(L.planCardWhen(), line(f.when, type('when'), L.planCardWhenHint(), L.planCardWhenLabel(), keys(save))) +
    field(L.planCardAbout(), area(f.about, type('about'), '', L.planCardNotes(), 4)) +
    '<div class="buttons">' + (d.id ? `<div class="hit link danger"${onClick(drop)}>${t(L.planCardDelete())}</div><div class="grow"></div>` : '') + button(L.commonCancel(), shut) +
    (d.id ? '' : button(L.planCardAddMore(), ok ? () => saveCard(true) : null, ok ? '' : 'off')) + button(d.id ? L.cmdSave() : L.planAddCard(), ok ? save : null, ok ? 'primary' : 'primary off') + '</div>')
}
// 一张卡片：停上去浮起来一点，点开改。
const cardView = (e, q, also = '') => `<div class="card hit${e.key ? '' : ' later'}${S.planSee === e.id ? ' seen' : ''}${found(e, q) ? '' : ' dim'}" draggable="true" data-k="card-${e.id}" data-card="${e.id}" title="${t(L.planCardHint())}"${onClick(() => openCard(e.id))}>` +
  (e.when && also !== 'ch' ? `<div class="card-when ell">${marked(e.when, q)}${also === 'line' && lineOf(e.line) ? `<span class="faint"> · ${t(lineOf(e.line).name)}</span>` : ''}</div>` : '') +
  `<div class="card-title clamp${e.title ? '' : ' none'}">${e.title ? marked(e.title, q) : t(L.commonUntitled())}</div>` +
  (also === 'ch' ? `<div class="card-ch ell">${e.key ? marked(chTitle(e.key), q) : t(L.planNoChapter())}</div>` : '') + '</div>'

// 格子里的「添加」：停在格子上才显出来，和卡片一样宽，点了弹出新卡片。
const slotAdd = (label, act) => `<div class="slot-add hit" title="${t(L.planAddCardTitle())}" aria-label="${t(label)}"${onClick(act)}>${icon('plus', 'sm')}${t(L.planAddCard())}</div>`
// 一条情节线的名字：点名字改名，「…」里改名、挪位置、删除。按章节在表头，时间线在左边一列，两处一样。
function lineTitle(l, where) {
  if (S.planLine && S.planLine.id === l.id) {
    return `<div class="box line fit" data-k="lh-${where}-${l.id}-edit">${typed(S.planLine.name, (v) => set((s) => ({ planLine: { ...s.planLine, name: v } })), L.planLineNameLabel(), L.planLineNameLabel(), keys(saveLine, () => set({ planLine: null })) + onBlur(saveLine) + ' autofocus data-all')}</div>`
  }
  const ls = S.lines, n = S.events.filter((e) => e.line === l.id).length, menu = 'line-' + where + '-' + l.id
  return `<div class="fit ell hit head-name" data-k="lh-${where}-${l.id}" title="${t(L.planLineRenameHint())}"${onClick(() => set({ planLine: { id: l.id, name: l.name }, ...SHUT }))}>${t(l.name)}</div><div class="faint">${n}</div>` +
    `<div class="anchor"><button class="icon-btn muted" aria-label="${t(L.planLineActions(l.name))}" title="${t(L.commonMoreActions())}"${onClick(() => set({ open: S.open === menu ? '' : menu }))}>${ICON.dots}</button>` +
    (S.open === menu ? '<div class="pop line-pop">' + menuRow(L.planLineRename(), () => set({ planLine: { id: l.id, name: l.name }, ...SHUT })) + menuRow(where === 'grid' ? L.planLineMoveLeft() : L.commonMoveUp(), ls[0] === l ? null : () => shiftLine(l.id, -1)) + menuRow(where === 'grid' ? L.planLineMoveRight() : L.commonMoveDown(), ls.at(-1) === l ? null : () => shiftLine(l.id, 1)) + '<div class="hair"></div>' + menuRow(L.planLineDelete(), () => dropLine(l.id)) + '</div>' : '') + '</div>'
}
const addLineButton = () => `<div class="plan-add hit" title="${t(L.planLineAddHint())}"${onClick(() => addLine())}>${icon('plus', 'sm')}${t(L.planLineAdd())}</div>`

// 按章节：一章一行，一条情节线一列，卡片落在交叉的格子里；哪条线断了几章，一眼看得出。
// 左边的章节一列和上面的表头滚动时不动，「新增情节线」总在右上角。
function planGrid(q) {
  const ls = S.lines
  const outlineBox = (key, hint, rows) => `<div class="box area" data-k="outline-${key}"><textarea class="in" rows="${rows}" placeholder="${t(hint)}" aria-label="${t(chapterOf(key) ? L.planChapterOutline() : L.planVolumeOutline())}" autofocus` +
    `${onInput((v) => set((s) => ({ outlines: { ...s.outlines, [key]: v } })))}${onBlur(() => set({ planOutline: null }))}${onKey((ev) => { if (ev.key === 'Escape') { ev.stopPropagation(); set({ planOutline: null }) } })}>${t(S.outlines[key] || '')}</textarea></div>`
  const gist = (key, text, none, cls) => (S.planOutline === key ? outlineBox(key, none, cls === 'vol-gist' ? 3 : 4)
    : `<div class="${cls} hit${text ? '' : ' none'}" title="${t(L.planClickEdit())}"${onClick(() => set({ planOutline: key, planLine: null }))}>${text ? marked(text, q) : t(none)}</div>`)
  const cell = (c, l) => `<div class="board-cell" data-cell="${c.key}|${l.id}">${S.events.filter((e) => e.key === c.key && e.line === l.id).map((e) => cardView(e, q)).join('')}` +
    slotAdd(L.planSlotInChapter(titleOf(c), l.name), () => newCard(c.key, l.id)) + '</div>'
  const chRow = (c) => {
    const when = whenOf(eventsOf(c.key)), stray = S.events.filter((e) => e.key === c.key && !lineOf(e.line))
    return `<div class="board-side grid-ch" data-k="row-${c.key}"><div class="ch-name"><div class="hit ell" title="${t(L.planOpenBody())}"${onClick(goTo(c.key))}>${marked(titleOf(c), q)}</div>${goesBack(c) ? `<div class="tag" title="${t(L.planBacktrackHint())}">${t(L.planBacktrack())}</div>` : ''}</div>` +
      (when ? `<div class="note ell">${t(L.planWhenNote(when))}</div>` : '') + gist(c.key, gistOf(c), L.planWriteChapterOutline(), 'ch-gist clamp') +
      (stray.length ? `<div class="faint">${t(L.planStray(stray.length))}</div>` : '') + '</div>' + ls.map((l) => cell(c, l)).join('') + '<div class="board-fill"></div>'
  }
  const hits = (c) => !q || [titleOf(c), gistOf(c)].some((x) => x.includes(q)) || eventsOf(c.key).some((e) => found(e, q))
  const block = (key, title, inside) => {
    const shown = inside.filter(hits)
    if (q && !shown.length && ![title, S.outlines[key] || ''].some((x) => x.includes(q))) return '' // 卷名、卷纲搜到了，这一卷的标题留着
    const when = whenOf(inside.flatMap((c) => eventsOf(c.key)))
    return `<div class="grid-vol" data-k="vol-${key || 'loose'}"><div class="vol-in"><div class="vol-name"><div class="group-title">${marked(title, q)}</div><div class="note">${t(L.planChapterCount(inside.length))}${when ? ' · ' + t(when) : ''}</div></div>` +
      (key ? gist(key, (S.outlines[key] || '').trim(), L.planWriteVolumeOutline(), 'vol-gist') : '') + '</div></div>' + shown.map(chRow).join('')
  }
  const loneCh = S.chapters.filter((c) => !c.vol)
  const body = S.volumes.map(([key, title]) => block(key, title, S.chapters.filter((c) => c.vol === key))).join('') + (loneCh.length ? block('', L.planNoVolume(), loneCh) : '')
  return `<div class="board grid scroll" data-k="grid"><div class="board-table" style="grid-template-columns: 248px repeat(${ls.length}, 200px) minmax(160px, 1fr)">` +
    `<div class="board-head board-corner">${t(L.planChapters())}</div>` + ls.map((l) => `<div class="board-head">${lineTitle(l, 'grid')}</div>`).join('') + `<div class="board-head board-end">${addLineButton()}</div>` +
    // 章节里一处也没搜到，搜到的只有下面托盘里的卡片。
    (body || `<div class="grid-none">${empty(L.planNoneInChapters(q), L.planFoundCardsBelow(L.planNoChapter()))}</div>`) + '</div></div>' + tray(q)
}
const saveLine = () => set((s) => (s.planLine ? { lines: s.lines.map((l) => (l.id === s.planLine.id ? { ...l, name: s.planLine.name.trim() || l.name } : l)), planLine: null } : null))
// 新的一条线放在最后，名字先选中，打字就换掉。
const addLine = () => set((s) => { const id = 'l' + s.n, name = L.planLineNew(); return { lines: [...s.lines, { id, name }], n: s.n + 1, planLine: { id, name }, planOutline: null, ...SHUT } })
const shiftLine = (id, d) => set((s) => { const i = s.lines.findIndex((l) => l.id === id), list = [...s.lines];[list[i], list[i + d]] = [list[i + d], list[i]]; return { lines: list, ...SHUT } })
// 删掉一条线：它的卡片不删，放到「未放进章节」里，写在哪一章的在卡片上记着。
function dropLine(id) {
  const l = lineOf(id), n = S.events.filter((e) => e.line === id).length
  set((s) => ({ lines: s.lines.filter((x) => x.id !== id), events: s.events.map((e) => (e.line === id ? { ...e, line: '' } : e)), ...SHUT }))
  say(L.planLineDeleted(l.name), n ? L.planLineDeletedCards(n) : '', 'ok')
}

// 下面一条：想到了、还没定写在哪的卡片。拖进上面的格子，就放进那一章。
function tray(q) {
  const cards = S.events.filter((e) => !e.key), all = cards.length
  return `<div class="tray" data-k="tray" data-cell="|"><div class="tray-head"><div class="group-title">${t(L.planNoChapter())}</div><div class="note">${t(all ? L.planTrayCount(all) : L.planTrayEmpty())}</div><div class="grow"></div>` +
    `<div class="hit link"${onClick(() => newCard('', ''))}>${t(L.planTrayAdd())}</div></div>` +
    (cards.length ? `<div class="tray-cards">${cards.map((e) => cardView(e, q, 'line')).join('')}</div>` : '') + '</div>'
}

// 时间线上的一列：挨着的、故事时间写得一样的几张并成一列；没写时间的自成一列。
function timeCols(events) {
  const cols = []
  for (const e of events) { const last = cols.at(-1); if (last && e.when && last.when === e.when) last.cards.push(e); else cols.push({ when: e.when, cards: [e] }) }
  return cols
}
// 拖到时间线上的一格：排进那一列的末尾，时间跟着那一列，换到那一行的情节线。col 是 end 的，排到最后。
function dropAt(id, col, line) {
  set((s) => {
    const e = s.events.find((x) => x.id === id), rest = s.events.filter((x) => x !== e)
    const to = col === 'end' ? null : timeCols(s.events)[+col]
    const stay = to && to.cards.length === 1 && to.cards[0] === e
    const last = to && !stay ? to.cards.filter((x) => x !== e).at(-1) : null
    const at = stay ? s.events.indexOf(e) : last ? rest.indexOf(last) + 1 : rest.length
    const moved = { ...e, line, when: to ? to.when : e.when }
    return { events: [...rest.slice(0, at), moved, ...rest.slice(at)] }
  })
}

// 时间线：同一叠卡片按故事里发生的先后从左往右排开，一条情节线一行，和「按章节」的列对得上。
// 实线框的已经放进章节，虚线框的还没有。点一列的时间，整列一起改；左边一列和上面的时间滚动时不动。
function planTimeline(q) {
  const cols = timeCols(S.events)
  const lanes = [...S.lines, ...(S.events.some((e) => !lineOf(e.line)) ? [{ id: '', name: L.planNoLine() }] : [])]
  const inLane = (e, l) => (l.id ? e.line === l.id : !lineOf(e.line))
  const add = (l, when, after) => slotAdd(L.planSlotAtTime(when || L.planLastSlot(), l.name), () => newCard('', l.id, after, when))
  const timePop = (c) => '<div class="pop time-pop" data-k="time-pop">' +
    `<div class="pop-label">${t(L.planTimeColumn(c.cards.length))}</div>` +
    `<div class="box line">${typed(S.planTime.when, (v) => set((s) => ({ planTime: { ...s.planTime, when: v } })), L.planTimeHint(), L.planTimeAria(), keys(saveTime) + ' autofocus data-all')}</div>` +
    `<div class="pop-buttons">${button(L.commonCancel(), () => set(SHUT))}${button(L.cmdSave(), saveTime, 'primary')}</div></div>`
  const time = (c, i) => `<div class="board-head tl-time anchor left" data-k="time-${i}">` +
    `<div class="tl-when hit ell${c.when ? '' : ' none'}" title="${t(L.planClickEditTime())}"${onClick(() => set({ open: 'time', planTime: { i, when: c.when } }))}>${c.when ? marked(c.when, q) : t(L.planNoTime())}</div>` +
    `<div class="tl-axis"><i class="ev-dot${c.cards.some((e) => e.key) ? '' : ' later'}"></i></div>${S.open === 'time' && S.planTime.i === i ? timePop(c) : ''}</div>`
  const lane = (l) => `<div class="board-side tl-lane">${l.id ? lineTitle(l, 'tl') : `<div class="fit ell head-name">${t(l.name)}</div>`}</div>` +
    cols.map((c, i) => `<div class="board-cell" data-slot="${i}|${l.id}">${c.cards.filter((e) => inLane(e, l)).map((e) => cardView(e, q, 'ch')).join('')}${add(l, c.when, c.cards.at(-1).id)}</div>`).join('') +
    `<div class="board-cell" data-slot="end|${l.id}">${add(l, '', S.events.length ? S.events.at(-1).id : null)}</div>`
  const off = S.chapters.filter((c) => !eventsOf(c.key).length)
  return `<div class="tl-note"><span><i class="tl-key"></i>${t(L.planInChapter())}</span><span><i class="tl-key later"></i>${t(L.planNoChapter())}</span><span class="faint">${t(L.planTimelineHint())}</span><span class="grow"></span>` +
    (off.length ? `<span>${t(off.length === 1 ? L.planOneNoCards(titleOf(off[0])) : L.planChaptersNoCards(off.length))}</span>` : '') + '</div>' +
    `<div class="board tl scroll" data-k="tl"><div class="board-table" style="grid-template-columns: 168px repeat(${cols.length}, 200px) 200px">` +
    `<div class="board-head tl-time board-corner"><div class="tl-when none">${t(L.planLine())}</div><div class="tl-axis"></div></div>` + cols.map(time).join('') +
    `<div class="board-head tl-time"><div class="tl-when none">${t(L.planTimeEnd())}</div><div class="tl-axis end"></div></div>` +
    lanes.map(lane).join('') + `<div class="board-side tl-lane">${addLineButton()}</div><div class="board-fill" style="grid-column: 2 / -1"></div></div></div>`
}
// 改一列的时间：这一列的几张一起改。改成和旁边一列一样的，就并到一起。
const saveTime = () => set((s) => {
  if (!s.planTime) return null
  const col = timeCols(s.events)[s.planTime.i], when = s.planTime.when.trim()
  return { events: s.events.map((e) => (col.cards.includes(e) ? { ...e, when } : e)), planTime: null, ...SHUT }
})

// 大纲文件在软件外被改坏了：整页一句话，正文和设定照常。不用「重试」：软件每两秒重读一次，改好了自己回来。
function planBroken() {
  return `<div class="editor"><div class="toolbar"><div class="crumb here">${t(L.pagePlan())}</div></div><div class="plan-none">` +
    blank('warn', L.planBrokenTitle(), L.planBrokenAbout(),
      `<div class="note faint">${t(L.planBrokenLine(12))}</div>` + buttons(button(L.commonOpenFolder(), osAsk(L.shelfShowFolder())))) + '</div></div>'
}

function planPage() {
  if (S.planBroken) return planBroken()
  const timeline = S.plan === '时间线', q = S.planQ.trim()
  const loose = S.events.filter((e) => !e.key).length
  // 搜着的时候，上面那一句换成找到了多少；一个也没找到，整页换成一句话和「清除搜索」，表头和下面的托盘都不留。
  const cards = q ? S.events.filter((e) => found(e, q)).length : 0
  const chs = q && !timeline ? S.chapters.filter((c) => [titleOf(c), gistOf(c)].some((x) => x.includes(q))).length + S.volumes.filter(([key, title]) => [title, S.outlines[key] || ''].some((x) => x.includes(q))).length : 0
  const sum = !q ? (loose ? L.planSummaryLoose(S.chapters.length, S.lines.length, S.events.length, loose) : L.planSummary(S.chapters.length, S.lines.length, S.events.length))
    : cards || chs ? (chs && cards ? L.planFoundBoth(chs, cards) : chs ? L.planFoundChapters(chs) : L.planFoundCards(cards)) : L.planNoResults()
  const clear = () => set({ planQ: '' })
  const none = q && !cards && !chs
  return `<div class="editor"><div class="toolbar"><div class="crumb here">${t(L.pagePlan())}</div><div class="note plan-sum">` + t(sum) + '</div><div class="grow"></div>' +
    `<div class="box line plan-find">${ICON.find}${typed(S.planQ, (v) => set({ planQ: v }), timeline ? L.planSearchCards() : L.planSearchCardsAndOutlines(), L.planSearch(), keys(() => {}, clear))}` +
    (q ? `<button class="icon-btn muted" aria-label="${t(L.planClearSearch())}" title="${t(L.planClearSearchHint())}"${onClick(clear)}>${icon('cross', 'sm')}</button>` : '') + '</div>' +
    seg(S.plan, [['按章节', '', L.planByChapter()], ['时间线', '', L.planTimelineView()]], (v) => set({ plan: v, planSee: null, planOutline: null, planLine: null, ...SHUT }), L.planViewAria()) + '</div>' +
    (none ? '<div class="plan-none">' + blank('find', L.planNotFound(q), timeline ? L.planSearchTimeline() : L.planSearchChapters(), buttons(button(L.planClearSearch(), clear))) + '</div>'
      : timeline ? planTimeline(q) : planGrid(q)) + '</div>'
}

// ───────── 检查 ─────────

const KINDS = ['全部', '不一致', '名字有误', '新出现']
// 检查的分类：状态里记的是上面那几个字，显示时换成当前语言的字。
const kindWord = (k) => ({ 全部: L.checkKindAll(), 不一致: L.checkKindMismatch(), 名字有误: L.checkKindName(), 新出现: L.checkKindNew() })[k] ?? k

// 一条处理完了：从列表里拿掉，接着看下一条。
const settle = (id) => (s) => {
  const of = s.checks.filter((c) => s.kind === '全部' || c.kind === s.kind), at = of.findIndex((c) => c.id === id)
  const next = of[at + 1] || of[at - 1]
  return { checks: s.checks.filter((c) => c.id !== id), cid: next ? next.id : null }
}
// 一条检查结果能怎么处理，最后一样是主要的：检查页的按钮和页边批注上的字是同一份。不改的点错了，在提示条上撤销。
function checkActs(c) {
  const ch = chapterOf(c.key), e = S.entries.find((x) => x.id === c.entry)
  const leave = (word) => () => { const was = { checks: S.checks, cid: S.cid }; set(settle(c.id)); const id = say(word, '', 'ok', [[L.cmdUndo(), () => { unsay(id); set(was) }]]) }
  const add = () => { set((s) => ({ ...settle(c.id)(s), entries: [...s.entries, { id: 'e' + s.n, place: 'body', cat: c.cat, names: [c.title], text: c.as, src: { key: c.key, quote: c.quote } }], n: s.n + 1, log: did(s, L.checkLogAdded(c.cat, c.as)) })); say(L.checkAddedSetting(), '', 'ok') }
  if (c.kind === '新出现') return [[L.checkIgnore(), leave(L.checkIgnored())], [L.checkAddToSettings(), add]]
  return [[L.checkNoChange(), leave(L.checkMarkedNoChange())], ...(c.kind === '不一致' && e ? [[L.checkEditSetting(), () => set({ page: 'write', paneOpen: true, focus: false, tab: '设定', sv: 'list', sid: e.id, form: editing(e) })]] : []),
    [L.checkEditText(), ch ? goTo(ch.key, { key: ch.key, quote: c.quote }) : null]]
}

function checkPage() {
  const of = S.checks.filter((c) => S.kind === '全部' || c.kind === S.kind)
  const shown = of.find((c) => c.id === S.cid) || of[0]
  const count = (k) => (k === '全部' ? S.checks.length : S.checks.filter((c) => c.kind === k).length)
  const about = (c) => {
    const ch = chapterOf(c.key), at = chTitle(c.key)
    if (c.kind === '不一致') { const e = S.entries.find((x) => x.id === c.entry); return L.checkMismatchAbout(at, e ? e.cat + ' · ' + (e.names[0] || clip(e.text, 8)) : L.checkDeleted()) }
    return c.kind === '名字有误' ? L.checkNameAbout(at, c.right) : c.also ? L.checkFirstSeenMore(c.cat, at, c.also.length + 1) : L.checkFirstSeen(c.cat, at)
  }
  const row = (c) => `<div class="check-row row${shown && c.id === shown.id ? ' on' : ''}"${onClick(() => set({ cid: c.id }))}><div class="fit"><div class="ell">${t(c.title)}</div><div class="note ell">${t(about(c))}</div></div><div class="side">${t(kindWord(c.kind))}</div></div>`
  // 「检查全书」：先数出该查几章，问一句再开始。
  const owed = due()
  const again = () => {
    if (!S.entries.some((e) => e.place === 'body')) say(L.checkNoSettings())
    else if (!owed.length) say(L.checkAllDone())
    else set({ open: S.open === 'checkAll' ? '' : 'checkAll' })
  }
  const ask = () => `<div class="pop ask-pop" data-k="ask-pop"><div class="pop-title">${t(L.checkOwedTitle(owed.length))}</div><div class="note">${t(L.checkOwedNote(owed.length))}</div>` +
    `<div class="pop-buttons">${button(L.commonCancel(), () => set(SHUT))}${button(L.checkStart(), () => { set(SHUT); checkNow(...owed.map((c) => c.key)) }, 'primary')}</div></div>`
  const last = S.chapters.filter((c) => !S.never[c.key] && sizeOf(c) >= 200).at(-1)
  const running = chapterOf(S.checking), lost = S.chapters.find((c) => S.failed[c.key])
  const none = !S.entries.some((e) => e.place === 'body') ? [L.checkNoSettingsTitle(), L.checkNoSettingsAbout()]
    : S.checks.length ? [L.checkNoneInKind(), ''] : !last ? [L.checkNotYet(), L.checkNotYetAbout(L.checkAllBook())]
    : [L.checkNoneFound(), S.when === '手动检查' ? L.checkManualNote(L.checkModeManual(), L.checkRecheck()) : L.checkAutoNote()]
  const unfinished = lost && !S.checking // 失败的那章不当作没查出问题，上面那一行也不说「已检查至」，像是那以前都查过了
  if (unfinished) none.splice(0, 2, L.checkIncomplete(), L.checkFailedAt(titleOf(lost), S.failed[lost.key]))

  let detail = `<div class="check-detail">${blank(unfinished ? 'warn' : 'checked', S.checking ? L.checkRunningTitle() : none[0], S.checking ? L.checkRunningAbout() : none[1])}</div>`
  if (shown) {
    const ch = chapterOf(shown.key), e = S.entries.find((x) => x.id === shown.entry)
    const text = ch ? ch.paras[shown.para] || '' : ''
    const lit = (words, part) => words.split(part).map(t).join(`<span class="lit">${t(part)}</span>`)
    const seeText = ch ? goTo(ch.key, { key: ch.key, quote: shown.quote }) : null
    const seeEntry = () => set({ page: 'write', paneOpen: true, focus: false, tab: '设定', sv: 'list', sid: e.id, form: null })
    const next = () => set({ cid: (of[of.indexOf(shown) + 1] || of[0]).id })
    const right = shown.kind === '不一致' && e ? `<div class="fit"><div class="check-from"><div class="fit ell">${t(L.checkFromSetting(e.cat, e.names[0] || ''))}</div><div class="hit link"${onClick(seeEntry)}>${t(L.checkSeeSetting())}</div></div><div class="check-card">${lit(e.text, shown.says)}</div></div>`
      : shown.kind === '名字有误' ? `<div class="fit"><div class="check-from"><div class="fit ell">${t(L.checkNameInSettings())}</div></div><div class="check-card"><span class="lit">${t(shown.right)}</span></div></div>`
      : `<div class="fit"><div class="check-from"><div class="fit ell">${t(L.checkCanAdd(shown.cat))}</div></div><div class="check-card"><span class="lit">${t(shown.title)}</span>${t(L.checkColon())}${t(shown.as)}</div></div>`
    const acts = checkActs(shown).map(([label, act], i, all) => button(label, act, i === all.length - 1 ? 'primary' : '')).join('')
    detail = '<div class="check-detail" data-k="check-detail">' +
      `<div class="check-title"><div class="fit ell">${t(shown.title)}</div><div class="side">${of.indexOf(shown) + 1} / ${of.length}</div><div class="hit link${of.length > 1 ? '' : ' off'}"${onClick(of.length > 1 ? next : null)}>${t(L.checkNext())}</div></div>` +
      `<div class="check-sides"><div class="fit"><div class="check-from"><div class="fit ell">${t(ch ? L.checkTextPara(titleOf(ch), shown.para + 1) : L.checkTextParaNoChapter(shown.para + 1))}</div><div class="hit link"${onClick(seeText)}>${t(L.checkSeeText())}</div></div><div class="check-card">${lit(text, shown.quote)}</div></div>${right}</div>` +
      (shown.also ? `<div class="check-also">${t(shown.kind === '新出现' ? L.checkAlsoNew() : L.checkAlsoSimilar())}</div>` + shown.also.map((a) => `<div class="check-more"><div class="side">${t(a.at)}</div><div class="fit">${t(a.text)}</div><div class="side">${t(a.note)}</div></div>`).join('') : '') +
      `<div class="grow"></div><div class="check-foot"><div class="hit link${of.length > 1 ? '' : ' off'}"${onClick(of.length > 1 ? next : null)}>${t(L.checkLater())}</div><div class="grow"></div>${acts}</div></div>`
  }
  return '<div class="check-list">' +
    // 不另写「检查」二字：标题栏正中已经说了在哪一页。
    '<div class="check-head">' +
    (running ? `<div class="muted">${t(S.queue.length ? L.checkRunningMore(titleOf(running), S.queue.length) : L.checkRunning(titleOf(running)))}</div><div class="grow"></div><div class="hit link"${onClick(() => set({ checking: false, queue: [] }))}>${t(L.checkStop())}</div>`
      : `<div class="muted">${t(unfinished ? L.checkProgress(S.chapters.filter((c) => !S.never[c.key] && sizeOf(c) >= 200 && !S.failed[c.key]).length, S.chapters.filter((c) => S.failed[c.key]).length) : last ? L.checkCheckedUpTo(titleOf(last)) : '')}</div><div class="grow"></div><div class="anchor"><div class="hit link"${onClick(again)}>${t(last ? L.checkRecheck() : L.checkAllBook())}</div>${S.open === 'checkAll' ? ask() : ''}</div>`) + '</div>' +
    seg(S.kind + ' ' + count(S.kind), KINDS.map((k) => [k + ' ' + count(k), '', kindWord(k) + ' ' + count(k)]), (v) => set({ kind: v.split(' ')[0], cid: null }), L.checkKindAria()) +
    `<div class="check-rows scroll">${of.map(row).join('')}</div></div>` + detail
}

// ───────── 作品分类 ─────────

function saveCat() {
  const name = S.catForm.name.trim(), land = S.catForm.land.trim()
  if (!name) { say(L.catsNameEmpty()); return }
  if (!land) { say(L.catsLandEmpty()); return }
  set((s) => ({ cats: s.cats.includes(name) ? s.cats : [...s.cats.filter((c) => c !== '其他'), name, '其他'], lands: { ...s.lands, [name]: land }, catForm: { name: '', land: '' } }))
  say(L.catsSaved(name), '', 'ok')
}
function dropCat(name) {
  set((s) => ({ cats: s.cats.filter((c) => c !== name), entries: s.entries.map((e) => (e.cat === name ? { ...e, cat: '其他' } : e)), catDrop: '' }))
  say(L.catsDeleted(name), '', 'ok')
}

// 删除分类：有设定的先问一句（设定要移到「其他」），空的直接删。
function catsPage() {
  const count = (c) => S.entries.filter((e) => e.cat === c && e.place !== 'dropped').length
  const type = (key) => (v) => set((s) => ({ catForm: { ...s.catForm, [key]: v } }))
  // 同名的是改那一条，不是加：按保存以前就说是哪条、现在落在哪，也能「取消」不改了。
  const name = S.catForm.name.trim()
  const what = !name ? '' : S.cats.includes(name) ? L.catsWillEdit(name, S.lands[name] || L.catsLandDefault()) : L.catsWillAdd()
  const row = (c) => (S.catDrop === c
    ? `<div class="cat-row"><div class="fit">${t(L.catsMoveTo(count(c), '其他'))}</div>${button(L.commonCancel(), () => set({ catDrop: '' }))}${button(L.catsConfirmDelete(), () => dropCat(c), 'danger')}</div>`
    : `<div class="cat-row hover-acts"><div class="cat-name">${t(c)}</div><div class="fit muted">${t(S.lands[c] || L.catsLandDefault())}</div><div class="side">${t(L.catsCount(count(c)))}</div><div class="hit link"${onClick(() => { set({ catForm: { name: c, land: S.lands[c] || L.catsLandDefault() } }); document.querySelector('.cat-form').scrollIntoView({ block: 'nearest' }) })}>${t(L.catsEdit())}</div>` +
      (c === '其他' ? `<div class="off">${t(L.commonDelete())}</div>` : `<div class="hit link"${onClick(() => (count(c) ? set({ catDrop: c }) : dropCat(c)))}>${t(L.commonDelete())}</div>`) + '</div>')
  return '<div class="editor"><div class="paper scroll"><div class="column narrow">' +
    `<div class="page-title">${t(L.cmdCategories())}</div><div class="note">${t(L.catsNote())}</div>` +
    `<div class="cat-rows"><div class="cat-row cols"><div class="cat-name">${t(L.catsName())}</div><div class="fit">${t(L.catsDest())}</div></div>${S.cats.map(row).join('')}</div>` +
    `<div class="group-title">${t(L.catsFormTitle())}</div>` +
    `<div class="cat-form">${field(L.catsName(), line(S.catForm.name, type('name'), '', L.catsNameLabel()))}${field(L.catsDest(), line(S.catForm.land, type('land'), L.catsLandHint(L.catsLandDefault()), L.catsDest(), keys(saveCat)))}${S.cats.includes(name) ? button(L.commonCancel(), () => set({ catForm: { name: '', land: '' } })) : ''}${S.catForm.name.trim() && S.catForm.land.trim() ? button(L.catsSave(), saveCat, 'primary') : button(L.catsSave(), null, 'primary off')}</div>` +
    (what ? `<div class="note">${t(what)}</div>` : '') +
    '</div></div></div>'
}

// ───────── 书架 ─────────

// 原型开不了系统的窗口，只说一声本来会开什么。
const osAsk = (what) => () => say(L.shelfOSTitle(), L.shelfOSStep(what))

// 添加已有作品：原型里当作每次选的都是同一个文件夹。头一回加上，再点就说已经在了。
function addBook() {
  if (S.shelf.some((b) => b.id === 'found')) { say(L.shelfAlready()); return }
  set((s) => ({ shelf: [...s.shelf, { id: 'found', name: BOOK.found, kind: '出版小说', words: 21000, at: null }] }))
  say(L.shelfAdded(), BOOK.found, 'ok')
}

// 换一本书：外观、书架、设置里的东西带过去，别的照那本书刚打开的样子。
function openBook(id) {
  stopAll()
  set({ ...fresh(id), ...Object.fromEntries(KEPT.map((k) => [k, S[k]])) })
}

function shelfPage() {
  const all = S.shelf
  const open = (b) => onClick(b.gone ? null : () => openBook(b.id))
  const about = (b) => genreWord(b.kind) + ' · ' + wordsShown(b.words)
  const more = (b) => `<div class="anchor"><button class="icon-btn muted" aria-label="${t(L.commonMoreActions())}" title="${t(L.commonMoreActions())}"${onClick(() => set({ open: S.open === 'book-' + b.id ? '' : 'book-' + b.id }))}>${ICON.dots}</button>` +
    (S.open === 'book-' + b.id ? '<div class="pop shelf-pop">' + menuRow(L.commonOpenFolder(), () => { set(SHUT); osAsk(L.shelfShowFolder())() }) + menuRow(L.shelfRenameBook(), () => set({ dialog: { kind: 'nameBook', id: b.id, name: b.name }, ...SHUT })) + menuRow(L.shelfRemove(), () => removeBook(b)) + '</div>' : '') + '</div>'
  const removeBook = (b) => { set((s) => ({ shelf: s.shelf.filter((x) => x.id !== b.id), ...SHUT })); say(L.shelfRemoved(b.name), '', 'ok') }
  const card = (b) => `<div class="shelf-card${b.gone ? ' gone' : ' hit'}"${open(b)}><div class="shelf-card-top"><div class="shelf-logo">${t(b.name[0])}</div><div class="grow"></div>${b.gone ? '' : more(b)}</div>` +
    `<div class="shelf-name ell">${t(b.name)}</div><div class="note">${t(about(b))}</div><div class="grow"></div>` +
    (b.gone ? `<div class="shelf-gone"><div class="fit danger">${t(L.shelfFolderMissing())}</div><div class="hit link"${onClick(osAsk(L.shelfPickBookFolder()))}>${t(L.shelfLocate())}</div><div class="hit link"${onClick(() => removeBook(b))}>${t(L.shelfRemoveShort())}</div></div>` : `<div class="faint">${t(b.at === null ? L.shelfNotOpened() : timeShown(b.at))}</div>`) + '</div>'
  const row = (b) => `<div class="shelf-row${b.gone ? ' gone' : ' hit'}"${open(b)}><div class="shelf-logo">${t(b.name[0])}</div><div class="fit"><div class="shelf-row-name"><span class="shelf-name">${t(b.name)}</span><span class="note">${t(about(b))}</span></div><div class="faint ell">${t(S.library + sep() +b.name)}</div></div>` +
    (b.gone ? `<div class="danger">${t(L.shelfFolderMissing())}</div><div class="hit link"${onClick(osAsk(L.shelfPickBookFolder()))}>${t(L.shelfRelocate())}</div><div class="hit link"${onClick(() => removeBook(b))}>${t(L.shelfRemove())}</div>` : `<div class="faint">${t(b.at === null ? L.shelfNotOpened() : timeShown(b.at))}</div>${more(b)}`) + '</div>'
  const bring = button(L.shelfImportMore(), startImport)
  const create = button(L.shelfNewBook(), () => set({ dialog: { kind: 'newBook', name: '', type: BOOK.kinds[0] } }), 'primary')
  if (!all.length) {
    // AI 关着时，这一页不讲 AI。
    return '<div class="shelf">' + (aiOn() ? blank('book', L.shelfStartTitle(), L.shelfStartAbout(),
      `<div class="shelf-facts"><div>${t(L.shelfFactFiles())}</div><div>${t(L.shelfFactAI())}</div><div>${t(L.shelfFactKey())}</div></div>` +
      buttons(button(L.shelfAddExisting(), addBook), bring, create))
      : blank('book', L.shelfStartTitle(), L.shelfEmptyAbout(),
        `<div class="shelf-facts"><div>${t(L.shelfFactFilesNoAI())}</div></div>` + buttons(button(L.shelfAddExisting(), addBook), bring, create))) + '</div>'
  }
  return '<div class="shelf scroll"><div class="shelf-column">' +
    `<div class="shelf-head"><div class="fit"><div class="shelf-title">${t(L.shelfTitle())}</div><div class="shelf-sub"><span class="ell">${t(L.shelfCountLibrary(all.length, S.library))}</span><span class="hit link"${onClick(osAsk(L.shelfPickLibrary()))}>${t(L.shelfChange())}</span></div></div>` +
    `<div class="seg icons" aria-label="${t(L.shelfView())}">${[[L.shelfList(), 'list', false], [L.shelfCards(), 'grid', true]].map(([label, pic, cards]) => `<div class="hit${S.cards === cards ? ' on' : ''}" title="${t(label)}"${onClick(() => set({ cards }))}>${icon(pic)}</div>`).join('')}</div>` +
    button(L.shelfAddExisting(), addBook) + bring + create + '</div>' +
    (S.cards ? `<div class="shelf-cards">${all.map(card).join('')}</div>` : `<div class="shelf-rows">${all.map(row).join('')}</div>`) +
    '</div></div>'
}


// ───────── 导入书籍 ─────────

// 导入书籍是书架上的两个对话框：先是一张小表单（作品名称、体裁、位置），「下一步」再弹出预览，左边是切出来的卷和章，右边是选中的那一章，
// 标题和正文都能改。左边的树和正文页的侧栏是同一套操作：重命名、新建章节、新建文件夹、上移下移、移到哪一卷、拖动，右键点出同样的菜单；
// 只有「移到回收站」换成「删除」（作品还没建，没有回收站），点了马上生效，提示条上能撤销。
// 切错了还可以在标题右边补救：并入上一章，或者从光标处另起一章。预览里的「确定」才新建作品，原文件不动。
// S.imp 和正文页一样把卷和章分开记：volumes 是 [id, 标题]，chapters 每章有 id、title、paras、vol（所在的卷，没有就是空）；
// sel 是正看着的那一章，folds 是折起的卷，rename 是正在改名的一行，menu 是右键点出来的菜单，
// at 是改过的位置（没改过就跟着名字走），can 是光标正停在能另起一章的地方。
const impWords = (c) => c.paras.join('').replace(/\s/g, '').length
const impBox = () => document.querySelector('.import-prose')
const impCur = () => S.imp.chapters.find((c) => c.id === S.imp.sel)
const impOpen = () => S.dialog && S.dialog.kind === 'importPreview' && !!S.imp
const impChapters = () => S.imp.chapters
const impPatch = (patch) => set((s) => ({ imp: { ...s.imp, ...patch } }))
const impShut = () => impPatch({ menu: null })

function startImport() {
  let n = 0
  const volumes = [], chapters = []
  let vol = ''
  for (const r of BOOK.source.rows) {
    if (r.vol) { vol = 'v' + n++; volumes.push([vol, r.vol]) } else { const k = n++; chapters.push({ id: 'i' + k, title: r.title, vol, paras: Array.from({ length: r.paras }, (_, p) => BOOK.source.lines[(k + p) % BOOK.source.lines.length]) }) }
  }
  set({ imp: { name: BOOK.source.file.replace(/\.[^.]+$/, ''), type: BOOK.kinds[0], at: null, volumes, chapters, sel: chapters[0].id, folds: {}, can: false, n, rename: null, menu: null }, dialog: { kind: 'importForm' }, ...SHUT })
}

// 作品叫什么、放在哪。名字空着，或者那个文件夹里已有文件，都不能导入。
function impWhere() {
  const imp = S.imp, name = imp.name.trim(), at = (imp.at ?? S.library + sep() +(name || L.shelfUntitledBook())).trim()
  const taken = !!at && S.shelf.some((b) => S.library + sep() +b.name === at)
  return { name, at, taken, ok: !!name && !!at && !taken }
}

// 换到一章：正文回到最上面。
function impShow(id) {
  impPatch({ sel: id, can: false })
  const box = document.querySelector('.imp-text')
  if (box) box.scrollTop = 0
}

// 改一处切分：先把改之前的存下，提示条上点「撤销」就整个退回去。
// 这个撤销只管到下一处改动：之后又改了别的（打字、改名、挪动），提示条就收起，免得撤销把后来的改动一起退掉。
let impUndo = ''
const impRetire = () => { if (impUndo) { unsay(impUndo); impUndo = '' } }
function importChange(then, words) {
  impRetire()
  const before = { ...S.imp, rename: null, menu: null }
  set((s) => ({ imp: { ...then(s.imp), rename: null, menu: null } }))
  const box = document.querySelector('.imp-text')
  if (box) box.scrollTop = 0
  impUndo = say(words, '', 'ok', [[L.cmdUndo(), () => { impRetire(); if (S.imp) set({ imp: before }) }]])
}

// 一章前面的那一章：只在同一卷里找，卷里的第一章没有上一章。
const impPrev = (c) => { const peers = S.imp.chapters.filter((x) => x.vol === c.vol); return peers[peers.indexOf(c) - 1] }
const impMergeable = () => { const c = impCur(); return !!c && !!impPrev(c) }

// 并入上一章：这一章的标题当作一段，和正文一起接到上一章后面。
function importMerge() {
  if (!impMergeable()) return
  const gone = impCur(), prev = impPrev(gone)
  importChange((imp) => ({ ...imp, sel: prev.id, can: false, chapters: imp.chapters.filter((c) => c.id !== gone.id).map((c) => (c.id === prev.id ? { ...c, paras: [...c.paras, gone.title, ...gone.paras] } : c)) }), L.shelfMergedInto(gone.title))
}

// 光标在正文第几段第几个字后面；不在正文里时没有。
const impCaret = () => { const box = impBox(), s = box && spanNow(box); return s && s[1] }
// 光标两边一边是本章留下的字，一边是新的一章；哪边没有字都不能分。
function impParts() {
  const at = impCaret(), c = S.imp && impCur()
  if (!at || !c || !c.paras.length) return null
  const head = [...c.paras.slice(0, at.para), c.paras[at.para].slice(0, at.off)].filter((p) => p.trim())
  const rest = [c.paras[at.para].slice(at.off), ...c.paras.slice(at.para + 1)].filter((p) => p.trim())
  return head.length && rest.length ? [head, rest] : null
}

// 从光标处另起一章：光标后面的字成为新的一章，排在这一章后面。它的第一行短、不以句号结尾，就当标题，否则叫「未命名」。
function importSplit() {
  const parts = impParts()
  if (!parts) return
  const [head, rest] = parts, c = impCur(), first = rest[0].trim()
  const titled = first.length <= 20 && !/[。！？…”」]$/.test(first), title = titled ? first : L.commonUntitled(), nid = 'i' + S.imp.n
  importChange((imp) => ({ ...imp, n: imp.n + 1, sel: nid, can: false, chapters: imp.chapters.flatMap((x) => (x.id === c.id ? [{ ...x, paras: head }, { id: nid, title, vol: c.vol, paras: titled ? rest.slice(1) : rest }] : [x])) }), L.shelfSplitDone(title))
  putCaret({ para: 0, off: 0 }, false, impBox())
}

// 光标挪到哪儿，「从光标处另起一章」跟着能点或灰着。
document.addEventListener('selectionchange', () => {
  if (!impOpen() || composing) return
  const can = !!impParts()
  if (can !== S.imp.can) impPatch({ can })
})

function impParas(id, paras) {
  impRetire()
  set((s) => ({ imp: { ...s.imp, chapters: s.imp.chapters.map((c) => (c.id === id ? { ...c, paras } : c)) } }))
}
// 浏览器替作者改了正文上的字：读回来。拼音还没选完字时先不读。
function importWrote() {
  const box = impBox()
  if (!box || composing) return
  const at = spanNow(box), read = blocks(box).map((e) => textOf(e).replace(/\n/g, ''))
  const paras = read.length ? read : [box.textContent]
  impParas(S.imp.sel, paras.length === 1 && !paras[0] ? [] : paras)
  if (at) putCaret(at[1], false, box)
}
// 把选中的换成 text，没选中就放在光标那里；text 里的换行另起一段。
function impPut(text) {
  const box = impBox(), span = box && spanNow(box), c = impCur()
  if (!span) return
  const [a, b] = span, old = c.paras.length ? c.paras : [''], lines = text.split('\n')
  const paras = [...old.slice(0, a.para), ...((old[a.para] || '').slice(0, a.off) + text + (old[b.para] || '').slice(b.off)).split('\n'), ...old.slice(b.para + 1)]
  impParas(c.id, paras.length === 1 && !paras[0] ? [] : paras)
  putCaret({ para: a.para + lines.length - 1, off: (lines.length > 1 ? 0 : a.off) + lines.at(-1).length }, true, box)
}
const importKey = (ev) => { if (ev.key === 'Enter' && !ev.isComposing) { ev.preventDefault(); impPut('\n') } }
const importPaste = (ev) => { ev.preventDefault(); impPut(plain(ev.clipboardData.getData('text/plain'))) }

// ───────── 导入书籍：树里的操作，和正文页侧栏一样 ─────────

const impIsVol = (key) => S.imp.volumes.some((v) => v[0] === key)
const impTitleOf = (key) => { const v = S.imp.volumes.find((x) => x[0] === key); return v ? v[1] : S.imp.chapters.find((c) => c.id === key).title }

// 新建章节：开着一章就建在它后面；在一卷上右键建的（into 是那一卷），就建在这一卷的末尾。名字全选上等着换。
function impNewChapter(into) {
  impRetire()
  set((s) => {
    const imp = s.imp, cur = imp.chapters.find((c) => c.id === imp.sel), home = typeof into === 'string' ? into : cur ? cur.vol : ''
    const anchor = typeof into === 'string' ? imp.chapters.filter((c) => c.vol === home).at(-1) : cur, id = 'i' + imp.n
    const at = anchor ? imp.chapters.indexOf(anchor) + 1 : imp.chapters.length
    const name = L.shelfNewChapterName()
    const chapters = [...imp.chapters.slice(0, at), { id, title: name, vol: home, paras: [] }, ...imp.chapters.slice(at)]
    return { imp: { ...imp, chapters, n: imp.n + 1, sel: id, can: false, menu: null, folds: home ? { ...imp.folds, [home]: false } : imp.folds, rename: { key: id, t: name } } }
  })
}
function impNewFolder() {
  impRetire()
  const name = L.shelfNewFolderName()
  set((s) => ({ imp: { ...s.imp, volumes: [...s.imp.volumes, ['v' + s.imp.n, name]], n: s.imp.n + 1, menu: null, rename: { key: 'v' + s.imp.n, t: name } } }))
}
// 名字定下来：按了回车，或者光标离开了输入框。空着就还是原来的。
function impRenameNow() {
  const r = S.imp && S.imp.rename
  if (r && r.t.trim() && r.t.trim() !== impTitleOf(r.key)) impRetire()
  set((s) => {
    if (!s.imp || !s.imp.rename) return null
    const { key, t: title } = s.imp.rename, name = title.trim()
    if (!name) return { imp: { ...s.imp, rename: null } }
    return { imp: { ...s.imp, rename: null, chapters: s.imp.chapters.map((c) => (c.id === key ? { ...c, title: name } : c)), volumes: s.imp.volumes.map((v) => (v[0] === key ? [key, name] : v)) } }
  })
}
// 上移、下移：和同一层里前一个、后一个换位置。
function impShift(key, d) {
  impRetire()
  set((s) => {
    const imp = s.imp
    const swap = (list, same, at) => {
      const peers = list.filter(same), other = peers[peers.indexOf(list[at]) + d]
      if (!other) return list
      const out = [...list], j = list.indexOf(other)
      out[at] = other; out[j] = list[at]
      return out
    }
    const v = imp.volumes.findIndex((x) => x[0] === key), c = imp.chapters.findIndex((x) => x.id === key)
    return { imp: { ...imp, menu: null, volumes: v >= 0 ? swap(imp.volumes, () => true, v) : imp.volumes, chapters: c >= 0 ? swap(imp.chapters, (x) => x.vol === imp.chapters[c].vol, c) : imp.chapters } }
  })
}
function impMoveInto(key, vol) {
  impRetire()
  set((s) => {
    const c = s.imp.chapters.find((x) => x.id === key)
    return { imp: { ...s.imp, menu: null, chapters: [...s.imp.chapters.filter((x) => x !== c), { ...c, vol }], folds: vol ? { ...s.imp.folds, [vol]: false } : s.imp.folds } }
  })
}
// 拖一行：放到一卷上就进这一卷的末尾，放到一章上就排到它前面。卷不能放进卷里。
function impDrop(key, target) {
  impRetire()
  set((s) => {
    const imp = s.imp, c = imp.chapters.find((x) => x.id === key), into = imp.volumes.some((v) => v[0] === target), before = imp.chapters.find((x) => x.id === target)
    if (!c || c === before) return null
    const rest = imp.chapters.filter((x) => x !== c)
    if (into) return { imp: { ...imp, chapters: [...rest, { ...c, vol: target }], folds: { ...imp.folds, [target]: false } } }
    if (!before) return null
    const at = rest.indexOf(before)
    return { imp: { ...imp, chapters: [...rest.slice(0, at), { ...c, vol: before.vol }, ...rest.slice(at)] } }
  })
}
// 删除：一卷连同里面的章一起走。正看着的那一章没了，就换到挨着它的一章。
function impDelete(key) {
  const imp = S.imp, vol = imp.volumes.find((v) => v[0] === key), gone = imp.chapters.filter((c) => (vol ? c.vol === key : c.id === key)), title = impTitleOf(key)
  importChange((i) => {
    const chapters = i.chapters.filter((c) => !gone.includes(c))
    const sel = chapters.some((c) => c.id === i.sel) ? i.sel : (chapters[Math.min(i.chapters.indexOf(gone[0]), chapters.length - 1)] || { id: '' }).id
    return { ...i, chapters, sel, can: false, volumes: i.volumes.filter((v) => v !== vol) }
  }, L.shelfDeleted(title))
}

let impDragged = ''
const impNodeAt = (ev) => { const row = ev.target.closest('[data-imp-node]'); return row && impDragged && row.dataset.impNode !== impDragged ? row : null }
root.addEventListener('dragstart', (ev) => { const row = ev.target.closest('[data-imp-node]'); if (row) { impDragged = row.dataset.impNode; ev.dataTransfer.effectAllowed = 'move' } })
root.addEventListener('dragover', (ev) => { const row = impNodeAt(ev); if (row) { ev.preventDefault(); row.classList.add('over') } })
root.addEventListener('dragleave', (ev) => { const row = ev.target.closest('[data-imp-node]'); if (row) row.classList.remove('over') })
root.addEventListener('drop', (ev) => { const row = impNodeAt(ev); if (row) { ev.preventDefault(); impDrop(impDragged, row.dataset.impNode) } })
root.addEventListener('dragend', () => { impDragged = '' })

// 右键点出来的菜单，和侧栏里的一样（「移到」也是下一级）；浮在预览上面。
function importMenu() {
  const m = S.imp.menu, key = m.key, vol = impIsVol(key), c = S.imp.chapters.find((x) => x.id === key), win = document.getElementById('win')
  const spots = vol ? [] : [[L.shelfTopLevel(), ''], ...S.imp.volumes.map((v) => [v[1], v[0]])].filter(([, to]) => to !== c.vol)
  const rows = [[L.commonRename(), () => impPatch({ menu: null, rename: { key, t: impTitleOf(key) } })], [L.cmdNewChapter(), () => impNewChapter(vol ? key : undefined)], [L.cmdNewFolder(), impNewFolder]]
  return `<div class="veil float-veil top"${onClick(impShut)}${onMenu((ev) => { ev.preventDefault(); impShut() })}></div>` +
    `<div class="ctx pop top${m.x > win.offsetWidth - 208 - 208 ? ' flip' : ''}" data-k="imp-menu"${subOver(!!m.into, (on) => impPatch({ menu: { ...S.imp.menu, into: on } }))} style="left:${Math.min(m.x, win.offsetWidth - 208)}px;top:${Math.min(m.y, win.offsetHeight - Math.max(250, 148 + Math.min(spots.length, 10) * 30))}px">` +
    rows.map(([label, act]) => menuRow(label, act)).join('') + HAIR +
    menuRow(L.commonMoveUp(), () => impShift(key, -1)) + menuRow(L.commonMoveDown(), () => impShift(key, 1)) +
    (spots.length ? subRow(L.shelfMoveTo(), m.into, () => impPatch({ menu: { ...S.imp.menu, into: true } }), 'imp-into', spots.map(([label, to]) => menuRow(label, () => impMoveInto(key, to))).join('')) : '') + HAIR +
    menuRow(L.commonDelete(), () => impDelete(key)) + '</div>'
}

// 第一个对话框：作品叫什么、什么体裁、放在哪，和「新建作品」同样的字。名字空着、文件夹里已有文件时「下一步」是灰的。
function importFormDialog() {
  const imp = S.imp, w = impWhere(), type = (key) => (v) => set((s) => ({ imp: { ...s.imp, [key]: v } }))
  const folder = w.at.split(/[\\/]/).filter(Boolean).at(-1) || w.at
  const where = !w.at ? `<div class="note">${t(L.shelfLocationNeeded())}</div>` : w.taken ? `<div class="note danger">${t(L.shelfFolderTaken(folder))}</div>` : `<div class="note">${t(L.shelfWillCreate(folder))}</div>`
  const next = () => { if (w.ok) set({ dialog: { kind: 'importPreview' }, ...SHUT }) }
  return dialog('small', dialogHead(L.shelfImportBook(), L.shelfImportAbout()) +
    `<div class="field"><div class="label">${t(L.shelfFile())}</div><div class="pick-line"><div class="box line fit"><div class="fit ell">${t(BOOK.source.file)}</div></div>${button(L.shelfChangeFile(), osAsk(L.shelfPickImport()))}</div>` +
    `<div class="note">${t(L.shelfDetected(imp.volumes.length, imp.chapters.length))}</div></div>` +
    field(L.shelfBookName(), line(imp.name, type('name'), '', L.shelfBookName(), keys(next) + ' autofocus')) + (w.name ? '' : `<div class="note">${t(L.shelfNameNeeded())}</div>`) +
    field(L.shelfGenre(), choose('imp-kind', imp.type, BOOK.kinds.map((k) => [k, genreWord(k)]), type('type'), L.shelfGenre())) +
    `<div class="field"><div class="label">${t(L.shelfLocation())}</div><div class="pick-line"><div class="box line fit">${typed(w.at, type('at'), '', L.shelfBookLocation(), keys(next))}</div>${button(L.shelfChangePlace(), osAsk(L.shelfPickNewLocation()))}</div>${where}</div>` +
    buttons(button(L.commonCancel(), () => set({ dialog: null, imp: null, ...SHUT })), button(L.commonNext(), w.ok ? next : null, w.ok ? 'primary' : 'primary off')))
}

// 第二个对话框：预览。左边是切出来的卷和章，和正文页的侧栏一样：卷能折起，字数指针移上去才出来，右键、拖动、改名都和那里一样；
// 右边是选中的那一章，标题和正文都能改，标题右边是补救切分的两样；按下它们不拿走正文的光标（data-keep）。「确定」新建作品并打开它。
function importPreviewDialog() {
  const imp = S.imp, w = impWhere(), cur = impCur(), chapters = imp.chapters
  const total = chapters.reduce((n, c) => n + impWords(c), 0), merge = impMergeable()
  const menu = (key) => onMenu((ev) => { ev.preventDefault(); const [x, y] = spot(ev.clientX, ev.clientY); impPatch({ menu: { key, x, y } }) })
  // 改名字的那一行还是那一行：图标、缩进都不动，只是名字换成输入框。回车和光标离开都算定了，Esc 不改。
  const naming = (key, pic, deep) => `<div class="tree-row naming${pic ? '' : ' leaf'}${deep ? ' deep' : ''}${key === imp.sel ? ' on' : ''}" data-k="imp-${key}">${pic}` +
    `${typed(imp.rename.t, (v) => set((s) => (s.imp && s.imp.rename ? { imp: { ...s.imp, rename: { ...s.imp.rename, t: v } } } : null)), '', L.commonRename(), keys(impRenameNow, () => impPatch({ rename: null })) + onBlur(impRenameNow) + ' autofocus data-all')}</div>`
  const leaf = (c) => (imp.rename && imp.rename.key === c.id ? naming(c.id, '', c.vol)
    : `<div class="tree-row leaf hit${c.vol ? ' deep' : ''}${c.id === imp.sel ? ' on' : ''}" draggable="true" data-k="imp-${c.id}" data-imp-node="${c.id}"${onClick(() => impShow(c.id))}${menu(c.id)}><div class="fit ell">${t(c.title.trim() || L.commonUntitled())}</div><div class="count">${t(L.commonNumber(impWords(c)))}</div></div>`)
  const volume = ([key, title]) => {
    const folded = !!imp.folds[key], pic = icon(folded ? 'right' : 'down', 'xs'), mine = chapters.filter((c) => c.vol === key)
    return (imp.rename && imp.rename.key === key ? naming(key, pic)
      : `<div class="tree-row hit" data-k="imp-${key}" data-imp-node="${key}"${onClick(() => impPatch({ folds: { ...imp.folds, [key]: !folded } }))}${menu(key)}>${pic}<div class="fit ell">${t(title)}</div><div class="count">${t(L.commonNumber(mine.reduce((n, c) => n + impWords(c), 0)))}</div></div>`) +
      (folded ? '' : mine.map(leaf).join(''))
  }
  const tree = imp.volumes.map(volume).join('') + chapters.filter((c) => !c.vol).map(leaf).join('')
  const add = `<div class="import-tree-head"><div class="faint fit">${t(L.shelfVolumesChapters())}</div><button class="icon-btn muted" aria-label="${t(L.cmdNewChapter())}" title="${t(L.cmdNewChapter())}"${onClick(() => impNewChapter())}>${ICON.plus}</button></div>`
  const acts = '<div class="import-acts" data-keep>' +
    `<div class="hit link${merge ? '' : ' off'}" title="${t(merge ? L.shelfMergeHint() : L.shelfMergeNone())}"${onClick(merge ? importMerge : null)}>${t(L.shelfMerge())}</div>` +
    `<div class="hit link${imp.can ? '' : ' off'}" title="${t(imp.can ? L.shelfSplitHint() : L.shelfSplitNeedCursor())}"${onClick(imp.can ? importSplit : null)}>${t(L.shelfSplit())}</div></div>`
  const toProse = () => putCaret({ para: 0, off: 0 }, false, impBox())
  const side = !cur ? '<div class="snap-side">' + empty(L.shelfNoChapters(), L.shelfNoChaptersAbout()) + '</div>' : '<div class="snap-side">' +
    `<div class="import-head"><input class="in chapter-title import-title" value="${t(cur.title)}" placeholder="${t(L.commonUntitled())}" aria-label="${t(L.shelfChapterTitle())}" spellcheck="false"${onInput((v) => { impRetire(); set((s) => ({ imp: { ...s.imp, chapters: s.imp.chapters.map((c) => (c.id === cur.id ? { ...c, title: v } : c)) } })) })}${keys(toProse)}>${acts}</div><div class="note">${t(L.shelfSplitNote())}</div>` +
    `<div class="snap-text imp-text scroll"><div class="import-prose" contenteditable="plaintext-only" spellcheck="false" aria-label="${t(L.shelfTextLabel())}"${onInput(importWrote)}${onKey(importKey)}${onPaste(importPaste)}>${(cur.paras.length ? cur.paras : ['']).map((p, i) => `<div data-para="${i}">${p ? t(p) : '<br>'}</div>`).join('')}</div></div>` +
    `<div class="faint">${t(L.shelfChapterWords(impWords(cur)))}</div></div>`
  const ok = !!chapters.length
  const go = () => {
    const id = 'book' + S.n, words = total
    set((s) => ({ shelf: [{ id, name: w.name, kind: imp.type, words, at: stamp() }, ...s.shelf], n: s.n + 1, dialog: null, imp: null }))
    openBook(id)
    say(L.shelfImported(chapters.length), w.name, 'ok')
  }
  return dialog('preview', dialogHead(L.shelfPreviewTitle(BOOK.source.file), L.shelfPreviewAbout(imp.volumes.length, imp.chapters.length, total)) +
    '<div class="snaps">' +
    `<div class="import-tree">${add}<div class="tree scroll">${tree}</div></div>` + side + '</div>' +
    buttons(button(L.commonCancel(), () => set({ dialog: null, imp: null, ...SHUT })), button(L.commonBack(), () => set({ dialog: { kind: 'importForm' }, ...SHUT })), button(L.commonOK(), ok ? go : null, ok ? 'primary' : 'primary off'))) +
    (imp.menu ? importMenu() : '')
}
