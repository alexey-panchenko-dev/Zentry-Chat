import { test } from 'node:test'
import assert from 'node:assert/strict'
import { registerHooks } from 'node:module'

// Keep the real controller; replace network and browser devices at its boundaries.
registerHooks({
  resolve(specifier, context, next) {
    if (specifier === '@/api') return { url: 'data:text/javascript,export const api=globalThis.callTestAPI;export const errorText=e=>e.message', shortCircuit: true }
    if (specifier === 'pusher-js') return { url: 'data:text/javascript,export default class Pusher {}', shortCircuit: true }
    return next(specifier, context)
  },
})
const tracks = () => [{ kind: 'audio', enabled: true, stopped: false, stop() { this.stopped = true } }, { kind: 'video', enabled: true, stopped: false, stop() { this.stopped = true } }]
class Stream {
  constructor(items = []) { this.items = items }
  getTracks() { return this.items }
  addTrack(track) { this.items.push(track) }
}
class Peer {
  static instances = []
  constructor() { this.added = []; Peer.instances.push(this) }
  addTrack() {}
  async createOffer() { return { type: 'offer', sdp: 'v=0 offer' } }
  async createAnswer() { return { type: 'answer', sdp: 'v=0 answer' } }
  async setLocalDescription(value) { this.localDescription = value }
  async setRemoteDescription(value) { this.remoteDescription = value }
  async addIceCandidate(value) { assert.ok(this.remoteDescription); this.added.push(value) }
  close() { this.closed = true }
}
globalThis.MediaStream = Stream
globalThis.RTCPeerConnection = Peer
Object.defineProperty(globalThis.navigator, 'mediaDevices', { configurable: true, value: { async getUserMedia() { return new Stream(tracks()) } } })
let current, signals, sent
globalThis.callTestAPI = {
  async get(url) {
    if (url === '/calls') return { data: { calls: current ? [current] : [] } }
    if (url.endsWith('/signals')) return { data: { signals, has_more: false } }
    return { data: { call: { ...current } } }
  },
  async post(url, body) {
    if (url.endsWith('/signals')) { sent.push(body); return { data: {} } }
    if (url.endsWith('/actions')) current = { ...current, status: body.action === 'accept' ? 'accepted' : 'ended', version: current.version + 1 }
    return { data: { call: { ...current } } }
  },
}
const { CallController } = await import('../src/calls.ts')
function reset(caller = 'me') {
  Peer.instances = []; sent = []; signals = []
  current = { id: 'call', conversation_id: 'dialog', caller_id: caller, callee_id: caller === 'me' ? 'peer' : 'me', status: 'ringing', version: 1 }
}

test('Caller offers once, applies answer, toggles devices and releases tracks on end', async () => {
  reset()
  const client = new CallController('me', () => {})
  await client.start('dialog')
  const localTracks = client.view.local.getTracks()
  current = { ...current, status: 'accepted', version: 2 }
  signals = [{ id: '1', type: 'answer', payload: { type: 'answer', sdp: 'v=0 answer' } }]
  await client.sync()
  signals = []
  await client.sync()
  assert.equal(sent.filter((s) => s.type === 'offer').length, 1)
  assert.equal(Peer.instances[0].remoteDescription.type, 'answer')
  client.toggle('audio')
  assert.equal(localTracks[0].enabled, false)
  await client.finish()
  assert.ok(localTracks.every((track) => track.stopped))
  assert.equal(Peer.instances[0].closed, true)
  assert.equal(client.view.call.status, 'ended')
  client.dispose()
})

test('Callee queues early ICE, accepts offer and sends answer', async () => {
  reset('peer')
  const client = new CallController('me', () => {})
  await client.sync()
  signals = [
    { id: '1', type: 'ice', payload: { candidate: 'candidate:test' } },
    { id: '2', type: 'offer', payload: { type: 'offer', sdp: 'v=0 offer' } },
  ]
  await client.action('accept')
  assert.equal(sent.filter((s) => s.type === 'answer').length, 1)
  assert.equal(Peer.instances[0].added.length, 1)
  signals = []
  await client.finish()
  client.dispose()
})

test('Permission finishing after a cancelled call does not leave camera running', async () => {
  reset()
  let resolveMedia
  const stream = new Stream(tracks())
  navigator.mediaDevices.getUserMedia = () => new Promise((resolve) => { resolveMedia = resolve })
  const client = new CallController('me', () => {})
  const starting = client.start('dialog')
  while (!resolveMedia) await new Promise((resolve) => setImmediate(resolve))
  current = { ...current, status: 'ended', version: 2 }
  await client.sync()
  resolveMedia(stream)
  await starting
  assert.ok(stream.getTracks().every((track) => track.stopped))
  assert.equal(client.view.local, null)
  client.dispose()
})

test('Remote browser stream retains both audio and video tracks', async () => {
  reset()
  navigator.mediaDevices.getUserMedia = async () => new Stream(tracks())
  const client = new CallController('me', () => {})
  await client.start('dialog')
  current = { ...current, status: 'accepted', version: 2 }
  await client.sync()
  const peer = Peer.instances[0]
  const incoming = new Stream(tracks())
  for (const track of incoming.getTracks()) peer.ontrack({ track, streams: [incoming] })
  assert.equal(client.view.remote, incoming)
  assert.deepEqual(client.view.remote.getTracks().map((track) => track.kind), ['audio', 'video'])
  await client.finish()
  client.dispose()
})

test('Separate browser stream associations never replace received audio with video', async () => {
  reset()
  navigator.mediaDevices.getUserMedia = async () => new Stream(tracks())
  const client = new CallController('me', () => {})
  await client.start('dialog')
  current = { ...current, status: 'accepted', version: 2 }
  await client.sync()
  const [audio, video] = tracks()
  audio.id = 'remote-audio'; video.id = 'remote-video'
  const peer = Peer.instances[0]
  peer.ontrack({ track: audio, streams: [new Stream([audio])] })
  peer.ontrack({ track: video, streams: [new Stream([video])] })
  assert.deepEqual(client.view.remote.getTracks().map(t => t.kind), ['audio', 'video'])
  await client.finish()
  client.dispose()
})
