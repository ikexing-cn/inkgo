// 各种对话框，和侧栏里「已备份」点开的那一块。
'use strict'

// ───────── 备份 ─────────

// 备份为什么没成功：[短的, 标题, 怎么办]。why 空着的是找不到那块盘。
const BACKUP_FAILS = {
  disk: ['找不到 E 盘', '找不到 E 盘，本次备份未完成', '作品未受影响，请接入硬盘后重试。'],
  dir: ['找不到备份文件夹', '找不到备份文件夹，本次备份未完成', '作品未受影响，文件夹可能已被移动或改名。'],
  deny: ['没有权限写入', '备份文件夹无法写入，本次备份未完成', '作品未受影响，该文件夹没有写入权限。'],
}
const backupWhy = () => BACKUP_FAILS[S.backup.why || 'disk']

// 立即备份：还没选过地方，就先问放哪。
function backupNow() {
  if (S.backup.state === 'none') { set({ dialog: { kind: 'backup', place: BOOK.places[0][1] }, ...SHUT }); return }
  set((s) => ({ backup: { ...s.backup, state: 'busy' }, ...SHUT }))
  later(1200, () => set((s) => ({ backup: { ...s.backup, state: 'ok', why: '', at: '刚刚', copies: s.backup.copies + 1 } })))
}
const backupSet = (change) => set((s) => ({ backup: { ...s.backup, ...change } }))
const backupNext = (b) => (b.close ? '关闭作品时' : b.hourly ? '一小时后' : b.daily ? '明天' : '仅在点击「立即备份」时')
const backupTimes = (b, change) => tickbox(b.close, '关闭作品时', () => change({ close: !b.close })) + tickbox(b.daily, '每天一次（当天有写作时）', () => change({ daily: !b.daily })) + tickbox(b.hourly, '写作时每小时一次', () => change({ hourly: !b.hourly }))

// 侧栏里那一行点开的：备份得怎么样，没成功的话怎么办。
function backupPop() {
  const b = S.backup
  if (b.state === 'none') return '<div class="pop backup-pop" data-k="backup-pop"><div class="pop-title">尚未备份</div><div class="note">作品目前仅保存在本机。</div>' + `<div class="pop-buttons">${button('选择位置…', backupNow, 'primary')}</div></div>`
  if (b.state === 'fail') {
    return '<div class="pop backup-pop" data-k="backup-pop"><div class="pop-title">' + backupWhy()[1] + '</div>' + `<div class="note">${backupWhy()[2]}${b.at ? '上次备份于' + t(b.at) + '。' : '尚无成功的备份。'}</div>` +
      `<div class="pop-buttons"><div class="hit link"${onClick(() => set((s) => ({ backup: { ...s.backup, hush: true }, ...SHUT })))}>今日不再提醒</div>${button('更换位置', osAsk('选择备份文件夹'))}${button('重试', backupNow, 'primary')}</div></div>`
  }
  return `<div class="pop backup-pop" data-k="backup-pop"><div class="pop-title">${t(b.at)} 已备份</div><div class="note">备份至 ${t(b.dir)}<br>下次：${backupNext(b)}</div>` +
    `<div class="pop-buttons">${button('备份设置…', () => set({ dialog: { kind: 'settings', tab: '备份', edit: '' }, ...SHUT }))}${button(b.state === 'busy' ? '正在备份…' : '立即备份', b.state === 'busy' ? null : backupNow, b.state === 'busy' ? 'primary off' : 'primary')}</div></div>`
}

// 第一次备份：问放哪、备份时机。
function backupDialog(d) {
  const type = (change) => set((s) => ({ dialog: { ...s.dialog, ...change } }))
  const times = { close: d.close ?? true, daily: d.daily ?? true, hourly: d.hourly ?? false }
  const leave = () => set({ dialog: d.back ? { kind: 'settings', tab: '备份', edit: '' } : null, ...SHUT })
  const start = () => {
    set((s) => ({ backup: { ...s.backup, ...times, state: 'busy', dir: d.place, copies: 0 }, dialog: d.back ? { kind: 'settings', tab: '备份', edit: '' } : null }))
    later(1200, () => backupSet({ state: 'ok', at: '刚刚', copies: 1 }))
  }
  return dialog('small', dialogHead('作品目前仅保存在本机', '选择其他位置，定期保存副本。') +
    `<div class="places">${BOOK.places.map(([label, dir]) => `<div class="place hit${d.place === dir ? ' on' : ''}"${onClick(() => type({ place: dir }))}><div class="ring${d.place === dir ? ' on' : ''}"></div><div class="fit"><div>${t(label)}</div><div class="note ell">在本机检测到 · ${t(dir)}</div></div></div>`).join('')}` +
    `<div class="place hit"${onClick(osAsk('选择备份文件夹'))}><div class="ring"></div><div class="fit"><div>选择其他文件夹…</div><div class="note">移动硬盘、U 盘或其他同步文件夹</div></div></div></div>` +
    `<div class="field"><div class="label">备份时机</div><div class="ticks">${backupTimes(times, type)}</div></div>` +
    `<div class="buttons"><div class="hit link"${onClick(leave)}>稍后再说</div><div class="grow"></div>${button('开始备份', d.place ? start : null, d.place ? 'primary' : 'primary off')}</div>`)
}

// ───────── 设置 ─────────

const useAt = (i, change) => set((s) => ({ uses: s.uses.map((u, n) => (n === i ? { ...u, ...change } : u)) }))
const PROTOCOLS = ['OpenAI Chat Completions（多数兼容服务）', 'OpenAI Responses', 'Anthropic Messages']

function saveService() {
  const f = S.dialog.svc, name = f.name.trim()
  if (!name || !f.url.trim()) { say('显示名称和 API 地址为必填项'); return }
  const models = f.models.filter((m) => m.id.trim()).map((m) => ({ ...m, id: m.id.trim() }))
  // 上下文上限和最大输出是正整数（可写 128K、1M），最大输出小于上下文上限；压缩阈值是整数，在一半到九成五之间。
  const wrong = [
    [(m) => !count(m.max) || !count(m.out), '的上下文上限和最大输出须为正整数，可写成 128K、1M'],
    [(m) => !/^\d+$/.test(m.at.trim()), '的压缩阈值须为整数'],
    [(m) => count(m.out) >= count(m.max), '的最大输出须小于上下文上限'],
    [(m) => Number(m.at) < 50 || Number(m.at) > 95, '的压缩阈值须在 50% 到 95% 之间'],
  ].map(([bad, why]) => [models.find(bad), why]).find(([m]) => m)
  if (wrong) { say(wrong[0].id + wrong[1], '', 'fail'); return }
  const made = { id: f.id.trim() || name.toLowerCase(), name, url: f.url.trim(), host: f.url.trim().replace(/^https?:\/\//, '').split('/')[0], proto: f.proto, env: f.env.trim(), models: models.map((m) => ({ ...m, max: size(count(m.max)), out: size(count(m.out)), at: m.at.trim() })) }
  set((s) => ({ services: s.dialog.edit === 'new' ? [...s.services, made] : s.services.map((x) => (x.id === made.id ? made : x)), dialog: { ...s.dialog, edit: '', svc: null } }))
  say('提供商已保存', '', 'ok')
}
function fetchModels() {
  const id = say('正在读取可用模型…', '', 'busy', [['取消', () => { clearTimeout(timer); unsay(id) }]])
  const timer = aiLater(1200, () => { unsay(id); set((s) => (s.dialog && s.dialog.svc ? { dialog: { ...s.dialog, svc: { ...s.dialog.svc, models: (BOOK.services.find((x) => x.id === s.dialog.svc.id) || BOOK.services[0]).models.map(limits) } } } : null)); say('已获取模型目录，保存提供商后生效') })
}

// 这部作品累计用了多少 token，按内置价格估算花费；表里没有价格的模型不估。
function spent() {
  const used = S.book === BOOK.shelf[0].id ? BOOK.spent : []
  const cost = (id, input, cached, output) => { const p = (BOOK.table[id] || [])[2]; return p ? '约 ' + p[3] + (((input - cached) * p[0] + cached * p[1] + output * p[2]) / 1e6).toFixed(2) : '无价格' }
  const row = ([id, input, cached, output]) => `<div class="spent-row"><div class="ell">${t(id)}</div><div class="fit ell">输入 ${tokens(input)}，缓存命中 ${Math.round((cached / input) * 100)}% · 输出 ${tokens(output)}</div><div>${cost(id, input, cached, output)}</div></div>`
  return '<div class="prefs-group"><div class="group-title">本作品用量</div><div class="note">按内置价格估算，以服务商账单为准。</div></div>' +
    (used.length ? `<div class="spent">${used.map(row).join('')}</div>` : '<div class="note">尚未使用 AI。</div>')
}

function settingsModel(d) {
  const used = (svc) => S.page !== 'shelf' && S.uses.some((u) => u.service === svc.name) // 书架上没有打开的作品
  const use = (u, i) => {
    const same = u.service === '沿用讨论模型'
    const pickService = (name) => { const svc = S.services.find((x) => x.name === name); useAt(i, { service: name, model: svc && svc.models[0] ? svc.models[0].id : '' }) }
    return `<div class="use-row"><div class="use-name"><div>${t(u.job)}</div><div class="note ell">${t(u.about)}</div></div>` +
      choose('service-' + i, u.service, [...(i ? ['沿用讨论模型'] : []), ...S.services.map((x) => x.name)], pickService, u.job + '使用的服务') +
      (same ? '<div class="fit use-same">与讨论使用同一模型</div>' : `<div class="box line fit">${typed(u.model, (v) => useAt(i, { model: v }), '输入或选择模型 ID', u.job + '模型')}</div>`) +
      choose('effort-' + i, u.effort, BOOK.efforts, (e) => useAt(i, { effort: e }), u.job + '的思考程度') + '</div>'
  }
  const type = (key) => (v) => set((s) => ({ dialog: { ...s.dialog, svc: { ...s.dialog.svc, [key]: v } } }))
  // 改服务里的模型。ID 改成内置表里有的，三个数照表重填。
  const models = (change) => set((s) => ({ dialog: { ...s.dialog, svc: { ...s.dialog.svc, models: change(s.dialog.svc.models) } } }))
  const typeModel = (i, key) => (v) => models((list) => list.map((m, n) => (n !== i ? m : key === 'id' && BOOK.table[v.trim()] ? { ...limits(v.trim()), id: v } : { ...m, [key]: v })))
  const editing = (svc) => ({ edit: svc ? svc.id : 'new', svc: svc ? { id: svc.id, name: svc.name, url: svc.url, proto: svc.proto || PROTOCOLS[0], key: '', env: svc.env, models: svc.models.map((m) => ({ ...m })) } : { id: '', name: '', url: '', proto: PROTOCOLS[0], key: '', env: '', models: [] } })
  const limitRow = (m, i) => `<div class="limit-row"><div class="box line fit">${typed(m.id, typeModel(i, 'id'), '模型 ID', '模型 ID')}</div>` +
    `<div class="box line num">${typed(m.max, typeModel(i, 'max'), '', '上下文上限')}</div><div class="box line num">${typed(m.out, typeModel(i, 'out'), '', '最大输出')}</div><div class="box line pct">${typed(m.at, typeModel(i, 'at'), '', '压缩阈值')}</div>` +
    `<button class="icon-btn muted" aria-label="移除模型" title="移除模型"${onClick(() => models((list) => list.filter((_, n) => n !== i)))}>${icon('cross', 'sm')}</button></div>`
  const form = () => {
    const f = d.svc
    return '<div class="service-form">' +
      `<div class="pair id-first">${field('ID（小写字母、数字、- _）', line(f.id, type('id'), '', 'Provider ID（小写）', d.edit === 'new' ? '' : ' disabled'))}${field('显示名称', line(f.name, type('name'), '', '显示名称'))}</div>` +
      field('API 地址', line(f.url, type('url'), 'https://api.example.com/v1', 'API 地址')) +
      (!f.url.trim() ? '' : /^https?:\/\/(localhost|127\.0\.0\.1|\[::1\])([:/]|$)/.test(f.url.trim()) ? '<div class="note">本机地址：请确认该服务不会转发到云端</div>' : '<div class="note amber">远程服务：讨论内容将发送到该地址</div>') +
      field('接口协议', choose('proto', f.proto, PROTOCOLS, type('proto'), 'API 协议')) +
      `<div class="pair">${field('API key', line(f.key, type('key'), '留空则保留已存的 key', 'API key', ' type="password"'))}${field('或读取环境变量', line(f.env, type('env'), '例如 DEEPSEEK_API_KEY', '环境变量名称'))}</div>` +
      `<div class="note">key 保存在系统钥匙串中，不会写入配置文件或作品。${f.env.trim() && BOOK.services.some((x) => x.env === f.env.trim()) ? '已检测到环境变量 ' + t(f.env.trim()) + '。' : ''}</div>` +
      '<div class="field"><div class="label">可用模型</div><div class="limits">' +
      (f.models.length ? '<div class="limit-row limit-head"><div class="fit">模型 ID</div><div class="num">上下文上限</div><div class="num">最大输出</div><div class="pct">压缩阈值 %</div><div class="gap"></div></div>' + f.models.map(limitRow).join('') : '<div class="note">尚无模型，可获取可用模型或手动添加。</div>') + '</div></div>' +
      '<div class="note">数值默认取自内置模型表，表中没有的模型按保守值填写；上下文上限和最大输出可写成 128K、1M。讨论超过压缩阈值时，较早的对话会整理成摘要。</div>' +
      `<div class="buttons">${d.edit === 'new' ? '' : `<div class="hit link danger"${onClick(() => say('已清除保存的 key'))}>清除已保存 key</div>`}${button('添加模型', () => models((list) => [...list, limits('')]))}${button('获取可用模型', fetchModels)}${button('保存提供商', saveService, 'primary')}</div></div>`
  }
  const card = (svc) => `<div class="service"><div class="service-head"><div class="fit"><div class="ell">${t(svc.name)}</div><div class="note ell">${t(svc.host)} · ${svc.models.length} 个模型${used(svc) ? ' · 本作品使用中' : ''}</div></div>` +
    button(d.edit === svc.id ? '收起' : '编辑', () => set((s) => ({ dialog: { ...s.dialog, ...(d.edit === svc.id ? { edit: '', svc: null } : editing(svc)) }, ...SHUT }))) + '</div>' + (d.edit === svc.id ? form() : '') + '</div>'
  return (S.page === 'shelf' ? '' : '<div class="prefs-group"><div class="group-title">本作品使用</div><div class="note">思考程度选 Default 时由服务商决定。修改自下次发送起生效。</div></div>' +
    `<div class="uses">${S.uses.map(use).join('')}</div>` + spent() +
    '<div class="prefs-group"><div class="group-title">写作时</div></div>' +
    `<div class="ticks">${tickbox(S.auto, '停笔后自动续写', () => set({ auto: !S.auto }))}<div class="note tick-note">开启后每次停笔都会调用一次 AI。</div>${tickbox(S.recap, '章节修改后更新摘要', () => set({ recap: !S.recap }))}<div class="note tick-note">手动填写的摘要不会被覆盖。</div></div>`) +
    '<div class="prefs-group"><div class="group-title">模型服务</div><div class="note">保存在本机，所有作品共用</div></div>' +
    `<div class="services">${S.services.map(card).join('')}` +
    (d.edit === 'new' ? `<div class="service">${form()}</div>` : `<div class="service add hit"${onClick(() => set((s) => ({ dialog: { ...s.dialog, ...editing(null) }, ...SHUT })))}>${ICON.plus}添加服务</div>`) + '</div>' +
    `<div class="prefs-links"><div class="hit link"${onClick(osAsk('用记事本打开配置文件'))}>打开配置文件</div><div class="hit link"${onClick(() => say('配置已重新读取'))}>重新加载配置</div></div>`
}

function settingsBackup() {
  const b = S.backup
  if (b.state === 'none') {
    return `<div class="backup-card"><div class="fit"><div class="group-title">尚未备份</div><div class="note">作品目前仅保存在本机。</div></div>${button('选择位置…', () => set({ dialog: { kind: 'backup', place: BOOK.places[0][1], back: true }, ...SHUT }), 'primary')}</div>`
  }
  const title = { ok: b.at + ' 已备份', busy: '正在备份…', fail: '备份失败' }[b.state]
  const row = (label, inner) => `<div class="prefs-row"><div class="prefs-label">${label}</div>${inner}</div>`
  return `<div class="backup-card"><div class="fit"><div class="group-title">${t(title)}</div><div class="note">${b.state === 'fail' ? backupWhy()[0] + '。' + backupWhy()[2] : `${t(S.shelf.find((x) => x.id === S.book).words)} · ${S.chapters.length} 章 · 已存 ${b.copies} 份${b.copies > 1 ? '，最早 9 月 28 日' : ''}`}</div></div>` +
    button(b.state === 'fail' ? '重试' : '立即备份', b.state === 'busy' ? null : backupNow, b.state === 'busy' ? 'primary off' : 'primary') + '</div>' +
    row('备份到', `<div class="fit ell">${t(b.dir)}</div>${button('更换', osAsk('选择备份文件夹'))}`) +
    row('备份时机', `<div class="ticks">${backupTimes(b, backupSet)}</div>`) +
    row('保留份数', `<div class="keep">${choose('keep', b.keep, ['最近 10 份', '最近 30 份', '最近 90 份'], (keep) => backupSet({ keep }), '保留份数')}</div><div class="note">每份约 1.1 MB，共 ${(b.copies * 1.1).toFixed(1)} MB</div>`)
}

// 「AI」那一部分的最上面：一个开关，管本机的所有作品。点了不马上变，先问一句。
const aiSwitch = () => `<div class="ticks">${tickbox(S.noAI, '关闭 AI', () => set((s) => ({ dialog: { ...s.dialog, ask: s.noAI ? 'on' : 'off' }, ...SHUT })))}` +
  '<div class="note tick-note">隐藏全部 AI 功能，不发送任何内容。</div></div>'

// 关闭、开启 AI 前问的那一句，浮在设置上面。
function askAI(ask) {
  const off = ask === 'off', back = () => set((s) => ({ dialog: { ...s.dialog, ask: '' } }))
  const about = off ? '隐藏全部 AI 功能，不再发送任何内容；正在进行的 AI 任务会停止。讨论、摘要和检查结果不会删除，重新开启后恢复。'
    : '重新显示 AI 功能。自动摘要和检查会按设置把章节内容发给模型服务；未配置模型服务时不发送任何内容。'
  return `<div class="scrim top"${onClick(back)}></div><div class="dialog-wrap top"><div class="dialog ask" data-k="dialog-ask">` +
    dialogHead(off ? '是否关闭 AI？' : '是否开启 AI？', about) + buttons(button('取消', back), button(off ? '关闭 AI' : '开启 AI', () => turnAI(!off), 'primary')) + '</div></div>'
}

// 关闭 AI：正在做的 AI 任务停下。没采纳的起草、改写丢掉；没回完的那句讨论不要了，作者的话还在，和点「取消请求」一样。
// 正开着的讨论、检查两页回到正文，右栏的讨论回到设定，标题下面的摘要收起。讨论、摘要、检查结果都还在，重新开启后照旧。
// 提示条一并收起：上面说的多半是 AI 的事，有的还能点「重试」。
function turnAI(on) {
  if (!on && S.waiting) stopReply()
  set((s) => (on ? { noAI: false, dialog: { ...s.dialog, ask: '' } } : {
    noAI: true, dialog: { ...s.dialog, ask: '' }, toasts: [], ai: null, checking: false, queue: [],
    summaries: Object.fromEntries(Object.entries(s.summaries).map(([key, sum]) => [key, sum.busy ? { ...sum, busy: false } : sum])),
    page: s.page === 'talk' || s.page === 'check' ? 'write' : s.page, sv: s.sv === 'ideas' ? 'list' : s.sv, box: s.box === '摘要' ? '' : s.box,
  }))
}

function settingsDialog(d) {
  // 书架上没有打开的作品，只有本机的两部分。AI 关着时，「提醒与检查」里只剩提醒。
  const remind = aiOn() ? '提醒与检查' : '提醒'
  const tabs = S.page === 'shelf' ? ['AI', '外观'] : ['AI', '外观', remind, '备份']
  // 每一类的标题和下面那句说明留在上面不动，只有内容滚动。
  const about = { AI: aiOn() ? '是否使用 AI，以及使用哪些模型。' : '仅在本机生效，适用于所有作品。', 外观: '仅在本机生效，适用于所有作品。', [remind]: '本作品的提醒规则，修改后立即生效。', 备份: '为整部作品另存一份副本，以防设备故障。' }
  const lateNow = S.marks.filter((m) => m.st === 'open' && BOOK.now - m.ch >= S.late).length
  const when = [['写完一章后检查', '推荐，每章调用一次 AI'], ['手动检查', '平时不调用 AI'], ['边写边检查', '停笔即检查，调用最多']]
  const step = (by) => () => set((s) => ({ late: Math.max(5, s.late + by) }))
  const body = d.tab === 'AI' ? aiSwitch() + (aiOn() ? settingsModel(d) : '') : d.tab === '备份' ? settingsBackup()
    : d.tab === '外观' ? `<div class="prefs-row"><div class="prefs-label">颜色</div>${seg(S.look, [['跟随系统', 'screen'], ['浅色', 'sun'], ['深色', 'moon']], (v) => set({ look: v }), '颜色')}</div>`
    : '<div class="prefs-group"><div class="group-title">伏笔</div></div>' +
      `<div class="prefs-row"><div class="fit"><div>超过多少章未回收时提醒</div><div class="note">到期后归入「久未回收」，不弹窗。</div></div><div class="stepper"><div class="hit" title="减少 5 章"${onClick(step(-5))}>−</div><div class="stepper-n">${S.late} 章</div><div class="hit" title="增加 5 章"${onClick(step(5))}>+</div></div></div>` +
      `<div class="note">当前有 ${lateNow} 个伏笔久未回收。</div>` +
      (aiOn() ? `<div class="prefs-group"><div class="group-title">检查正文和设定</div></div>${radios(S.when, when, (v) => set({ when: v }))}` : '')
  return dialog('prefs', `<div class="prefs-nav"><div class="prefs-nav-title">设置</div>${tabs.map((name) => `<div class="nav hit${d.tab === name ? ' on' : ''}"${onClick(() => set({ dialog: { kind: 'settings', tab: name, edit: '' }, ...SHUT }))}>${name}</div>`).join('')}</div>` +
    `<div class="prefs-main"><div class="prefs-head"><div class="prefs-title">${d.tab}</div><div class="note">${about[d.tab]}</div></div><div class="prefs-body scroll" data-k="prefs-${d.tab}">${body}</div></div><button class="icon-btn muted prefs-close" aria-label="关闭设置" title="关闭设置"${onClick(shut)}>${icon('cross')}</button>`) + (d.ask ? askAI(d.ask) : '')
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
  return dialog('source', dialogHead('摘取原文', '拖选一段原文，作为已确定的设定加入正文。原文不会被修改。') +
    `<div class="source-text scroll"${onUp(read)}>${t(d.text)}</div>` +
    `<div class="source-line">${choose('source-cat', d.cat, S.cats, (v) => set((s) => ({ dialog: { ...s.dialog, cat: v } })), '摘取分类')}<div class="${d.picked ? 'ink' : 'faint'}">${d.picked ? '已选 ' + d.picked.length + ' 字' : '尚未选中文字'}</div><div class="grow"></div>` +
    `<div class="hit link"${onClick(shut)}>收起原文</div>${button('加入设定', add, d.picked ? 'primary' : 'primary off')}</div>`)
}

function takeDialog(d) {
  const type = (key) => (v) => set((s) => ({ dialog: { ...s.dialog, [key]: v } }))
  const add = () => {
    const text = d.text.trim()
    if (!text) return
    set((s) => ({ dialog: null, ...SHUT, n: s.n + 1, entries: [...s.entries, { id: 'e' + s.n, place: 'body', cat: d.cat, names: [], text, src: d.src }], log: did(s, '新增到正文 · ' + d.cat + '：' + text) }))
    say('已加入设定正文', '', 'ok')
  }
  return dialog('small', dialogHead('加入设定', '选中的原文将记为出处；设定内容可改用自己的表述。') +
    field('设定内容', area(d.text, type('text'), '', '设定内容', 4)) + field('设定分类', choose('take-cat', d.cat, S.cats, type('cat'), '设定分类')) +
    buttons(button('取消', shut), button('加入', add, d.text.trim() ? 'primary' : 'primary off')))
}

function draftDialog(d) {
  const cur = chapterOf(S.ch), [model, effort] = writer()
  const type = (key) => (v) => set((s) => ({ dialog: { ...s.dialog, [key]: v } }))
  const before = S.chapters.slice(0, S.chapters.indexOf(cur)).filter((c) => !S.summaries[c.key] || !S.summaries[c.key].text)
  const had = S.chapters.indexOf(cur) - before.length
  const fill = () => { set((s) => ({ dialog: { ...s.dialog, total: before.length } })); before.forEach((c, i) => later(i * 500, () => recap(c.key))) }
  return dialog('small', dialogHead('起草本章', 'AI 仅将章纲扩写为正文，不添加章纲之外的重大情节。写完先供你查看，采纳后才写入正文，写入前保存快照；写入的文字带有“AI 起草”标记，修改过的部分不再带标记。') +
    `<div class="quote">${t(S.outlines[cur.key])}</div>` +
    field('目标字数（选填）', line(d.goal, type('goal'), '由 AI 按章纲判断', '目标字数')) +
    (cur.paras.length ? field('本章已有内容', seg(d.how, ['接在已有内容之后', '覆盖已有内容'], type('how'), '已有内容的处理方式')) : '') +
    `<div class="note">用 ${t(model)} · 推理 ${t(effort)}；可在设置中「AI」配置里更换。</div>` +
    (before.length && d.total ? `<div class="note">正在补齐摘要：${d.total - before.length} / ${d.total}。补齐前起草，AI 只能参考已有摘要。</div>`
      : before.length ? `<div class="warn-line"><div class="fit">前文有 ${before.length} 章尚无摘要，起草时 AI 无法了解这些章节的内容。</div><div class="hit link"${onClick(fill)}>补齐摘要（${before.length} 次调用）</div></div>`
      : had > 0 ? `<div class="note">起草时 AI 会参考前 ${had} 章的摘要，以衔接前文。</div>` : '') +
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

// 快照是在什么之前存的。AI 关着时，「AI 改动前」只写「改动前」：快照还在，不提 AI。
const snapKind = (kind) => (kind === 'AI 改动前' && !aiOn() ? '改动前' : kind)

function snapsDialog(d) {
  const cur = chapterOf(S.ch), list = S.snaps[cur.key] || []
  const shown = list.find((x) => x.id === d.id) || list[0]
  const pick = (change) => () => set((s) => ({ dialog: { ...s.dialog, ...change } }))
  const restore = () => {
    set((s) => ({ dialog: null, ...snap(s, cur.key, '恢复前'), ...retype(s, cur.key, () => shown.paras) }))
    say('已恢复到 ' + shown.at + ' 的版本，恢复前的内容已存为快照')
  }
  const same = shown && shown.paras.join('\n') === cur.paras.join('\n')
  return dialog('wide', dialogHead('快照 · ' + titleOf(cur), '打开章节时、连续写作每十分钟、恢复前、' + (aiOn() ? '采纳 AI 起草或改写前、' : '') + '全书替换前都会保存一份。最近七天的全部保留，更早的每天保留最后一份。') +
    (shown
      ? `<div class="snaps"><div class="snap-list scroll">${list.map((x) => `<div class="snap-row row${x === shown ? ' on' : ''}"${onClick(pick({ id: x.id }))}><div>${t(x.at)}</div><div class="note">${t(snapKind(x.kind))} · ${x.paras.join('').length} 字</div></div>`).join('')}</div>` +
        `<div class="snap-side"><div class="snap-tools">${tickbox(d.diff, '与当前对比', pick({ diff: !d.diff }))}<div class="fit note ell">${same ? '该快照与当前正文相同' : d.diff ? '红色删除线为快照中有而当前没有的内容，绿色为当前新增的内容' : ''}</div></div>` +
        (shown.ai && aiOn() ? `<div class="card"><div class="who">${t(shown.ai.head)}</div><div class="extra">${t(shown.ai.saw)}</div></div>` : '') +
        `<div class="snap-text scroll">${d.diff ? compare(shown.paras, cur.paras) : shown.paras.map((p) => `<div>${t(p)}</div>`).join('') || '<div class="faint">该快照为空</div>'}</div></div></div>`
      : '<div class="empty"><div>本章暂无快照</div></div>') +
    buttons(button('关闭', shut), button('恢复这个版本', shown && !same ? restore : null, shown && !same ? 'primary' : 'primary off')))
}

function exportDialog(d) {
  const cur = chapterOf(S.ch), book = S.shelf.find((b) => b.id === S.book)
  const ranges = ['全书', ...(cur && cur.vol ? ['本卷'] : []), ...(cur ? ['本章'] : [])]
  const type = (key) => (v) => set((s) => ({ dialog: { ...s.dialog, [key]: v } }))
  const name = d.range === '本章' ? titleOf(cur) : d.range === '本卷' ? S.volumes.find((v) => v[0] === cur.vol)[1] : book.name
  const out = () => { set({ dialog: null, ...SHUT }); say('已导出到 ' + S.library + '\\' + name.replace('：', ' ') + '.txt') }
  return dialog('small', dialogHead('导出正文', '按章节树顺序合并为一个 .txt 文件，各文件夹和章节前加上标题。') +
    field('导出范围', seg(d.range, ranges, type('range'), '导出范围')) + tickbox(d.indent, '段首加两个全角空格', () => type('indent')(!d.indent)) +
    buttons(button('取消', shut), button('导出', out, 'primary')))
}

// 全书替换前再问一句。
function swapDialog(d) {
  return dialog('ask', dialogHead('全书替换', '将全书 ' + d.chapters + ' 章中的 ' + d.total + ' 处“' + d.q + '”替换为' + (d.to ? '“' + d.to + '”' : '空（删除）') + '？仅修改正文，设定和章纲不变；各章替换前均保存快照。') +
    buttons(button('取消', shut), button('确定替换', swapBook, 'primary')))
}

// 关标签页时那一章还有没存的字：存了再关、不存就关（回到存着的那一份）、或者不关。
// 三个按钮照系统问「要不要保存」的顺序排：保存、不保存、取消。
function unsavedDialog(d) {
  const drop = () => set((s) => (!s.saved && s.kept && s.kept.key === s.ch ? { ...retext(s, s.kept.paras), dirty: s.kept.dirty, summaries: s.kept.summaries } : null))
  const then = (first) => () => { first(); set({ saved: true, kept: null, dialog: null }); shutTabs(d.gone, true) }
  return dialog('ask', dialogHead('保存对「' + titleOf(chapterOf(S.ch)) + '」的修改？', '不保存将丢失上次保存后的修改。') +
    `<div class="buttons asked">${button('保存', then(() => {}), 'primary')}${button('不保存', then(drop))}${button('取消', shut)}</div>`)
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
      : button('恢复', back(b)) + `<div class="hit link"${onClick(() => set({ dialog: { kind: 'bin', sure: b.id } }))}>彻底删除</div>`) + '</div>'
  return dialog('small', dialogHead('回收站', '已删除的章节及其内容暂存于此，恢复后回到原文件夹末尾。') +
    `<div class="bin-rows scroll">${S.bin.length ? S.bin.map(row).join('') : '<div class="empty"><div>回收站为空</div></div>'}</div>` + buttons(button('关闭', shut)))
}

// 第三项写着 ai 的，是 AI 的那几组、几行：AI 关着时不列。
function keysDialog() {
  const shown = (list) => list.filter(([, , ai]) => !ai || aiOn())
  return dialog('keys', dialogHead('快捷键') + `<div class="keys scroll">${shown(BOOK.keys).map(([group, of]) => `<div class="head">${group}</div>` + shown(of).map(([key, does]) => `<div class="key-row"><div class="key">${t(key)}</div><div class="fit">${t(does)}</div></div>`).join('')).join('')}</div>` + buttons(button('关闭', shut)))
}

// ───────── 书架上的两个对话框 ─────────

function newBookDialog(d) {
  const type = (key) => (v) => set((s) => ({ dialog: { ...s.dialog, [key]: v } }))
  const name = d.name.trim() || '未命名作品', at = (d.at ?? S.library + '\\' + name).trim()
  const create = () => {
    if (!at) { say('请选择作品位置', '', 'fail'); return }
    if (S.shelf.some((b) => S.library + '\\' + b.name === at)) { say(at + ' 中已有文件，请更换名称或位置', '', 'fail'); return }
    const id = 'book' + S.n
    set((s) => ({ shelf: [{ id, name, kind: d.type, words: '暂无正文', at: '刚刚' }, ...s.shelf], n: s.n + 1, dialog: null }))
    openBook(id)
  }
  return dialog('small', dialogHead('新建作品') + field('作品名称', line(d.name, type('name'), '之后可修改', '作品名称', keys(create) + ' autofocus')) + field('体裁', choose('book-kind', d.type, BOOK.kinds, type('type'), '体裁')) +
    `<div class="field"><div class="label">位置</div><div class="pick-line"><div class="box line fit">${typed(d.at ?? S.library + '\\' + name, type('at'), '', '作品位置', keys(create))}</div>${button('更改…', osAsk('选择新作品所在的文件夹'))}</div><div class="note">将新建该文件夹；该文件夹须不存在或为空。</div></div>` +
    buttons(button('取消', shut), button('创建', create, 'primary')))
}

function nameBookDialog(d) {
  const save = () => { const name = d.name.trim(); if (!name) say('作品名不能为空', '', 'fail'); else set((s) => ({ shelf: s.shelf.map((b) => (b.id === d.id ? { ...b, name } : b)), dialog: null })) }
  return dialog('small', dialogHead('重命名作品') + line(d.name, (v) => set((s) => ({ dialog: { ...s.dialog, name: v } })), '', '新作品名', keys(save) + ' autofocus') + '<div class="note">仅修改作品名，文件夹不变。</div>' + buttons(button('取消', shut), button('保存', save, 'primary')))
}

const DIALOGS = { backup: backupDialog, settings: settingsDialog, take: takeDialog, source: sourceDialog, draft: draftDialog, snaps: snapsDialog, export: exportDialog, swap: swapDialog, unsaved: unsavedDialog, bin: binDialog, keys: keysDialog, newBook: newBookDialog, nameBook: nameBookDialog }
const dialogs = () => (S.dialog ? DIALOGS[S.dialog.kind](S.dialog) : '')
