/**
 * Runs the checks a commit waits for, all at once, and says which passed:
 * go vet, the toolkit's vet, go test, the notes, the prototype; and
 * DESIGN.md's own linter when DESIGN.md is not as it was committed (that one
 * needs the network). The tests that time a frame run alone afterwards.
 * What a check printed is shown only if it failed.
 * Usage: node scripts/verify.ts          every check
 *        node scripts/verify.ts --design also the linter, changed or not
 */
import { spawn, spawnSync } from 'node:child_process'

// Tests that hold a frame to a limit in milliseconds, and their packages.
// Beside the other checks the machine is too busy for those limits (a 9 ms
// frame took 31), so they wait until the rest are done.
const timed = { tests: '^(TestPhonePreviewTypingCost|TestOutlineFrameCost)$', packages: './desk ./desk/outline' }

const checks: [name: string, command: string][] = [
  ['go vet', 'go vet ./...'],
  ['ui vet', 'npm run -s verify-ui'],
  ['go test', `go test -skip "${timed.tests}" ./...`],
  ['notes', 'npm run -s verify-notes'],
  ['prototype', 'npm run -s verify-prototype'],
]
const touched = spawnSync('git', ['status', '--porcelain', '--', 'DESIGN.md']).stdout.toString().trim() !== ''
if (touched || process.argv.includes('--design')) checks.push(['design', 'npm run -s verify-design'])

const began = Date.now()
const seconds = (from: number) => ((Date.now() - from) / 1000).toFixed(1) + 's'
const check = ([name, command]: [string, string]) => new Promise<{ name: string; ok: boolean; said: string }>((done) => {
  const from = Date.now()
  let said = ''
  const run = spawn(command, { shell: true })
  run.stdout.on('data', (d) => (said += d))
  run.stderr.on('data', (d) => (said += d))
  run.on('close', (code) => {
    console.log(`${code === 0 ? 'ok    ' : 'FAILED'} ${name} (${seconds(from)})`)
    done({ name, ok: code === 0, said })
  })
})
const results = await Promise.all(checks.map(check))
results.push(await check(['frame timing', `go test -run "${timed.tests}" ${timed.packages}`]))

const failed = results.filter((r) => !r.ok)
for (const r of failed) console.error(`\n── ${r.name} ──\n${r.said.trim()}`)
console.log(`\n${failed.length ? failed.length + ' check(s) failed' : 'all ' + results.length + ' checks passed'} in ${seconds(began)}`)
process.exit(failed.length ? 1 : 0)
