/**
 * Opens a local page in a headless Edge or Chrome and drives it over the
 * DevTools protocol, with Node's own WebSocket and fetch: clicks land where
 * a pointer would, text goes in the way typing does.
 *
 * The page's clock is the caller's. Its timers do not wait for the wall
 * clock: they fire when time is let pass (sleep), at once and in order, so a
 * wait of three seconds takes none, and nothing fires between two steps by
 * itself. Each click, key and typing lets STEP ms pass, as a hand would.
 */
import { spawn } from 'node:child_process'
import { existsSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { pathToFileURL } from 'node:url'

export interface Page {
  /** Evaluates an expression in the page and returns its value. */
  js(expression: string): Promise<any>
  /** Lets ms of the page's time pass: its timers that fall due fire, in order. It does not take ms. */
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
  close(): Promise<void>
}

const browsers = [
  'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',
  'C:/Program Files/Microsoft/Edge/Application/msedge.exe',
  'C:/Program Files/Google/Chrome/Application/chrome.exe',
  '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
  '/usr/bin/google-chrome',
  '/usr/bin/chromium',
]

/** How much of the page's time a click, a key or typing takes. */
const STEP = 150

// Put in the page before its own scripts: setTimeout, clearTimeout and Date.now go by a clock that only __pass moves.
// __pass ends by waiting one turn of the real clock, for what the browser itself queued (focus, the selection) to land.
const CLOCK = `(() => {
  const wait = window.setTimeout.bind(window), start = Date.now(), due = new Map()
  let now = 0, next = 1
  window.setTimeout = (act, ms = 0, ...args) => { const id = next++; due.set(id, { at: now + Math.max(ms, 0), act: () => act(...args) }); return id }
  window.clearTimeout = (id) => { due.delete(id) }
  Date.now = () => start + now
  window.__pass = (ms) => {
    const until = now + ms
    for (;;) {
      let first = 0
      for (const [id, t] of due) if (t.at <= until && (!first || t.at < due.get(first).at)) first = id
      if (!first) break
      const t = due.get(first)
      due.delete(first)
      now = Math.max(now, t.at)
      t.act()
    }
    now = until
    return new Promise((done) => wait(done, 0))
  }
})()`

type Started = { proc: ReturnType<typeof spawn>; profile: string; wsURL: string }

/**
 * Stops the browser and removes its profile. Told to close over its DevTools
 * endpoint, when there is one, the browser takes its helper processes with
 * it; the profile can go only once they have let go of it.
 */
async function stop(proc: Started['proc'], profile: string, wsURL?: string) {
  if (wsURL) {
    try {
      const ws = new WebSocket(wsURL)
      await new Promise((ok, fail) => { ws.onopen = ok; ws.onerror = fail })
      ws.send(JSON.stringify({ id: 1, method: 'Browser.close' }))
      await new Promise<void>((r) => { const t = setTimeout(r, 5000); ws.onclose = () => { clearTimeout(t); r() } })
    } catch { /* it is gone already */ }
  }
  proc.kill()
  for (let i = 0; i < 50; i++) {
    try { rmSync(profile, { recursive: true, force: true }); return } catch { await new Promise((r) => setTimeout(r, 100)) }
  }
}

/** Starts browser headless with a profile of its own; it fails, and is stopped, if it does not listen within 15 seconds or quits first. */
function start(browser: string): Promise<Started> {
  const profile = mkdtempSync(join(tmpdir(), 'prototype-'))
  // Without --edge-skip-compat-layer-relaunch, Edge starts itself again in
  // another process and leaves: this one seems to quit, and that one runs on,
  // out of reach, holding the profile. Chrome ignores the flag.
  const proc = spawn(browser, ['--headless=new', '--edge-skip-compat-layer-relaunch', '--disable-gpu', '--hide-scrollbars', '--remote-debugging-port=0', `--user-data-dir=${profile}`, '--window-size=1400,1000', 'about:blank'], { stdio: ['ignore', 'ignore', 'pipe'] })
  return new Promise<Started>((ok, fail) => {
    let err = ''
    const give = (why: string) => {
      clearTimeout(timer)
      void stop(proc, profile)
      fail(new Error(why + (err ? ': ' + err.slice(0, 300) : '')))
    }
    const timer = setTimeout(() => give('did not start in 15 seconds'), 15000)
    proc.on('exit', () => give('quit before it started'))
    proc.stderr!.on('data', (d) => {
      err += d
      const m = err.match(/DevTools listening on (ws:\/\/\S+)/)
      if (m) { clearTimeout(timer); proc.removeAllListeners('exit'); ok({ proc, profile, wsURL: m[1] }) }
    })
  })
}

/** Opens file and waits for ready (a selector) to be there. shots: a folder to save pictures to. */
export async function open(file: string, ready: string, shots?: string): Promise<Page> {
  const found = [process.env.BROWSER, ...browsers].filter((p): p is string => !!p && existsSync(p))
  if (!found.length) throw new Error('no Edge or Chrome found: set BROWSER to the path of one')
  if (shots) mkdirSync(shots, { recursive: true })
  // One that will not start headless (an Edge mid-update does that) gives way to the next one there.
  const failed: string[] = []
  let started: Started | undefined
  for (const browser of found) {
    try { started = await start(browser); break } catch (e) { failed.push(browser + ': ' + (e as Error).message) }
  }
  if (!started) throw new Error('no browser started:' + failed.map((f) => '\n  ' + f).join(''))
  const { proc, profile, wsURL } = started
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
  const idle = (ms: number) => new Promise<void>((r) => setTimeout(r, ms))
  const js = async (expression: string) => {
    const r = await send('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true })
    if (r.result.exceptionDetails) throw new Error(r.result.exceptionDetails.exception?.description ?? r.result.exceptionDetails.text)
    return r.result.result.value
  }
  const close = async () => {
    ws.close()
    await stop(proc, profile, wsURL)
  }

  await send('Runtime.enable')
  await send('Page.enable')
  await send('Page.addScriptToEvaluateOnNewDocument', { source: CLOCK })
  // What moves as it appears runs by the browser's own clock, which the steps do not wait for: asked for
  // reduced motion, the page puts everything in place at once, and what is measured and pictured is where it stays.
  await send('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-reduced-motion', value: 'reduce' }] })
  await send('Page.navigate', { url: pathToFileURL(file).href })
  for (let i = 0; !(await js(`!!document.querySelector(${JSON.stringify(ready)})`)); i++) {
    if (i === 50) { await close(); throw new Error(`${file} did not draw ${ready}: ${errors.join('; ') || 'no error logged'}`) }
    await idle(100)
  }
  const sleep = async (ms: number) => { await js(`__pass(${ms})`) }

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
    await sleep(STEP)
  }
  const keyCodes: Record<string, number> = { Enter: 13, Escape: 27, Backspace: 8, Delete: 46, Tab: 9, F2: 113, F11: 122, '/': 191, '=': 187, '-': 189 }
  const key: Page['key'] = async (key, ctrl = false) => {
    const code = keyCodes[key] ?? key.toUpperCase().charCodeAt(0)
    for (const type of ['rawKeyDown', 'keyUp']) await send('Input.dispatchKeyEvent', { type, key, code: key, windowsVirtualKeyCode: code, modifiers: ctrl ? 2 : 0 })
    await sleep(STEP)
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
      await sleep(STEP)
    },
    type: async (label, words) => {
      if (!(await js(`(() => { const e = ${field(label)}; if (!e) return false; e.focus(); return true })()`))) throw new Error('no field ' + label)
      await send('Input.insertText', { text: words })
      await sleep(STEP)
    },
    text: async (words) => {
      await send('Input.insertText', { text: words })
      await sleep(STEP)
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
