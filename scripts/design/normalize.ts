/**
 * Moves what a design page writes out onto the design rules: text sizes,
 * corners, shadows, line heights and icon sizes become var(--…), padding,
 * margins and gaps move onto the spacing scale. Halfway between two levels
 * goes to the larger. Prints what it would change; --write saves.
 * Usage: node scripts/design/normalize.ts <design project dir> [--write]
 */
import { readdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { level, levels, read, readRules, snapSpace } from './rules.ts'

const dir = process.argv[2]
if (!dir) {
  console.error('Usage: node scripts/design/normalize.ts <design project dir> [--write]')
  process.exit(2)
}
const write = process.argv.includes('--write')
const r = readRules(dir)
const report = new Map<string, number>()
const odd: string[] = []
const count = (k: string) => report.set(k, (report.get(k) ?? 0) + 1)
const fmt = (n: number) => String(n)

const textSizes = levels(r, 'fs', ['fs-read', 'fs-paper'])
// Prose to read, set loose, takes the reading or the manuscript's size.
const proseSizes = new Map([...levels(r, 'fs')].filter(([, n]) => n === 'fs-read' || n === 'fs-paper'))
const corners = levels(r, 'r', ['r-round', 'r-window', 'r-window-win'])
const lineHeights = levels(r, 'lh')
const textLineHeights = levels(r, 'lh', ['lh-paper'])
const icons = levels(r, 'icon')

// Shadows by their look, the first that matches.
const shadows: [RegExp, string][] = [
  [/^inset 0 0 0 (\.5px|1px)/, 'inset'],
  [/^0 0 0 3px/, 'focus'],
  [/^0 10px 30px rgba\(0,0,0,\.1\)$/, 'board'],
  [/^0 (1px|0) (2px|3px) rgba\(0,0,0,\.0[5-9]\)$/, 'raise'],
  [/^0 1px 3px rgba\(0,0,0,\.(08|2)\),\s*0 10px 30px/, 'board'],
  [/^0 (6|8)px (20|24|28)px/, 'pop'],
  [/^0 0 0 1px rgba\(0,0,0,\.07\),\s*0 18px 44px/, 'pop'],
  [/^0 20px 60px/, 'dialog'],
]

function fixStyle(style: string): string {
  const lh = style.match(/line-height:\s*([0-9.]+)(?![0-9.]*px)/)
  const prose = lh !== null && Number(lh[1]) >= 1.8
  style = style.replace(/font-size:\s*([0-9.]+)px/g, (_, v) => {
    const t = level(prose ? proseSizes : textSizes, Number(v))
    count(`font-size ${v} -> ${t}`)
    return `font-size:var(--${t})`
  })
  style = style.replace(/border-radius:\s*([0-9.px ]+?)(?=;|$|"|')/g, (whole, vals: string) => {
    const parts = vals.trim().split(/\s+/).map((p) => {
      const v = parseFloat(p)
      if (v === 0) return '0'
      if (/width:\s*1266px/.test(style)) return v >= 10 ? 'var(--r-window)' : 'var(--r-window-win)'
      if (v >= 50) return 'var(--r-round)'
      // 8 is a card's corner when it pads like one, a control's otherwise.
      if (v === 8) {
        const pad = style.match(/(?:^|;)\s*padding:\s*([0-9.]+)px/)
        return pad && Number(pad[1]) >= 12 ? 'var(--r-card)' : 'var(--r-control)'
      }
      return `var(--${level(corners, v)})`
    })
    count(`border-radius ${vals.trim()} -> ${parts.join(' ')}`)
    return 'border-radius:' + parts.join(' ')
  })
  style = style.replace(/box-shadow:\s*([^;"']+)/g, (whole, v: string) => {
    v = v.trim()
    if (/^(var\(|\{\{|none)/.test(v)) return whole
    const hit = shadows.find(([re]) => re.test(v))
    if (!hit) {
      odd.push(`box-shadow ${v}`)
      return whole
    }
    count(`box-shadow -> shadow-${hit[1]}`)
    return `box-shadow:var(--shadow-${hit[1]})`
  })
  // Only the manuscript's own size is set like the manuscript.
  const paper = /font-size:\s*var\(--fs-paper\)|font-size:\s*\{\{/.test(style)
  style = style.replace(/line-height:\s*([0-9.]+)(?![0-9.]*px)(?=;|$|"|'|\s)/g, (_, v) => {
    const t = level(paper ? lineHeights : textLineHeights, Number(v))
    count(`line-height ${v} -> ${t}`)
    return `line-height:var(--${t})`
  })
  return style
}

function fixSpacing(text: string): string {
  return text.replace(/((?<![a-z-])(?:padding|margin)(?:-(?:top|right|bottom|left))?\s*:\s*|(?<![a-z-])(?:row-|column-)?gap\s*:\s*)([^;"'}]+)/g, (_, prop: string, val: string) =>
    prop + val.replace(/(?<![0-9.a-z-])(-?)([0-9.]+)px/g, (m, neg: string, v: string) => {
      if (neg) return m // pairs with a size elsewhere
      const s = snapSpace(r, Number(v))
      if (s !== Number(v)) count(`${prop.split(/[-:\s]/)[0]} ${v} -> ${s}`)
      return fmt(s) + 'px'
    }))
}

function fixIcons(text: string): string {
  return text.replace(/<svg width="([0-9.]+)" height="\1"([^>]*)>/g, (_, v, rest: string) => {
    const t = level(icons, Number(v))
    count(`svg ${v} -> ${t}`)
    const size = `width:var(--${t});height:var(--${t})`
    // One style attribute: a browser drops the second.
    if (/ style="/.test(rest)) return `<svg${rest.replace(/ style="/, ` style="${size};`)}>`
    return `<svg style="${size}"${rest}>`
  })
}

let files = 0
for (const page of readdirSync(dir).filter((f) => f.endsWith('.dc.html')).sort()) {
  const p = join(dir, page)
  const s = read(p)
  let t = s.replace(/style="([^"]*)"/g, (_, v) => `style="${fixStyle(v)}"`)
  t = t.replace(/style='([^']*)'/g, (_, v) => `style='${fixStyle(v)}'`)
  t = t.replace(/(<style>)([\s\S]*?)(<\/style>)/g, (_, a, v, b) => a + fixStyle(v) + b)
  t = fixIcons(fixSpacing(t))
  if (t !== s) {
    files++
    if (write) writeFileSync(p, t)
  }
}

for (const [k, v] of [...report].sort()) console.log(String(v).padStart(5), k)
for (const o of odd) console.log('odd:', o)
console.log(`${files} page(s) ${write ? 'changed' : 'would change'}`)
