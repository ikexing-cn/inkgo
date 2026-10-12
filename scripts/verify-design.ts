/**
 * Checks a copy of the Design artifact's project/ folder against the design
 * rules (scripts/design/check.ts), and that desk/tokens.css is still a
 * verbatim copy of the design's.
 * Usage: node scripts/verify-design.ts <design project dir>
 */
import { readdirSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { problems } from './design/check.ts'
import { read, readRules } from './design/rules.ts'

const dir = process.argv[2]
if (!dir) {
  console.error('Usage: node scripts/verify-design.ts <design project dir>')
  process.exit(2)
}

const design = readRules(dir)
const errors: string[] = []

if (read(resolve('desk/tokens.css')) !== design.css) {
  errors.push('desk/tokens.css differs from the design\'s tokens.css: copy it over verbatim')
}

if (!design.space.length || !design.board.length) errors.push('tokens.css lacks --space or --space-board')
if (!design.alpha.length) errors.push('tokens.css lacks --alpha')

const pages = readdirSync(dir).filter((f) => f.endsWith('.dc.html')).sort()
// The rules page shows the colors' values themselves.
for (const page of pages) errors.push(...problems(design, page, read(join(dir, page)), { colors: page !== 'System.dc.html' }))

if (errors.length) {
  for (const e of errors) console.error(e)
  console.error(`\n${errors.length} problem(s) in ${pages.length} pages`)
  process.exit(1)
}
console.log(`design ok: ${pages.length} pages, ${design.declared.size} tokens`)
