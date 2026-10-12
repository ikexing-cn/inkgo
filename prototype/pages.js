// 正文以外的几页：讨论、检查、作品分类，和书架。
'use strict'

// ───────── 讨论 ─────────

function talkPage() {
  const talk = S.tv === 'one' ? S.talks.find((x) => x.id === S.tid) : null
  const starts = ['我想写一个总在下雨的城市', '主角有个说不出口的秘密', '这个故事的冲突在哪']
  return '<div class="editor">' +
    `<div class="toolbar${S.paneOpen ? '' : ' wide'}"><div class="crumb">总讨论</div>${icon('right', 'xs', 'faint')}<div class="crumb here ell">${t(talk ? talk.title : '新对话')}</div><div class="grow"></div>` +
    (talk ? `<div class="count-all">${talk.msgs.length} 条发言</div>` : '') +
    (S.paneOpen ? '' : `<button class="icon-btn muted" aria-label="展开设定" title="展开设定文档"${onClick(() => set({ paneOpen: true }))}>${svg('icon-md', PANEL + path('M15 5v14'))}</button>`) + '</div>' +
    (talk
      ? `<div class="talk big scroll" id="talk-page">${said(talk)}</div>`
      : '<div class="paper">' + blank('spark', '从一个模糊的念头聊起', '写下脑子里的那一点想法。讨论会在后台整理成右边的设定文档，你定下的会成为之后讨论的依据，放下的不再被提起。',
        `<div class="starts">${starts.map((words) => `<div class="hit"${onClick(() => send(words))}>${words}</div>`).join('')}</div>`) + '</div>') +
    '<div class="composer big" data-k="composer-page"><div class="box send-box">' +
    `<textarea class="in" rows="2" placeholder="说说你的想法，或问 AI 一个问题…" aria-label="讨论输入"${onInput((v) => set({ draft: v }))}${onKey((ev) => { if (ev.key === 'Enter' && ev.ctrlKey) send() })}>${t(S.draft)}</textarea>` +
    `<div class="send-line">${modelPick()}<div class="grow"></div><div class="faint">Ctrl+Enter</div><div class="send hit${S.draft.trim() && !S.waiting ? '' : ' off'}" title="发送"${onClick(() => send())}>${ICON.send}</div></div>` +
    '</div></div></div>'
}

// ───────── 检查 ─────────

const KINDS = ['全部', '对不上', '名字写错', '新出现']

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
    const ch = chapterOf(c.key), at = ch ? chapterName(ch) : '已删的章'
    if (c.kind === '对不上') { const e = S.entries.find((x) => x.id === c.entry); return at + ' ↔ 设定「' + (e ? e.cat + ' · ' + (e.names[0] || clip(e.text, 8)) : '已删除') + '」' }
    return c.kind === '名字写错' ? at + ' · 应为「' + c.right + '」' : c.cat + ' · ' + at + '起出现 ' + c.times + ' 次'
  }
  const row = (c) => `<div class="check-row row${shown && c.id === shown.id ? ' on' : ''}"${onClick(() => set({ cid: c.id }))}><div class="fit"><div class="ell">${t(c.title)}</div><div class="note ell">${t(about(c))}</div></div><div class="side">${c.kind}</div></div>`
  // 「检查全书」：先数出该查几章，问一句再开始。
  const owed = due()
  const again = () => {
    if (!S.entries.some((e) => e.place === 'body')) say('设定还是空的，没有可以对照的')
    else if (!owed.length) say('各章都查过了，正文没有再改')
    else set({ open: S.open === 'checkAll' ? '' : 'checkAll' })
  }
  const ask = () => `<div class="pop ask-pop" data-k="ask-pop"><div class="pop-title">有 ${owed.length} 章没查过或改过</div><div class="note">每章用一次 AI，一共 ${owed.length} 次。</div>` +
    `<div class="pop-buttons"><div class="grow"></div>${button('取消', () => set(SHUT))}${button('开始检查', () => { set(SHUT); checkNow(...owed.map((c) => c.key)) }, 'primary')}</div></div>`
  const last = S.chapters.filter((c) => !S.never[c.key] && counted(c) >= 200).at(-1)
  const running = chapterOf(S.checking), lost = S.chapters.find((c) => S.failed[c.key])
  const none = !S.entries.some((e) => e.place === 'body') ? ['设定还是空的', '先在讨论里把设定定下来，才有可以对照的。']
    : S.checks.length ? ['这一类没有要看的', ''] : !last ? ['还没检查过', '点左上的「检查全书」，或者写完一章自动检查。']
    : ['没有对不上的地方', S.when === '我点了才查' ? '现在是「我点了才查」，要查时点「再查一次」。' : '写完一章会自动检查，结果列在这里。']
  if (lost && !S.checking) none[1] = chapterName(lost) + '没查成：' + S.failed[lost.key]

  let detail = `<div class="check-detail">${blank('checked', S.checking ? '正在检查…' : none[0], S.checking ? '查完的结果会列在左边。' : none[1])}</div>`
  if (shown) {
    const ch = chapterOf(shown.key), e = S.entries.find((x) => x.id === shown.entry)
    const text = ch ? ch.paras[shown.para] || '' : ''
    const lit = (words, part) => words.split(part).map(t).join(`<span class="lit">${t(part)}</span>`)
    const seeText = ch ? goTo(ch.key, { key: ch.key, quote: shown.quote }) : null
    const seeEntry = () => set({ page: 'write', paneOpen: true, focus: false, tab: '设定', sv: 'list', sid: e.id, form: null })
    const next = () => set({ cid: (of[of.indexOf(shown) + 1] || of[0]).id })
    const right = shown.kind === '对不上' && e ? `<div class="fit"><div class="check-from"><div class="fit ell">设定 · ${t(e.cat)} · ${t(e.names[0] || '')}</div><div class="hit ink"${onClick(seeEntry)}>看设定</div></div><div class="check-card">${lit(e.text, shown.says)}</div></div>`
      : shown.kind === '名字写错' ? `<div class="fit"><div class="check-from"><div class="fit ell">设定里的名字</div></div><div class="check-card"><span class="lit">${t(shown.right)}</span></div></div>`
      : `<div class="fit"><div class="check-from"><div class="fit ell">可以加入设定 · ${t(shown.cat)}</div></div><div class="check-card"><span class="lit">${t(shown.title)}</span>：${t(shown.as)}</div></div>`
    const add = () => { set((s) => ({ ...settle(shown.id)(s), entries: [...s.entries, { id: 'e' + s.n, place: 'body', cat: shown.cat, names: [shown.title], text: shown.as, src: { key: shown.key, quote: shown.quote } }], n: s.n + 1, log: did(s, '新增到正文 · ' + shown.cat + '：' + shown.as) })); say('已加入设定', '', 'ok') }
    const acts = shown.kind === '新出现' ? button('不用记', () => set(settle(shown.id))) + button('加入设定', add, 'primary')
      : button('是故意的', () => set(settle(shown.id))) + (shown.kind === '对不上' && e ? button('改设定', () => set({ page: 'write', paneOpen: true, focus: false, tab: '设定', sv: 'list', sid: e.id, form: editing(e) })) : '') + button('改正文', seeText, 'primary')
    detail = '<div class="check-detail" data-k="check-detail">' +
      `<div class="check-title"><div class="fit ell">${t(shown.title)}</div><div class="side">${of.indexOf(shown) + 1} / ${of.length}</div><div class="hit muted${of.length > 1 ? '' : ' off'}"${onClick(of.length > 1 ? next : null)}>下一条</div></div>` +
      `<div class="check-sides"><div class="fit"><div class="check-from"><div class="fit ell">正文 · ${ch ? t(chapterName(ch)) : ''} 第 ${shown.para + 1} 段</div><div class="hit ink"${onClick(seeText)}>看原文</div></div><div class="check-card">${lit(text, shown.quote)}</div></div>${right}</div>` +
      (shown.also ? '<div class="check-also">别的章也有类似的地方</div>' + shown.also.map((a) => `<div class="check-more"><div class="side">${t(a.at)}</div><div class="fit">${t(a.text)}</div><div class="side">${t(a.note)}</div></div>`).join('') : '') +
      `<div class="grow"></div><div class="check-foot"><div class="hit muted${of.length > 1 ? '' : ' off'}"${onClick(of.length > 1 ? next : null)}>以后再说</div><div class="grow"></div>${acts}</div></div>`
  }
  return '<div class="check-list">' +
    `<div class="check-head"><div class="page-title">检查</div><div class="grow"></div>` +
    (running ? `<div class="muted">正在查${t(chapterName(running))}${S.queue.length ? '，还有 ' + S.queue.length + ' 章' : ''}</div><div class="hit ink"${onClick(() => set({ checking: false, queue: [] }))}>停下</div>`
      : `<div class="muted">${last ? '查到' + t(chapterName(last)) : ''}</div><div class="anchor"><div class="hit ink"${onClick(again)}>${last ? '再查一次' : '检查全书'}</div>${S.open === 'checkAll' ? ask() : ''}</div>`) + '</div>' +
    seg(S.kind + ' ' + count(S.kind), KINDS.map((k) => k + ' ' + count(k)), (v) => set({ kind: v.split(' ')[0], cid: null }), '检查分类') +
    `<div class="check-rows scroll">${of.map(row).join('')}</div></div>` + detail
}

// ───────── 作品分类 ─────────

function saveCat() {
  const name = S.catForm.name.trim(), land = S.catForm.land.trim()
  if (!name) { say('分类名称不能为空'); return }
  if (!land) { say('请填写落点，比如“设定库”或“大纲”'); return }
  set((s) => ({ cats: s.cats.includes(name) ? s.cats : [...s.cats.filter((c) => c !== '其他'), name, '其他'], lands: { ...s.lands, [name]: land }, catForm: { name: '', land: '' } }))
  say('分类“' + name + '”已保存')
}
function dropCat(name) {
  set((s) => ({ cats: s.cats.filter((c) => c !== name), entries: s.entries.map((e) => (e.cat === name ? { ...e, cat: '其他' } : e)), catDrop: '' }))
  say('已删除分类“' + name + '”')
}

function catsPage() {
  const count = (c) => S.entries.filter((e) => e.cat === c && e.place !== 'dropped').length
  const type = (key) => (v) => set((s) => ({ catForm: { ...s.catForm, [key]: v } }))
  const row = (c) => (S.catDrop === c
    ? `<div class="cat-row"><div class="fit">${count(c)} 条设定将移到“其他”</div>${button('取消', () => set({ catDrop: '' }))}${button('确认删除', () => dropCat(c), 'danger')}</div>`
    : `<div class="cat-row"><div class="cat-name">${t(c)}</div><div class="fit muted">落点：${t(S.lands[c] || '设定库')}</div><div class="side">${count(c)} 条</div><div class="hit ink"${onClick(() => set({ catForm: { name: c, land: S.lands[c] || '设定库' } }))}>编辑</div>` +
      (c === '其他' ? '<div class="off">删除</div>' : `<div class="hit muted"${onClick(() => set({ catDrop: c }))}>删除</div>`) + '</div>')
  return '<div class="editor"><div class="paper scroll"><div class="column narrow">' +
    '<div class="page-title">作品分类</div><div class="note">设定文档按分类分节，每条设定只归入一个分类。“落点”是它成熟后该去的地方，比如设定库或大纲。</div>' +
    `<div class="cat-rows">${S.cats.map(row).join('')}</div>` +
    '<div class="group-title">添加或修改分类</div>' +
    `<div class="cat-form">${field('名称（同名即修改）', line(S.catForm.name, type('name'), '', '分类名称'))}${field('落点', line(S.catForm.land, type('land'), '例如 设定库', '落点', keys(saveCat)))}${button('保存分类', saveCat, 'primary')}</div>` +
    '</div></div></div>'
}

// ───────── 书架 ─────────

// 原型开不了系统的窗口，只说一声本来会开什么。
const osAsk = (what) => () => say('原型里不会打开系统的窗口', '软件里这一步是：' + what)

// 添加已有作品：原型里当作每次选的都是同一个文件夹。头一回加上，再点就说已经在了。
function addBook() {
  if (S.shelf.some((b) => b.id === 'found')) { say('这部作品已经在书架上了'); return }
  set((s) => ({ shelf: [...s.shelf, { id: 'found', name: BOOK.found, kind: '出版小说', words: '2.1 万字', at: '还没打开过' }] }))
  say('已加入书架', BOOK.found)
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
  const removeBook = (b) => { set((s) => ({ shelf: s.shelf.filter((x) => x.id !== b.id), ...SHUT })); say('已从书架移除，文件夹没有动：' + b.name) }
  const card = (b) => `<div class="shelf-card${b.gone ? ' gone' : ' hit'}"${open(b)}><div class="shelf-card-top"><div class="shelf-logo">${t(b.name[0])}</div><div class="grow"></div>${b.gone ? '' : more(b)}</div>` +
    `<div class="shelf-name ell">${t(b.name)}</div><div class="note">${t(about(b))}</div><div class="grow"></div>` +
    (b.gone ? `<div class="shelf-gone"><div class="fit danger">找不到文件夹</div><div class="hit ink"${onClick(osAsk('选这部作品现在在哪个文件夹'))}>定位</div><div class="hit muted"${onClick(() => removeBook(b))}>移除</div></div>` : `<div class="faint">${t(b.at)}</div>`) + '</div>'
  const row = (b) => `<div class="shelf-row${b.gone ? ' gone' : ' hit'}"${open(b)}><div class="shelf-logo">${t(b.name[0])}</div><div class="fit"><div class="shelf-row-name"><span class="shelf-name">${t(b.name)}</span><span class="note">${t(about(b))}</span></div><div class="faint ell">${t(S.library + '\\' + b.name)}</div></div>` +
    (b.gone ? `<div class="danger">找不到文件夹</div><div class="hit ink"${onClick(osAsk('选这部作品现在在哪个文件夹'))}>重新定位…</div><div class="hit muted"${onClick(() => removeBook(b))}>从书架移除</div>` : `<div class="faint">${t(b.at)}</div>${more(b)}`) + '</div>'
  const create = button('新建作品', () => set({ dialog: { kind: 'newBook', name: '', type: BOOK.kinds[0] } }), 'primary')
  if (!all.length) {
    return '<div class="shelf">' + blank('book', '从一个念头开始', '和 AI 慢慢聊，讨论会在后台整理成设定文档。你定下的会被遵守，放下的 AI 不再提起。',
      '<div class="shelf-facts"><div>· 作品、对话和设定文档都是你文件夹里的普通文件，可以备份、同步或用别的编辑器打开。</div><div>· AI 只在你发送时回应，不会自己往下写。</div><div>· API key 存在系统钥匙串里，不会写进作品文件夹。</div></div>' +
      buttons(button('添加已有作品…', addBook), create)) + '</div>'
  }
  return '<div class="shelf scroll"><div class="shelf-column">' +
    `<div class="shelf-head"><div class="fit"><div class="shelf-title">我的作品</div><div class="shelf-sub"><span class="ell">${all.length} 部 · 作品库：${t(S.library)}</span><span class="hit ink"${onClick(osAsk('选择作品库文件夹'))}>更改</span></div></div>` +
    `<div class="seg icons" aria-label="显示">${[['列表', 'list', false], ['卡片', 'grid', true]].map(([label, pic, cards]) => `<div class="hit${S.cards === cards ? ' on' : ''}" title="${label}"${onClick(() => set({ cards }))}>${icon(pic)}</div>`).join('')}</div>` +
    button('添加已有作品…', addBook) + create + '</div>' +
    (S.cards ? `<div class="shelf-cards">${all.map(card).join('')}</div>` : `<div class="shelf-rows">${all.map(row).join('')}</div>`) +
    '</div></div>'
}
