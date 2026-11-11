import { mkdir, readFile, writeFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'

// Imports the publicly displayed images requested for Zently. Artwork remains
// copyright of its creators; source attribution accompanies the local files.
const sourceURL = 'https://emojigraph.org/ru/apple/'
const client = new URL('../', import.meta.url)
const images = new URL('public/emoji/apple/', client)
const data = new URL('src/emoji-data/', client)
await Promise.all([mkdir(images, { recursive: true }), mkdir(data, { recursive: true })])
const html = await (await fetch(sourceURL, { signal: AbortSignal.timeout(30_000) })).text()
const entries = new Map()
for (const match of html.matchAll(/<img\s+src="(\/media\/72\/apple\/([^"/]+))"[\s\S]*?alt="([^"]*)"[^>]*>/g)) {
  const code = match[2].replace(/\.png$/, '').split('_').at(-1)
  if (!/^[0-9a-f]+(?:-[0-9a-f]+)*$/.test(code)) continue
  const key = code.split('-').filter(part => part !== 'fe0f').join('-')
  const emoji = String.fromCodePoint(...code.split('-').map(part => parseInt(part, 16)))
  const name = match[3].replace(/^Как выглядит эмодзи /, '').replace(/ в Apple\.$/, '').replace(/&amp;/g, '&').replace(/&#39;/g, "'").replace(/&quot;/g, '"')
  entries.set(key, { key, emoji, name, source: new URL(match[1], sourceURL).href })
}
if (entries.size < 3000) throw new Error('Incomplete source catalogue; import stopped')
const catalogue = [...entries.values()]
await writeFile(new URL('keys.json', data), JSON.stringify(catalogue.map(item => item.key)))
await writeFile(new URL('catalogue.json', data), JSON.stringify(catalogue.map(({ key, emoji, name }) => ({ key, emoji, name }))))
await writeFile(new URL('SOURCE.md', images), `# Apple emoji artwork\n\nSource: ${sourceURL}\nRetrieved: ${new Date().toISOString().slice(0,10)}\n\nImages displayed by Emojigraph remain copyright of their respective creators, including Apple. They are not covered by Zently's source code license.\n\nThe catalogue contains ${catalogue.length} images actually listed by the source page. Images are served locally and loaded on demand; no message text is sent to Emojigraph.\n`)
let next = 0, completed = 0, downloaded = 0, bytes = 0
const failed = []
await Promise.all(Array.from({ length: 4 }, async () => {
  while (next < catalogue.length) {
    const item = catalogue[next++]
    const path = new URL(`${item.key}.png`, images)
    try { const existing = await readFile(path); if (existing.length > 8) { completed++; continue } } catch { /* First import. */ }
    let lastError
    for (let attempt = 0; attempt < 3; attempt++) {
      try {
        const response = await fetch(item.source, { signal: AbortSignal.timeout(20_000) })
        if (!response.ok) throw new Error(`HTTP ${response.status}`)
        const body = Buffer.from(await response.arrayBuffer())
        if (body.readUInt32BE(0) !== 0x89504e47) throw new Error('Not a PNG')
        await writeFile(path, body); downloaded++; bytes += body.length; lastError = undefined; break
      } catch (error) { lastError = error; await new Promise(resolve => setTimeout(resolve, (attempt + 1) * 700)) }
    }
    if (lastError) failed.push({ key: item.key, error: lastError.message })
    if (++completed % 300 === 0) console.log(`${completed}/${catalogue.length} emoji images ready`)
  }
}))
console.log(JSON.stringify({ catalogue: catalogue.length, downloaded, bytes, failed, directory: fileURLToPath(images) }))
if (failed.length) process.exitCode = 1
