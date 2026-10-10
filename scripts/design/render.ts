/**
 * Renders each design page with a headless Edge or Chrome, the way the
 * canvas shows it before its runtime fills the templates. With --against it
 * renders another copy too, counts the pixels that changed, and writes
 * index.html beside the shots to look at them side by side. It also says
 * when a page runs past its board in canvas.json.
 * Usage: node scripts/design/render.ts <design project dir> <out dir> [--against <old project dir>] [--only <Page,…>]
 * BROWSER names the browser when neither is where Windows puts it.
 */
import { execFileSync } from 'node:child_process'
import { existsSync, mkdirSync, readdirSync, readFileSync, writeFileSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { pathToFileURL } from 'node:url'
import { inflateSync } from 'node:zlib'
import { read } from './rules.ts'

const [dir, out] = process.argv.slice(2)
const at = process.argv.indexOf('--against')
const old = at > 0 ? process.argv[at + 1] : undefined
const on = process.argv.indexOf('--only')
const only = on > 0 ? new Set(process.argv[on + 1].split(',')) : undefined
if (!dir || !out || (at > 0 && !old)) {
  console.error('Usage: node scripts/design/render.ts <design project dir> <out dir> [--against <old project dir>] [--only <Page,…>]')
  process.exit(2)
}

const browser = [
  process.env.BROWSER,
  'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',
  'C:/Program Files/Google/Chrome/Application/chrome.exe',
].find((p) => p && existsSync(p))
if (!browser) {
  console.error('No Edge or Chrome found; set BROWSER')
  process.exit(2)
}

interface Image { width: number; height: number; px: Uint8Array /* RGBA */ }

/** Decodes the 8-bit, non-interlaced RGB or RGBA PNG a browser writes. */
function decodePNG(buf: Buffer): Image {
  let pos = 8, width = 0, height = 0, type = 0
  const idat: Buffer[] = []
  while (pos < buf.length) {
    const len = buf.readUInt32BE(pos), kind = buf.toString('ascii', pos + 4, pos + 8)
    const data = buf.subarray(pos + 8, pos + 8 + len)
    if (kind === 'IHDR') {
      width = data.readUInt32BE(0); height = data.readUInt32BE(4); type = data[9]
      if (data[8] !== 8 || (type !== 2 && type !== 6) || data[12] !== 0) throw new Error('unsupported PNG')
    } else if (kind === 'IDAT') idat.push(data)
    pos += 12 + len
  }
  const bpp = type === 6 ? 4 : 3, stride = width * bpp
  const raw = inflateSync(Buffer.concat(idat))
  const cur = new Uint8Array(stride), prev = new Uint8Array(stride)
  const px = new Uint8Array(width * height * 4)
  for (let y = 0; y < height; y++) {
    const f = raw[y * (stride + 1)], row = raw.subarray(y * (stride + 1) + 1, (y + 1) * (stride + 1))
    for (let i = 0; i < stride; i++) {
      const a = i >= bpp ? cur[i - bpp] : 0, b = prev[i], c = i >= bpp ? prev[i - bpp] : 0
      let p = 0
      if (f === 1) p = a
      else if (f === 2) p = b
      else if (f === 3) p = (a + b) >> 1
      else if (f === 4) {
        const q = a + b - c, pa = Math.abs(q - a), pb = Math.abs(q - b), pc = Math.abs(q - c)
        p = pa <= pb && pa <= pc ? a : pb <= pc ? b : c
      }
      cur[i] = (row[i] + p) & 255
    }
    for (let x = 0; x < width; x++) {
      const o = (y * width + x) * 4
      px[o] = cur[x * bpp]; px[o + 1] = cur[x * bpp + 1]; px[o + 2] = cur[x * bpp + 2]; px[o + 3] = bpp === 4 ? cur[x * bpp + 3] : 255
    }
    prev.set(cur)
  }
  return { width, height, px }
}

/** How tall the page's content is: the last row that is not the background. */
function contentHeight(im: Image): number {
  const at = (x: number, y: number) => (y * im.width + x) * 4
  const bg = at(im.width - 3, im.height - 3)
  for (let y = im.height - 1; y > 0; y--) {
    for (let x = 0; x < im.width; x += 5) {
      const o = at(x, y)
      if (im.px[o] !== im.px[bg] || im.px[o + 1] !== im.px[bg + 1] || im.px[o + 2] !== im.px[bg + 2]) return y + 1
    }
  }
  return 0
}

function changed(a: Image, b: Image): number {
  if (a.width !== b.width || a.height !== b.height) return -1
  let n = 0
  for (let i = 0; i < a.px.length; i += 4) {
    if (Math.abs(a.px[i] - b.px[i]) + Math.abs(a.px[i + 1] - b.px[i + 1]) + Math.abs(a.px[i + 2] - b.px[i + 2]) > 24) n++
  }
  return n
}

function shot(project: string, page: string, width: number, height: number, png: string): Image {
  execFileSync(browser!, ['--headless=new', '--disable-gpu', '--hide-scrollbars', `--window-size=${width},${height}`,
    `--screenshot=${resolve(png)}`, pathToFileURL(resolve(project, page)).href], { stdio: 'ignore' })
  return decodePNG(readFileSync(png))
}

mkdirSync(out, { recursive: true })
const canvas = existsSync(join(dir, 'canvas.json')) ? JSON.parse(read(join(dir, 'canvas.json'))) : {}
const board = (page: string): { h?: number } | undefined => {
  const find = (o: any): any => {
    if (!o || typeof o !== 'object') return undefined
    if (o[page] && typeof o[page] === 'object') return o[page]
    for (const v of Object.values(o)) {
      const r = find(v)
      if (r) return r
    }
  }
  return find(canvas)
}

const rows: string[] = []
for (const page of readdirSync(dir).filter((f) => f.endsWith('.dc.html')).sort()) {
  const name = page.replace(/\.dc\.html$/, '')
  if (only && !only.has(name)) continue
  const preview = read(join(dir, page)).match(/"\$preview":\{"width":(\d+),"height":(\d+)/)
  const width = preview ? Number(preview[1]) : 1394
  // Room past the board, to see when the page runs over it.
  const height = (preview ? Number(preview[2]) : 6000) + 3000
  const now = shot(dir, page, width, height, join(out, `${name}.png`))
  const tall = contentHeight(now), h = board(page)?.h
  let line = `${name.padEnd(16)} content ${String(tall).padStart(5)}  board ${h ?? '-'}`
  // A component's template shows every branch before the runtime picks one,
  // so only a board page's height means anything.
  if (h && h >= 200 && tall > h) line += '  ← runs past its board'
  let pair = `<img src="${name}.png">`
  if (old && existsSync(join(old, page))) {
    const was = shot(old, page, width, height, join(out, `${name}-old.png`))
    const n = changed(was, now)
    line += `  changed ${n < 0 ? 'size' : n} px`
    pair = `<img src="${name}-old.png"><img src="${name}.png">`
  }
  console.log(line)
  rows.push(`<h2>${name}</h2><p>${line}</p><div class="pair">${pair}</div>`)
}
writeFileSync(join(out, 'index.html'), `<!doctype html><meta charset="utf-8"><title>Design render</title>
<style>body{font:14px system-ui;margin:24px;background:#eee}.pair{display:flex;gap:12px;align-items:flex-start}.pair img{width:${old ? '49%' : '100%'};border:1px solid #ccc}</style>
${old ? '<p>Left: before. Right: now.</p>' : ''}${rows.join('\n')}`)
