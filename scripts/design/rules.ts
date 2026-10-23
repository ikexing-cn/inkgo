/**
 * The design rules as desk/kit/tokens.css states them: the names it declares,
 * the value of each level, the spacing and alpha scales. The check reads the
 * levels from here, so tokens.css stays the one place they are.
 */
import { readFileSync } from 'node:fs'
import { join } from 'node:path'

export interface Rules {
  css: string
  /** Every name tokens.css declares, without the leading dashes. */
  declared: Set<string>
  /** The light value of each declared name, as written. */
  value: Map<string, string>
  /** Padding, margins and gaps take these; the board's are for boards and whole-page room. */
  space: number[]
  board: number[]
  /** What a see-through color or a faded element takes, besides 0 and 1. */
  alpha: number[]
}

export const read = (p: string) => readFileSync(p, 'utf8').replace(/\r\n/g, '\n')

export function readRules(dir: string): Rules {
  const css = read(join(dir, 'kit', 'tokens.css'))
  const value = new Map<string, string>()
  const bare = css.replace(/\/\*[\s\S]*?\*\//g, '')
  for (const b of bare.matchAll(/([^{}]+)\{([^}]*)\}/g)) {
    if (b[1].trim() === '.dark') continue
    for (const d of b[2].matchAll(/--([a-z0-9-]+)\s*:\s*([^;]+);/g)) value.set(d[1], d[2].trim())
  }
  const scale = (name: string) => (value.get(name) ?? '').split(/\s+/).filter(Boolean).map(Number)
  return { css, declared: new Set(value.keys()), value, space: scale('space'), board: scale('space-board'), alpha: scale('alpha') }
}

/** The nearest of values to v; halfway between two goes to the larger. */
export function nearest(values: number[], v: number): number {
  return values.reduce((best, s) => {
    const d = Math.abs(s - v), bd = Math.abs(best - v)
    return d < bd || (d === bd && s > best) ? s : best
  })
}

export const onScale = (r: Rules, v: number) => v >= 100 || r.space.includes(v) || r.board.includes(v)

/** Where an alpha of v goes on its scale; none and full stay. */
export const snapAlpha = (r: Rules, v: number) => (v === 0 || v === 1 ? v : nearest(r.alpha, v))

/** An alpha the way the pages write it: .2, not 0.2. */
export const alphaText = (v: number) => String(v).replace(/^0\./, '.')

/**
 * The alphas a page writes, with fix called for each: the last number of an
 * rgba(), the share of a color mixed with transparent, and a plain opacity.
 */
export function eachAlpha(text: string, fix: (kind: string, v: number) => number): string {
  text = text.replace(/rgba\(([^()]*?),\s*([0-9.]+)\s*\)/g, (_, rgb, a) => `rgba(${rgb},${alphaText(fix('rgba', Number(a)))})`)
  text = text.replace(/(color-mix\(in srgb,\s*[^,]+?\s)([0-9.]+)(%,\s*transparent\))/g, (_, a, v, b) => a + Math.round(fix('color-mix', Number(v) / 100) * 100) + b)
  return text.replace(/((?<![a-z-])opacity:\s*)([0-9.]+)/g, (_, a, v) => a + alphaText(fix('opacity', Number(v))))
}

/**
 * A style's fixed height as the control's it is: an element that holds text
 * or pads its sides and is 20 to 40 high is a control, as is a 28 square (an
 * icon button) and a 40 bar. Returns the style with the height's level named.
 */
export function controlHeight(r: Rules, style: string): string {
  const hs = levels(r, 'h')
  if (!hs.size) return style
  return style.replace(/(^|;)(\s*)height:\s*([0-9.]+)px/g, (whole, lead, sp, v) => {
    const n = Number(v)
    const control = n >= 20 && n < 40 && (/font-size:|padding:\s*0 /.test(style) || (n === 28 && style.includes('width:28px')))
    const bar = n === 40 && /width:\s*(46px|100%)|border-bottom:/.test(style)
    if (!control && !bar) return whole
    return `${lead}${sp}height:var(--${level(hs, n)})`
  })
}

/** The levels named prefix-*, but for those in skip, with their numeric values. */
export function levels(r: Rules, prefix: string, skip: string[] = []): Map<number, string> {
  const out = new Map<number, string>()
  for (const [name, v] of r.value) {
    if (!name.startsWith(prefix + '-') || skip.includes(name)) continue
    const n = parseFloat(v)
    if (!Number.isNaN(n) && !out.has(n)) out.set(n, name)
  }
  return out
}

/** The level of v among levels: the nearest, halfway going to the larger. */
export function level(ls: Map<number, string>, v: number): string {
  return ls.get(nearest([...ls.keys()], v))!
}
