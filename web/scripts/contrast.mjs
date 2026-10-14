// Contrast guard for the web UI's design tokens (run by `npm run lint`).
//
// Reads src/styles/tokens.css, pulls the custom properties of the dark, light, and
// system-light theme blocks, and fails (exit 1) when any foreground/background pair the UI
// relies on drops below its WCAG 2.2 AA threshold: 4.5:1 for text, 3:1 for non-text marks
// (input borders, outline ticks). It also fails when the 'system' block under
// prefers-color-scheme: light drifts from the light block, since CSS can't share one.
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const TEXT = 4.5
const NON_TEXT = 3

// [foreground, background, minimum ratio, what it is]
const PAIRS = [
  ['--ink', '--surface', TEXT, 'text'],
  ['--ink-2', '--surface', TEXT, 'secondary text'],
  ['--field', '--surface', NON_TEXT, 'input borders'],
  ['--s2', '--surface', TEXT, '2xx green'],
  ['--s3', '--surface', TEXT, '3xx yellow'],
  ['--s4', '--surface', TEXT, '4xx dark orange'],
  ['--s5', '--surface', TEXT, '5xx red'],
  ['--violet', '--surface', TEXT, 'violet (UI)'],
  ['--todo', '--surface', NON_TEXT, 'not fetched outline'],
  ['--cut', '--surface', NON_TEXT, 'past budget outline'],
  ['--on-fill', '--s2', TEXT, 'text on green (Start, Resume)'],
  ['--on-fill', '--s5', TEXT, 'text on red (Stop)'],
  ['--on-fill', '--violet', TEXT, 'text on violet (Save, Check redirects)'],
  ['--ink', '--ground', TEXT, 'text on the page ground'],
  ['--ink-2', '--ground', TEXT, 'secondary text on the page ground'],
  ['--violet', '--ground', TEXT, 'links on the page ground'],
  ['--ink', '--raised', TEXT, 'active rail item'],
  ['--ink-2', '--raised', TEXT, 'secondary text on raised controls'],
  ['--ink', '--notice-bg', TEXT, 'notice text'],
  ['--s3', '--notice-bg', NON_TEXT, 'warning notice border'],
  ['--s2', '--ground', TEXT, '2xx green on the page ground'],
  ['--s3', '--ground', TEXT, '3xx yellow on the page ground'],
  ['--s4', '--ground', TEXT, '4xx dark orange on the page ground'],
  ['--s5', '--ground', TEXT, '5xx red on the page ground'],
]

const here = dirname(fileURLToPath(import.meta.url))
const tokensPath = join(here, '..', 'src', 'styles', 'tokens.css')

let css
try {
  css = readFileSync(tokensPath, 'utf8')
} catch (err) {
  console.error(`contrast: cannot read ${tokensPath}: ${err.message}`)
  process.exit(1)
}
css = css.replace(/\/\*[\s\S]*?\*\//g, '')

// Innermost rules only: a selector run with no braces, then a body with no braces. The
// @media wrapper's own brace is skipped, so the system block is found by its selector.
function themeBlock(attr) {
  const rule = /([^{}]+)\{([^{}]*)\}/g
  for (const m of css.matchAll(rule)) {
    const selectors = m[1].split(',').map((s) => s.trim())
    if (!selectors.includes(`:root[data-theme='${attr}']`)) continue
    const vars = {}
    for (const decl of m[2].matchAll(/(--[a-z0-9-]+)\s*:\s*([^;]+);/g)) {
      vars[decl[1]] = decl[2].trim().toLowerCase()
    }
    return vars
  }
  return null
}

function luminance(hex) {
  const m = /^#([0-9a-f]{6})$/.exec(hex)
  if (!m) throw new Error(`not a #rrggbb color: ${hex}`)
  const [r, g, b] = [0, 2, 4].map((i) => {
    const c = parseInt(m[1].slice(i, i + 2), 16) / 255
    return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

function ratio(a, b) {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}

const failures = []
const themes = { dark: themeBlock('dark'), light: themeBlock('light') }

for (const [name, vars] of Object.entries(themes)) {
  if (!vars) {
    failures.push(`${name}: no :root[data-theme='${name}'] block in tokens.css`)
    continue
  }
  for (const [fg, bg, min, what] of PAIRS) {
    if (!vars[fg] || !vars[bg]) {
      failures.push(`${name}: ${fg} or ${bg} is not defined`)
      continue
    }
    let r
    try {
      r = ratio(vars[fg], vars[bg])
    } catch (err) {
      failures.push(`${name}: ${err.message}`)
      continue
    }
    const line = `${name.padEnd(5)} ${what.padEnd(40)} ${fg} on ${bg}: ${r.toFixed(2)} (min ${min})`
    if (r < min) failures.push(line)
    else console.log(`ok    ${line}`)
  }
}

const system = themeBlock('system')
if (!system) {
  failures.push("system: no :root[data-theme='system'] block in tokens.css")
} else if (themes.light) {
  const keys = new Set([...Object.keys(themes.light), ...Object.keys(system)])
  for (const k of keys) {
    if (themes.light[k] !== system[k]) {
      failures.push(`system: ${k} is ${system[k] ?? 'missing'}, light block has ${themes.light[k] ?? 'missing'}`)
    }
  }
}

if (failures.length > 0) {
  for (const f of failures) console.error(`FAIL  ${f}`)
  process.exit(1)
}
console.log('contrast: all token pairs pass WCAG 2.2 AA')
