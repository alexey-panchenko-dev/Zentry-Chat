import { readFile, writeFile } from 'node:fs/promises'

const value = process.argv[2]
if (!value) throw new Error('Usage: npm run configure:hosting -- https://YOUR-API.onrender.com')
const api = new URL(value)
if (api.protocol !== 'https:' || api.username || api.password || api.search || api.hash || api.pathname !== '/') {
  throw new Error('Use the HTTPS origin of your API, without paths, credentials or query parameters.')
}
const configPath = new URL('../vercel.json', import.meta.url)
const existing = JSON.parse(await readFile(configPath, 'utf8'))
const config = {
  ...existing,
  $schema: 'https://openapi.vercel.sh/vercel.json',
  framework: 'vite',
  buildCommand: 'npm run build',
  outputDirectory: 'dist',
  rewrites: [
    { source: '/api/:path*', destination: api.origin + '/api/:path*' },
    { source: '/:path*', destination: '/index.html' },
  ],
}
await writeFile(configPath, JSON.stringify(config, null, 2) + '\n')
await writeFile(new URL('../../vercel.json', import.meta.url), JSON.stringify({
  ...config,
  installCommand: 'npm --prefix client ci',
  buildCommand: 'npm --prefix client run build',
  outputDirectory: 'client/dist',
}, null, 2) + '\n')
console.log('Updated both Vercel root layouts, preserving security headers. API:', api.origin)
