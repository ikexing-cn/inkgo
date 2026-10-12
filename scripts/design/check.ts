/**
 * What a file of interface breaks of the design rules: it writes var(--…)
 * for colors, text sizes, corners, shadows, line heights, icon sizes and the
 * heights of controls, never the numbers, keeps spacing and alphas on their
 * scales, and only names what tokens.css declares. The prototype is held to
 * it.
 */
import { controlHeight, eachAlpha, onScale, snapAlpha, type Rules } from './rules.ts'

// The colors the system draws, not the app: macOS's traffic lights, and the
// white cross on Windows' red close button.
const systemColors = new Set(['#ff5f57', '#febc2e', '#28c840', '#fff'])

const rules: { what: string; re: RegExp }[] = [
  { what: 'a text size written out', re: /font-size:\s*[0-9.]+px/g },
  { what: 'a corner written out', re: /border-radius:[^;"'}]*?(?<![0-9.])(?:[1-9][0-9.]*|0?\.[0-9]+)px/g },
  { what: 'a shadow written out', re: /box-shadow:\s*(?!\s|var\(|none|\{\{)[^;"'}]+/g },
  { what: 'a shadow written out', re: /'(?:inset )?-?[0-9.]+(?:px)? -?[0-9.]+px [0-9.]+px[^']*rgba\([^']*'/g },
  { what: 'a text size written out', re: /fontSize:\s*'[0-9.]+px'/g },
  // A line height in px is a row's height, set to line things up.
  { what: 'a line height written out', re: /line-height:\s*[0-9.]+(?![0-9.]*px)(?=[;"'}\s]|$)/g },
  { what: 'an icon size written out', re: /<svg[^>]*\s(?:width|height)="[0-9.]+"/g },
]

// The spacing scale: padding, margins and gaps take only these values, the
// board's for the boards and whole-page room. From 100 on it is a layout
// size, and a negative one pairs with a size elsewhere.
const spacingProp = /(?<![a-z-])(?:padding|margin)(?:-(?:top|right|bottom|left))?\s*:([^;"'}]+)|(?<![a-z-])(?:row-|column-)?gap\s*:([^;"'}]+)/g

/**
 * The problems of one file, each led by name and line. sheet: true for a file
 * that styles in a style sheet, so its rules are held to the control heights
 * as style attributes are.
 */
export function problems(design: Rules, name: string, text: string, { sheet = false } = {}): string[] {
  const errors: string[] = []
  const where = (i: number) => `${name}:${text.slice(0, i).split('\n').length}`
  for (const { what, re } of rules) {
    for (const m of text.matchAll(re)) errors.push(`${where(m.index!)}: ${what}: ${m[0].slice(0, 60)}`)
  }
  for (const m of text.matchAll(spacingProp)) {
    for (const v of (m[1] ?? m[2]).matchAll(/(?<![0-9.a-z-])(-?)([0-9.]+)px/g)) {
      if (!v[1] && !onScale(design, Number(v[2]))) errors.push(`${where(m.index!)}: a spacing off the scale: ${m[0].slice(0, 60)}`)
    }
  }
  // Alphas keep to their own scale, and the scrim behind a dialog is one color.
  const scrim = (design.value.get('scrim') ?? '').replace(/\s+/g, '')
  for (const m of text.matchAll(/rgba\([^()]*\)/g)) {
    if (scrim && m[0].replace(/\s+/g, '') === scrim) errors.push(`${where(m.index!)}: the scrim written out: ${m[0]}`)
  }
  eachAlpha(text, (kind, v) => {
    if (snapAlpha(design, v) !== v) errors.push(`${name}: an alpha off the scale: ${kind} ${v}`)
    return v
  })
  for (const m of text.matchAll(sheet ? /style="([^"]*)"|\{([^{}]*)\}/g : /style="([^"]*)"/g)) {
    const style = m[1] ?? m[2]
    if (controlHeight(design, style) !== style) errors.push(`${where(m.index!)}: a control's height written out: ${style.match(/height:\s*[0-9.]+px/)?.[0]}`)
  }
  for (const m of text.matchAll(/var\(--([a-z0-9-]+)/g)) {
    if (!design.declared.has(m[1])) errors.push(`${where(m.index!)}: --${m[1]} is not in tokens.css`)
  }
  for (const m of text.matchAll(/#[0-9a-fA-F]{6}\b|#[0-9a-fA-F]{3}\b/g)) {
    if (!systemColors.has(m[0].toLowerCase())) errors.push(`${where(m.index!)}: a color written out: ${m[0]}`)
  }
  return errors
}
