/**
 * Opens a local page in a headless Edge or Chrome and drives it over the
 * DevTools protocol, with Node's own WebSocket and fetch: clicks land where
 * a pointer would, text goes in the way typing does.
 */
import { spawn } from 'node:child_process'
import { existsSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { pathToFileURL } from 'node:url'

export interface Page {
  /** Evaluates an expression in the page and returns its value. */
  js(expression: string): Promise<any>
  sleep(ms: number): Promise<void>
  /** Clicks the nth thing that reads exactly words (or is titled so) and takes clicks. */
  click(words: string, nth?: number): Promise<void>
  /** Presses the right button at a point of the page. */
  rclick(x: number, y: number): Promise<void>
  /** Presses the left button at one point, moves to another and lets go: how a pointer picks words. */
  drag(from: [number, number], to: [number, number]): Promise<void>
  /** Types into the field labelled so. */
  type(label: string, words: string): Promise<void>
  /** Presses a key: one of those named in keyCodes below, or a letter or sign; ctrl holds Ctrl down with it. */
  key(key: string, ctrl?: boolean): Promise<void>
  /** Types words where the caret is. */
  text(words: string): Promise<void>
  /** Empties the field labelled so. */
  clear(label: string): Promise<void>
  /** Saves a picture of the page, when pictures were asked for. */
  shot(name: string): Promise<void>
  /** What the page logged as an error, and what it threw. */
  errors: string[]
  close(): void
}

const browsers = [
  'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',
  'C:/Program Files/Microsoft/Edge/Application/msedge.exe',
  'C:/Program Files/Google/Chrome/Application/chrome.exe',
  '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
  '/usr/bin/google-chrome',
  '/usr/bin/chromium',
]

/** Opens file and waits for ready (a selector) to be there. shots: a folder to save pictures to. */
export async function open(file: string, ready: string, shots?: string): Promise<Page> {
  const browser = [process.env.BROWSER, ...browsers].find((p) => p && existsSync(p))
  if (!browser) throw new Error('no Edge or Chrome found: set BROWSER to the path of one')
  if (shots) mkdirSync(shots, { recursive: true })
  const profile = mkdtempSync(join(tmpdir(), 'prototype-'))
  const proc = spawn(browser, ['--headless=new', '--disable-gpu', '--hide-scrollbars', '--remote-debugging-port=0', `--user-data-dir=${profile}`, '--window-size=1400,1000', 'about:blank'], { stdio: ['ignore', 'ignore', 'pipe'] })
  const wsURL = await new Promise<string>((ok, fail) => {
    let err = ''
    proc.stderr.on('data', (d) => { err += d; const m = err.match(/DevTools listening on (ws:\/\/\S+)/); if (m) ok(m[1]) })
    setTimeout(() => fail(new Error('the browser did not start: ' + err.slice(0, 300))), 15000)
  })
  const targets = await (await fetch(`http://127.0.0.1:${new URL(wsURL).port}/json`)).json() as { type: string; webSocketDebuggerUrl: string }[]
  const ws = new WebSocket(targets.find((t) => t.type === 'page')!.webSocketDebuggerUrl)
  await new Promise((r) => (ws.onopen = r))

  let seq = 0
  const waits = new Map<number, (m: any) => void>()
  const errors: string[] = []
  ws.onmessage = (ev) => {
    const m = JSON.parse(String(ev.data))
    if (m.id && waits.has(m.id)) { waits.get(m.id)!(m); waits.delete(m.id) }
    if (m.method === 'Runtime.consoleAPICalled' && m.params.type === 'error') errors.push(m.params.args.map((a: any) => a.value ?? a.description ?? '').join(' '))
    if (m.method === 'Runtime.exceptionThrown') errors.push(m.params.exceptionDetails.exception?.description ?? m.params.exceptionDetails.text)
  }
  const send = (method: string, params = {}) => new Promise<any>((r) => { const id = ++seq; waits.set(id, r); ws.send(JSON.stringify({ id, method, params })) })
  const sleep = (ms: number) => new Promise<void>((r) => setTimeout(r, ms))
  const js = async (expression: string) => {
    const r = await send('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true })
    if (r.result.exceptionDetails) throw new Error(r.result.exceptionDetails.exception?.description ?? r.result.exceptionDetails.text)
    return r.result.result.value
  }
  const close = () => {
    ws.close()
    proc.kill()
    try { rmSync(profile, { recursive: true, force: true }) } catch { /* the browser may still hold it */ }
  }

  await send('Runtime.enable')
  await send('Page.enable')
  await send('Page.navigate', { url: pathToFileURL(file).href })
  for (let i = 0; !(await js(`!!document.querySelector(${JSON.stringify(ready)})`)); i++) {
    if (i === 50) { close(); throw new Error(`${file} did not draw ${ready}: ${errors.join('; ') || 'no error logged'}`) }
    await sleep(100)
  }

  // What reads words: its whole text, its own words beside a count or an icon, or its title.
  const find = `(words, nth) => {
    const flat = (t) => (t || '').replace(/\\s+/g, '')
    const own = (e) => [...e.childNodes].filter((n) => n.nodeType === 3).map((n) => n.data).join('')
    const out = []
    for (const h of document.querySelectorAll('body *')) {
      if (flat(h.innerText) !== flat(words) && flat(own(h)) !== flat(words) && h.getAttribute('title') !== words) continue
      const e = h.closest('[data-click]')
      if (e && !out.includes(e)) out.push(e)
    }
    return out[nth]
  }`
  const field = (label: string) => `document.querySelector('[aria-label=${JSON.stringify(label)}],[placeholder=${JSON.stringify(label)}]')`
  const press = async (button: 'left' | 'right', x: number, y: number) => {
    for (const type of ['mousePressed', 'mouseReleased']) await send('Input.dispatchMouseEvent', { type, x, y, button, clickCount: 1 })
    await sleep(150)
  }
  const keyCodes: Record<string, number> = { Enter: 13, Escape: 27, Backspace: 8, Delete: 46, Tab: 9, F11: 122, '/': 191 }
  const key: Page['key'] = async (key, ctrl = false) => {
    const code = keyCodes[key] ?? key.toUpperCase().charCodeAt(0)
    for (const type of ['rawKeyDown', 'keyUp']) await send('Input.dispatchKeyEvent', { type, key, code: key, windowsVirtualKeyCode: code, modifiers: ctrl ? 2 : 0 })
    await sleep(150)
  }

  return {
    js, sleep, errors, close, key,
    click: async (words, nth = 0) => {
      const at = await js(`(() => { const e = (${find})(${JSON.stringify(words)}, ${nth}); if (!e) return null; e.scrollIntoView({ block: 'center' }); const r = e.getBoundingClientRect(); return [r.x + r.width / 2, r.y + r.height / 2] })()`)
      if (!at) throw new Error('nothing to click reads ' + words)
      await press('left', at[0], at[1])
    },
    rclick: (x, y) => press('right', x, y),
    drag: async (from, to) => {
      await send('Input.dispatchMouseEvent', { type: 'mousePressed', x: from[0], y: from[1], button: 'left', clickCount: 1 })
      await send('Input.dispatchMouseEvent', { type: 'mouseMoved', x: to[0], y: to[1], button: 'left', buttons: 1 })
      await send('Input.dispatchMouseEvent', { type: 'mouseReleased', x: to[0], y: to[1], button: 'left', clickCount: 1 })
      await sleep(150)
    },
    type: async (label, words) => {
      if (!(await js(`(() => { const e = ${field(label)}; if (!e) return false; e.focus(); return true })()`))) throw new Error('no field ' + label)
      await send('Input.insertText', { text: words })
      await sleep(150)
    },
    text: async (words) => {
      await send('Input.insertText', { text: words })
      await sleep(150)
    },
    clear: async (label) => {
      await js(`${field(label)}.select()`)
      await key('Backspace')
    },
    shot: async (name) => {
      if (!shots) return
      const r = await send('Page.captureScreenshot', { format: 'png' })
      writeFileSync(join(shots, name + '.png'), Buffer.from(r.result.data, 'base64'))
    },
  }
}
