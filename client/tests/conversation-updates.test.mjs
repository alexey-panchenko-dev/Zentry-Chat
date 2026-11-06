import { test } from 'node:test'
import assert from 'node:assert/strict'
import { registerHooks } from 'node:module'

registerHooks({
  resolve(specifier, context, next) {
    if (specifier === '@/api') return { url: 'data:text/javascript,export const api=globalThis.conversationAPI;export const errorText=e=>e.message', shortCircuit: true }
    if (specifier === 'pusher-js') return { url: 'data:text/javascript,export default globalThis.ConversationPusher', shortCircuit: true }
    return next(specifier, context)
  },
})
let client, loadConfig
let created=0, configReads=0
globalThis.ConversationPusher = class {
  handlers = {}
  channels = new Map()
  connection = { bind: (event,fn) => {this.state=fn}, unbind: () => {} }
  constructor() { client = this; created++ }
  subscribe(name) {
    this.channel = name
    if(!this.channels.has(name)) this.channels.set(name, { handlers:new Map(), bind(event,fn){if(!this.handlers.has(event))this.handlers.set(event,new Set());this.handlers.get(event).add(fn)}, unbind(event,fn){this.handlers.get(event)?.delete(fn)} })
    const channel=this.channels.get(name)
    this.handlers = new Proxy({}, {get:(_,event)=>(data)=>{for(const fn of channel.handlers.get(event)??[])fn(data)}})
    return channel
  }
  unsubscribe(name){this.channels.delete(name)}
  disconnect() { this.disconnected = true }
}
globalThis.conversationAPI = { get: () => {configReads++;return loadConfig()} }
const { watchConversations } = await import('../src/conversation-updates.ts')
const ready = () => new Promise(resolve => setImmediate(resolve))

test('discovers dialogs without an open chat and catches up on every subscription', async () => {
  loadConfig = async () => ({ data: { enabled: true, key: 'key', cluster: 'eu' } })
  let refreshes = 0
  const stop = watchConversations('recipient', () => refreshes++)
  await ready()
  assert.equal(client.channel, 'private-user-recipient')
  client.handlers['pusher:subscription_succeeded']()
  client.handlers['conversation.updated']({ conversation_id: 'new-dialog' })
  client.handlers['pusher:subscription_succeeded']()
  assert.equal(refreshes, 3)
  stop()
  assert.equal(client.disconnected, true)
  client.handlers['conversation.updated']({ conversation_id: 'late' })
  assert.equal(refreshes, 3)
})

test('logout before config loads never subscribes to the previous account', async () => {
  let resolve
  loadConfig = () => new Promise(done => { resolve = done })
  client = undefined
  const stop = watchConversations('previous-user', () => assert.fail('refresh after logout'))
  stop()
  resolve({ data: { enabled: true } })
  await ready()
  assert.equal(client, undefined)
})

test('disabled realtime or config failure leaves HTTP recovery available', async () => {
  for (const config of [async () => ({ data: { enabled: false } }), async () => { throw new Error('offline') }]) {
    loadConfig = config
    client = undefined
    const stop = watchConversations('recipient', () => assert.fail('unexpected refresh'))
    await ready()
    assert.equal(client, undefined)
    stop()
  }
})

test('multiple consumers share one connection and one personal subscription', async () => {
  loadConfig = async () => ({data:{enabled:true,key:'key',cluster:'eu'}})
  const beforeCreated=created,beforeConfig=configReads
  let a=0,b=0
  const stopA=watchConversations('same',()=>a++)
  const stopB=watchConversations('same',()=>b++)
  await ready()
  assert.equal(created-beforeCreated,1)
  assert.equal(configReads-beforeConfig,1)
  assert.equal(client.channels.size,1)
  client.handlers['conversation.updated']({conversation_id:'new'})
  assert.equal(a,1);assert.equal(b,1)
  stopA()
  assert.notEqual(client.disconnected,true)
  client.handlers['conversation.updated']({conversation_id:'next'})
  assert.equal(a,1);assert.equal(b,2)
  stopB()
  assert.equal(client.disconnected,true)
})

test('cold-start config timeout recovers without reloading or creating duplicate sockets', async () => {
  const originalTimer=globalThis.setTimeout, originalClear=globalThis.clearTimeout
  let retry, attempts=0, latestStatus=false
  globalThis.setTimeout=(fn,ms)=>{assert.equal(ms,60000);retry=fn;return 123}
  globalThis.clearTimeout=()=>{}
  loadConfig=async()=>{attempts++;if(attempts===1)throw Error('cold start timeout');return {data:{enabled:true,key:'key',cluster:'eu'}}}
  const before=created
  const stop=watchConversations('recovering',()=>{},ready=>latestStatus=ready)
  try {
    await ready()
    assert.equal(attempts,1);assert.equal(latestStatus,false)
    retry();await ready()
    assert.equal(attempts,2);assert.equal(created-before,1)
    client.handlers['pusher:subscription_succeeded']()
    assert.equal(latestStatus,true)
  } finally {stop();globalThis.setTimeout=originalTimer;globalThis.clearTimeout=originalClear}
})
