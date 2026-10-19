// 正文以外的几页：讨论、大纲、检查、作品分类，和书架。
'use strict'

// ───────── 讨论 ─────────

function talkPage() {
  const talk = S.tv === 'one' ? S.talks.find((x) => x.id === S.tid) : null
  // 设定里已经有东西的书，接着往下聊，不再从一个想法开始。
  const begun = S.entries.length > 0
  const [opening, about] = begun ? ['接着聊这部作品', 'AI 会参考已有的设定和正文。'] : ['从一个初步的想法开始', '写下初步的想法。讨论中定下的内容，AI 会当场记进右侧的设定文档；已确定的设定将作为后续讨论的依据，已放弃的不再提及。']
  const starts = begun ? ['接下来的情节怎么走', '哪些设定还没想清楚', '梳理一下人物关系'] : ['我想写一个总在下雨的城市', '主角有个说不出口的秘密', '这个故事的冲突在哪']
  return '<div class="editor">' +
    `<div class="toolbar"><div class="crumb">总讨论</div>${icon('right', 'xs', 'faint')}<div class="crumb here ell">${t(talk ? talk.title : '新对话')}</div><div class="grow"></div>` +
    (talk ? `<div class="count-all">${talk.msgs.length} 条发言</div>` : '') +
    (S.paneOpen ? '' : `<button class="icon-btn muted" aria-label="展开设定" title="展开设定文档"${onClick(() => set({ paneOpen: true }))}>${svg('icon-md', PANEL + path('M15 5v14'))}</button>`) + '</div>' +
    (talk
      ? `<div class="talk big scroll" id="talk-page">${said(talk)}</div>`
      : '<div class="paper">' + blank('spark', opening, about,
        `<div class="starts">${starts.map((words) => `<div class="hit"${onClick(offer(words))}>${words}</div>`).join('')}</div>`) + '</div>') +
    '<div class="composer big" data-k="composer-page"><div class="box send-box">' +
    `<textarea class="in" rows="2" placeholder="输入想法，或向 AI 提问…" aria-label="讨论输入"${onInput((v) => set({ draft: v }))}${onKey(sendKey)}>${t(S.draft)}</textarea>` +
    `<div class="send-line">${modelPick()}<div class="grow"></div><div class="faint">Ctrl+Enter</div><div class="send hit${S.draft.trim() && !S.waiting ? '' : ' off'}" title="发送（Ctrl+Enter）"${onClick(() => send())}>${ICON.send}</div></div>` +
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
  const drop = () => { set((s) => ({ events: s.events.filter((x) => x.id !== d.id), dialog: null })); say('已删除卡片「' + f.title + '」', '', 'ok') }
  const chs = ['未放进章节', ...S.chapters.map(titleOf)], lines = ['不分情节线', ...S.lines.map((l) => l.name)]
  const pickCh = (v) => type('key')(v === chs[0] ? '' : S.chapters.find((c) => titleOf(c) === v).key)
  const pickLine = (v) => type('line')(v === lines[0] ? '' : S.lines.find((l) => l.name === v).id)
  return dialog('small', dialogHead(d.id ? '修改卡片' : '新卡片', d.id ? '' : d.n ? '已添加「' + S.events.find((e) => e.id === d.after).title + '」，接着记下一张。' : '一张卡片记一件事。只有「发生了什么」要写，别的都可以以后再补。') +
    // 「继续添加」以后换一个新的输入框，光标回到它里面。
    `<div class="field" data-k="card-title-${d.n}"><div class="label">发生了什么</div>${line(f.title, type('title'), '一句话，例如：林晚下井', '事件', keys(save) + ' autofocus')}</div>` +
    `<div class="pair">${field('写在哪一章', choose('card-ch', f.key ? titleOf(chapterOf(f.key)) : chs[0], chs, pickCh, '写在哪一章'))}${field('情节线', choose('card-line', lineOf(f.line) ? lineOf(f.line).name : lines[0], lines, pickLine, '情节线'))}</div>` +
    field('故事中的时间（选填）', line(f.when, type('when'), '例如：十月六日夜。不写也行，按先后排', '故事中的时间', keys(save))) +
    field('说明（选填）', area(f.about, type('about'), '', '说明', 4)) +
    '<div class="buttons">' + (d.id ? `<div class="hit link danger"${onClick(drop)}>删除卡片</div><div class="grow"></div>` : '') + button('取消', shut) +
    (d.id ? '' : button('继续添加', ok ? () => saveCard(true) : null, ok ? '' : 'off')) + button(d.id ? '保存' : '添加', ok ? save : null, ok ? 'primary' : 'primary off') + '</div>')
}
// 一张卡片：停上去浮起来一点，点开改。
const cardView = (e, q, also = '') => `<div class="card hit${e.key ? '' : ' later'}${S.planSee === e.id ? ' seen' : ''}${found(e, q) ? '' : ' dim'}" draggable="true" data-k="card-${e.id}" data-card="${e.id}" title="点击修改；拖到别的格子挪过去"${onClick(() => openCard(e.id))}>` +
  (e.when && also !== 'ch' ? `<div class="card-when ell">${marked(e.when, q)}${also === 'line' && lineOf(e.line) ? `<span class="faint"> · ${t(lineOf(e.line).name)}</span>` : ''}</div>` : '') +
  `<div class="card-title clamp${e.title ? '' : ' none'}">${e.title ? marked(e.title, q) : '未命名'}</div>` +
  (also === 'ch' ? `<div class="card-ch ell">${e.key ? marked(chTitle(e.key), q) : '未放进章节'}</div>` : '') + '</div>'

// 格子里的「添加」：停在格子上才显出来，和卡片一样宽，点了弹出新卡片。
const slotAdd = (label, act) => `<div class="slot-add hit" title="添加卡片" aria-label="${t(label)}"${onClick(act)}>${icon('plus', 'sm')}添加</div>`
// 一条情节线的名字：点名字改名，「…」里改名、挪位置、删除。按章节在表头，时间线在左边一列，两处一样。
function lineTitle(l, where) {
  if (S.planLine && S.planLine.id === l.id) {
    return `<div class="box line fit" data-k="lh-${where}-${l.id}-edit">${typed(S.planLine.name, (v) => set((s) => ({ planLine: { ...s.planLine, name: v } })), '情节线的名字', '情节线的名字', keys(saveLine, () => set({ planLine: null })) + onBlur(saveLine) + ' autofocus data-all')}</div>`
  }
  const ls = S.lines, n = S.events.filter((e) => e.line === l.id).length, menu = 'line-' + where + '-' + l.id
  return `<div class="fit ell hit head-name" data-k="lh-${where}-${l.id}" title="点击改名"${onClick(() => set({ planLine: { id: l.id, name: l.name }, ...SHUT }))}>${t(l.name)}</div><div class="faint">${n}</div>` +
    `<div class="anchor"><button class="icon-btn muted" aria-label="情节线「${t(l.name)}」的操作" title="更多操作"${onClick(() => set({ open: S.open === menu ? '' : menu }))}>${ICON.dots}</button>` +
    (S.open === menu ? '<div class="pop line-pop">' + menuRow('改名', () => set({ planLine: { id: l.id, name: l.name }, ...SHUT })) + menuRow(where === 'grid' ? '左移' : '上移', ls[0] === l ? null : () => shiftLine(l.id, -1)) + menuRow(where === 'grid' ? '右移' : '下移', ls.at(-1) === l ? null : () => shiftLine(l.id, 1)) + '<div class="hair"></div>' + menuRow('删除情节线', () => dropLine(l.id)) + '</div>' : '') + '</div>'
}
const addLineButton = () => `<div class="plan-add hit" title="多一条情节线"${onClick(() => addLine())}>${icon('plus', 'sm')}新增情节线</div>`

// 按章节：一章一行，一条情节线一列，卡片落在交叉的格子里；哪条线断了几章，一眼看得出。
// 左边的章节一列和上面的表头滚动时不动，「新增情节线」总在右上角。
function planGrid(q) {
  const ls = S.lines
  const outlineBox = (key, hint, rows) => `<div class="box area" data-k="outline-${key}"><textarea class="in" rows="${rows}" placeholder="${hint}" aria-label="${chapterOf(key) ? '章纲' : '卷纲'}" autofocus` +
    `${onInput((v) => set((s) => ({ outlines: { ...s.outlines, [key]: v } })))}${onBlur(() => set({ planOutline: null }))}${onKey((ev) => { if (ev.key === 'Escape') { ev.stopPropagation(); set({ planOutline: null }) } })}>${t(S.outlines[key] || '')}</textarea></div>`
  const gist = (key, text, none, cls) => (S.planOutline === key ? outlineBox(key, none, cls === 'vol-gist' ? 3 : 4)
    : `<div class="${cls} hit${text ? '' : ' none'}" title="点击修改"${onClick(() => set({ planOutline: key, planLine: null }))}>${text ? marked(text, q) : none}</div>`)
  const cell = (c, l) => `<div class="board-cell" data-cell="${c.key}|${l.id}">${S.events.filter((e) => e.key === c.key && e.line === l.id).map((e) => cardView(e, q)).join('')}` +
    slotAdd(`在${titleOf(c)}的${l.name}里添加卡片`, () => newCard(c.key, l.id)) + '</div>'
  const chRow = (c) => {
    const when = whenOf(eventsOf(c.key)), stray = S.events.filter((e) => e.key === c.key && !lineOf(e.line))
    return `<div class="board-side grid-ch" data-k="row-${c.key}"><div class="ch-name"><div class="hit ell" title="打开正文"${onClick(goTo(c.key))}>${marked(titleOf(c), q)}</div>${goesBack(c) ? '<div class="tag" title="这一章写到了比前面各章更早的事">回溯</div>' : ''}</div>` +
      (when ? `<div class="note ell">故事中 ${t(when)}</div>` : '') + gist(c.key, gistOf(c), '写章纲…', 'ch-gist clamp') +
      (stray.length ? `<div class="faint">另有 ${stray.length} 张不在任何情节线</div>` : '') + '</div>' + ls.map((l) => cell(c, l)).join('') + '<div class="board-fill"></div>'
  }
  const hits = (c) => !q || [titleOf(c), gistOf(c)].some((x) => x.includes(q)) || eventsOf(c.key).some((e) => found(e, q))
  const block = (key, title, inside) => {
    const shown = inside.filter(hits)
    if (q && !shown.length && ![title, S.outlines[key] || ''].some((x) => x.includes(q))) return '' // 卷名、卷纲搜到了，这一卷的标题留着
    const when = whenOf(inside.flatMap((c) => eventsOf(c.key)))
    return `<div class="grid-vol" data-k="vol-${key || 'loose'}"><div class="vol-in"><div class="vol-name"><div class="group-title">${marked(title, q)}</div><div class="note">${inside.length} 章${when ? ' · ' + t(when) : ''}</div></div>` +
      (key ? gist(key, (S.outlines[key] || '').trim(), '写卷纲…', 'vol-gist') : '') + '</div></div>' + shown.map(chRow).join('')
  }
  const loneCh = S.chapters.filter((c) => !c.vol)
  const body = S.volumes.map(([key, title]) => block(key, title, S.chapters.filter((c) => c.vol === key))).join('') + (loneCh.length ? block('', '不在卷中', loneCh) : '')
  return `<div class="board grid scroll" data-k="grid"><div class="board-table" style="grid-template-columns: 248px repeat(${ls.length}, 200px) minmax(160px, 1fr)">` +
    '<div class="board-head board-corner">章节</div>' + ls.map((l) => `<div class="board-head">${lineTitle(l, 'grid')}</div>`).join('') + `<div class="board-head board-end">${addLineButton()}</div>` +
    // 章节里一处也没搜到，搜到的只有下面托盘里的卡片。
    (body || `<div class="grid-none">${empty('章节里没有「' + q + '」', '找到的卡片在下面「未放进章节」里。')}</div>`) + '</div></div>' + tray(q)
}
const saveLine = () => set((s) => (s.planLine ? { lines: s.lines.map((l) => (l.id === s.planLine.id ? { ...l, name: s.planLine.name.trim() || l.name } : l)), planLine: null } : null))
// 新的一条线放在最后，名字先选中，打字就换掉。
const addLine = () => set((s) => { const id = 'l' + s.n; return { lines: [...s.lines, { id, name: '新情节线' }], n: s.n + 1, planLine: { id, name: '新情节线' }, planOutline: null, ...SHUT } })
const shiftLine = (id, d) => set((s) => { const i = s.lines.findIndex((l) => l.id === id), list = [...s.lines];[list[i], list[i + d]] = [list[i + d], list[i]]; return { lines: list, ...SHUT } })
// 删掉一条线：它的卡片不删，放到「未放进章节」里，写在哪一章的在卡片上记着。
function dropLine(id) {
  const l = lineOf(id), n = S.events.filter((e) => e.line === id).length
  set((s) => ({ lines: s.lines.filter((x) => x.id !== id), events: s.events.map((e) => (e.line === id ? { ...e, line: '' } : e)), ...SHUT }))
  say('已删除情节线「' + l.name + '」', n ? '它的 ' + n + ' 张卡片还在原来的章里，可在时间线上找到' : '', 'ok')
}

// 下面一条：想到了、还没定写在哪的卡片。拖进上面的格子，就放进那一章。
function tray(q) {
  const cards = S.events.filter((e) => !e.key), all = cards.length
  return `<div class="tray" data-k="tray" data-cell="|"><div class="tray-head"><div class="group-title">未放进章节</div><div class="note">${all ? all + ' 张 · 拖进上面的格子就放进那一章' : '想到了、还没定写在哪的事，先记在这里'}</div><div class="grow"></div>` +
    `<div class="hit link"${onClick(() => newCard('', ''))}>记一张</div></div>` +
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
  const lanes = [...S.lines, ...(S.events.some((e) => !lineOf(e.line)) ? [{ id: '', name: '不分情节线' }] : [])]
  const inLane = (e, l) => (l.id ? e.line === l.id : !lineOf(e.line))
  const add = (l, when, after) => slotAdd(`在${when || '最后'}的${l.name}里添加卡片`, () => newCard('', l.id, after, when))
  const timePop = (c) => '<div class="pop time-pop" data-k="time-pop">' +
    `<div class="pop-label">这一列 ${c.cards.length} 张卡片在故事中的时间</div>` +
    `<div class="box line">${typed(S.planTime.when, (v) => set((s) => ({ planTime: { ...s.planTime, when: v } })), '例如：十月六日夜', '这一列的时间', keys(saveTime) + ' autofocus data-all')}</div>` +
    `<div class="pop-buttons">${button('取消', () => set(SHUT))}${button('保存', saveTime, 'primary')}</div></div>`
  const time = (c, i) => `<div class="board-head tl-time anchor left" data-k="time-${i}">` +
    `<div class="tl-when hit ell${c.when ? '' : ' none'}" title="点击修改这一列的时间"${onClick(() => set({ open: 'time', planTime: { i, when: c.when } }))}>${c.when ? marked(c.when, q) : '未写时间'}</div>` +
    `<div class="tl-axis"><i class="ev-dot${c.cards.some((e) => e.key) ? '' : ' later'}"></i></div>${S.open === 'time' && S.planTime.i === i ? timePop(c) : ''}</div>`
  const lane = (l) => `<div class="board-side tl-lane">${l.id ? lineTitle(l, 'tl') : `<div class="fit ell head-name">${t(l.name)}</div>`}</div>` +
    cols.map((c, i) => `<div class="board-cell" data-slot="${i}|${l.id}">${c.cards.filter((e) => inLane(e, l)).map((e) => cardView(e, q, 'ch')).join('')}${add(l, c.when, c.cards.at(-1).id)}</div>`).join('') +
    `<div class="board-cell" data-slot="end|${l.id}">${add(l, '', S.events.length ? S.events.at(-1).id : null)}</div>`
  const off = S.chapters.filter((c) => !eventsOf(c.key).length)
  return `<div class="tl-note"><span><i class="tl-key"></i>已放进章节</span><span><i class="tl-key later"></i>未放进章节</span><span class="faint">时间可以不写，左右的先后就是故事里的先后</span><span class="grow"></span>` +
    (off.length ? `<span>${off.length === 1 ? '「' + t(titleOf(off[0])) + '」' : off.length + ' 章'}还没有卡片</span>` : '') + '</div>' +
    `<div class="board tl scroll" data-k="tl"><div class="board-table" style="grid-template-columns: 168px repeat(${cols.length}, 200px) 200px">` +
    '<div class="board-head tl-time board-corner"><div class="tl-when none">情节线</div><div class="tl-axis"></div></div>' + cols.map(time).join('') +
    '<div class="board-head tl-time"><div class="tl-when none">之后</div><div class="tl-axis end"></div></div>' +
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
  return '<div class="editor"><div class="toolbar"><div class="crumb here">大纲</div></div><div class="plan-none">' +
    blank('warn', '大纲文件读不出来', 'outline.json 可能在软件外被改过。正文和设定不受影响；改好文件后，这一页会自己重新读取。',
      '<div class="note faint">第 12 行附近有错</div>' + buttons(button('打开所在文件夹', osAsk('在资源管理器中显示这部作品的文件夹')))) + '</div></div>'
}

function planPage() {
  if (S.planBroken) return planBroken()
  const timeline = S.plan === '时间线', q = S.planQ.trim()
  const loose = S.events.filter((e) => !e.key).length
  // 搜着的时候，上面那一句换成找到了多少；一个也没找到，整页换成一句话和「清除搜索」，表头和下面的托盘都不留。
  const cards = q ? S.events.filter((e) => found(e, q)).length : 0
  const chs = q && !timeline ? S.chapters.filter((c) => [titleOf(c), gistOf(c)].some((x) => x.includes(q))).length + S.volumes.filter(([key, title]) => [title, S.outlines[key] || ''].some((x) => x.includes(q))).length : 0
  const sum = !q ? S.chapters.length + ' 章 · ' + S.lines.length + ' 条情节线 · ' + S.events.length + ' 张卡片' + (loose ? '，' + loose + ' 张未放进章节' : '')
    : cards || chs ? '找到 ' + [chs && chs + ' 处章名或章纲', cards && cards + ' 张卡片'].filter(Boolean).join('、') : '没有找到'
  const clear = () => set({ planQ: '' })
  const none = q && !cards && !chs
  return '<div class="editor"><div class="toolbar"><div class="crumb here">大纲</div><div class="note plan-sum">' + t(sum) + '</div><div class="grow"></div>' +
    `<div class="box line plan-find">${ICON.find}${typed(S.planQ, (v) => set({ planQ: v }), timeline ? '搜索卡片' : '搜索卡片和章纲', '搜索大纲', keys(() => {}, clear))}` +
    (q ? `<button class="icon-btn muted" aria-label="清除搜索" title="清除搜索（Esc）"${onClick(clear)}>${icon('cross', 'sm')}</button>` : '') + '</div>' +
    seg(S.plan, ['按章节', '时间线'], (v) => set({ plan: v, planSee: null, planOutline: null, planLine: null, ...SHUT }), '大纲的看法') +
    (S.planPane ? '' : `<button class="icon-btn muted" aria-label="展开右栏" title="展开右栏，查看设定"${onClick(() => set({ planPane: true }))}>${svg('icon-md', PANEL + path('M15 5v14'))}</button>`) + '</div>' +
    (none ? '<div class="plan-none">' + blank('find', '没有找到「' + q + '」', timeline ? '时间线上搜的是卡片：发生了什么、说明、故事中的时间和所在的章。' : '按章节搜的是章名、章纲、卷纲和卡片里的字。', buttons(button('清除搜索', clear))) + '</div>'
      : timeline ? planTimeline(q) : planGrid(q)) + '</div>'
}

// ───────── 检查 ─────────

const KINDS = ['全部', '不一致', '名字有误', '新出现']

// 一条处理完了：从列表里拿掉，接着看下一条。
const settle = (id) => (s) => {
  const of = s.checks.filter((c) => s.kind === '全部' || c.kind === s.kind), at = of.findIndex((c) => c.id === id)
  const next = of[at + 1] || of[at - 1]
  return { checks: s.checks.filter((c) => c.id !== id), cid: next ? next.id : null }
}

function checkPage() {
  const of = S.checks.filter((c) => S.kind === '全部' || c.kind === S.kind)
  const shown = of.find((c) => c.id === S.cid) || of[0]
  const count = (k) => (k === '全部' ? S.checks.length : S.checks.filter((c) => c.kind === k).length)
  const about = (c) => {
    const ch = chapterOf(c.key), at = chTitle(c.key)
    if (c.kind === '不一致') { const e = S.entries.find((x) => x.id === c.entry); return at + ' ↔ 设定「' + (e ? e.cat + ' · ' + (e.names[0] || clip(e.text, 8)) : '已删除') + '」' }
    return c.kind === '名字有误' ? at + ' · 应为「' + c.right + '」' : c.cat + ' · 「' + at + '」起出现' + (c.also ? '，共 ' + (c.also.length + 1) + ' 章' : '')
  }
  const row = (c) => `<div class="check-row row${shown && c.id === shown.id ? ' on' : ''}"${onClick(() => set({ cid: c.id }))}><div class="fit"><div class="ell">${t(c.title)}</div><div class="note ell">${t(about(c))}</div></div><div class="side">${c.kind}</div></div>`
  // 「检查全书」：先数出该查几章，问一句再开始。
  const owed = due()
  const again = () => {
    if (!S.entries.some((e) => e.place === 'body')) say('设定为空，无可对照内容')
    else if (!owed.length) say('各章均已检查，正文无新改动')
    else set({ open: S.open === 'checkAll' ? '' : 'checkAll' })
  }
  const ask = () => `<div class="pop ask-pop" data-k="ask-pop"><div class="pop-title">${owed.length} 章未检查或有改动</div><div class="note">每章调用一次 AI，共 ${owed.length} 次。</div>` +
    `<div class="pop-buttons">${button('取消', () => set(SHUT))}${button('开始检查', () => { set(SHUT); checkNow(...owed.map((c) => c.key)) }, 'primary')}</div></div>`
  const last = S.chapters.filter((c) => !S.never[c.key] && counted(c) >= 200).at(-1)
  const running = chapterOf(S.checking), lost = S.chapters.find((c) => S.failed[c.key])
  const none = !S.entries.some((e) => e.place === 'body') ? ['设定为空', '请先在讨论中确定设定，才能对照检查。']
    : S.checks.length ? ['这一类没有待处理的', ''] : !last ? ['尚未检查', '点击左上角「检查全书」，或写完一章后自动检查。']
    : ['没有发现不一致', S.when === '手动检查' ? '当前为「手动检查」，需要时点击「重新检查」。' : '写完一章后自动检查，结果显示在此处。']
  const unfinished = lost && !S.checking // 失败的那章不当作没查出问题，上面那一行也不说「已检查至」，像是那以前都查过了
  if (unfinished) none.splice(0, 2, '检查未完成', '「' + titleOf(lost) + '」检查失败：' + S.failed[lost.key])

  let detail = `<div class="check-detail">${blank(unfinished ? 'warn' : 'checked', S.checking ? '正在检查…' : none[0], S.checking ? '检查结果将显示在左侧。' : none[1])}</div>`
  if (shown) {
    const ch = chapterOf(shown.key), e = S.entries.find((x) => x.id === shown.entry)
    const text = ch ? ch.paras[shown.para] || '' : ''
    const lit = (words, part) => words.split(part).map(t).join(`<span class="lit">${t(part)}</span>`)
    const seeText = ch ? goTo(ch.key, { key: ch.key, quote: shown.quote }) : null
    const seeEntry = () => set({ page: 'write', paneOpen: true, focus: false, tab: '设定', sv: 'list', sid: e.id, form: null })
    const next = () => set({ cid: (of[of.indexOf(shown) + 1] || of[0]).id })
    const right = shown.kind === '不一致' && e ? `<div class="fit"><div class="check-from"><div class="fit ell">设定 · ${t(e.cat)} · ${t(e.names[0] || '')}</div><div class="hit link"${onClick(seeEntry)}>查看设定</div></div><div class="check-card">${lit(e.text, shown.says)}</div></div>`
      : shown.kind === '名字有误' ? `<div class="fit"><div class="check-from"><div class="fit ell">设定中的名字</div></div><div class="check-card"><span class="lit">${t(shown.right)}</span></div></div>`
      : `<div class="fit"><div class="check-from"><div class="fit ell">可加入设定 · ${t(shown.cat)}</div></div><div class="check-card"><span class="lit">${t(shown.title)}</span>：${t(shown.as)}</div></div>`
    const add = () => { set((s) => ({ ...settle(shown.id)(s), entries: [...s.entries, { id: 'e' + s.n, place: 'body', cat: shown.cat, names: [shown.title], text: shown.as, src: { key: shown.key, quote: shown.quote } }], n: s.n + 1, log: did(s, '新增到正文 · ' + shown.cat + '：' + shown.as) })); say('已加入设定', '', 'ok') }
    const acts = shown.kind === '新出现' ? button('忽略', () => set(settle(shown.id))) + button('加入设定', add, 'primary')
      : button('无需修改', () => set(settle(shown.id))) + (shown.kind === '不一致' && e ? button('修改设定', () => set({ page: 'write', paneOpen: true, focus: false, tab: '设定', sv: 'list', sid: e.id, form: editing(e) })) : '') + button('修改正文', seeText, 'primary')
    detail = '<div class="check-detail" data-k="check-detail">' +
      `<div class="check-title"><div class="fit ell">${t(shown.title)}</div><div class="side">${of.indexOf(shown) + 1} / ${of.length}</div><div class="hit link${of.length > 1 ? '' : ' off'}"${onClick(of.length > 1 ? next : null)}>下一条</div></div>` +
      `<div class="check-sides"><div class="fit"><div class="check-from"><div class="fit ell">正文 · ${ch ? t(titleOf(ch)) + ' · ' : ''}第 ${shown.para + 1} 段</div><div class="hit link"${onClick(seeText)}>查看原文</div></div><div class="check-card">${lit(text, shown.quote)}</div></div>${right}</div>` +
      (shown.also ? `<div class="check-also">${shown.kind === '新出现' ? '其他章节中也出现' : '其他章节的类似情况'}</div>` + shown.also.map((a) => `<div class="check-more"><div class="side">${t(a.at)}</div><div class="fit">${t(a.text)}</div><div class="side">${t(a.note)}</div></div>`).join('') : '') +
      `<div class="grow"></div><div class="check-foot"><div class="hit link${of.length > 1 ? '' : ' off'}"${onClick(of.length > 1 ? next : null)}>稍后再说</div><div class="grow"></div>${acts}</div></div>`
  }
  return '<div class="check-list">' +
    `<div class="check-head"><div class="page-title">检查</div><div class="grow"></div>` +
    (running ? `<div class="muted">正在检查「${t(titleOf(running))}」${S.queue.length ? '，还有 ' + S.queue.length + ' 章' : ''}</div><div class="hit link"${onClick(() => set({ checking: false, queue: [] }))}>停止</div>`
      : `<div class="muted">${unfinished ? '已检查 ' + S.chapters.filter((c) => !S.never[c.key] && counted(c) >= 200 && !S.failed[c.key]).length + ' 章，' + S.chapters.filter((c) => S.failed[c.key]).length + ' 章失败' : last ? '已检查至「' + t(titleOf(last)) + '」' : ''}</div><div class="anchor"><div class="hit link"${onClick(again)}>${last ? '重新检查' : '检查全书'}</div>${S.open === 'checkAll' ? ask() : ''}</div>`) + '</div>' +
    seg(S.kind + ' ' + count(S.kind), KINDS.map((k) => k + ' ' + count(k)), (v) => set({ kind: v.split(' ')[0], cid: null }), '检查分类') +
    `<div class="check-rows scroll">${of.map(row).join('')}</div></div>` + detail
}

// ───────── 作品分类 ─────────

function saveCat() {
  const name = S.catForm.name.trim(), land = S.catForm.land.trim()
  if (!name) { say('分类名称不能为空'); return }
  if (!land) { say('请填写落点，例如“设定库”或“大纲”'); return }
  set((s) => ({ cats: s.cats.includes(name) ? s.cats : [...s.cats.filter((c) => c !== '其他'), name, '其他'], lands: { ...s.lands, [name]: land }, catForm: { name: '', land: '' } }))
  say('分类“' + name + '”已保存', '', 'ok')
}
function dropCat(name) {
  set((s) => ({ cats: s.cats.filter((c) => c !== name), entries: s.entries.map((e) => (e.cat === name ? { ...e, cat: '其他' } : e)), catDrop: '' }))
  say('已删除分类“' + name + '”', '', 'ok')
}

// 删除分类：有设定的先问一句（设定要移到「其他」），空的直接删。
function catsPage() {
  const count = (c) => S.entries.filter((e) => e.cat === c && e.place !== 'dropped').length
  const type = (key) => (v) => set((s) => ({ catForm: { ...s.catForm, [key]: v } }))
  // 同名的是改那一条，不是加：按保存以前就说是哪条、现在落在哪，也能「取消」不改了。
  const name = S.catForm.name.trim()
  const what = !name ? '' : S.cats.includes(name) ? `将修改已有的分类“${name}”，现在的落点是“${S.lands[name] || '设定库'}”。` : '将添加新分类。'
  const row = (c) => (S.catDrop === c
    ? `<div class="cat-row"><div class="fit">${count(c)} 条设定将移到“其他”</div>${button('取消', () => set({ catDrop: '' }))}${button('确认删除', () => dropCat(c), 'danger')}</div>`
    : `<div class="cat-row"><div class="cat-name">${t(c)}</div><div class="fit muted">落点：${t(S.lands[c] || '设定库')}</div><div class="side">${count(c)} 条</div><div class="hit link"${onClick(() => { set({ catForm: { name: c, land: S.lands[c] || '设定库' } }); document.querySelector('.cat-form').scrollIntoView({ block: 'nearest' }) })}>编辑</div>` +
      (c === '其他' ? '<div class="off">删除</div>' : `<div class="hit link"${onClick(() => (count(c) ? set({ catDrop: c }) : dropCat(c)))}>删除</div>`) + '</div>')
  return '<div class="editor"><div class="paper scroll"><div class="column narrow">' +
    '<div class="page-title">作品分类</div><div class="note">设定文档按分类分节，每条设定只归入一个分类。“落点”指设定成熟后的去处，例如设定库或大纲。</div>' +
    `<div class="cat-rows">${S.cats.map(row).join('')}</div>` +
    '<div class="group-title">添加或修改分类</div>' +
    `<div class="cat-form">${field('名称', line(S.catForm.name, type('name'), '', '分类名称'))}${field('落点', line(S.catForm.land, type('land'), '例如 设定库', '落点', keys(saveCat)))}${S.cats.includes(name) ? button('取消', () => set({ catForm: { name: '', land: '' } })) : ''}${S.catForm.name.trim() && S.catForm.land.trim() ? button('保存分类', saveCat, 'primary') : button('保存分类', null, 'primary off')}</div>` +
    (what ? `<div class="note">${t(what)}</div>` : '') +
    '</div></div></div>'
}

// ───────── 书架 ─────────

// 原型开不了系统的窗口，只说一声本来会开什么。
const osAsk = (what) => () => say('原型不会打开系统窗口', '软件中此步骤为：' + what)

// 添加已有作品：原型里当作每次选的都是同一个文件夹。头一回加上，再点就说已经在了。
function addBook() {
  if (S.shelf.some((b) => b.id === 'found')) { say('该作品已在书架上'); return }
  set((s) => ({ shelf: [...s.shelf, { id: 'found', name: BOOK.found, kind: '出版小说', words: '2.1 万字', at: '尚未打开' }] }))
  say('已加入书架', BOOK.found, 'ok')
}

// 换一本书：外观、书架、设置里的东西带过去，别的照那本书刚打开的样子。
function openBook(id) {
  stopAll()
  set({ ...fresh(id), ...Object.fromEntries(KEPT.map((k) => [k, S[k]])) })
}

function shelfPage() {
  const all = S.shelf
  const open = (b) => onClick(b.gone ? null : () => openBook(b.id))
  const about = (b) => b.kind + ' · ' + b.words
  const more = (b) => `<div class="anchor"><button class="icon-btn muted" aria-label="更多操作" title="更多操作"${onClick(() => set({ open: S.open === 'book-' + b.id ? '' : 'book-' + b.id }))}>${ICON.dots}</button>` +
    (S.open === 'book-' + b.id ? '<div class="pop shelf-pop">' + menuRow('打开所在文件夹', () => { set(SHUT); osAsk('在资源管理器中显示这部作品的文件夹')() }) + menuRow('重命名…', () => set({ dialog: { kind: 'nameBook', id: b.id, name: b.name }, ...SHUT })) + menuRow('从书架移除', () => removeBook(b)) + '</div>' : '') + '</div>'
  const removeBook = (b) => { set((s) => ({ shelf: s.shelf.filter((x) => x.id !== b.id), ...SHUT })); say('已从书架移除，文件夹未改动：' + b.name, '', 'ok') }
  const card = (b) => `<div class="shelf-card${b.gone ? ' gone' : ' hit'}"${open(b)}><div class="shelf-card-top"><div class="shelf-logo">${t(b.name[0])}</div><div class="grow"></div>${b.gone ? '' : more(b)}</div>` +
    `<div class="shelf-name ell">${t(b.name)}</div><div class="note">${t(about(b))}</div><div class="grow"></div>` +
    (b.gone ? `<div class="shelf-gone"><div class="fit danger">找不到文件夹</div><div class="hit link"${onClick(osAsk('选择该作品当前所在的文件夹'))}>定位</div><div class="hit link"${onClick(() => removeBook(b))}>移除</div></div>` : `<div class="faint">${t(b.at)}</div>`) + '</div>'
  const row = (b) => `<div class="shelf-row${b.gone ? ' gone' : ' hit'}"${open(b)}><div class="shelf-logo">${t(b.name[0])}</div><div class="fit"><div class="shelf-row-name"><span class="shelf-name">${t(b.name)}</span><span class="note">${t(about(b))}</span></div><div class="faint ell">${t(S.library + '\\' + b.name)}</div></div>` +
    (b.gone ? `<div class="danger">找不到文件夹</div><div class="hit link"${onClick(osAsk('选择该作品当前所在的文件夹'))}>重新定位…</div><div class="hit link"${onClick(() => removeBook(b))}>从书架移除</div>` : `<div class="faint">${t(b.at)}</div>${more(b)}`) + '</div>'
  const create = button('新建作品', () => set({ dialog: { kind: 'newBook', name: '', type: BOOK.kinds[0] } }), 'primary')
  if (!all.length) {
    // AI 关着时，这一页不讲 AI。
    return '<div class="shelf">' + (aiOn() ? blank('book', '从一个想法开始', '与 AI 逐步讨论，定下的内容当场记进设定文档。已确定的设定会被遵守，已放弃的 AI 不再提及。',
      '<div class="shelf-facts"><div>· 作品、对话和设定文档均为文件夹中的普通文件，可备份、同步或用其他编辑器打开。</div><div>· AI 仅在你发送消息时回应，不会自行续写。</div><div>· API key 保存在系统钥匙串中，不会写入作品文件夹。</div></div>' +
      buttons(button('添加已有作品…', addBook), create))
      : blank('book', '从一个想法开始', '新建一部作品，或添加已有的作品文件夹。',
        '<div class="shelf-facts"><div>· 作品和设定文档均为文件夹中的普通文件，可备份、同步或用其他编辑器打开。</div></div>' + buttons(button('添加已有作品…', addBook), create))) + '</div>'
  }
  return '<div class="shelf scroll"><div class="shelf-column">' +
    `<div class="shelf-head"><div class="fit"><div class="shelf-title">我的作品</div><div class="shelf-sub"><span class="ell">${all.length} 部 · 作品库：${t(S.library)}</span><span class="hit link"${onClick(osAsk('选择作品库文件夹'))}>更改</span></div></div>` +
    `<div class="seg icons" aria-label="显示">${[['列表', 'list', false], ['卡片', 'grid', true]].map(([label, pic, cards]) => `<div class="hit${S.cards === cards ? ' on' : ''}" title="${label}"${onClick(() => set({ cards }))}>${icon(pic)}</div>`).join('')}</div>` +
    button('添加已有作品…', addBook) + create + '</div>' +
    (S.cards ? `<div class="shelf-cards">${all.map(card).join('')}</div>` : `<div class="shelf-rows">${all.map(row).join('')}</div>`) +
    '</div></div>'
}
