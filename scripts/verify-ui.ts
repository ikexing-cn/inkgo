/**
 * Runs the toolkit's own vet (mygo vet) over the whole module: it runs
 * go vet ./... first, then finds values of one frame (a Context, an Element)
 * kept past it, in a struct or by a goroutine. Nothing is let through.
 * Usage: node scripts/verify-ui.ts
 */
import { spawnSync } from 'node:child_process'
import { existsSync, mkdirSync, readdirSync, readFileSync, rmSync } from 'node:fs'

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

// Once, from the module's root: mygo vet reads ./... itself. A run for each
// package that draws ran go vet and loaded the packages again each time.
const run = spawnSync(tool, ['vet', '.'], { encoding: 'utf8' })
if (run.status !== 0) {
  console.error(((run.stdout ?? '') + (run.stderr ?? '') + (run.error?.message ?? '')).trim().replaceAll('\\', '/'))
  console.error('\nmygo vet failed')
  process.exit(1)
}
console.log('vet ok: go vet and mygo vet, the whole module')
