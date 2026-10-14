/**
 * Runs the toolkit's own vet (mygo vet) over the packages that draw: it
 * finds values of one frame (a Context, an Element) kept past it, in a
 * struct or by a goroutine.
 * One place is let through, and only that one: the builder of our own menus
 * holds the Context while a menu is built, in the one call, as the toolkit's
 * ui.Menu does. When it no longer does, this says so, to take the line out.
 * Usage: node scripts/verify-ui.ts
 */
import { spawn, spawnSync } from 'node:child_process'
import { existsSync, mkdirSync, readdirSync, readFileSync, rmSync } from 'node:fs'

const packages = ['desk', 'editor', 'cmd/poc']
const allowed = [{ file: 'desk/menu.go', says: 'native UI build value stored in a struct' }]

// The tool is built once for each version of the toolkit and kept in .tmp:
// linking it takes longer than all it then checks.
const version = /^\s*github\.com\/egoist\/mygo (v\S+)/m.exec(readFileSync('go.mod', 'utf8'))?.[1]
if (!version) {
  console.error('go.mod does not say which github.com/egoist/mygo')
  process.exit(1)
}
const tool = `.tmp/mygo-${version}${process.platform === 'win32' ? '.exe' : ''}`
if (!existsSync(tool)) {
  mkdirSync('.tmp', { recursive: true })
  for (const old of readdirSync('.tmp').filter((f) => f.startsWith('mygo-'))) rmSync('.tmp/' + old)
  const built = spawnSync('go', ['build', '-o', tool, 'github.com/egoist/mygo/cmd/mygo'], { encoding: 'utf8' })
  if (built.status !== 0) {
    console.error(`mygo ${version} did not build:\n${(built.stdout + built.stderr).trim()}`)
    process.exit(1)
  }
}

// The packages at once: each is read on its own.
const runs = await Promise.all(packages.map((dir) => new Promise<{ dir: string; code: number | null; said: string }>((done) => {
  let said = ''
  const run = spawn(tool, ['vet', dir])
  run.stdout.on('data', (d) => (said += d))
  run.stderr.on('data', (d) => (said += d))
  run.on('error', (e) => done({ dir, code: 1, said: said + e.message }))
  run.on('close', (code) => done({ dir, code, said }))
})))

const found: string[] = []
for (const { dir, code, said } of runs) {
  const lines = said.split(/\r?\n/).filter((l) => /\.go:\d+:\d+: /.test(l))
  if (code !== 0 && !lines.length) {
    console.error(`mygo vet ${dir} did not run:\n${said.trim()}`)
    process.exit(1)
  }
  found.push(...lines.map((l) => l.trim().replaceAll('\\', '/')))
}

const passed = (line: string) => allowed.find((a) => line.includes('/' + a.file + ':') && line.includes(a.says))
const problems = found.filter((l) => !passed(l))
const unused = allowed.filter((a) => !found.some((l) => passed(l) === a))
for (const p of problems) console.error(p)
for (const a of unused) console.error(`${a.file} no longer has what scripts/verify-ui.ts lets through: take it out of the list`)
if (problems.length || unused.length) {
  console.error(`\n${problems.length + unused.length} problem(s) with values kept past their frame`)
  process.exit(1)
}
console.log(`ui ok: ${packages.join(', ')}; ${allowed.length} place let through`)
