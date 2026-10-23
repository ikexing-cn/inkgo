/**
 * Runs the toolkit's own vet (mygo vet) over the packages that draw: it
 * finds values of one frame (a Context, an Element) kept past it, in a
 * struct or by a goroutine. Nothing is let through.
 * Usage: node scripts/verify-ui.ts
 */
import { spawn, spawnSync } from 'node:child_process'
import { existsSync, mkdirSync, readdirSync, readFileSync, rmSync } from 'node:fs'

function deskPackages(dir: string): string[] {
  const entries = readdirSync(dir, { withFileTypes: true })
  const here = entries.some((entry) => entry.isFile() && entry.name.endsWith('.go')) ? [dir] : []
  return here.concat(entries.filter((entry) => entry.isDirectory()).flatMap((entry) => deskPackages(dir + '/' + entry.name)))
}

const packages = [...deskPackages('desk'), 'editor', 'cmd/inkgo']

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

const failed = runs.filter((r) => r.code !== 0)
for (const r of failed) console.error(r.said.trim().replaceAll('\\', '/'))
if (failed.length) {
  console.error(`\nmygo vet: ${failed.map((r) => r.dir).join(', ')}`)
  process.exit(1)
}
console.log(`ui ok: ${packages.join(', ')}`)
