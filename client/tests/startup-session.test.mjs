import { test } from 'node:test'
import assert from 'node:assert/strict'
import { startSessionRequest, takeStartupSession } from '../src/startup-session.ts'

test('early authentication shares one request and cannot reuse a consumed account', async () => {
  const original = globalThis.fetch
  let calls = 0
  globalThis.fetch = async (url, options) => {
    calls++
    assert.equal(url, '/api/auth/me')
    assert.equal(options.cache, 'no-store')
    assert.equal(options.credentials, 'same-origin')
    return { status: 200, ok: true, json: async () => ({ user: { id: String(calls), name: 'test' } }) }
  }
  try {
    const first = startSessionRequest()
    assert.equal(startSessionRequest(), first)
    assert.equal(takeStartupSession(), first)
    assert.equal(takeStartupSession(), undefined)
    assert.equal((await first).id, '1')
    assert.equal((await startSessionRequest()).id, '2')
    takeStartupSession()
  } finally { globalThis.fetch = original }
})
test('unauthorized startup opens login, while server errors remain errors', async () => {
  const original = globalThis.fetch
  try {
    globalThis.fetch = async () => ({ status: 401, ok: false })
    assert.equal(await startSessionRequest(), null)
    takeStartupSession()
    globalThis.fetch = async () => ({ status: 503, ok: false })
    await assert.rejects(startSessionRequest(), /проверить вход/)
    takeStartupSession()
  } finally { globalThis.fetch = original }
})
