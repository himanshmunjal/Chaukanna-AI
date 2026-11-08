// Checks the translations, and fails the build if one has gone missing:
//   - every string the interface asks for exists in hi.json (the master list)
//   - every language has exactly the same strings as hi.json
//   - placeholders such as {n} survive, and each value is in the right script

import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join } from 'node:path'

const root = new URL('..', import.meta.url).pathname
const load = (lang) => JSON.parse(readFileSync(join(root, `src/i18n/${lang}.json`), 'utf8'))
const SCRIPTS = {
  hi: /\p{Script=Devanagari}/u, mr: /\p{Script=Devanagari}/u, ta: /\p{Script=Tamil}/u, te: /\p{Script=Telugu}/u,
  kn: /\p{Script=Kannada}/u, ml: /\p{Script=Malayalam}/u, bn: /\p{Script=Bengali}/u, gu: /\p{Script=Gujarati}/u, pa: /\p{Script=Gurmukhi}/u,
}

const master = load('hi')
const problems = []

function sourceFiles(dir) {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name)
    return statSync(path).isDirectory() ? sourceFiles(path) : /\.tsx?$/.test(name) ? [path] : []
  })
}

const need = (key, where) => { if (!(key in master)) problems.push(`${where}: "${key}" is not in hi.json`) }
const literal = (s) => s.replace(/\\'/g, "'")

for (const file of sourceFiles(join(root, 'src'))) {
  const text = readFileSync(file, 'utf8')
  const name = file.slice(root.length)
  for (const m of text.matchAll(/\bt\('((?:[^'\\]|\\.)+)'/g)) need(literal(m[1]), name)
  for (const m of text.matchAll(/\bt\("([^"]+)"/g)) need(m[1], name)
  // Labels kept in tables and translated where they're shown.
  if (/format\.ts$|DrillPage|CheckPage|WhatNextCard/.test(name)) {
    for (const m of text.matchAll(/\b(?:label|title): '([^']+)'/g)) need(m[1], name)
  }
  for (const table of ['SCAM_TYPES', 'STATUS_TEXT']) {
    const block = text.match(new RegExp(`const ${table}[^=]*= \\{([\\s\\S]*?)\\n\\}`))
    if (block) for (const m of block[1].matchAll(/: '([^']+)'/g)) need(m[1], name)
  }
  if (/lib\/speakerphone\.ts$/.test(name)) {
    for (const m of text.matchAll(/onError\('((?:[^'\\]|\\.)+)'\)/g)) need(literal(m[1]), name)
    for (const m of text.matchAll(/return '((?:[^'\\]|\\.)+)'/g)) need(literal(m[1]), name)
  }
}

// After-call advice is written by the backend and matched here by its exact text.
const report = readFileSync(join(root, '../backend/internal/report/report.go'), 'utf8')
const advice = report.slice(report.indexOf('var Links'), report.indexOf('Draft: draft('))
for (const m of advice.matchAll(/"([A-Z][^"]{10,})"/g)) need(m[1], 'backend report.go')

const holes = (s) => (s.match(/\{\w+\}/g) ?? []).sort().join()
for (const lang of Object.keys(SCRIPTS)) {
  const strings = load(lang)
  for (const key of Object.keys(master)) {
    if (!(key in strings)) { problems.push(`${lang}: missing "${key}"`); continue }
    if (holes(strings[key]) !== holes(key)) problems.push(`${lang}: placeholders differ in "${key}"`)
    if (!SCRIPTS[lang].test(strings[key])) problems.push(`${lang}: not in its script: "${key}" = "${strings[key]}"`)
  }
  for (const key of Object.keys(strings)) if (!(key in master)) problems.push(`${lang}: extra "${key}"`)
}

if (problems.length) {
  console.error(problems.join('\n'))
  console.error(`\n${problems.length} translation problem(s)`)
  process.exit(1)
}
console.log(`translations ok: ${Object.keys(master).length} strings in ${Object.keys(SCRIPTS).length} languages plus English`)
