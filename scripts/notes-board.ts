/**
 * Notes board: a local page that shows every Agent Note, where it stands and
 * which notes it links to, re-read from disk and git on every change.
 * Run: npm run notes-board [-- --root <checkout>] [--port 4790]
 */
import { createServer } from 'node:http'
import { execFileSync } from 'node:child_process'
import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs'
import { createHash } from 'node:crypto'
import { dirname, posix, relative, resolve, sep } from 'node:path'
import { fileURLToPath } from 'node:url'

const args = process.argv.slice(2)
function arg(name: string): string | undefined {
  const i = args.indexOf(name)
  return i >= 0 ? args[i + 1] : undefined
}

const here = dirname(fileURLToPath(import.meta.url))
const repo = resolve(arg('--root') ?? resolve(here, '..'))
const port = Number(arg('--port') ?? 4790)
// agent-note-tree.ts resolves the note root once, when it is imported.
process.env.AGENT_NOTE_ROOT = resolve(repo, '.agents/notes')
const { agentNoteRoot, walkAgentNoteTree } = await import('./agent-note-tree.ts')

const LINK = /\[([^\]]+)\]\(([^)\s]+)\)/g

interface Note {
  rel: string
  lifecycle: string
  cls: string
  date: string
  title: string
  status: string
  reason: string
  archived: string
  problem: string
  sections: string[]
  links: string[]
  broken: string[]
  code: string[]
  backlinks: string[]
  dirty: string
  commits: { date: string; subject: string }[]
}

function listArchived(): string[] {
  const dir = resolve(agentNoteRoot, 'archived')
  if (!existsSync(dir)) return []
  const out: string[] = []
  for (const cls of readdirSync(dir, { withFileTypes: true })) {
    if (!cls.isDirectory()) continue
    for (const f of readdirSync(resolve(dir, cls.name))) {
      if (f.endsWith('.md')) out.push(`archived/${cls.name}/${f}`)
    }
  }
  return out.sort()
}

function noteFiles(): string[] {
  return [...walkAgentNoteTree().notes.map((n) => n.rel), ...listArchived()]
}

function plain(md: string): string {
  return md.replace(LINK, '$1').replace(/[`*]/g, '').replace(/\s+/g, ' ').trim()
}

function parse(rel: string): Note {
  const [lifecycle = '', cls = '', base = ''] = rel.split('/')
  const text = readFileSync(resolve(agentNoteRoot, rel), 'utf8').replace(/\r\n/g, '\n')
  const lines = text.split('\n')
  const title = (lines.find((l) => l.startsWith('# ')) ?? base).replace(/^# (Agent Note:\s*)?/, '')
  const statusLine = lines.find((l) => l.startsWith('Status:')) ?? ''
  const [status = '', ...rest] = statusLine.replace(/^Status:\s*/, '').split(/\s+—\s+/)
  const archived = (lines.find((l) => l.startsWith('Archived:')) ?? '').replace(/^Archived:\s*/, '')
  const sections = lines.filter((l) => l.startsWith('## ')).map((l) => l.slice(3))

  // First paragraph (or list) under ## Problem, flattened to one line.
  let problem = ''
  const p = lines.findIndex((l) => l.startsWith('## Problem'))
  if (p >= 0) {
    const body: string[] = []
    for (const l of lines.slice(p + 1)) {
      if (l.startsWith('#')) break
      if (!l.trim()) {
        if (body.length) break
        continue
      }
      body.push(l)
    }
    problem = plain(body.join(' ')).slice(0, 320)
  }

  const links = new Set<string>()
  const broken = new Set<string>()
  const code = new Set<string>()
  const from = dirname(resolve(agentNoteRoot, rel))
  for (const m of text.matchAll(LINK)) {
    const target = (m[2] ?? '').split('#')[0] ?? ''
    if (!target || /^[a-z]+:/i.test(target) || target.includes('…')) continue
    const abs = resolve(from, target)
    if (abs.startsWith(agentNoteRoot + sep)) {
      const r = relative(agentNoteRoot, abs).split(sep).join('/')
      if (r === rel) continue
      if (existsSync(abs)) links.add(r)
      else broken.add(r)
    } else if (abs.startsWith(repo + sep) && !abs.includes(`${sep}.agents${sep}`)) {
      code.add(relative(repo, abs).split(sep).join('/'))
    }
  }

  return {
    rel, lifecycle, cls, date: base.slice(0, 10), title,
    status, reason: rest.join(' — '), archived, problem, sections,
    links: [...links], broken: [...broken], code: [...code],
    backlinks: [], dirty: '', commits: [],
  }
}

function git(...a: string[]): string {
  try {
    return execFileSync('git', a, { cwd: repo, encoding: 'utf8', maxBuffer: 64 << 20, stdio: ['ignore', 'pipe', 'ignore'] })
  } catch {
    return ''
  }
}

// One pass over the history of .agents/notes, following renames, so a note
// moved from proposed/ to implemented/ keeps its earlier commits.
function history(notes: Map<string, Note>): void {
  const alias = new Map<string, string>()
  for (const rel of notes.keys()) alias.set(`.agents/notes/${rel}`, rel)
  let commit = { date: '', subject: '' }
  for (const line of git('log', '-M', '--name-status', '--format=@@%cI%x09%s', '--', '.agents/notes').split('\n')) {
    if (line.startsWith('@@')) {
      const [date = '', ...s] = line.slice(2).split('\t')
      commit = { date, subject: s.join('\t') }
      continue
    }
    const cols = line.split('\t')
    if (cols.length < 2) continue
    const now = cols[cols.length - 1] ?? ''
    const owner = alias.get(now)
    if (!owner) continue
    notes.get(owner)?.commits.push(commit)
    if (cols[0]?.startsWith('R') && cols[1]) alias.set(cols[1], owner)
  }
}

function dirty(notes: Map<string, Note>): void {
  for (const line of git('status', '--porcelain', '-uall', '--', '.agents/notes').split('\n')) {
    if (line.length < 4) continue
    const path = line.slice(3).split(' -> ').pop() ?? ''
    const note = notes.get(path.replace(/^"|"$/g, '').replace(/^\.agents\/notes\//, ''))
    if (note) note.dirty = line.slice(0, 2).trim() === '??' ? '新建' : '已改'
  }
}

let cache = { key: '', body: '' }

function snapshot(): string {
  const files = noteFiles()
  const stamp = files.map((rel) => {
    const s = statSync(resolve(agentNoteRoot, rel))
    return `${rel}:${s.mtimeMs}:${s.size}`
  })
  const key = [git('rev-parse', 'HEAD').trim(), ...stamp].join('\n')
  if (key === cache.key) return cache.body

  const notes = new Map(files.map((rel) => [rel, parse(rel)]))
  for (const n of notes.values()) for (const l of n.links) notes.get(l)?.backlinks.push(n.rel)
  history(notes)
  dirty(notes)
  const list = [...notes.values()]
  const version = createHash('sha1').update(key).digest('hex').slice(0, 12)
  const branch = git('rev-parse', '--abbrev-ref', 'HEAD').trim()
  cache = { key, body: JSON.stringify({ version, repo, branch, read: new Date().toISOString(), notes: list }) }
  return cache.body
}

const page = resolve(here, 'notes-board.html')

createServer((req, res) => {
  const url = new URL(req.url ?? '/', 'http://localhost')
  try {
    if (url.pathname === '/') {
      res.writeHead(200, { 'content-type': 'text/html; charset=utf-8' })
      res.end(readFileSync(page))
    } else if (url.pathname === '/api/notes') {
      const body = snapshot()
      res.writeHead(200, { 'content-type': 'application/json; charset=utf-8', 'cache-control': 'no-store' })
      res.end(body)
    } else if (url.pathname === '/api/note') {
      const rel = posix.normalize(url.searchParams.get('rel') ?? '')
      if (!noteFiles().includes(rel)) throw new Error('no such note')
      res.writeHead(200, { 'content-type': 'text/plain; charset=utf-8', 'cache-control': 'no-store' })
      res.end(readFileSync(resolve(agentNoteRoot, rel)))
    } else {
      res.writeHead(404).end()
    }
  } catch (e) {
    res.writeHead(500, { 'content-type': 'text/plain; charset=utf-8' }).end(String(e))
  }
}).listen(port, '127.0.0.1', () => {
  console.log(`notes board: http://127.0.0.1:${port}/  (reading ${agentNoteRoot})`)
})
