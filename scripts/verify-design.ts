/**
 * Checks a copy of the Design artifact's project/ folder against the design
 * rules: pages write var(--…) for colors, text sizes, corners and shadows,
 * never the numbers, keep spacing on its scale, and only name what
 * tokens.css declares. Also checks that desk/tokens.css is still a verbatim
 * copy of the design's.
 * Usage: node scripts/verify-design.ts <design project dir>
 */
import { readdirSync, readFileSync } from 'node:fs'
import { join, resolve } from 'node:path'

const dir = process.argv[2]
if (!dir) {
  console.error('Usage: node scripts/verify-design.ts <design project dir>')
  process.exit(2)
}

const read = (p: string) => readFileSync(p, 'utf8').replace(/\r\n/g, '\n')
const tokens = read(join(dir, 'tokens.css'))
const declared = new Set([...tokens.matchAll(/--([a-z0-9-]+)\s*:/g)].map((m) => m[1]))
const errors: string[] = []

if (read(resolve('desk/tokens.css')) !== tokens) {
  errors.push('desk/tokens.css differs from the design\'s tokens.css: copy it over verbatim')
}

// The colors the system draws, not the app: macOS's traffic lights, and the
// white cross on Windows' red close button.
const systemColors = new Set(['#ff5f57', '#febc2e', '#28c840', '#fff'])

const rules: { what: string; re: RegExp }[] = [
  { what: 'a text size written out', re: /font-size:\s*[0-9.]+px/g },
  { what: 'a corner written out', re: /border-radius:[^;"'}]*?(?<![0-9.])(?:[1-9][0-9.]*|0?\.[0-9]+)px/g },
  { what: 'a shadow written out', re: /box-shadow:\s*(?!var\(|none|\{\{)[^;"'}]+/g },
  { what: 'a shadow written out', re: /'(?:inset )?-?[0-9.]+(?:px)? -?[0-9.]+px [0-9.]+px[^']*rgba\([^']*'/g },
  { what: 'a text size written out', re: /fontSize:\s*'[0-9.]+px'/g },
]

// The spacing scale: padding, margins and gaps take only these values, the
// board's for the boards and whole-page room. From 100 on it is a layout
// size, and a negative one pairs with a size elsewhere.
const scale = (name: string) => {
  const m = tokens.match(new RegExp(`--${name}:\\s*([0-9 ]+);`))
  if (!m) {
    errors.push(`tokens.css lacks --${name}`)
    return []
  }
  return m[1].trim().split(/\s+/).map(Number)
}
const spacing = new Set([...scale('space'), ...scale('space-board')])
const spacingProp = /(?<![a-z-])(?:padding|margin)(?:-(?:top|right|bottom|left))?\s*:([^;"'}]+)|(?<![a-z-])(?:row-|column-)?gap\s*:([^;"'}]+)/g

const pages = readdirSync(dir).filter((f) => f.endsWith('.dc.html')).sort()
for (const page of pages) {
  const text = read(join(dir, page))
  const where = (i: number) => `${page}:${text.slice(0, i).split('\n').length}`
  for (const { what, re } of rules) {
    for (const m of text.matchAll(re)) errors.push(`${where(m.index!)}: ${what}: ${m[0].slice(0, 60)}`)
  }
  for (const m of text.matchAll(spacingProp)) {
    for (const v of (m[1] ?? m[2]).matchAll(/(?<![0-9.a-z-])(-?)([0-9.]+)px/g)) {
      const n = Number(v[2])
      if (!v[1] && n < 100 && !spacing.has(n)) errors.push(`${where(m.index!)}: a spacing off the scale: ${m[0].slice(0, 60)}`)
    }
  }
  for (const m of text.matchAll(/var\(--([a-z0-9-]+)/g)) {
    if (!declared.has(m[1])) errors.push(`${where(m.index!)}: --${m[1]} is not in tokens.css`)
  }
  // The rules page shows the colors' values themselves.
  if (page === 'System.dc.html') continue
  for (const m of text.matchAll(/#[0-9a-fA-F]{6}\b|#[0-9a-fA-F]{3}\b/g)) {
    if (!systemColors.has(m[0].toLowerCase())) errors.push(`${where(m.index!)}: a color written out: ${m[0]}`)
  }
}

if (errors.length) {
  for (const e of errors) console.error(e)
  console.error(`\n${errors.length} problem(s) in ${pages.length} pages`)
  process.exit(1)
}
console.log(`design ok: ${pages.length} pages, ${declared.size} tokens`)
