/**
 * Runs the checks a commit waits for, all at once, and says which passed:
 * go vet, go test, the notes, the prototype; and DESIGN.md's own linter when
 * DESIGN.md is not as it was committed (that one needs the network).
 * What a check printed is shown only if it failed.
 * Usage: node scripts/verify.ts          every check
 *        node scripts/verify.ts --design also the linter, changed or not
 */
import { spawn, spawnSync } from 'node:child_process'

const checks: [name: string, command: string][] = [
  ['go vet', 'go vet ./...'],
  ['go test', 'go test ./...'],
  ['notes', 'npm run -s verify-notes'],
  ['prototype', 'npm run -s verify-prototype'],
]
const touched = spawnSync('git', ['status', '--porcelain', '--', 'DESIGN.md']).stdout.toString().trim() !== ''
if (touched || process.argv.includes('--design')) checks.push(['design', 'npm run -s verify-design'])

const began = Date.now()
const seconds = (from: number) => ((Date.now() - from) / 1000).toFixed(1) + 's'
const results = await Promise.all(checks.map(([name, command]) => new Promise<{ name: string; ok: boolean; said: string }>((done) => {
  const from = Date.now()
  let said = ''
  const run = spawn(command, { shell: true })
  run.stdout.on('data', (d) => (said += d))
  run.stderr.on('data', (d) => (said += d))
  run.on('close', (code) => {
    console.log(`${code === 0 ? 'ok    ' : 'FAILED'} ${name} (${seconds(from)})`)
    done({ name, ok: code === 0, said })
  })
})))

const failed = results.filter((r) => !r.ok)
for (const r of failed) console.error(`\n── ${r.name} ──\n${r.said.trim()}`)
console.log(`\n${failed.length ? failed.length + ' check(s) failed' : 'all ' + results.length + ' checks passed'} in ${seconds(began)}`)
process.exit(failed.length ? 1 : 0)
