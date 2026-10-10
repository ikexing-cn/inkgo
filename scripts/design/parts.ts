/**
 * Pieces the design's generated pages share, written the way the design rules
 * want them: var(--…) for colors, text sizes, corners, shadows, line heights
 * and icons, spacing on the scale.
 */

/** The icons, on a 24×24 canvas, stroked with round ends. */
export const icon = {
  plus: 'M12 5v14M5 12h14',
  down: 'M6 9l6 6 6-6',
  check: 'M20 6L9 17l-5-5',
  x: 'M6 6l12 12M18 6L6 18',
  err: 'M12 3l9.5 17h-19zM12 10v4.5M12 17.5h.01',
  warn: 'M12 3a9 9 0 100 18 9 9 0 000-18zM12 7.5v5.5M12 16.5h.01',
  ok: 'M12 3a9 9 0 100 18 9 9 0 000-18zM8 12.5l2.8 2.8L16.5 9.5',
  info: 'M12 3a9 9 0 100 18 9 9 0 000-18zM12 11v5.5M12 7.5h.01',
  book: 'M4 5a2 2 0 012-2h13v16H6a2 2 0 00-2 2zM4 19V5',
  import: 'M12 4v11M7 10l5 5 5-5M5 20h14',
} as const

export type IconSize = 'xs' | 'sm' | 'md' | 'lg'

export function svg(path: string, size: IconSize = 'md', color = 'currentColor', stroke = 1.8): string {
  return `<svg viewBox="0 0 24 24" fill="none" stroke="${color}" stroke-width="${stroke}" stroke-linecap="round" stroke-linejoin="round" style="width:var(--icon-${size});height:var(--icon-${size});flex:none"><path d="${path}"></path></svg>`
}

/** A drop-down, closed. */
export function select(text: string, width?: number): string {
  const w = width ? `width:${width}px;` : ''
  return `<div style="${w}height:var(--h-control);box-sizing:border-box;border:1px solid var(--border);border-radius:var(--r-control);display:flex;align-items:center;gap:6px;padding:0 10px;font-size:var(--fs-control);background:var(--bg);min-width:0"><div style="flex:1;white-space:nowrap;overflow:hidden;text-overflow:ellipsis">${text}</div><div style="color:var(--faint)">${svg(icon.down, 'xs', 'currentColor', 2)}</div></div>`
}

/** A check box, alone. */
export function box(on: boolean, disabled = false, margin = ''): string {
  const op = disabled ? 'opacity:var(--opacity-disabled);' : ''
  return `<div style="${op}width:16px;height:16px;${margin ? 'flex:none;' : ''}box-sizing:border-box;border-radius:var(--r-xs);border:1.5px solid ${on ? 'var(--ink)' : 'var(--control)'};background:${on ? 'var(--ink)' : 'var(--bg)'};display:flex;align-items:center;justify-content:center${margin}">`
    + (on ? svg(icon.check, 'xs', 'var(--on-ink)', 3) : '') + '</div>'
}

/** A check box with its label and the line that explains it. */
export function check(on: boolean, label: string, sub: string): string {
  return `<div style="display:flex;gap:10px;align-items:flex-start">${box(on, false, ';margin-top:2px')}<div style="display:flex;flex-direction:column;gap:4px"><div style="font-size:var(--fs-body)">${label}</div><div style="font-size:var(--fs-note);color:var(--muted);line-height:var(--lh-body)">${sub}</div></div></div>`
}

export type Tone = 'err' | 'warn' | 'ok' | 'info' | 'busy'
const tone: Record<Tone, string> = { err: 'var(--danger)', warn: 'var(--warn)', ok: 'var(--success)', info: 'var(--muted)', busy: 'var(--muted)' }

export interface Notice {
  detail?: string
  actions?: string[]
  close?: boolean
  /** How much of its time is left, in percent; none for a notice that stays. */
  timer?: number
  count?: number
  width?: number
}

/** The floating notice of the 提示 page. */
export function notice(kind: Tone, title: string, n: Notice = {}): string {
  const { detail = '', actions = [], close = true, timer, count = 0, width = 360 } = n
  const color = tone[kind]
  const mark = kind === 'busy'
    ? '<div style="width:16px;height:16px;flex:none;border-radius:var(--r-round);box-sizing:border-box;border:2px solid var(--dim);border-top-color:var(--muted)"></div>'
    : svg(icon[kind], 'md', color, 1.9)
  const acts = actions.length
    ? '<div style="display:flex;gap:14px;margin-top:8px">' + actions.map((a, i) =>
      `<div style="font-size:var(--fs-note);font-weight:${i === 0 ? 600 : 400};color:${i === 0 ? 'var(--ink)' : 'var(--muted)'}">${a}</div>`).join('') + '</div>'
    : ''
  const det = detail ? `<div style="font-size:var(--fs-note);color:var(--muted);line-height:var(--lh-body);margin-top:2px">${detail}</div>` : ''
  const cnt = count ? `<div style="font-size:var(--fs-small);color:var(--muted);background:var(--hover);border-radius:var(--r-control);padding:0 6px;line-height:16px;margin-left:6px">×${count}</div>` : ''
  const cls = close ? `<div style="color:var(--faint);margin:2px -2px 0 4px">${svg(icon.x, 'sm', 'currentColor', 2)}</div>` : ''
  const bar = timer === undefined ? ''
    : `<div style="position:absolute;left:0;right:0;bottom:0;height:2px;background:var(--line)"><div style="width:${timer}%;height:2px;background:${color};opacity:.55"></div></div>`
  return `<div style="position:relative;width:${width}px;box-sizing:border-box;background:var(--raised);border:1px solid var(--border);border-radius:var(--r-card);box-shadow:var(--shadow-pop);padding:12px 14px 12px 14px;display:flex;gap:10px;align-items:flex-start;overflow:hidden;color:var(--ink)">`
    + `<div style="margin-top:2px;flex:none">${mark}</div>`
    + `<div style="flex:1;min-width:0"><div style="display:flex;align-items:center"><div style="font-size:var(--fs-body);font-weight:600">${title}</div>${cnt}</div>${det}${acts}</div>`
    + `${cls}${bar}</div>`
}

/** The page around a generated board: tokens.css, the canvas runtime, the board's width. */
export function page(title: string, height: number, body: string): string {
  return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<title>${title}</title>
<script src="./support.js"></script>
<link rel="stylesheet" href="tokens.css">
</head>
<body>
<x-dc>
<helmet>
<style>body{margin:0;background:var(--page);font-family:'PingFang SC','Microsoft YaHei','Noto Sans SC',system-ui,sans-serif;color:var(--ink)}</style>
</helmet>
<section style="padding:64px;display:flex;flex-direction:column;gap:56px;width:1394px;box-sizing:border-box">
${body}
</section>
</x-dc>
<script type="text/x-dc" data-dc-script data-props='{"$preview":{"width":1394,"height":${height}}}'>
class Component extends DCLogic {
  renderVals() { return {}; }
}
</script>
</body>
</html>
`
}
