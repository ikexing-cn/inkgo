/**
 * Writes the two files through which the prototype says its words in more than
 * one language, from what the app will use too:
 *
 * - prototype/locales.js: the catalogs of locales/ (ARB, one file a language,
 *   the source zh-Hans), in the shape mygo's runtime takes.
 * - prototype/i18n.js: mygo's own formatter, packages/runtime/src/i18n.ts of
 *   the mygo in go.mod, with its types stripped, as one global I18N: a page
 *   opened from the disk can load neither modules nor JSON.
 *
 * It also fails when what the prototype draws in place of mygo's own words (macOS's role items) says
 * other than the app will, with mygo's catalogs and the mygo.<key> of locales/.
 *
 * Usage: node scripts/prototype/locales.ts [--check]
 * With --check it writes nothing and fails when either file is not what it
 * would write.
 */
import { execFileSync } from 'node:child_process'
import { readdirSync, readFileSync, writeFileSync } from 'node:fs'
import { stripTypeScriptTypes } from 'node:module'
import { join } from 'node:path'

const SOURCE = 'zh-Hans'
const check = process.argv.includes('--check')
const mygo = execFileSync('go', ['list', '-m', '-f', '{{.Dir}} {{.Version}}', 'github.com/egoist/mygo'], { encoding: 'utf8' }).trim()
const [dir, version] = [mygo.slice(0, mygo.lastIndexOf(' ')), mygo.slice(mygo.lastIndexOf(' ') + 1)]

// The catalogs. Every message with an argument declares it in the source's @key, so that the order of the
// arguments, the order mygo generate gives them, is written down rather than guessed. A mygo.<key> changes
// one of mygo's own words in the app, which mygo i18n check checks against mygo's catalog; the prototype has
// no function for it, but says it where it draws what mygo draws (below).
const files = readdirSync('locales').filter((f) => f.endsWith('.json')).sort()
const read = (f: string) => JSON.parse(readFileSync(join('locales', f), 'utf8')) as Record<string, unknown>
const tags = files.map((f) => f.slice(0, -'.json'.length))
if (!tags.includes(SOURCE)) throw new Error(`locales/${SOURCE}.json is missing`)
const languages = [SOURCE, ...tags.filter((t) => t !== SOURCE)]
const source = read(SOURCE + '.json')
const params: Record<string, string[]> = {}
const messages: Record<string, Record<string, string>> = {}
const ownWord = (key: string) => key.startsWith('mygo.')
for (const [key, value] of Object.entries(source)) {
  if (key.startsWith('@') || ownWord(key)) continue
  if (!/^[A-Za-z][A-Za-z0-9_]*$/.test(key)) throw new Error(`${SOURCE}: ${key} is not a key: a letter, then letters, digits or _`)
  const meta = (source['@' + key] ?? {}) as { placeholders?: Record<string, unknown> }
  params[key] = Object.keys(meta.placeholders ?? {})
  if (String(value).includes('{') && !params[key].length) throw new Error(`${SOURCE}: ${key} takes arguments but its @${key} declares no placeholders`)
}
for (const tag of languages) {
  const all = read(tag + '.json'), out: Record<string, string> = {}
  for (const [key, value] of Object.entries(all)) {
    if (key.startsWith('@') || ownWord(key)) continue
    if (!(key in params)) throw new Error(`${tag}: ${key} is not in ${SOURCE}`)
    out[key] = String(value)
  }
  messages[tag] = out
}
// What mygo draws itself in the app, macOS's role items, the prototype draws with keys of its own, which must
// say what the app will. mygo looks its word up language by language, the catalog's mygo.<key> before its
// own translation: in the language shown, then in the source language, then in English.
const MYGO: Record<string, string> = {
  appAbout: 'menuAbout', appHide: 'menuHide', appHideOthers: 'menuHideOthers', appQuit: 'menuQuitApp',
  winMinimize: 'menuMinimize', winZoom: 'menuZoom', winBringAll: 'menuBringAllToFront',
}
const mygoCatalog = (tag: string) => {
  const path = tag === 'en' ? join(dir, 'internal', 'l10n', 'en.json') : join(dir, 'locales', tag, tag + '.json')
  try { return JSON.parse(readFileSync(path, 'utf8')) as Record<string, unknown> } catch { return {} }
}
const english = mygoCatalog('en'), sourceWords = mygoCatalog(SOURCE)
for (const tag of languages) {
  const own = read(tag + '.json'), theirs = mygoCatalog(tag)
  for (const [key, word] of Object.entries(MYGO)) {
    const app = String(own['mygo.' + word] ?? theirs[word] ?? source['mygo.' + word] ?? sourceWords[word] ?? english[word]).replace('{app}', 'Inkgo')
    const shown = messages[tag][key] ?? messages[SOURCE][key]
    if (shown !== app) throw new Error(`${tag}: ${key} says «${shown}» where the app will show mygo's ${word}, «${app}»: change one of them`)
  }
}
const fallbacks = Object.fromEntries(languages.map((t) => [t, t === SOURCE ? [t] : [t, SOURCE]]))
const catalog = '// 生成的文件，别手改：改 locales/ 里的翻译表，再跑 node scripts/prototype/locales.ts。\n' +
  "'use strict'\n\n" +
  `const CATALOG = ${JSON.stringify({ source: SOURCE, languages, fallbacks, params, messages }, null, 1)}\n`

// mygo's formatter. Its one constructor with parameter properties is written out by hand, which stripping
// types cannot do; the rest only loses its types and its exports.
let ts = readFileSync(join(dir, 'packages', 'runtime', 'src', 'i18n.ts'), 'utf8').replace(/\r\n/g, '\n')
const ctor = /constructor\(\n\s*\/\*\* The message\. \*\/\n\s*readonly source: string,\n\s*\/\*\*[^\n]*\*\/\n\s*readonly offset: number,\n\s*reason: string,\n\s*\) \{\n(\s*)super\(([^\n]*)\);\n/
if (!ctor.test(ts)) throw new Error(`mygo ${version}: the constructor of MessageSyntaxError is not the one this script rewrites`)
ts = ts.replace(ctor, (_, pad: string, sup: string) => `constructor(source: string, offset: number, reason: string) {\n${pad}super(${sup});\n${pad}this.source = source;\n${pad}this.offset = offset;\n`)
const js = stripTypeScriptTypes(ts, { mode: 'strip' }).replace(/^export (?=(?:async )?(?:function|const|let|class) )/gm, '')
if (/^\s*(?:export|import)\b/m.test(js)) throw new Error(`mygo ${version}: i18n.ts has an export or import this script does not handle`)
const formatter = `// 生成的文件，别手改：mygo ${version} 的 packages/runtime/src/i18n.ts 去掉类型，由 node scripts/prototype/locales.ts 写出。\n` +
  "'use strict'\n\n" +
  `const I18N = (() => {\n${js.trimEnd()}\n\nreturn { createCatalog, formatMessage, isPseudoLocale, pseudoLocales }\n})()\n`

let stale = 0
for (const [path, text] of [['prototype/locales.js', catalog], ['prototype/i18n.js', formatter]]) {
  let was = ''
  try { was = readFileSync(path, 'utf8') } catch {}
  if (was === text) continue
  if (check) { console.error(`${path} is not what locales/ and mygo ${version} make: run node scripts/prototype/locales.ts`); stale++ } else writeFileSync(path, text)
}
if (stale) process.exit(1)
