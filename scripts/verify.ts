/**
 * Runs the checks a commit waits for, all at once, and says which passed:
 * go vet with the toolkit's vet, host and extension i18n messages and generated catalog freshness,
 * go test, the notes, the prototype; and
 * DESIGN.md's own linter when DESIGN.md is not as it was committed (that one
 * needs the network). desk's tests run in several processes beside the rest,
 * and the tests that time a frame run alone afterwards.
 * What a check printed is shown only if it failed.
 * Usage: node scripts/verify.ts          every check
 *        node scripts/verify.ts --design also the linter, changed or not
 */
import { spawn, spawnSync } from 'node:child_process'

// Tests that hold frames to a time limit, and their packages.
// Beside the other checks the machine is too busy for those limits (a 9 ms
// frame took 31), so they run on their own afterwards.
const timed = { tests: '^(TestPhonePreviewTypingCost|TestOutlineFrameCost)$', packages: './desk ./ext/outline' }

// desk's tests take a minute one after another. They change the package's
// globals (such as where the library is), so
// t.Parallel() would have them tread on each other; split between
// processes, each has globals of its own.
const deskProcesses = 4

type Result = { name: string; ok: boolean; said: string }
const began = Date.now()
const seconds = (from: number) => ((Date.now() - from) / 1000).toFixed(1) + 's'

// A command is a line for the shell, or the words of one without it: a
// pattern for -run would lose its ^ and | to cmd.exe.
const run = (command: string | string[]) => new Promise<{ ok: boolean; out: string; said: string }>((done) => {
  let out = ''
  let said = ''
  const p = typeof command === 'string' ? spawn(command, { shell: true }) : spawn(command[0], command.slice(1))
  p.stdout.on('data', (d) => ((out += d), (said += d)))
  p.stderr.on('data', (d) => (said += d))
  p.on('error', (err) => (said += err.message))
  p.on('close', (code) => done({ ok: code === 0, out, said }))
})
const say = (name: string, ok: boolean, from: number) => console.log(`${ok ? 'ok    ' : 'FAILED'} ${name} (${seconds(from)})`)
const check = async (name: string, command: string | string[]): Promise<Result> => {
  const from = Date.now()
  const { ok, said } = await run(command)
  say(name, ok, from)
  return { name, ok, said }
}

// go test for every package but desk, and desk's tests dealt out to the
// processes by name. Finding them takes a few seconds, and a build of desk's
// tests when they changed, so the other checks do not wait for it.
const goTests = async (): Promise<Result[]> => {
  const from = Date.now()
  const [packages, listed] = await Promise.all([run(['go', 'list', './...']), run(['go', 'test', '-list', '.', './desk'])])
  const tests: Promise<Result>[] = []
  if (packages.ok) {
    const others = packages.out.trim().split(/\s+/).filter((p) => p !== 'writer/desk')
    tests.push(check('go test', ['go', 'test', '-skip', timed.tests, ...others]))
  } else {
    say('go list', false, from)
    tests.push(Promise.resolve({ name: 'go list', ok: false, said: packages.said }))
  }
  if (listed.ok) {
    const names = listed.out.split(/\r?\n/).filter((name) => name.startsWith('Test') && !new RegExp(timed.tests).test(name)).sort()
    for (let i = 0; i < deskProcesses; i++) {
      const own = names.filter((_, n) => n % deskProcesses === i)
      tests.push(check(`desk ${i + 1}/${deskProcesses}`, ['go', 'test', '-run', `^(${own.join('|')})$`, './desk']))
    }
  } else {
    say('desk test list', false, from)
    tests.push(Promise.resolve({ name: 'desk test list', ok: false, said: listed.said }))
  }
  return Promise.all(tests)
}

const checks: [name: string, command: string][] = [
  // mygo vet runs go vet ./... before its own check.
  ['vet', 'npm run -s verify-ui'],
  ['notes', 'npm run -s verify-notes'],
  ['prototype', 'npm run -s verify-prototype'],
]
const touched = spawnSync('git', ['status', '--porcelain', '--', 'DESIGN.md']).stdout.toString().trim() !== ''
if (touched || process.argv.includes('--design')) checks.push(['design', 'npm run -s verify-design'])

const [rest, gos] = await Promise.all([Promise.all(checks.map(([name, command]) => check(name, command))), goTests()])
const results = [...rest, ...gos]
results.push(await check('frame timing', `go test -run "${timed.tests}" ${timed.packages}`))

const failed = results.filter((r) => !r.ok)
for (const r of failed) console.error(`\n── ${r.name} ──\n${r.said.trim()}`)
console.log(`\n${failed.length ? failed.length + ' check(s) failed' : 'all ' + results.length + ' checks passed'} in ${seconds(began)}`)
process.exit(failed.length ? 1 : 0)
