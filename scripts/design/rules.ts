/**
 * The design rules as the design's tokens.css states them: the names it
 * declares, the value of each level, the spacing scale. The design scripts
 * read the levels from here, so tokens.css stays the one place they are.
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
}

export const read = (p: string) => readFileSync(p, 'utf8').replace(/\r\n/g, '\n')

export function readRules(dir: string): Rules {
  const css = read(join(dir, 'tokens.css'))
  const value = new Map<string, string>()
  const bare = css.replace(/\/\*[\s\S]*?\*\//g, '')
  for (const b of bare.matchAll(/([^{}]+)\{([^}]*)\}/g)) {
    if (b[1].trim() === '.dark') continue
    for (const d of b[2].matchAll(/--([a-z0-9-]+)\s*:\s*([^;]+);/g)) value.set(d[1], d[2].trim())
  }
  const scale = (name: string) => (value.get(name) ?? '').split(/\s+/).filter(Boolean).map(Number)
  return { css, declared: new Set(value.keys()), value, space: scale('space'), board: scale('space-board') }
}

/** The nearest of values to v; halfway between two goes to the larger. */
export function nearest(values: number[], v: number): number {
  return values.reduce((best, s) => {
    const d = Math.abs(s - v), bd = Math.abs(best - v)
    return d < bd || (d === bd && s > best) ? s : best
  })
}

/** Where a padding, margin or gap of v goes on the scale. A layout size (100 on) stays. */
export function snapSpace(r: Rules, v: number): number {
  if (v >= 100) return v
  const all = [...r.space, ...r.board]
  if (v > Math.max(...all)) return Math.max(...all)
  return nearest(all, v)
}

export const onScale = (r: Rules, v: number) => v >= 100 || r.space.includes(v) || r.board.includes(v)

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
