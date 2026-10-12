// 各种对话框，和侧栏里「已备份」点开的那一块。
'use strict'

// ───────── 备份 ─────────

// 备份为什么没成功：[短的, 标题, 怎么办]。why 空着的是找不到那块盘。
const BACKUP_FAILS = {
  disk: ['找不到 E 盘', '找不到 E 盘，这次没备份上', '稿子没丢，插上硬盘再重试。'],
  dir: ['找不到备份文件夹', '找不到备份文件夹，这次没备份上', '稿子没丢，文件夹可能被移走或改名。'],
  deny: ['没有权限写入', '备份文件夹不让写，这次没备份上', '稿子没丢，这个文件夹不让写入。'],
}
const backupWhy = () => BACKUP_FAILS[S.backup.why || 'disk']

// 立即备份：还没选过地方，就先问放哪。
function backupNow() {
  if (S.backup.state === 'none') { set({ dialog: { kind: 'backup', place: BOOK.places[0][1] }, ...SHUT }); return }
  set((s) => ({ backup: { ...s.backup, state: 'busy' }, ...SHUT }))
  later(1200, () => set((s) => ({ backup: { ...s.backup, state: 'ok', why: '', at: '刚刚', copies: s.backup.copies + 1 } })))
}
const backupSet = (change) => set((s) => ({ backup: { ...s.backup, ...change } }))
const backupNext = (b) => (b.close ? '关闭作品时' : b.hourly ? '一小时后' : b.daily ? '明天' : '只在点「立即备份」时')
const backupTimes = (b, change) => tickbox(b.close, '关闭作品时', () => change({ close: !b.close })) + tickbox(b.daily, '每天一次（如果当天写过）', () => change({ daily: !b.daily })) + tickbox(b.hourly, '写作时每小时一次', () => change({ hourly: !b.hourly }))

// 侧栏里那一行点开的：备份得怎么样，没成功的话怎么办。
function backupPop() {
  const b = S.backup
  if (b.state === 'none') return '<div class="pop backup-pop" data-k="backup-pop"><div class="pop-title">还没备份过</div><div class="note">稿子现在只存在这台电脑上。</div>' + `<div class="pop-buttons">${button('选一个地方…', backupNow, 'primary')}</div></div>`
  if (b.state === 'fail') {
    return '<div class="pop backup-pop" data-k="backup-pop"><div class="pop-title">' + backupWhy()[1] + '</div>' + `<div class="note">${backupWhy()[2]}${b.at ? '上次备份在' + t(b.at) + '。' : '还没有备份成功过。'}</div>` +
      `<div class="pop-buttons">${button('重试', backupNow, 'primary')}${button('换个地方', osAsk('选备份到哪个文件夹'))}<div class="hit muted"${onClick(() => set((s) => ({ backup: { ...s.backup, hush: true }, ...SHUT })))}>今天不提醒</div></div></div>`
  }
  return `<div class="pop backup-pop" data-k="backup-pop"><div class="pop-title">${t(b.at)} 备份过</div><div class="note">备份在 ${t(b.dir)}<br>下次：${backupNext(b)}</div>` +
    `<div class="pop-buttons">${button(b.state === 'busy' ? '正在备份…' : '立即备份', b.state === 'busy' ? null : backupNow, b.state === 'busy' ? 'primary off' : 'primary')}${button('备份设置…', () => set({ dialog: { kind: 'settings', tab: '备份', edit: '' }, ...SHUT }))}</div></div>`
}

// 第一次备份：问放哪、什么时候备份。
function backupDialog(d) {
  const type = (change) => set((s) => ({ dialog: { ...s.dialog, ...change } }))
  const times = { close: d.close ?? true, daily: d.daily ?? true, hourly: d.hourly ?? false }
  const leave = () => set({ dialog: d.back ? { kind: 'settings', tab: '备份', edit: '' } : null, ...SHUT })
  const start = () => {
    set((s) => ({ backup: { ...s.backup, ...times, state: 'busy', dir: d.place, copies: 0 }, dialog: d.back ? { kind: 'settings', tab: '备份', edit: '' } : null }))
    later(1200, () => backupSet({ state: 'ok', at: '刚刚', copies: 1 }))
  }
  return dialog('small', dialogHead('稿子现在只存在这台电脑上', '选个别的地方，定时多存一份。') +
    `<div class="places">${BOOK.places.map(([label, dir]) => `<div class="place hit${d.place === dir ? ' on' : ''}"${onClick(() => type({ place: dir }))}><div class="ring${d.place === dir ? ' on' : ''}"></div><div class="fit"><div>${t(label)}</div><div class="note ell">在这台电脑上找到了 · ${t(dir)}</div></div></div>`).join('')}` +
    `<div class="place hit"${onClick(osAsk('选备份到哪个文件夹'))}><div class="ring"></div><div class="fit"><div>选别的文件夹…</div><div class="note">移动硬盘、U 盘或别的同步文件夹</div></div></div></div>` +
    `<div class="field"><div class="label">什么时候备份</div><div class="ticks">${backupTimes(times, type)}</div></div>` +
    `<div class="buttons"><div class="hit muted"${onClick(leave)}>以后再说</div><div class="grow"></div>${button('开始备份', d.place ? start : null, d.place ? 'primary' : 'primary off')}</div>`)
}

// ───────── 设置 ─────────

const useAt = (i, change) => set((s) => ({ uses: s.uses.map((u, n) => (n === i ? { ...u, ...change } : u)) }))
const PROTOCOLS = ['OpenAI Chat Completions（多数兼容服务）', 'OpenAI Responses', 'Anthropic Messages']

function saveService() {
  const f = S.dialog.svc, name = f.name.trim()
  if (!name || !f.url.trim()) { say('显示名称和 API 地址都要填'); return }
  const made = { id: f.id.trim() || name.toLowerCase(), name, url: f.url.trim(), host: f.url.trim().replace(/^https?:\/\//, '').split('/')[0], proto: f.proto, env: f.env.trim(), models: f.models.split('\n').map((m) => m.trim()).filter(Boolean) }
  set((s) => ({ services: s.dialog.edit === 'new' ? [...s.services, made] : s.services.map((x) => (x.id === made.id ? made : x)), dialog: { ...s.dialog, edit: '', svc: null } }))
  say('提供商已保存', '', 'ok')
}
function fetchModels() {
  const id = say('正在读取可用模型…', '', 'busy', [['取消', () => { clearTimeout(timer); unsay(id) }]])
  const timer = later(1200, () => { unsay(id); set((s) => (s.dialog && s.dialog.svc ? { dialog: { ...s.dialog, svc: { ...s.dialog.svc, models: (BOOK.services.find((x) => x.id === s.dialog.svc.id) || BOOK.services[0]).models.join('\n') } } } : null)); say('已获取模型目录，保存提供商后生效') })
}

function settingsModel(d) {
  const used = (svc) => S.uses.some((u) => u.service === svc.name)
  const use = (u, i) => {
    const same = u.service === '沿用讨论模型'
    const pickService = (name) => { const svc = S.services.find((x) => x.name === name); useAt(i, { service: name, model: svc ? svc.models[0] : '' }) }
    return `<div class="use-row"><div class="use-name"><div>${t(u.job)}</div><div class="note ell">${t(u.about)}</div></div>` +
      choose('service-' + i, u.service, [...(i ? ['沿用讨论模型'] : []), ...S.services.map((x) => x.name)], pickService, u.job + '用的服务') +
      (same ? '<div class="fit use-same">和讨论用同一个模型</div>' : `<div class="box line fit">${typed(u.model, (v) => useAt(i, { model: v }), '输入或选择模型 ID', u.job + '模型')}</div>`) +
      choose('effort-' + i, u.effort, BOOK.efforts, (e) => useAt(i, { effort: e }), u.job + '的思考程度') + '</div>'
  }
  const type = (key) => (v) => set((s) => ({ dialog: { ...s.dialog, svc: { ...s.dialog.svc, [key]: v } } }))
  const editing = (svc) => ({ edit: svc ? svc.id : 'new', svc: svc ? { id: svc.id, name: svc.name, url: svc.url, proto: svc.proto || PROTOCOLS[0], key: '', env: svc.env, models: svc.models.join('\n') } : { id: '', name: '', url: '', proto: PROTOCOLS[0], key: '', env: '', models: '' } })
  const form = () => {
    const f = d.svc
    return '<div class="service-form">' +
      `<div class="pair id-first">${field('ID（小写字母、数字、- _）', line(f.id, type('id'), '', 'Provider ID（小写）', d.edit === 'new' ? '' : ' disabled'))}${field('显示名称', line(f.name, type('name'), '', '显示名称'))}</div>` +
      field('API 地址', line(f.url, type('url'), 'https://api.example.com/v1', 'API 地址')) +
      (!f.url.trim() ? '' : /^https?:\/\/(localhost|127\.0\.0\.1|\[::1\])([:/]|$)/.test(f.url.trim()) ? '<div class="note">本机地址：请确认这个服务不会再转发到云端</div>' : '<div class="note amber">远程服务：你的讨论内容会发送到这个地址</div>') +
      field('接口协议', choose('proto', f.proto, PROTOCOLS, type('proto'), 'API 协议')) +
      `<div class="pair">${field('API key', line(f.key, type('key'), '留空则保留已存的 key', 'API key', ' type="password"'))}${field('或读取环境变量', line(f.env, type('env'), '例如 DEEPSEEK_API_KEY', '环境变量名称'))}</div>` +
      `<div class="note">key 存入系统钥匙串，不会写进配置文件或作品。${f.env.trim() && BOOK.services.some((x) => x.env === f.env.trim()) ? '已检测到环境变量 ' + t(f.env.trim()) + '。' : ''}</div>` +
      field('可用模型（每行一个；也可以在上方直接输入目录外的模型）', area(f.models, type('models'), '', '模型目录', 3)) +
      `<div class="buttons">${button('获取可用模型', fetchModels)}${d.edit === 'new' ? '' : `<div class="hit danger"${onClick(() => say('已清除保存的 key'))}>清除已保存 key</div>`}<div class="grow"></div>${button('保存提供商', saveService, 'primary')}</div></div>`
  }
  const card = (svc) => `<div class="service"><div class="service-head"><div class="fit"><div class="ell">${t(svc.name)}</div><div class="note ell">${t(svc.host)} · ${svc.models.length} 个模型${used(svc) ? ' · 这部作品在用' : ''}</div></div>` +
    button(d.edit === svc.id ? '收起' : '编辑', () => set((s) => ({ dialog: { ...s.dialog, ...(d.edit === svc.id ? { edit: '', svc: null } : editing(svc)) }, ...SHUT }))) + '</div>' + (d.edit === svc.id ? form() : '') + '</div>'
  // 保存前先看三处模型填全了没有。
  const lacks = () => {
    const [talk, book, pen] = S.uses, alone = (u) => u.service !== '沿用讨论模型' && !u.model.trim()
    if (!talk.model.trim()) return '讨论模型需要同时选择服务和填写模型 ID'
    return alone(book) ? '整理模型选了单独的服务，还需要填写模型 ID' : alone(pen) ? '写作模型选了单独的服务，还需要填写模型 ID' : ''
  }
  const saveBudget = () => (lacks() ? say(lacks(), lacks().startsWith('讨论') ? '' : '或改回“沿用讨论模型”', 'fail') : /^\d+$/.test(S.budget.trim()) ? (Number(S.budget) < 8192 ? say('上下文预算至少为 8192，请确保不超过模型支持的窗口') : say('设置已保存，下一条消息起生效', '', 'ok')) : say('上下文预算需要是整数'))
  return '<div class="prefs-title">模型</div><div class="note">给这部作品选模型。</div>' +
    '<div class="prefs-group"><div class="group-title">这部作品用</div><div class="note">思考程度选 Default 时由服务商决定。</div></div>' +
    `<div class="uses">${S.uses.map(use).join('')}` +
    `<div class="use-row"><div class="use-name">上下文预算</div><div class="box line budget">${typed(S.budget, (v) => set({ budget: v }), '', '上下文预算')}</div><div class="fit note">token，含回答预留</div>${button('保存', saveBudget, 'primary')}</div></div>` +
    '<div class="note">不要超过模型的上下文窗口；放不下的设定会在回复下的“AI 看到了什么”里列出。改动从下一次发送开始生效。</div>' +
    '<div class="prefs-group"><div class="group-title">写作时</div></div>' +
    `<div class="ticks">${tickbox(S.auto, '停笔后自动续写', () => set({ auto: !S.auto }))}<div class="note tick-note">打开后每次停笔都会用一次 AI。</div>${tickbox(S.recap, '改完一章后更新摘要', () => set({ recap: !S.recap }))}<div class="note tick-note">你自己写的摘要不会被覆盖。</div></div>` +
    '<div class="prefs-group"><div class="group-title">模型服务</div><div class="note">存在这台电脑上，所有作品共用</div></div>' +
    `<div class="services">${S.services.map(card).join('')}` +
    (d.edit === 'new' ? `<div class="service">${form()}</div>` : `<div class="service add hit"${onClick(() => set((s) => ({ dialog: { ...s.dialog, ...editing(null) }, ...SHUT })))}>${ICON.plus}添加服务</div>`) + '</div>' +
    `<div class="prefs-links"><div class="hit muted"${onClick(osAsk('用记事本打开配置文件'))}>打开配置文件</div><div class="hit muted"${onClick(() => say('配置已重新读取'))}>重新加载配置</div></div>`
}

function settingsBackup() {
  const b = S.backup
  if (b.state === 'none') {
    return '<div class="prefs-title">备份</div><div class="note">把整部作品多存一份，防电脑坏。</div>' +
      `<div class="backup-card"><div class="fit"><div class="group-title">还没有备份</div><div class="note">稿子现在只存在这台电脑上。</div></div>${button('选一个地方…', () => set({ dialog: { kind: 'backup', place: BOOK.places[0][1], back: true }, ...SHUT }), 'primary')}</div>`
  }
  const title = { ok: b.at + ' 备份过', busy: '正在备份…', fail: '备份没成功' }[b.state]
  const row = (label, inner) => `<div class="prefs-row"><div class="prefs-label">${label}</div>${inner}</div>`
  return '<div class="prefs-title">备份</div><div class="note">把整部作品多存一份，防电脑坏。</div>' +
    `<div class="backup-card"><div class="fit"><div class="group-title">${t(title)}</div><div class="note">${b.state === 'fail' ? backupWhy()[0] + '。' + backupWhy()[2] : `${t(S.shelf.find((x) => x.id === S.book).words)} · ${S.chapters.length} 章 · 已存 ${b.copies} 份${b.copies > 1 ? '，最早 9 月 28 日' : ''}`}</div></div>` +
    button(b.state === 'fail' ? '重试' : '立即备份', b.state === 'busy' ? null : backupNow, b.state === 'busy' ? 'primary off' : 'primary') + '</div>' +
    row('备份到', `<div class="fit ell">${t(b.dir)}</div>${button('换一个', osAsk('选备份到哪个文件夹'))}`) +
    row('什么时候备份', `<div class="ticks">${backupTimes(b, backupSet)}</div>`) +
    row('留几份', `<div class="keep">${choose('keep', b.keep, ['最近 10 份', '最近 30 份', '最近 90 份'], (keep) => backupSet({ keep }), '留几份')}</div><div class="note">每份约 1.1 MB，共 ${(b.copies * 1.1).toFixed(1)} MB</div>`)
}

function settingsDialog(d) {
  const tabs = ['模型', '外观', '提醒与检查', '备份']
  const lateNow = S.marks.filter((m) => m.st === 'open' && BOOK.now - m.ch >= S.late).length
  const when = [['写完一章就查', '推荐，每章用一次 AI'], ['我点了才查', '平时不用 AI'], ['边写边查', '停笔就查，最费额度']]
  const step = (by) => () => set((s) => ({ late: Math.max(5, s.late + by) }))
  const body = d.tab === '模型' ? settingsModel(d) : d.tab === '备份' ? settingsBackup()
    : d.tab === '外观' ? '<div class="prefs-title">外观</div><div class="note">只在这台电脑上生效，所有作品都一样。</div>' +
      `<div class="prefs-row"><div class="prefs-label">颜色</div>${seg(S.look, [['跟随系统', 'screen'], ['浅色', 'sun'], ['深色', 'moon']], (v) => set({ look: v }), '颜色')}</div>`
    : '<div class="prefs-title">提醒与检查</div><div class="note">这部作品的提醒规则，改了立刻生效。</div>' +
      '<div class="prefs-group"><div class="group-title">伏笔</div></div>' +
      `<div class="prefs-row"><div class="fit"><div>多久没回收就提醒</div><div class="note">到时归入「久未回收」，不弹窗。</div></div><div class="stepper"><div class="hit" title="少 5 章"${onClick(step(-5))}>−</div><div class="stepper-n">${S.late} 章</div><div class="hit" title="多 5 章"${onClick(step(5))}>+</div></div></div>` +
      `<div class="note">现在有 ${lateNow} 个伏笔久未回收。</div>` +
      `<div class="prefs-group"><div class="group-title">检查正文和设定</div></div>${radios(S.when, when, (v) => set({ when: v }))}`
  return dialog('prefs', `<div class="prefs-nav"><div class="prefs-nav-title">设置</div>${tabs.map((name) => `<div class="nav hit${d.tab === name ? ' on' : ''}"${onClick(() => set({ dialog: { kind: 'settings', tab: name, edit: '' }, ...SHUT }))}>${name}</div>`).join('')}</div>` +
    `<div class="prefs-body scroll" data-k="prefs-${d.tab}">${body}</div><button class="icon-btn muted prefs-close" aria-label="关闭设置" title="关闭设置"${onClick(shut)}>${icon('cross')}</button>`)
}

// ───────── 正文那一页的几个对话框 ─────────

// 加入设定：选中的字先放进框里，可以改成自己的说法。
// 摘取原文：从一句话里拖选一段，照原话记成设定。
function sourceDialog(d) {
  const read = () => {
    const sel = getSelection(), box = document.querySelector('.source-text')
    const picked = sel && !sel.isCollapsed && box.contains(sel.anchorNode) && box.contains(sel.focusNode) ? sel.toString().trim() : ''
    if (picked !== d.picked) set((s) => ({ dialog: { ...s.dialog, picked } }))
  }
  const add = () => {
    if (!d.picked) return
    set((s) => ({ dialog: null, ...SHUT, n: s.n + 1, entries: [...s.entries, { id: 'e' + s.n, place: 'body', cat: d.cat, names: [], text: d.picked }], log: did(s, '新增到正文 · ' + d.cat + '：' + d.picked) }))
    say('已加入设定正文', '', 'ok')
  }
  return dialog('source', dialogHead('摘取原文', '拖选一段原话，作为你定下的设定加入正文。原话不会被改动。') +
    `<div class="source-text scroll"${onUp(read)}>${t(d.text)}</div>` +
    `<div class="source-line">${choose('source-cat', d.cat, S.cats, (v) => set((s) => ({ dialog: { ...s.dialog, cat: v } })), '摘取分类')}<div class="${d.picked ? 'ink' : 'faint'}">${d.picked ? '已选 ' + d.picked.length + ' 字' : '还没有选中文字'}</div><div class="grow"></div>` +
    `<div class="hit muted"${onClick(shut)}>收起原文</div>${button('加入设定', add, d.picked ? 'primary' : 'primary off')}</div>`)
}

function takeDialog(d) {
  const type = (key) => (v) => set((s) => ({ dialog: { ...s.dialog, [key]: v } }))
  const add = () => {
    const text = d.text.trim()
    if (!text) return
    set((s) => ({ dialog: null, ...SHUT, n: s.n + 1, entries: [...s.entries, { id: 'e' + s.n, place: 'body', cat: d.cat, names: [], text, src: d.src }], log: did(s, '新增到正文 · ' + d.cat + '：' + text) }))
    say('已加入设定正文', '', 'ok')
  }
  return dialog('small', dialogHead('加入设定', '选中的原文作为出处记下来；设定可以换成你自己的说法。') +
    field('设定内容', area(d.text, type('text'), '', '设定内容', 4)) + field('设定分类', choose('take-cat', d.cat, S.cats, type('cat'), '设定分类')) +
    buttons(button('取消', shut), button('加入', add, d.text.trim() ? 'primary' : 'primary off')))
}

function draftDialog(d) {
  const cur = chapterOf(S.ch), [model, effort] = writer()
  const type = (key) => (v) => set((s) => ({ dialog: { ...s.dialog, [key]: v } }))
  const before = S.chapters.slice(0, S.chapters.indexOf(cur)).filter((c) => !S.summaries[c.key] || !S.summaries[c.key].text)
  const had = S.chapters.indexOf(cur) - before.length
  const fill = () => { set((s) => ({ dialog: { ...s.dialog, total: before.length } })); before.forEach((c, i) => later(i * 500, () => recap(c.key))) }
  return dialog('small', dialogHead('起草本章', 'AI 只把章纲写成文字，不加章纲里没有的大事件。起草前存一份快照，写出的文字带“AI 起草”标记，你改过的地方会脱离标记。') +
    `<div class="quote">${t(S.outlines[cur.key])}</div>` +
    field('目标字数（可以不填）', line(d.goal, type('goal'), '由 AI 按章纲判断', '目标字数')) +
    (cur.paras.length ? field('这一章已经有内容', seg(d.how, ['接在已写的后面', '覆盖已写的内容'], type('how'), '已有内容怎么办')) : '') +
    `<div class="note">用 ${t(model)} · 推理 ${t(effort)}；可以在“模型设置”里换。</div>` +
    (before.length && d.total ? `<div class="note">正在补摘要：${d.total - before.length} / ${d.total}。补完之前起草，AI 只看得到已有的摘要。</div>`
      : before.length ? `<div class="warn-line"><div class="fit">前面有 ${before.length} 章还没有摘要，起草时 AI 不知道这些章里发生了什么。</div><div class="hit ink"${onClick(fill)}>补齐摘要（${before.length} 次调用）</div></div>`
      : had > 0 ? `<div class="note">起草时 AI 会读前面 ${had} 章的摘要，接上前情。</div>` : '') +
    buttons(button('取消', shut), button('开始起草', draftNow, 'primary')))
}

// 两份稿子并起来看：old 里有、now 里没有的划掉，now 里新写的标绿。
function compare(old, now) {
  const out = []
  let i = 0, j = 0
  while (i < old.length || j < now.length) {
    if (i < old.length && old[i] === now[j]) { out.push(['', old[i]]); i++; j++ } else if (j < now.length && !old.includes(now[j])) out.push(['add', now[j++]])
    else if (i < old.length) out.push(['cut', old[i++]])
    else out.push(['add', now[j++]])
  }
  return out.map(([cls, text]) => `<div${cls ? ` class="${cls}"` : ''}>${t(text)}</div>`).join('')
}

function snapsDialog(d) {
  const cur = chapterOf(S.ch), list = S.snaps[cur.key] || []
  const shown = list.find((x) => x.id === d.id) || list[0]
  const pick = (change) => () => set((s) => ({ dialog: { ...s.dialog, ...change } }))
  const restore = () => {
    set((s) => ({ dialog: null, ...snap(s, cur.key, '恢复前'), ...retype(s, cur.key, () => shown.paras) }))
    say('已恢复到 ' + shown.at + ' 的版本，恢复前的内容也存了快照')
  }
  const same = shown && shown.paras.join('\n') === cur.paras.join('\n')
  return dialog('wide', dialogHead('快照 · ' + titleOf(cur), '打开章节、连续写作每十分钟、恢复之前、AI 起草或采纳改写之前、全书替换之前都会存一份。最近七天的全部保留，更早的每天留最后一份。') +
    (shown
      ? `<div class="snaps"><div class="snap-list scroll">${list.map((x) => `<div class="snap-row row${x === shown ? ' on' : ''}"${onClick(pick({ id: x.id }))}><div>${t(x.at)}</div><div class="note">${t(x.kind)} · ${x.paras.join('').length} 字</div></div>`).join('')}</div>` +
        `<div class="snap-side"><div class="snap-tools">${tickbox(d.diff, '和现在对比', pick({ diff: !d.diff }))}<div class="fit note ell">${same ? '这一份和现在的正文一样' : d.diff ? '红色划掉的是快照里有、现在没有的，绿色是现在新写的' : ''}</div></div>` +
        (shown.ai ? `<div class="card"><div class="who">${t(shown.ai.head)}</div><div class="extra">${t(shown.ai.saw)}</div>${shown.ai.left ? `<div class="fail">超出上下文预算没放进去的设定 ${shown.ai.left} 条</div>` : ''}</div>` : '') +
        `<div class="snap-text scroll">${d.diff ? compare(shown.paras, cur.paras) : shown.paras.map((p) => `<div>${t(p)}</div>`).join('') || '<div class="faint">这一份是空的</div>'}</div></div></div>`
      : '<div class="empty"><div>这一章还没有快照</div></div>') +
    buttons(button('关闭', shut), button('恢复这个版本', shown && !same ? restore : null, shown && !same ? 'primary' : 'primary off')))
}

function exportDialog(d) {
  const cur = chapterOf(S.ch), book = S.shelf.find((b) => b.id === S.book)
  const ranges = ['全书', ...(cur && cur.vol ? ['本卷'] : []), ...(cur ? ['本章'] : [])]
  const type = (key) => (v) => set((s) => ({ dialog: { ...s.dialog, [key]: v } }))
  const name = d.range === '本章' ? titleOf(cur) : d.range === '本卷' ? S.volumes.find((v) => v[0] === cur.vol)[1] : book.name
  const out = () => { set({ dialog: null, ...SHUT }); say('已导出到 ' + S.library + '\\' + name.replace('：', ' ') + '.txt') }
  return dialog('small', dialogHead('导出正文', '按章节树的顺序拼成一个 .txt，每个文件夹和章节前写上标题。') +
    field('导出范围', seg(d.range, ranges, type('range'), '导出范围')) + tickbox(d.indent, '段首加两个全角空格', () => type('indent')(!d.indent)) +
    buttons(button('取消', shut), button('导出', out, 'primary')))
}

// 全书替换前再问一句。
function swapDialog(d) {
  return dialog('small', dialogHead('全书替换', '把全书 ' + d.chapters + ' 章里的 ' + d.total + ' 处“' + d.q + '”换成' + (d.to ? '“' + d.to + '”' : '空（删掉）') + '？只改正文，设定和章纲不动；每章换之前都存一份快照。') +
    buttons(button('取消', shut), button('确定替换', swapBook, 'primary')))
}

// ───────── 侧栏点出来的几个对话框 ─────────

function binDialog(d) {
  const back = (b) => () => {
    set((s) => {
      const vol = b.volume, home = (c) => (vol || s.volumes.some((v) => v[0] === c.vol) ? c.vol : '')
      return { bin: s.bin.filter((x) => x !== b), volumes: vol ? [...s.volumes, vol] : s.volumes, chapters: [...s.chapters, ...b.chapters.map((c) => ({ ...c, vol: home(c) }))] }
    })
    say('已恢复「' + b.title + '」')
  }
  const row = (b) => `<div class="bin-row"><div class="fit"><div class="ell">${t(b.title)}</div><div class="note">${t(b.at)} 删除${b.volume ? ' · 连同里面的 ' + b.chapters.length + ' 章' : ''}</div></div>` +
    (d.sure === b.id ? button('取消', () => set({ dialog: { kind: 'bin', sure: '' } })) + button('确认彻底删除', () => set((s) => ({ bin: s.bin.filter((x) => x !== b), dialog: { kind: 'bin', sure: '' } })), 'danger')
      : button('恢复', back(b)) + `<div class="hit muted"${onClick(() => set({ dialog: { kind: 'bin', sure: b.id } }))}>彻底删除</div>`) + '</div>'
  return dialog('small', dialogHead('回收站', '删掉的章节连同它里面的内容先放在这里，恢复后回到原来所在文件夹的末尾。') +
    `<div class="bin-rows scroll">${S.bin.length ? S.bin.map(row).join('') : '<div class="empty"><div>回收站是空的</div></div>'}</div>` + buttons(button('关闭', shut)))
}

function keysDialog() {
  return dialog('keys', dialogHead('快捷键') + `<div class="keys scroll">${BOOK.keys.map(([group, of]) => `<div class="head">${group}</div>` + of.map(([key, does]) => `<div class="key-row"><div class="key">${t(key)}</div><div class="fit">${t(does)}</div></div>`).join('')).join('')}</div>` + buttons(button('关闭', shut)))
}

// ───────── 书架上的两个对话框 ─────────

function newBookDialog(d) {
  const type = (key) => (v) => set((s) => ({ dialog: { ...s.dialog, [key]: v } }))
  const name = d.name.trim() || '未命名作品', at = (d.at ?? S.library + '\\' + name).trim()
  const create = () => {
    if (!at) { say('请选择作品放在哪里', '', 'fail'); return }
    if (S.shelf.some((b) => S.library + '\\' + b.name === at)) { say(at + ' 里已经有文件了，换个名字或位置', '', 'fail'); return }
    const id = 'book' + S.n
    set((s) => ({ shelf: [{ id, name, kind: d.type, words: '还没有正文', at: '刚刚' }, ...s.shelf], n: s.n + 1, dialog: null }))
    openBook(id)
  }
  return dialog('small', dialogHead('新建作品') + field('作品名称', line(d.name, type('name'), '可以之后再改', '作品名称', keys(create) + ' autofocus')) + field('体裁', choose('book-kind', d.type, BOOK.kinds, type('type'), '体裁')) +
    `<div class="field"><div class="label">放在</div><div class="pick-line"><div class="box line fit">${typed(d.at ?? S.library + '\\' + name, type('at'), '', '作品位置', keys(create))}</div>${button('更改…', osAsk('新作品放在哪个文件夹里'))}</div><div class="note">会新建这个文件夹；它必须不存在或是空的。</div></div>` +
    buttons(button('取消', shut), button('创建', create, 'primary')))
}

function nameBookDialog(d) {
  const save = () => { const name = d.name.trim(); if (!name) say('作品名不能为空', '', 'fail'); else set((s) => ({ shelf: s.shelf.map((b) => (b.id === d.id ? { ...b, name } : b)), dialog: null })) }
  return dialog('small', dialogHead('重命名作品') + line(d.name, (v) => set((s) => ({ dialog: { ...s.dialog, name: v } })), '', '新作品名', keys(save) + ' autofocus') + '<div class="note">只改作品名，文件夹不动。</div>' + buttons(button('取消', shut), button('保存', save, 'primary')))
}

const DIALOGS = { backup: backupDialog, settings: settingsDialog, take: takeDialog, source: sourceDialog, draft: draftDialog, snaps: snapsDialog, export: exportDialog, swap: swapDialog, bin: binDialog, keys: keysDialog, newBook: newBookDialog, nameBook: nameBookDialog }
const dialogs = () => (S.dialog ? DIALOGS[S.dialog.kind](S.dialog) : '')
