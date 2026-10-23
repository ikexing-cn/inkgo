/**
 * Checks the prototype in prototype/: that it keeps the design rules
 * (scripts/design/check.ts) against desk/kit/tokens.css, and that everything it
 * can do still works, clicked through in a headless Edge or Chrome
 * (scripts/prototype/walk.ts). It opens index.html from the disk, the way a
 * double click does.
 * Usage: node scripts/verify-prototype.ts
 * With SHOT=<dir> it also saves a picture of each step there.
 */
import { readdirSync } from 'node:fs'
import { resolve } from 'node:path'
import { problems } from './design/check.ts'
import { read, readRules } from './design/rules.ts'
import { open } from './prototype/browser.ts'
import { walk } from './prototype/walk.ts'

const design = readRules('desk')
const files = ['index.html', ...readdirSync('prototype').filter((f) => f.endsWith('.js')).sort()]
const errors = files.flatMap((f) => problems(design, 'prototype/' + f, read(resolve('prototype', f)), { sheet: f.endsWith('.html') }))
if (errors.length) {
  for (const e of errors) console.error(e)
  console.error(`\n${errors.length} problem(s) with the design rules`)
  process.exit(1)
}

const page = await open(resolve('prototype/index.html'), '#win', process.env.SHOT)
const steps: string[] = []
let failed = ''
try {
  await walk(page, (step) => steps.push(step))
} catch (e) {
  failed = (e as Error).message
  await page.shot('failed')
}
await page.close()
if (failed || page.errors.length) {
  if (failed) console.error(`stopped after "${steps.at(-1) ?? 'opening'}": ${failed}`)
  for (const e of new Set(page.errors)) console.error('the page said: ' + e)
  process.exit(1)
}
console.log(`prototype ok: the design rules, and ${steps.length} flows clicked through`)
