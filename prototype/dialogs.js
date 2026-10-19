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
  later(1200, () => set((s) => ({ backup: { ...s.backup, state: 'ok', why: '', at: stamp(), copies: s.backup.copies + 1 } })))
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
    later(1200, () => backupSet({ state: 'ok', at: stamp(), copies: 1 }))
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

// 名称里的拉丁字母小写，从第一个字母起留数字、- 和 _，空格当 -：新服务没填 ID 时用它。
const idFrom = (name) => name.toLowerCase().replace(/ +/g, '-').replace(/[^a-z0-9_-]/g, '').replace(/^[^a-z]+/, '').replace(/[-_]+$/, '')

// 服务表单里哪一栏保存不了、为什么，写在那一栏下面；空着的那栏不算错，只让按钮等。
// 上下文上限和最大输出是正整数（可写 128K、1M），最大输出小于上下文上限；压缩阈值是整数，在一半到九成五之间。
function serviceProblems(d) {
  const f = d.svc, fresh = d.edit === 'new', id = f.id.trim() || (fresh ? idFrom(f.name) : '')
  const models = f.models.filter((m) => m.id.trim()).map((m) => ({ ...m, id: m.id.trim() }))
  const wrong = [
    [(m) => !count(m.max) || !count(m.out), (m) => m.id + ' 的上下文上限和最大输出须为正整数，可写成 128K、1M'],
    [(m) => !/^\d+$/.test(m.at.trim()), (m) => m.id + ' 的压缩阈值须为整数'],
    [(m) => count(m.out) >= count(m.max), (m) => m.id + ' 的最大输出须小于上下文上限'],
    [(m) => Number(m.at) < 50 || Number(m.at) > 95, (m) => m.id + ' 的压缩阈值须在 50% 到 95% 之间'],
    [(m) => models.filter((x) => x.id === m.id).length > 1, (m) => '模型 ' + m.id + ' 重复'],
  ].map(([bad, why]) => [models.find(bad), why]).find(([m]) => m)
  const taken = fresh && S.services.find((x) => x.id === id)
  const p = {
    id, list: models,
    ID: !id ? '' : taken ? '该 ID 已被「' + taken.name + '」使用，请换一个' : /^[a-z][a-z0-9_-]*$/.test(id) ? '' : '提供商 ID 只能用小写字母、数字、- 和 _，并以字母开头',
    url: !f.url.trim() ? '' : !/^https?:\/\//.test(f.url.trim()) ? 'API 地址须以 http:// 或 https:// 开头' : !/^https?:\/\/[^/?#@\s]/.test(f.url.trim()) ? 'API 地址里没有域名'
      : /^https?:\/\/[^/?#@\s]+(\/[^?#\s]*)?$/.test(f.url.trim()) ? '' : 'API 地址不能带账号、查询参数或锚点',
    env: !f.env.trim() || /^[A-Za-z_]\w*$/.test(f.env.trim()) ? '' : '环境变量名只能用字母、数字和 _，不能以数字开头',
    models: wrong ? wrong[1](wrong[0]) : '',
  }
  p.fits = !!(id && f.name.trim() && f.url.trim()) && !p.ID && !p.url && !p.env && !p.models
  return p
}

function saveService() {
  const f = S.dialog.svc, name = f.name.trim(), p = serviceProblems(S.dialog)
  if (!p.fits) return
  const made = { id: p.id, name, url: f.url.trim(), host: f.url.trim().replace(/^https?:\/\//, '').split('/')[0], proto: f.proto, env: f.env.trim(), models: p.list.map((m) => ({ ...m, max: size(count(m.max)), out: size(count(m.out)), at: m.at.trim() })) }
  set((s) => ({ services: s.dialog.edit === 'new' ? [...s.services, made] : s.services.map((x) => (x.id === made.id ? made : x)), dialog: { ...s.dialog, edit: '', svc: null } }))
  say('提供商已保存', '', 'ok')
}
function fetchModels() {
  const id = say('正在读取可用模型…', '', 'busy', [['取消', () => { clearTimeout(timer); unsay(id) }]])
  const timer = aiLater(1200, () => { unsay(id); set((s) => (s.dialog && s.dialog.svc ? { dialog: { ...s.dialog, svc: { ...s.dialog.svc, models: (BOOK.services.find((x) => x.id === s.dialog.svc.id) || BOOK.services[0]).models.map(limits) } } } : null)); say('已获取模型目录，保存提供商后生效', '', 'ok') })
}

// 这部作品用了多少：按功能各一行，请求数、token、按内置价格估的花费；表里没有价格的模型不估。
// 每次请求都在 AI 记录里有一行，点「查看全部记录」一行一行看；只能看，不能删。
const FEATURES = ['讨论', '起草', '改写', '续写', '检查']
const spentOn = (rows) => rows.reduce((n, r) => { const p = (BOOK.table[r[3]] || [])[2], [input, cached, output] = r[6]; return n + (p ? ((input - cached) * p[0] + cached * p[1] + output * p[2]) / 1e6 : 0) }, 0)
function spent() {
  const all = S.records
  const row = (feature) => {
    const rows = all.filter((r) => r[1] === feature), sum = (i) => rows.reduce((n, r) => n + r[6][i], 0)
    return `<div class="spent-row"><div>${feature}</div><div class="fit ell">${rows.length} 次请求 · 输入 ${tokens(sum(0))}，缓存命中 ${Math.round((sum(1) / sum(0)) * 100)}% · 输出 ${tokens(sum(2))}</div><div>约 $${spentOn(rows).toFixed(2)}</div></div>`
  }
  const used = FEATURES.filter((f) => all.some((r) => r[1] === f))
  return '<div class="prefs-group"><div class="group-title">本作品用量</div><div class="note">按内置价格估算，以服务商账单为准。</div></div>' +
    (used.length ? `<div class="spent">${used.map(row).join('')}</div><div class="prefs-links"><div class="hit link"${onClick(() => set((s) => ({ dialog: { ...s.dialog, records: true, only: '全部', row: -1 }, ...SHUT })))}>查看全部记录（${all.length} 次请求）</div></div>` : '<div class="note">尚未使用 AI。</div>')
}
// AI 记录：每次请求一行，新的在上，失败的、取消的也在。点一行看这次发了哪几部分、服务商算的实际用量。不存发出去的字。
function records(d) {
  const shown = S.records.filter((r) => d.only === '全部' || r[1] === d.only)
  const pick = (change) => () => set((s) => ({ dialog: { ...s.dialog, ...change } }))
  const line = (r, i) => {
    const [at, feature, where, model, effort, parts, [input, cached, output], end] = r, open = d.row === i, cost = price(model, [r[6]])
    return `<div class="record${open ? ' on' : ''}"><div class="record-row hit"${onClick(pick({ row: open ? -1 : i }))}><div class="record-at">${t(at)}</div><div class="fit ell">${t(feature + ' · ' + where)}</div>` +
      `<div class="record-n">${tokens(input + output)}</div><div class="record-cost">${t(cost || '—')}</div><div class="record-end ell${end === '完成' ? '' : end === '已取消' ? ' muted' : ' danger'}">${t(end)}</div>${icon(open ? 'down' : 'right', 'xs')}</div>` +
      (open ? `<div class="record-more"><div>${t(model + ' · ' + effort)}</div><div>发送：${parts.map(([name, x]) => t(name) + ' ' + tokens(x)).join(' · ')}</div><div>实际用量：输入 ${tokens(input)}${cached ? '，其中缓存命中 ' + tokens(cached) : ''}；输出 ${tokens(output)}</div></div>` : '') + '</div>'
  }
  return `<div class="records-head"><div class="hit link"${onClick(pick({ records: false }))}>‹ 本作品用量</div><div class="grow"></div>${seg(d.only, ['全部', ...FEATURES], (only) => set((s) => ({ dialog: { ...s.dialog, only, row: -1 } })), '按功能筛选')}</div>` +
    `<div class="records">${shown.length ? shown.map((r) => line(r, S.records.indexOf(r))).join('') : '<div class="note">没有这类请求。</div>'}</div>`
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
    const f = d.svc, p = serviceProblems(d), red = (why) => (why ? `<div class="note danger">${t(why)}</div>` : '')
    return '<div class="service-form">' +
      `<div class="pair id-first">${field('ID（小写字母、数字、- _）', line(f.id, type('id'), d.edit === 'new' ? p.id : '', 'Provider ID（小写）', d.edit === 'new' ? '' : ' disabled'))}${field('显示名称', line(f.name, type('name'), '', '显示名称'))}</div>` + red(p.ID) +
      field('API 地址', line(f.url, type('url'), 'https://api.example.com/v1', 'API 地址')) +
      (!f.url.trim() ? '' : p.url ? red(p.url) : /^https?:\/\/(localhost|127\.0\.0\.1|\[::1\])([:/]|$)/.test(f.url.trim()) ? '<div class="note">本机地址：请确认该服务不会转发到云端</div>' : '<div class="note amber">远程服务：讨论内容将发送到该地址</div>') +
      field('接口协议', choose('proto', f.proto, PROTOCOLS, type('proto'), 'API 协议')) +
      `<div class="pair">${field('API key', line(f.key, type('key'), d.edit === 'new' ? '留空则不用 key' : '留空则保留已存的 key', 'API key', ' type="password"'))}${field('或读取环境变量', line(f.env, type('env'), '例如 DEEPSEEK_API_KEY', '环境变量名称') + red(p.env))}</div>` +
      `<div class="note">key 保存在系统钥匙串中，不会写入配置文件或作品。${f.env.trim() && BOOK.services.some((x) => x.env === f.env.trim()) ? '已检测到环境变量 ' + t(f.env.trim()) + '。' : ''}</div>` +
      '<div class="field"><div class="label">可用模型</div><div class="limits">' +
      (f.models.length ? '<div class="limit-row limit-head"><div class="fit">模型 ID</div><div class="num">上下文上限</div><div class="num">最大输出</div><div class="pct">压缩阈值 %</div><div class="gap"></div></div>' + f.models.map(limitRow).join('') : '<div class="note">尚无模型，可获取可用模型或手动添加。</div>') + '</div>' + red(p.models) + '</div>' +
      '<div class="note">数值默认取自内置模型表，表中没有的模型按保守值填写；上下文上限和最大输出可写成 128K、1M。讨论超过压缩阈值时，较早的对话会整理成摘要。</div>' +
      `<div class="buttons">${d.edit === 'new' ? button('取消', () => set((s) => ({ dialog: { ...s.dialog, edit: '', svc: null } }))) : `<div class="hit link danger"${onClick(() => say('已清除保存的 key', '', 'ok'))}>清除已保存 key</div>`}${button('添加模型', () => models((list) => [...list, limits('')]))}${button('获取可用模型', p.fits ? fetchModels : null, p.fits ? '' : 'off')}${button('保存提供商', p.fits ? saveService : null, p.fits ? 'primary' : 'primary off')}</div></div>`
  }
  // 刚打开的表单连同它那张卡片滚进视野：新加的在列表最下面，常常在看得见的地方下面。
  const open = (change, id) => { set((s) => ({ dialog: { ...s.dialog, ...change(s) }, ...SHUT })); if (id) document.querySelector(`[data-reveal="${id}"]`).scrollIntoView({ block: 'nearest' }) }
  const card = (svc) => `<div class="service" data-reveal="${t(svc.id)}"><div class="service-head"><div class="fit"><div class="ell">${t(svc.name)}</div><div class="note ell">${t(svc.host)} · ${svc.models.length} 个模型${used(svc) ? ' · 本作品使用中' : ''}</div></div>` +
    button(d.edit === svc.id ? '收起' : '编辑', () => (d.edit === svc.id ? open(() => ({ edit: '', svc: null })) : open(() => editing(svc), svc.id))) + '</div>' + (d.edit === svc.id ? form() : '') + '</div>'
  return (S.page === 'shelf' ? '' : '<div class="prefs-group"><div class="group-title">本作品的模型</div><div class="note">思考程度选 Default 时由服务商决定。修改自下次发送起生效。</div></div>' +
    `<div class="uses">${S.uses.map(use).join('')}</div>` + spent() +
    '<div class="prefs-group"><div class="group-title">写作时</div></div>' +
    `<div class="ticks">${tickbox(S.auto, '停笔后自动续写', () => set({ auto: !S.auto }))}<div class="note tick-note">开启后每次停笔都会调用一次 AI。</div></div>`) +
    '<div class="prefs-group"><div class="group-title">模型服务</div><div class="note">保存在本机，所有作品共用</div></div>' +
    `<div class="services">${S.services.map(card).join('')}` +
    (d.edit === 'new' ? `<div class="service" data-reveal="new">${form()}</div>` : `<div class="service add hit"${onClick(() => open(() => editing(null), 'new'))}>${ICON.plus}添加服务</div>`) + '</div>' +
    `<div class="prefs-links"><div class="hit link"${onClick(osAsk('用记事本打开配置文件'))}>打开配置文件</div><div class="hit link"${onClick(() => say('配置已重新读取', '', 'ok'))}>重新加载配置</div></div>`
}

function settingsBackup() {
  const b = S.backup
  if (b.state === 'none') {
    return `<div class="backup-card"><div class="fit"><div class="group-title">尚未备份</div><div class="note">作品目前仅保存在本机。</div></div>${button('选择位置…', () => set({ dialog: { kind: 'backup', place: BOOK.places[0][1], back: true }, ...SHUT }), 'primary')}</div>`
  }
  const title = { ok: b.at + ' 已备份', busy: '正在备份…', fail: '备份失败' }[b.state]
  const row = (label, inner) => `<div class="prefs-row"><div class="prefs-label">${label}</div>${inner}</div>`
  return `<div class="backup-card"><div class="fit"><div class="group-title">${t(title)}</div><div class="note">${b.state === 'fail' ? backupWhy()[0] + '。' + backupWhy()[2] : `${t(S.shelf.find((x) => x.id === S.book).words)} · ${S.chapters.length} 章 · 已存 ${b.copies} 份${b.copies > 1 ? '，最早 09-28' : ''}`}</div></div>` +
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
  const about = off ? '隐藏全部 AI 功能，不再发送任何内容；正在进行的 AI 任务会停止，没有处理的 AI 提议会丢弃。讨论、检查结果和 AI 记录不会删除，重新开启后恢复。'
    : '重新显示 AI 功能。检查会按设置把章节内容发给模型服务；未配置模型服务时不发送任何内容。'
  return `<div class="scrim top"${onClick(back)}></div><div class="dialog-wrap top"><div class="dialog ask" data-k="dialog-ask">` +
    dialogHead(off ? '是否关闭 AI？' : '是否开启 AI？', about) + buttons(button('取消', back), button(off ? '关闭 AI' : '开启 AI', () => turnAI(!off), 'primary')) + '</div></div>'
}

// 关闭 AI：正在做的 AI 任务停下。没采纳的提议丢掉；没回完的那句讨论不要了，作者的话还在，和点「取消请求」一样。
// 正开着的讨论、检查两页回到正文，右栏的讨论回到设定。讨论、检查结果、AI 记录都还在，重新开启后照旧：
// 关的时候停在讨论页、检查页的，开的时候还在正文页就回到那一页。
// 提示条一并收起：上面说的多半是 AI 的事，有的还能点「重试」。
function turnAI(on) {
  if (!on && S.waiting) stopReply()
  set((s) => (on ? { noAI: false, dialog: { ...s.dialog, ask: '' }, offFrom: '', page: s.offFrom && s.page === 'write' ? s.offFrom : s.page } : {
    noAI: true, dialog: { ...s.dialog, ask: '' }, toasts: [], ai: null, checking: false, queue: [], ...ruled(s, s.props.map((p) => p.id), '关闭 AI 时已丢弃'),
    page: s.page === 'talk' || s.page === 'check' ? 'write' : s.page, offFrom: s.page === 'talk' || s.page === 'check' ? s.page : '', sv: s.sv === 'ideas' ? 'list' : s.sv,
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
  const rec = d.tab === 'AI' && d.records && aiOn()
  const body = d.tab === 'AI' ? (rec ? records(d) : aiSwitch() + (aiOn() ? settingsModel(d) : '')) : d.tab === '备份' ? settingsBackup()
    : d.tab === '外观' ? `<div class="prefs-row"><div class="prefs-label">颜色</div>${seg(S.look, [['跟随系统', 'screen'], ['浅色', 'sun'], ['深色', 'moon']], (v) => set({ look: v }), '颜色')}</div>` +
      `<div class="prefs-row">${tickbox(S.names, '正文中标出设定的名字', () => set({ names: !S.names }))}</div><div class="note">名字下面一道虚线，点击在右栏查看这条设定。</div>`
    : '<div class="prefs-group"><div class="group-title">伏笔</div></div>' +
      `<div class="prefs-row"><div class="fit"><div>超过多少章未回收时提醒</div><div class="note">到期后归入「久未回收」，不弹窗。</div></div><div class="stepper"><div class="hit" title="减少 5 章"${onClick(step(-5))}>−</div><div class="stepper-n">${S.late} 章</div><div class="hit" title="增加 5 章"${onClick(step(5))}>+</div></div></div>` +
      `<div class="note">当前有 ${lateNow} 个伏笔久未回收。</div>` +
      (aiOn() ? `<div class="prefs-group"><div class="group-title">检查正文和设定</div></div>${radios(S.when, when, (v) => set({ when: v }))}` : '')
  return dialog('prefs', `<div class="prefs-nav"><div class="prefs-nav-title">设置</div>${tabs.map((name) => `<div class="nav hit${d.tab === name ? ' on' : ''}"${onClick(() => set({ dialog: { kind: 'settings', tab: name, edit: '' }, ...SHUT }))}>${name}</div>`).join('')}</div>` +
    `<div class="prefs-main"><div class="prefs-head"><div class="prefs-title">${rec ? 'AI 记录' : d.tab}</div><div class="note">${rec ? '本作品每次向模型发的请求记一行，失败、取消的也记；只记发了哪几部分、各多大，不存内容。' : about[d.tab]}</div></div><div class="prefs-body scroll" data-k="prefs-${rec ? 'records' : d.tab}">${body}</div></div><button class="icon-btn muted prefs-close" aria-label="关闭设置" title="关闭设置"${onClick(shut)}>${icon('cross')}</button>`) + (d.ask ? askAI(d.ask) : '')
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

// 起草在一段新的讨论里，用讨论模型。这里说清楚 AI 预先带上什么、大约多少 token；别的它要用时自己查。不预先报价。
function draftDialog(d) {
  const cur = chapterOf(S.ch), use = S.uses[0], prev = S.chapters[S.chapters.indexOf(cur) - 1]
  const type = (key) => (v) => set((s) => ({ dialog: { ...s.dialog, [key]: v } }))
  const cards = eventsOf(cur.key).length
  const sent = 1100 + S.entries.filter((e) => e.place === 'body').length * 260 + 600 + cards * 45 + (prev ? counted(prev) : 0) // 规则、设定、章纲卷纲、本章的卡片、上一章全文
  return dialog('small', dialogHead('起草本章', (cards ? 'AI 仅将章纲和本章的卡片扩写为正文，不添加这些之外的重大情节。' : 'AI 仅将章纲扩写为正文，不添加章纲之外的重大情节。') + '起草在一段新的讨论里进行：写好的先在正文里显示为提议，采纳后才写入，写入前保存快照；想改哪里，在讨论里接着说。') +
    `<div class="quote">${t(S.outlines[cur.key])}</div>` +
    field('目标字数（选填）', line(d.goal, type('goal'), '由 AI 按章纲判断', '目标字数') + (countOrNone(d.goal) ? '' : '<div class="note danger">目标字数须为正整数，或留空</div>')) +
    (cur.paras.length ? field('本章已有内容', dots(d.how, ['接在已有内容之后', '覆盖已有内容'], type('how'), '已有内容的处理方式')) : '') +
    `<div class="note">将带上${cards ? '本章的 ' + cards + ' 张卡片、' : ''}${prev ? '上一章全文、' : ''}${cur.vol ? '本卷各章章纲' : '各卷卷纲'}等，约 ${tokens(sent)} token；AI 需要时会自己再查。</div>` +
    `<div class="note">用讨论模型 ${t(use.model)} · ${t(use.effort)}；可在设置中「AI」配置里更换。</div>` +
    buttons(button('取消', shut), countOrNone(d.goal) ? button('开始起草', draftNow, 'primary') : button('开始起草', null, 'primary off')))
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
    say('已恢复到 ' + shown.at + ' 的版本，恢复前的内容已存为快照', '', 'ok')
  }
  const same = shown && shown.paras.join('\n') === cur.paras.join('\n')
  return dialog('wide', dialogHead('快照 · ' + titleOf(cur), '打开章节时、连续写作每十分钟、恢复前、' + (aiOn() ? '采纳 AI 提议前、' : '') + '全书替换前都会保存一份。最近七天的全部保留，更早的每天保留最后一份。') +
    (shown
      ? `<div class="snaps"><div class="snap-list scroll">${list.map((x) => `<div class="snap-row row${x === shown ? ' on' : ''}"${onClick(pick({ id: x.id }))}><div>${t(x.at)}</div><div class="note">${t(snapKind(x.kind))} · ${x.paras.join('').length} 字</div></div>`).join('')}</div>` +
        `<div class="snap-side"><div class="snap-tools">${tickbox(d.diff, '与当前对比', pick({ diff: !d.diff }))}<div class="fit note ell">${same ? '该快照与当前正文相同' : d.diff ? '红色删除线为快照中有而当前没有的内容，绿色为当前新增的内容' : ''}</div></div>` +
        (shown.ai && aiOn() ? `<div class="card"><div class="who">${t(shown.ai.head)}</div>${shown.ai.saw ? `<div class="extra">${t(shown.ai.saw)}</div>` : ''}</div>` : '') +
        `<div class="snap-text scroll">${d.diff ? compare(shown.paras, cur.paras) : shown.paras.map((p) => `<div>${t(p)}</div>`).join('') || '<div class="faint">该快照为空</div>'}</div></div></div>`
      : '<div class="empty"><div>本章暂无快照</div></div>') +
    buttons(button('关闭', shut), button('恢复这个版本', shown && !same ? restore : null, shown && !same ? 'primary' : 'primary off')))
}

// 导出：正文、大纲、设定勾几样导几样，各成一个文件。勾的几样、格式和笔名都记着，范围每次从「全书」起。
function exportDialog(d) {
  const cur = chapterOf(S.ch), book = S.shelf.find((b) => b.id === S.book), list = S.chapters
  const type = (key) => (v) => set((s) => ({ dialog: { ...s.dialog, [key]: v } }))
  const word = S.exportAs === 'Word', ticked = (what) => S.exportWhat.includes(what)
  const flip = (what) => () => set((s) => ({ exportWhat: ['正文', '大纲', '设定'].filter((w) => (w === what) !== s.exportWhat.includes(w)) }))
  const ranges = ['全书', ...(cur && cur.vol ? ['本卷'] : []), ...(cur ? ['本章'] : []), ...(list.length > 1 ? ['指定范围'] : [])]
  const range = ranges.includes(d.range) ? d.range : '全书'
  // 指定范围：从第一章起，到第三章止；「到」只列「从」那一章和它后面的。
  const at = (key) => list.findIndex((c) => c.key === key)
  const from = d.from ?? (list[0] || {}).key, to = d.to && at(d.to) >= at(from) ? d.to : (list[Math.min(at(from) + 2, list.length - 1)] || {}).key
  const keyOf = (title) => list.find((c) => titleOf(c) === title).key
  const pickFrom = (title) => { const k = keyOf(title); set((s) => ({ dialog: { ...s.dialog, from: k, to: at(to) < at(k) ? k : to } })) }
  const within = range === '全书' ? list : range === '本卷' ? list.filter((c) => c.vol === cur.vol) : range === '本章' ? [cur] : list.slice(at(from), at(to) + 1)
  const pen = d.pen ?? book.pen ?? S.pen
  const written = (key) => (S.outlines[key] || '').trim()
  // 勾了却导不出的，说一句为什么，导出时跳过它。
  const skip = {
    正文: !within.length && '还没有章节，不导出正文。',
    大纲: !within.length ? '还没有章节，不导出大纲。' : !within.some((c) => written(c.key) || written(c.vol)) && '所选范围内还没有写章纲，不导出大纲。',
    设定: !S.entries.some((e) => e.place === 'body') && '还没有已定的设定，不导出设定。',
  }
  const going = S.exportWhat.filter((w) => !skip[w])
  const part = range === '全书' ? '' : range === '本卷' ? S.volumes.find((v) => v[0] === cur.vol)[1] : range === '本章' ? titleOf(cur) : titleOf(chapterOf(from)) + (from === to ? '' : '至' + titleOf(chapterOf(to)))
  const nameOf = (what) => [book.name, what === '设定' ? '' : part, what === '正文' ? '' : what].filter(Boolean).join('-').replace(/：/g, ' ') + (word ? '.docx' : '.txt')
  const out = () => {
    set((s) => ({ dialog: null, ...SHUT, ...(ticked('正文') && word ? { pen, shelf: s.shelf.map((b) => (b.id === s.book ? { ...b, pen } : b)) } : {}) }))
    say(going.length > 1 ? '已导出 ' + going.length + ' 个文件到 ' + S.library : '已导出到 ' + S.library + '\\' + nameOf(going[0]), '', 'ok')
  }
  const about = S.exportWhat.length > 1 ? '勾选的几项各存为一个文件，放在同一文件夹。'
    : ticked('设定') ? '按分类列出已定的设定，不含待确认的。' : ticked('大纲') ? '按章节顺序列出各卷、各章的纲。'
    : word ? '首页写书名、笔名和字数，每章另起一页。' : '合并为一个文件，各卷和各章前加上标题。'
  const pickOf = (id, key, options, pick, label) => field(label, choose(id, titleOf(chapterOf(key)), options.map(titleOf), pick, label))
  return dialog('small', dialogHead('导出', about) +
    field('内容', `<div class="tick-row">${['正文', '大纲', '设定'].map((w) => tickbox(ticked(w), w, flip(w))).join('')}</div>`) +
    field('格式', dots(S.exportAs, ['Word', '纯文本'], (v) => set({ exportAs: v }), '导出格式')) +
    (ticked('正文') || ticked('大纲') ? field('范围', dots(range, ranges, type('range'), '导出范围')) +
      (range === '指定范围' ? `<div class="pair">${pickOf('export-from', from, list, pickFrom, '从')}${pickOf('export-to', to, list.slice(at(from)), (v) => type('to')(keyOf(v)), '到')}</div>` : '') : '') +
    (ticked('正文') && word ? field('笔名', line(pen, type('pen'), '选填，写在首页', '笔名')) : '') +
    (ticked('正文') && !word ? tickbox(d.indent, '段首加两个全角空格', () => type('indent')(!d.indent)) : '') +
    S.exportWhat.filter((w) => skip[w]).map((w) => `<div class="note">${t(skip[w])}</div>`).join('') +
    buttons(button('取消', shut), button('导出', going.length ? out : null, going.length ? 'primary' : 'primary off')))
}

// 全书替换前再问一句。
function swapDialog(d) {
  return dialog('ask', dialogHead('全书替换', '将全书 ' + d.chapters + ' 章中的 ' + d.total + ' 处“' + d.q + '”替换为' + (d.to ? '“' + d.to + '”' : '空（删除）') + '？仅修改正文，设定和章纲不变；各章替换前均保存快照。') +
    buttons(button('取消', shut), button('确定替换', swapBook, 'primary')))
}

// 关标签页时那一章还有没存的字：存了再关、不存就关（回到存着的那一份）、或者不关。
// 三个按钮照系统问「要不要保存」的顺序排：保存、不保存、取消。
function unsavedDialog(d) {
  const drop = () => set((s) => (!s.saved && s.kept && s.kept.key === s.ch ? { ...retext(s, s.kept.paras), dirty: s.kept.dirty } : null))
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
    say('已恢复「' + b.title + '」', '', 'ok')
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
  // 位置下面说那里是什么；书架上的作品都在作品库里，同名的文件夹里已有文件。
  const taken = S.shelf.some((b) => S.library + '\\' + b.name === at), fits = at && !taken
  // 路径长了框里只露出开头，要核对的文件夹名在说明里写出来。
  const folder = '「' + t(at.split(/[\\/]/).filter(Boolean).at(-1) || at) + '」'
  const where = !at ? '<div class="note">请填写作品位置。</div>' : taken ? `<div class="note danger">文件夹${folder}中已有文件，请更换名称或位置。</div>` : `<div class="note">将新建文件夹${folder}。</div>`
  const create = () => {
    if (!fits) return
    const id = 'book' + S.n
    set((s) => ({ shelf: [{ id, name, kind: d.type, words: '暂无正文', at: stamp() }, ...s.shelf], n: s.n + 1, dialog: null }))
    openBook(id)
  }
  return dialog('small', dialogHead('新建作品') + field('作品名称', line(d.name, type('name'), '之后可修改', '作品名称', keys(create) + ' autofocus')) + field('体裁', choose('book-kind', d.type, BOOK.kinds, type('type'), '体裁')) +
    `<div class="field"><div class="label">位置</div><div class="pick-line"><div class="box line fit">${typed(d.at ?? S.library + '\\' + name, type('at'), '', '作品位置', keys(create))}</div>${button('更改…', osAsk('选择新作品所在的文件夹'))}</div>${where}</div>` +
    buttons(button('取消', shut), button('创建', fits ? create : null, fits ? 'primary' : 'primary off')))
}

function nameBookDialog(d) {
  const name = d.name.trim()
  const save = () => { if (name) set((s) => ({ shelf: s.shelf.map((b) => (b.id === d.id ? { ...b, name } : b)), dialog: null })) }
  return dialog('small', dialogHead('重命名作品') + line(d.name, (v) => set((s) => ({ dialog: { ...s.dialog, name: v } })), '', '新作品名', keys(save) + ' autofocus') + '<div class="note">仅修改作品名，文件夹不变。</div>' + buttons(button('取消', shut), button('保存', name ? save : null, name ? 'primary' : 'primary off')))
}

// 关掉作品时还有没定的 AI 提议：先问一句。提议不存盘，关了就没了；它写的字在讨论里还看得到，要的话让 AI 再提一次。
function leaveDialog(d) {
  const n = S.props.length, close = () => { set((s) => ({ dialog: null, ...ruled(s, s.props.map((p) => p.id), '作品关闭时已丢弃') })); d.then() }
  return dialog('ask', dialogHead('还有 ' + n + ' 处 AI 提议没有处理', '关闭后将丢弃。') + buttons(button('取消', shut), button('丢弃并关闭', close, 'primary')))
}
// 要关掉这部作品（换一部、回到书架）：有没定的 AI 提议，先问。
const leave = (then) => () => (S.props.length ? set({ dialog: { kind: 'leave', then }, ...SHUT }) : then())

const DIALOGS = { leave: leaveDialog, backup: backupDialog, settings: settingsDialog, take: takeDialog, source: sourceDialog, draft: draftDialog, snaps: snapsDialog, export: exportDialog, swap: swapDialog, unsaved: unsavedDialog, bin: binDialog, keys: keysDialog, newBook: newBookDialog, nameBook: nameBookDialog, card: cardDialog }
const dialogs = () => (S.dialog ? DIALOGS[S.dialog.kind](S.dialog) : '')
