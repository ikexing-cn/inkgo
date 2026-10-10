/**
 * Writes the design's System.dc.html, the 设计规范 page: the levels of
 * tokens.css drawn out, the rules for using them, and the common components.
 * The levels and colors come from tokens.css, so after changing it, run this
 * again. --height sets the board's height here and in canvas.json; render.ts
 * tells how tall the page came out.
 * Usage: node scripts/design/system.ts <design project dir> [--height <px>]
 */
import { existsSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { box, check, icon, type IconSize, notice, page, select, svg } from './parts.ts'
import { read, readRules } from './rules.ts'

const dir = process.argv[2]
if (!dir) {
  console.error('Usage: node scripts/design/system.ts <design project dir> [--height <px>]')
  process.exit(2)
}
const at = process.argv.indexOf('--height')
const r = readRules(dir)
const px = (name: string) => parseFloat(r.value.get(name) ?? 'NaN')
const raw = (name: string) => {
  const v = r.value.get(name)
  if (v === undefined) throw new Error(`tokens.css lacks --${name}`)
  return v
}

function sec(title: string, sub: string, body: string): string {
  const s = sub ? `<div style="font-size:var(--fs-body);color:var(--muted);line-height:var(--lh-loose);max-width:1100px">${sub}</div>` : ''
  return `<div style="display:flex;flex-direction:column;gap:14px"><div style="font-size:var(--fs-board-h2);font-weight:600">${title}</div>${s}${body}</div>`
}

function panel(body: string, dark = false, pad = '24px 32px', gap = 20): string {
  return `<div class="${dark ? 'dark' : ''}" style="background:var(--bg);color:var(--ink);border-radius:var(--r-dialog);padding:${pad};display:flex;flex-direction:column;gap:${gap}px;box-shadow:var(--shadow-raise)">${body}</div>`
}

function table(cols: string[], rows: string[][], widths: string[]): string {
  const gt = widths.join(' ')
  const head = cols.map((c) => `<div style="font-size:var(--fs-note);color:var(--muted)">${c}</div>`).join('')
  const out = [`<div style="display:grid;grid-template-columns:${gt};gap:16px;padding:0 0 8px;border-bottom:1px solid var(--line)">${head}</div>`]
  for (const row of rows) {
    const cells = row.map((c) => `<div style="font-size:var(--fs-control);line-height:var(--lh-body);min-width:0">${c}</div>`).join('')
    out.push(`<div style="display:grid;grid-template-columns:${gt};gap:16px;padding:10px 0;border-bottom:1px solid var(--line);align-items:center">${cells}</div>`)
  }
  return '<div style="display:flex;flex-direction:column">' + out.join('') + '</div>'
}

const code = (t: string) => `<span style="font-family:Consolas,monospace;font-size:var(--fs-note);color:var(--ink-2);background:var(--hover);border-radius:var(--r-xs);padding:2px 6px">${t}</span>`

/** A list of levels: the sample, its name and value, where it is used, what was merged into it. */
function levelRows(cols: string, rows: [string, string, string, string, string][]): string {
  return '<div style="display:flex;flex-direction:column;border-top:1px solid var(--line)">' + rows.map(([name, value, sample, use, merged]) =>
    `<div style="display:grid;grid-template-columns:${cols};gap:16px;align-items:baseline;padding:12px 0;border-bottom:1px solid var(--line)">`
    + `<div style="font-size:var(--fs-control);font-weight:600">${name}</div>`
    + `<div style="font-size:var(--fs-note);color:var(--muted)">${value}</div>`
    + sample
    + `<div style="font-size:var(--fs-note);color:var(--muted)">${use}</div>`
    + `<div style="display:flex;flex-direction:column;gap:4px">${merged}</div></div>`).join('') + '</div>'
}
const was = (tok: string, merged: string) => `${code(tok)}<div style="font-size:var(--fs-small);color:var(--faint)">原 ${merged}</div>`

// ---- 字号 -------------------------------------------------------------------
const type: [string, string, number, string, string, string, string][] = [
  ['展示', 'fs-display', 700, 'lh-tight', '只用于欢迎页的大字', '我的作品', '26 · 30'],
  ['页标题', 'fs-title', 700, 'lh-tight', '设置页、独立页面的标题', '模型设置', '22 · 24'],
  ['对话框标题', 'fs-dialog', 600, 'lh-tight', '对话框、浮层面板的标题', '新建作品', '16 · 17 · 18'],
  ['分组', 'fs-group', 600, 'lh-body', '页面里的分组标题、卡片标题', '这部作品用', '14 · 14.5 · 15 · 15.5'],
  ['正文', 'fs-body', 400, 'lh-body', '列表项、表单项、说明正文', '停止输入后自动续写', '13.5'],
  ['控件', 'fs-control', 400, 'lh-tight', '按钮、输入框、下拉框里的字', '保存', '13'],
  ['说明', 'fs-note', 400, 'lh-body', '次要说明、副标题、文字按钮', '从下一条发言起生效。', '12 · 12.5'],
  ['小字', 'fs-small', 500, 'lh-tight', '小标签、计数、快捷键', '×3', '10.5 · 11 · 11.5'],
]
const typeRows = () => levelRows('110px 150px 300px minmax(0,1fr) 190px', type.map(([n, tok, w, lh, use, sample, merged]) => [
  n, `${px(tok)} / ${w} / ${raw(lh)}`,
  `<div style="font-size:var(--${tok});font-weight:${w};line-height:var(--${lh});white-space:nowrap;overflow:hidden">${sample}</div>`,
  use, was('--' + tok, merged)]))

// ---- 行距 -------------------------------------------------------------------
const leading: [string, string, string, string][] = [
  ['lh-tight', '标题、按钮、标签：只占一行的字', 'fs-body', '1.3 · 1.4'],
  ['lh-body', '列表项、表单项、说明：一两行的字', 'fs-body', '1.5 · 1.55 · 1.6'],
  ['lh-loose', '成段的说明、讨论气泡', 'fs-body', '1.65 · 1.7'],
  ['lh-read', '要读的正文：AI 回复、快照预览、引用的原文', 'fs-read', '1.75 · 1.8 · 1.85 · 1.9'],
  ['lh-paper', '正文栏的稿纸，只用在稿纸上', 'fs-paper', '1.95'],
]
const prose = '林晚站在巷口，手电在工具箱最底下。她没有去找。巷口那盏灯的灯罩上，有人用指甲划了一个「晚」字。'
const leadingRows = () => levelRows('110px 80px 420px minmax(0,1fr) 210px', leading.map(([tok, use, size, merged]) => [
  tok.slice(3), raw(tok),
  `<div style="font-size:var(--${size});line-height:var(--${tok});max-width:400px">${prose}</div>`,
  use, was('--' + tok, merged)]))

// ---- 图标 -------------------------------------------------------------------
const icons: [IconSize, string, string][] = [
  ['xs', '下拉箭头、复选框里的勾、窗口按钮', '10 · 11 · 12'],
  ['sm', '按钮、列表行里跟着字的图标；提示的关闭', '13 · 14'],
  ['md', '默认：图标按钮、提示的状态图标', '15 · 16'],
  ['lg', '空状态、导入区、欢迎页的大图标', '22 · 28'],
]
const iconRows = () => levelRows('110px 80px 120px minmax(0,1fr) 190px', icons.map(([s, use, merged]) => [
  s, raw('icon-' + s),
  `<div style="display:flex;align-items:center;gap:12px;color:var(--ink)">${svg(icon.book, s)}${svg(icon.import, s)}${svg(icon.ok, s)}</div>`,
  use, was('--icon-' + s, merged)]))

// ---- 颜色 -------------------------------------------------------------------
function colorValues(): [Map<string, [string, string]>, Map<string, string>] {
  const light = new Map<string, [string, string]>(), dark = new Map<string, string>()
  for (const b of r.css.matchAll(/([^{}]*)\{([^}]*)\}/g)) {
    const isDark = b[1].split('*/').pop()!.trim() === '.dark'
    for (const m of b[2].matchAll(/--([a-z0-9-]+):\s*(#[0-9a-fA-F]{6});(?:\s*\/\*\s*(.*?)\s*\*\/)?/g)) {
      if (isDark) dark.set(m[1], m[2])
      else light.set(m[1], [m[2], m[3] ?? ''])
    }
  }
  return [light, dark]
}

const colorGroups: [string, string[]][] = [
  ['底色', ['bg', 'raised', 'side', 'chip', 'hover', 'active']],
  ['文字', ['ink', 'ink-2', 'muted', 'faint', 'on-ink']],
  ['线', ['line', 'border', 'control', 'dim']],
  ['状态', ['danger', 'warn', 'success', 'amber', 'warn-bg', 'warn-bd', 'sel']],
]

function colors(): string {
  const [lv, dv] = colorValues()
  return colorGroups.map(([g, names]) => {
    const cells = names.map((n) => {
      const [hexl, note] = lv.get(n)!
      const hexd = dv.get(n) ?? hexl
      return `<div style="display:flex;flex-direction:column;gap:6px;width:150px">`
        + `<div style="display:flex;height:44px;border-radius:var(--r-control);overflow:hidden;box-shadow:var(--shadow-inset)">`
        + `<div style="flex:1;background:${hexl}"></div><div style="flex:1;background:${hexd}"></div></div>`
        + `<div style="font-size:var(--fs-note);font-weight:600">--${n}</div>`
        + `<div style="font-size:var(--fs-small);color:var(--muted);line-height:var(--lh-body)">${note}</div>`
        + `<div style="font-size:var(--fs-small);color:var(--faint);font-family:Consolas,monospace">${hexl.toLowerCase()} · ${hexd.toLowerCase()}</div></div>`
    }).join('')
    return `<div style="display:flex;flex-direction:column;gap:10px"><div style="font-size:var(--fs-control);font-weight:600">${g}</div>`
      + `<div style="display:flex;flex-wrap:wrap;gap:20px 20px">${cells}</div></div>`
  }).join('')
}

// ---- 圆角、阴影、间距 ---------------------------------------------------------
const radiiList: [string, string, string][] = [
  ['r-xs', '复选框、小标签、计数', '2 · 3 · 4 · 5'],
  ['r-control', '按钮、输入框、下拉框、图标按钮、菜单项', '5 · 6 · 7 · 8'],
  ['r-card', '分组列表、卡片、菜单、提示', '9 · 10'],
  ['r-dialog', '对话框、设置窗口、讨论气泡和输入框', '12 · 14 · 16'],
  ['r-round', '圆点、头像', '999'],
  ['r-window', '画板上的 macOS 窗口外框', '10'],
  ['r-window-win', '画板上的 Windows 窗口外框', '8'],
]

const radii = () => `<div style="display:flex;flex-wrap:wrap;gap:24px">` + radiiList.map(([t, u, m]) => {
  const v = px(t)
  return `<div style="display:flex;flex-direction:column;gap:8px;width:190px">`
    + `<div style="width:72px;height:52px;border:1.5px solid var(--ink-2);border-radius:var(--${t});background:var(--side)"></div>`
    + `<div style="font-size:var(--fs-control);font-weight:600">${v < 999 ? v + 'px' : '全圆'} ${code('--' + t)}</div>`
    + `<div style="font-size:var(--fs-note);color:var(--muted);line-height:var(--lh-body)">${u}</div>`
    + `<div style="font-size:var(--fs-small);color:var(--faint)">现在用到 ${m}</div></div>`
}).join('') + '</div>'

const shadowList: [string, string][] = [
  ['shadow-raise', '分段选择里选中的一段、输入框'],
  ['shadow-pop', '菜单、下拉展开、提示、待处理面板、工具条'],
  ['shadow-dialog', '对话框、设置窗口（下面另有 32% 黑色遮罩）'],
  ['shadow-focus', '聚焦的输入框外圈'],
  ['shadow-inset', '窗口按钮、色块的细边'],
  ['shadow-board', '只用于画板上的窗口外框，不属于软件界面'],
]

const shadows = (dark: boolean) => panel(`<div style="display:flex;flex-wrap:wrap;gap:32px 24px;padding:12px 8px">` + shadowList.map(([t, u]) =>
  `<div style="display:flex;flex-direction:column;gap:12px;width:160px">`
  + `<div style="height:64px;border-radius:var(--r-card);background:var(--raised);border:1px solid var(--border);box-shadow:var(--${t})"></div>`
  + `<div style="font-size:var(--fs-control);font-weight:600">${code('--' + t)}</div>`
  + `<div style="font-size:var(--fs-note);color:var(--muted);line-height:var(--lh-body)">${u}</div></div>`).join('') + '</div>', dark, '32px 32px')

function spacing(): string {
  const bars = (vals: number[], color: string) => vals.filter((s) => s !== 0).map((s) =>
    `<div style="display:flex;flex-direction:column;align-items:center;gap:6px;width:${Math.max(44, s)}px">`
    + `<div style="height:80px;display:flex;align-items:flex-end"><div style="width:${s}px;height:${s}px;background:${color};border-radius:var(--r-xs)"></div></div>`
    + `<div style="font-size:var(--fs-note);color:var(--muted)">${s}</div></div>`).join('')
  return `<div style="display:flex;gap:10px;align-items:flex-end">${bars(r.space, 'var(--ink-2)')}`
    + `<div style="width:24px"></div>${bars(r.board, 'var(--dim)')}</div>`
    + `<div style="font-size:var(--fs-note);color:var(--muted);line-height:var(--lh-body)">深色 12 个是界面用的；浅色 4 个只用于画板和整页的大块留白（欢迎页、空状态）。`
    + `不在表里的值取最近的一级，正好在两级中间取大的那级：1→2、3→4、5→6、7→8、9→10、11→12、18→20、22→24、28→32、36→40。</div>`
}

// ---- 透明度 -----------------------------------------------------------------
function opacity(): string {
  const v = raw('opacity-disabled')
  return `<div style="display:flex;gap:24px;align-items:center">${btn('primary', '保存')}${btn('primary', '保存', 'off')}${btn('secondary', '取消', 'off')}${field('例如 https://…', 'off')}${box(true, true)}</div>`
    + `<div style="font-size:var(--fs-note);color:var(--muted);line-height:var(--lh-body)">不能点的按钮、输入框、复选框整体降到 ${Number(v) * 100}%，不换颜色。`
    + `别处的半透明（提示底下倒计时的细条、规划图例的色条）是装饰，不算这一级，留到半透明色那一轮再收。</div>`
}

const sizes: [string, string, string][] = [
  ['控件高度', '32', '按钮、输入框、下拉框'],
  ['小控件高度', '28', '图标按钮、表格里的小按钮'],
  ['图标线宽', '1.8', '24×24 画布，圆头圆角'],
  ['阅读正文', '14.5（--fs-read）', '讨论气泡、讨论输入框、快照预览：要读的正文，和稿纸一样不在 8 级里'],
  ['稿纸', '17（--fs-paper）', '正文栏的默认字号；作者在「外观」里调了就按设置的来'],
  ['画板标题', '28 · 20（--fs-board-title、--fs-board-h2）', '画板上给作者看的页标题和小节标题，不属于软件界面'],
]

// ---- 组件 -------------------------------------------------------------------
type State = '' | 'hover' | 'off' | 'focus' | 'error'

function btn(kind: 'primary' | 'secondary' | 'ghost' | 'danger' | 'icon', text: string, state: State = '', ic?: keyof typeof icon): string {
  const op = state === 'off' ? 'opacity:var(--opacity-disabled);' : ''
  const lead = ic ? svg(icon[ic], 'sm', 'currentColor', 1.9) : ''
  const base = 'height:32px;box-sizing:border-box;display:inline-flex;align-items:center;gap:6px;border-radius:var(--r-control);font-size:var(--fs-control);white-space:nowrap;'
  let st = ''
  if (kind === 'primary') {
    st = `background:${state === 'hover' ? 'var(--ink-2)' : 'var(--ink)'};color:var(--on-ink);font-weight:600;padding:0 16px;`
  } else if (kind === 'secondary') {
    st = `background:${state === 'hover' ? 'var(--hover)' : 'var(--raised)'};border:1px solid var(--border);color:var(--ink);padding:0 14px;`
  } else if (kind === 'ghost') {
    st = `background:${state === 'hover' ? 'var(--hover)' : 'transparent'};color:${state === 'hover' ? 'var(--ink)' : 'var(--muted)'};padding:0 8px;height:28px;font-size:var(--fs-note);`
  } else if (kind === 'danger') {
    st = 'background:var(--danger);color:#fff;font-weight:600;padding:0 16px;' + (state === 'hover' ? 'filter:brightness(.92);' : '')
  } else {
    const bg = state === 'hover' ? 'var(--hover)' : 'transparent', col = state === 'hover' ? 'var(--ink)' : 'var(--muted)'
    return `<div style="${op}width:28px;height:28px;border-radius:var(--r-control);background:${bg};color:${col};display:flex;align-items:center;justify-content:center">${svg(icon[ic ?? 'plus'], 'md')}</div>`
  }
  return `<div style="${base}${st}${op}">${lead}${text}</div>`
}

function field(text: string, state: State = ''): string {
  const bd = state === 'focus' ? 'var(--ink-2)' : state === 'error' ? 'var(--danger)' : 'var(--border)'
  const ring = state === 'focus' ? 'box-shadow:var(--shadow-focus);' : ''
  const col = (state === '' || state === 'off') && text.startsWith('例如') ? 'var(--faint)' : 'var(--ink)'
  const op = state === 'off' ? 'opacity:var(--opacity-disabled);' : ''
  const caret = state === 'focus' ? '<div style="width:1px;height:15px;background:var(--ink);margin-left:2px"></div>' : ''
  const input = `<div style="${op}width:160px;height:32px;box-sizing:border-box;border:1px solid ${bd};${ring}border-radius:var(--r-control);background:var(--bg);`
    + `display:flex;align-items:center;padding:0 10px;font-size:var(--fs-control);color:${col}">${text}${caret}</div>`
  const msg = state === 'error' ? '<div style="font-size:var(--fs-note);color:var(--danger);margin-top:6px">要以 http(s):// 开头</div>' : ''
  return `<div>${input}${msg}</div>`
}

const seg = (items: string[], on: number) => `<div style="display:inline-flex;gap:2px;padding:4px;border-radius:var(--r-control);background:var(--active)">` + items.map((t, i) =>
  `<div style="padding:0 14px;height:26px;display:flex;align-items:center;border-radius:var(--r-control);font-size:var(--fs-control);`
  + `${i === on ? 'background:var(--raised);box-shadow:var(--shadow-raise);font-weight:600;color:var(--ink)' : 'color:var(--muted)'}">${t}</div>`).join('') + '</div>'

const pill = (t: string, color = 'var(--muted)') => `<div style="display:inline-flex;align-items:center;height:20px;padding:0 8px;border-radius:var(--r-xs);font-size:var(--fs-small);font-weight:500;`
  + `color:${color};background:color-mix(in srgb,${color} 12%,transparent)">${t}</div>`

function group(): string {
  const rows: [string, string, string][] = [['DeepSeek', 'api.deepseek.com · 3 个模型', 'var(--success)'], ['本机 Ollama', 'localhost:11434', 'var(--faint)']]
  return '<div style="width:420px;display:flex;flex-direction:column;border:1px solid var(--border);border-radius:var(--r-card);overflow:hidden;background:var(--raised)">' + rows.map(([n, s, d], i) =>
    `<div style="${i ? 'border-top:1px solid var(--line);' : ''}display:flex;align-items:center;gap:12px;padding:12px 14px"><div style="width:7px;height:7px;border-radius:var(--r-round);background:${d}"></div>`
    + `<div style="flex:1;display:flex;flex-direction:column;gap:2px"><div style="font-size:var(--fs-body);font-weight:500">${n}</div>`
    + `<div style="font-size:var(--fs-note);color:var(--muted)">${s}</div></div>${btn('secondary', '编辑')}</div>`).join('') + '</div>'
}

function menu(): string {
  const items: [string, string, boolean][] = [['复制', 'Ctrl+C', false], ['加入设定', '', true], ['删除', '', false]]
  return `<div style="width:200px;padding:4px;border-radius:var(--r-card);background:var(--raised);border:1px solid var(--border);box-shadow:var(--shadow-pop)">` + items.map(([t, k, on]) =>
    `<div style="display:flex;align-items:center;height:30px;padding:0 10px;border-radius:var(--r-control);font-size:var(--fs-control);`
    + `${on ? 'background:var(--hover);' : ''}color:${t === '删除' ? 'var(--danger)' : 'var(--ink)'}">`
    + `<div style="flex:1">${t}</div><div style="font-size:var(--fs-small);color:var(--faint)">${k}</div></div>`).join('') + '</div>'
}

const compRow = (name: string, spec: string, body: string) =>
  `<div style="display:grid;grid-template-columns:120px minmax(0,1fr) 300px;gap:24px;align-items:start;padding:20px 0;border-bottom:1px solid var(--line)">`
  + `<div style="font-size:var(--fs-body);font-weight:600;padding-top:6px">${name}</div>`
  + `<div style="display:flex;flex-wrap:wrap;gap:14px 20px;align-items:flex-start">${body}</div>`
  + `<div style="font-size:var(--fs-note);color:var(--muted);line-height:var(--lh-body);padding-top:4px">${spec}</div></div>`

const labelled = (label: string, el: string) => `<div style="display:flex;flex-direction:column;gap:6px;align-items:flex-start">${el}<div style="font-size:var(--fs-small);color:var(--faint)">${label}</div></div>`

const components = (dark: boolean) => panel([
  compRow('主按钮', '墨色实底，一个视图最多一个。高 32，左右 16，圆角 6，13/600。悬停变浅一级，禁用 40% 透明。',
    labelled('常态', btn('primary', '保存')) + labelled('悬停', btn('primary', '保存', 'hover')) + labelled('禁用', btn('primary', '保存', 'off'))
    + labelled('带图标', btn('primary', '添加', '', 'plus'))),
  compRow('次按钮', '浮层底色加边框。高 32，左右 14，圆角 6，13/400。',
    labelled('常态', btn('secondary', '取消')) + labelled('悬停', btn('secondary', '取消', 'hover')) + labelled('禁用', btn('secondary', '取消', 'off'))),
  compRow('文字按钮', '无底无框，灰字，悬停出底色。高 28，左右 8，12.5。用在次要操作、列表行尾。',
    labelled('常态', btn('ghost', '打开配置文件')) + labelled('悬停', btn('ghost', '打开配置文件', 'hover')) + labelled('禁用', btn('ghost', '打开配置文件', 'off'))),
  compRow('危险按钮', '只用在确认删除的对话框里，替代主按钮。',
    labelled('常态', btn('danger', '永久删除')) + labelled('悬停', btn('danger', '永久删除', 'hover'))),
  compRow('图标按钮', '28×28，图标 16，圆角 6。必须有提示文字（悬停出现，也给测试和读屏用）。',
    labelled('常态', btn('icon', '', '', 'plus')) + labelled('悬停', btn('icon', '', 'hover', 'plus')) + labelled('禁用', btn('icon', '', 'off', 'plus'))),
  compRow('输入框', '高 32，圆角 6，左右 10，13。聚焦时边框加深并带 3px 浅光圈；出错时边框变红，错误写在框下面，不弹提示。',
    labelled('常态', field('例如 https://…')) + labelled('聚焦', field('https://api.deep', 'focus'))
    + labelled('出错', field('api.deepseek.com', 'error')) + labelled('禁用', field('例如 https://…', 'off'))),
  compRow('下拉框', '和输入框同高同框，右侧 12 的下箭头。展开的列表用菜单的样子。',
    labelled('常态', select('deepseek-chat', 180)) + labelled('展开的列表', menu())),
  compRow('复选框', '16×16，圆角 4，选中为墨色实底。旁边的说明用「正文」加「说明」两行。',
    labelled('未选', box(false)) + labelled('选中', box(true)) + labelled('禁用', box(true, true))
    + check(true, '离开改过的章节时更新摘要', '你自己写的摘要不会被覆盖。')),
  compRow('分段选择', '底色 --active，选中的一段浮起来。三到四个选项时代替下拉框。', seg(['浅色', '深色', '跟随系统'], 0)),
  compRow('小标签', '高 20，圆角 4，11.5/500，底色是字色的 12%。',
    pill('草稿') + pill('已完成', 'var(--success)') + pill('待处理', 'var(--warn)') + pill('冲突', 'var(--danger)')),
  compRow('分组列表', '圆角 10，边框，行之间一条细线。设置里的列表都用它。', group()),
  compRow('提示', '详见「提示」页。宽 360，圆角 10，--shadow-pop，右上角弹出。', notice('ok', '模型设置已保存', { detail: '从下一条发言起生效。', timer: 35 })),
].join(''), dark, '8px 32px 12px', 0)

const rules = [
  '页面不写数字：颜色、字号、圆角、阴影、行距、图标大小、禁用透明度都写 var(--…)，名字在 tokens.css 里。表里没有的值，先在这页和 tokens.css 加上，再去用。间距从下面的刻度里挑（tokens.css 的 --space），不另起名字。',
  '仓库里 node scripts/verify-design.ts 检查设计稿：写了数字或用了 tokens.css 里没有的名字，就报出来。这一页由 scripts/design/system.ts 照 tokens.css 生成，改了 tokens.css 就重新生成。',
  '新组件先画进这一页，作者确认后再在别的页面里用；已有组件改样子，也先改这一页。',
  '代码里同名：Go 里的常量和这里的名字一一对应（例如 --r-control 对应 rControl，--lh-body 对应 lhBody），数值由测试和 tokens.css 逐一核对；界面代码不直接写数字。',
  '画板上给作者看的说明文字和窗口外框不属于软件界面，单独用 --fs-board-*、--r-window*、--shadow-board，不要用到界面上。',
  '正文栏的稿纸单列：字号 --fs-paper 是默认值，作者可在「外观」里调；行距 --lh-paper 只用在稿纸上。讨论气泡、输入框、快照预览这类要读的正文用 --fs-read 和 --lh-read。',
]

const decide: [string, string, string][] = [
  ['按钮圆角', '设计稿里按钮多是 6，代码里主按钮、次按钮是 8，有色按钮是 7。', '统一成 6，跟设计稿。'],
  ['字号合并', '代码里 12 号和 12.5 号、14 号和 15 号混着用，看不出区别。', '12 并进 12.5（说明），14、14.5、15.5 并进 15（分组）；表单项标签原来用 14/500 的，改成 13.5/500。'],
  ['对话框标题', '设置窗口标题 22，其他对话框（新建作品、回收站、快照）标题 17。', '保持两级：设置窗口算「页标题」，其他算「对话框标题」。'],
  ['提示的图标', '「提示」页画的图标是 17。', '改成 16，跟默认图标一致。'],
  ['间距', '设计稿里 1、3、5、7、9、18 这类零散值约 350 处，代码里约 60 处。', '作者 2026-10-10 要求统一：都并进最近一级，两级中间取大的；设计稿和代码都有检查。'],
  ['页面里的数字', '各页面还是直接写 13px、8px、0 8px 28px 这样的数字。', '作者 2026-10-09 要求做好基建：所有页面改成只写变量，相近的值并进最近的一级（例如稿纸 16.5 并进 17）；设计稿和代码都有检查。'],
  ['行距、图标、禁用', '设计稿里行距 14 种、图标 8 种，代码里行距 6 种、图标 6 种，禁用的透明度有 .3、.35、.4、.5 四种。', '作者 2026-10-10 同意统一：行距 5 级、图标 4 级、禁用一级 .4，都并进最近一级，两级中间取大的；设计稿和代码都有检查。'],
]

function body(): string {
  const stat = table(['', '设计稿里', '代码里', '这页定下的'], [
    ['字号', '14 种（不算画板标题）', '16 种', '8 级'],
    ['圆角', '10 种', '12 种', '4 级 + 全圆'],
    ['阴影', '15 种', '9 处', '5 级（画板另有 1 级）'],
    ['行距', '14 种', '6 种', '5 级'],
    ['图标', '8 种', '6 种', '4 级'],
    ['禁用透明度', '.4、.5', '.3、.35、.4', '1 级'],
    ['颜色', '已经统一在 tokens.css', '同一份 tokens.css', '不变'],
  ], ['120px', '1fr', '1fr', '1fr'])
  const sizeTable = table(['', '值', '用在哪'], sizes.map(([n, v, u]) => [n, v, u]), ['120px', '220px', '1fr'])
  const dec = table(['问题', '原来', '定下的'], decide.map(([a, b, c]) => [a, b, c]), ['120px', '1fr', '1fr'])
  const rulesHTML = rules.map((t) => `<div style="display:flex;gap:10px;font-size:var(--fs-body);line-height:var(--lh-loose)"><div style="color:var(--faint)">·</div><div>${t}</div></div>`).join('')
  return `
  <div style="display:flex;flex-direction:column;gap:10px">
    <div style="font-size:var(--fs-board-title);font-weight:700">设计规范 · 第一轮（已定）</div>
    <div style="font-size:var(--fs-body);color:var(--muted);line-height:var(--lh-loose);max-width:1000px">作者 2026-10-09 要求建一套设计规范，让各页面和代码的样子保持一致。颜色已经统一在 tokens.css 里；字号、圆角、阴影、间距、行距、图标大小还是每页各写各的，代码里也一样。这一页把它们收成固定的几级，再把常用组件画一遍。以后设计稿和代码都以这一页为准。</div>
  </div>
  ${sec('现状', '数的是设计稿里所有页面和代码 desk/ 里实际写过的值。', panel(stat))}
  ${sec('使用规则', '', '<div style="display:flex;flex-direction:column;gap:6px;max-width:1100px">' + rulesHTML + '</div>')}
  ${sec('字号', '8 级。粗细只用 400、500、600、700；行距用下面的 5 级。右边是现在被并进这一级的旧字号。', panel(typeRows()))}
  ${sec('行距', '5 级，写成字号的倍数。字越成段、越要细读，行距越松；稿纸单列一级。右边是被并进这一级的旧值。', panel(leadingRows()))}
  ${sec('颜色', '照搬 tokens.css，每格左半是浅色、右半是深色。页面只写 var(--名字)，不写色值。', panel(colors(), false, '24px 32px', 24))}
  ${sec('圆角', '4 级加全圆；画板上的窗口外框另有两级。', panel(radii()))}
  ${sec('阴影', '5 级，画板另有 1 级。深色模式阴影更重，否则看不出浮起。卡片、分组不用阴影，只用边框。代码一层阴影只画得出第一层。', '<div style="display:grid;grid-template-columns:1fr 1fr;gap:20px">' + shadows(false) + shadows(true) + '</div>')}
  ${sec('间距', 'padding、margin、gap 只用这些值。作者 2026-10-10 要求统一，设计稿和代码都已改好；100 以上的是布局尺寸（窗口按钮让出的 150 等），不算间距。', panel(spacing()))}
  ${sec('图标', '4 级，图标画在 24×24 的画布上、线宽 1.8，按这几级缩放。右边是被并进这一级的旧尺寸。', panel(iconRows()))}
  ${sec('禁用', '一级 --opacity-disabled。', panel(opacity()))}
  ${sec('尺寸', '', panel(sizeTable))}
  ${sec('组件 · 浅色', '每个组件画常态、悬停、禁用；输入框多画聚焦和出错。', components(false))}
  ${sec('组件 · 深色', '', components(true))}
  ${sec('已定', '作者 2026-10-09、10-10 确认按建议走，代码已经照这页改好。', panel(dec))}
`
}

const file = join(dir, 'System.dc.html')
const canvasFile = join(dir, 'canvas.json')
let height = at > 0 ? Number(process.argv[at + 1]) : NaN
if (Number.isNaN(height)) {
  const was = existsSync(file) ? read(file).match(/"\$preview":\{"width":\d+,"height":(\d+)/) : null
  height = was ? Number(was[1]) : 6000
}
writeFileSync(file, page('设计规范', height, body()))
if (existsSync(canvasFile)) {
  const canvas = JSON.parse(read(canvasFile))
  if (canvas.boards?.['System.dc.html']) {
    canvas.boards['System.dc.html'].h = height
    writeFileSync(canvasFile, JSON.stringify(canvas, null, 2) + '\n')
  }
}
console.log(`wrote ${file}, ${height}px tall`)
