import Pusher from 'pusher-js'
import { api, errorText } from '@/api'

export type Call = {
  id: string; conversation_id: string; caller_id: string; callee_id: string
  status: string; version: number; expires_at: string
}
export type CallView = {
  call: Call | null; local: MediaStream | null; remote: MediaStream | null
  connection: string; error: string; busy: boolean
  audio?: { sent: number; received: number; inputLevel: number | null; inputEnabled: boolean; inputMuted: boolean }
}
type Signal = { id: string; type: 'offer' | 'answer' | 'ice'; payload: RTCSessionDescriptionInit & RTCIceCandidateInit }
const active = (call: Call | null) => call?.status === 'ringing' || call?.status === 'accepted'
class MediaCancelled extends Error {}

export class CallController {
  view: CallView = { call: null, local: null, remote: null, connection: 'Подключаем канал звонков…', error: '', busy: false }
  private userID: string
  private update: (view: CallView) => void
  private disposed = false
  private pusher?: Pusher
  private timer?: ReturnType<typeof setInterval>
  private pc?: RTCPeerConnection
  private cursor = '0'
  private candidates: RTCIceCandidateInit[] = []
  private syncing = false
  private rerun = false
  private outgoing: Promise<void> = Promise.resolve()
  private offerSent = false
  private cancelMedia?: () => void
  private finishing = false

  constructor(userID: string, update: (view: CallView) => void) { this.userID = userID; this.update = update }
  private emit(patch: Partial<CallView>) {
    if (this.disposed) return
    this.view = { ...this.view, ...patch }; this.update(this.view)
  }
  private failure(error: unknown) { this.emit({ error: errorText(error) }) }

  async connect() {
    this.timer = setInterval(() => void this.sync(), 4000)
    await this.sync()
    try {
      const { data } = await api.get('/realtime/config')
      if (this.disposed) return
      if (!data.enabled) { this.emit({ connection: 'Pusher отключён. Используется проверка состояния каждые 4 секунды.' }); return }
      this.pusher = new Pusher(data.key, {
        cluster: data.cluster, forceTLS: true,
        channelAuthorization: { customHandler: ({ socketId, channelName }, callback) => {
          void api.post('/pusher/auth', { socket_id: socketId, channel_name: channelName })
            .then(({ data }) => { if (!this.disposed) callback(null, data) })
            .catch((error: unknown) => { if (!this.disposed) callback(new Error(errorText(error)), null) })
        } },
      })
      const channel = this.pusher.subscribe('private-user-' + this.userID)
      channel.bind('pusher:subscription_succeeded', () => {
        this.emit({ connection: 'Канал звонков подключён' }); void this.sync()
      })
      channel.bind('pusher:subscription_error', () => this.emit({ connection: 'Ошибка подписки. Проверяем состояние через API.' }))
      this.pusher.connection.bind('state_change', ({ current }: { current: string }) => {
        if (current !== 'connected') this.emit({ connection: 'Pusher: ' + current })
      })
      channel.bind('call.updated', () => void this.sync())
      channel.bind('call.signal', () => void this.sync())
    } catch (error) { this.failure(error) }
  }

  private apply(call: Call) {
    const old = this.view.call
    if (old?.id === call.id && old.version > call.version) return
    if (old?.id !== call.id) { this.release(); this.cursor = '0' }
    this.emit({ call })
    if (!active(call)) this.release()
  }

  async sync() {
    if (this.disposed) return
    if (this.syncing) { this.rerun = true; return }
    this.syncing = true
    try {
      do {
        this.rerun = false
        const old = this.view.call
        if (active(old)) {
          const { data } = await api.get<{ call: Call }>('/calls/' + old!.id)
          if (this.disposed) return
          this.apply(data.call)
        } else {
          const { data } = await api.get<{ calls: Call[] }>('/calls')
          if (this.disposed) return
          if (data.calls[0]) this.apply(data.calls[0])
        }
        const call = this.view.call
        if (call?.status === 'accepted' && this.view.local) {
          await this.ensurePeer(call)
          await this.receive(call)
          await this.inspectAudio()
        }
      } while (this.rerun && !this.disposed)
    } catch (error) {
      // A concurrent end can make the signal endpoint return 409.
      if ((error as { response?: { status: number } }).response?.status !== 409) this.failure(error)
    } finally { this.syncing = false }
  }

  private async media() {
    if (this.view.local) return
    if (!navigator.mediaDevices?.getUserMedia) throw new Error('Камера доступна на localhost или HTTPS.')
    const callID = this.view.call?.id
    let cancelled = false
    let cancel!: () => void
    const cancellation = new Promise<never>((_, reject) => { cancel = () => { cancelled = true; reject(new MediaCancelled()) } })
    this.cancelMedia = cancel
    const acquisition = navigator.mediaDevices.getUserMedia({ audio: true, video: true }).then(stream => {
      if (cancelled || this.disposed || this.view.call?.id !== callID || !active(this.view.call)) {
        stream.getTracks().forEach(track => track.stop())
        throw new MediaCancelled()
      }
      return stream
    })
    try { this.emit({ local: await Promise.race([acquisition, cancellation]) }) }
    finally { if (this.cancelMedia === cancel) this.cancelMedia = undefined }
  }

  async start(conversationID: string) {
    if (this.view.busy || active(this.view.call)) return
    this.emit({ busy: true, error: '' })
    try {
      // Reserve the call first; applying a new call resets resources.
      const id = crypto.randomUUID()
      const { data } = await api.post<{ call: Call }>('/conversations/' + conversationID + '/calls', { client_call_id: id })
      if (this.disposed) { this.abandon(data.call); return }
      this.apply(data.call)
      await this.media()
      await this.sync()
    } catch (error) {
      if (error instanceof MediaCancelled) return
      this.failure(error)
      if (active(this.view.call)) await this.finish()
      else this.release()
    } finally { this.emit({ busy: false }) }
  }

  async action(action: 'accept' | 'reject' | 'cancel' | 'end') {
    const call = this.view.call
    if (!call || this.view.busy) return
    this.emit({ busy: true, error: '' })
    try {
      if (action === 'accept') await this.media()
      const { data } = await api.post<{ call: Call }>('/calls/' + call.id + '/actions', { action })
      if (this.disposed) return
      this.apply(data.call)
      await this.sync()
    } catch (error) {
      if (error instanceof MediaCancelled) return
      this.failure(error)
      if (action === 'accept') this.release()
      void this.sync()
    } finally { this.emit({ busy: false }) }
  }

  async finish() {
    if (this.finishing) return
    this.finishing = true
    const call = this.view.call
    this.release()
    if (!active(call)) { this.finishing = false; return }
    try {
      const action = call!.status === 'accepted' ? 'end' : call!.caller_id === this.userID ? 'cancel' : 'reject'
      const { data } = await api.post<{ call: Call }>('/calls/' + call!.id + '/actions', { action })
      this.apply(data.call)
    } catch (error) { this.failure(error) }
    finally { this.finishing = false }
  }

  private send(call: Call, type: string, payload: object) {
    const body = { client_signal_id: crypto.randomUUID(), type, payload }
    const job = this.outgoing.then(async () => {
      if (this.disposed || this.view.call?.id !== call.id || this.view.call.status !== 'accepted') return
      // Same UUID on a network retry prevents duplicated signals.
      try { await api.post('/calls/' + call.id + '/signals', body) }
      catch (error) {
        if ((error as { response?: unknown }).response) throw error
        await api.post('/calls/' + call.id + '/signals', body)
      }
    })
    this.outgoing = job.catch((error: unknown) => this.failure(error))
    return job
  }

  private async ensurePeer(call: Call) {
    if (this.disposed || !this.view.local) return
    if (!this.pc) {
      const pc = new RTCPeerConnection({ iceServers: [{ urls: 'stun:stun.l.google.com:19302' }] })
      this.pc = pc
      const remote = new MediaStream()
      this.emit({ remote, connection: 'Устанавливаем видео соединение…' })
      this.view.local.getTracks().forEach((track) => pc.addTrack(track, this.view.local!))
      pc.ontrack = (event) => {
        if (this.pc !== pc) return
        const sameTrack = (a: MediaStreamTrack, b: MediaStreamTrack) => a === b || (!!a.id && a.id === b.id)
        if (!remote.getTracks().some(track => sameTrack(track, event.track))) remote.addTrack(event.track)
        // Browsers may associate audio/video with different streams. Only use
        // the associated stream if it contains every track already received.
        const associated = event.streams[0]
        const complete = associated && remote.getTracks().every(track => associated.getTracks().some(other => sameTrack(track, other)))
        // addTrack() does not dispatch addtrack for a locally constructed
        // stream. Publish a snapshot so React rebinds when late audio arrives.
        this.emit({ remote: complete ? associated : new MediaStream(remote.getTracks()) })
      }
      pc.onicecandidate = (event) => {
        if (this.pc === pc && event.candidate) void this.send(call, 'ice', event.candidate.toJSON()).catch(() => {})
      }
      pc.onconnectionstatechange = () => {
        if (this.pc !== pc) return
        const labels: Record<string, string> = { connected: 'Видео соединение установлено', connecting: 'Соединяемся…', disconnected: 'Связь прервана. Ждём восстановления…', failed: 'Соединение не удалось. Для разных сетей может понадобиться TURN.' }
        this.emit({ connection: labels[pc.connectionState] ?? pc.connectionState })
        if (pc.connectionState === 'failed') void this.finish()
      }
    }
    if (call.caller_id === this.userID && !this.offerSent) {
      this.offerSent = true
      const pc = this.pc
      await pc.setLocalDescription(await pc.createOffer())
      if (this.pc !== pc) return
      await this.send(call, 'offer', { type: 'offer', sdp: pc.localDescription!.sdp })
    }
  }

  private async receive(call: Call) {
    let more = true
    while (more && !this.disposed && this.pc) {
      const { data } = await api.get<{ signals: Signal[]; next_cursor: string; has_more: boolean }>('/calls/' + call.id + '/signals', { params: { after: this.cursor } })
      const pc = this.pc
      if (!pc || this.disposed || this.view.call?.id !== call.id || this.view.call.status !== 'accepted') return
      for (const signal of data.signals) {
        if (signal.type === 'ice') {
          if (pc.remoteDescription) await pc.addIceCandidate(signal.payload)
          else this.candidates.push(signal.payload)
        } else {
          await pc.setRemoteDescription(signal.payload)
          for (const candidate of this.candidates) await pc.addIceCandidate(candidate)
          this.candidates = []
          if (signal.type === 'offer') {
            await pc.setLocalDescription(await pc.createAnswer())
            await this.send(call, 'answer', { type: 'answer', sdp: pc.localDescription!.sdp })
          }
        }
        this.cursor = signal.id
      }
      more = data.has_more
    }
  }

  toggle(kind: 'audio' | 'video') {
    const tracks = this.view.local?.getTracks().filter((track) => track.kind === kind) ?? []
    tracks.forEach((track) => { track.enabled = !track.enabled })
    this.emit({})
  }
  private async inspectAudio() {
    const pc = this.pc
    if (!pc || typeof pc.getStats !== 'function') return
    try {
      const stats = await pc.getStats()
      if (this.pc !== pc) return
      let sent = 0, received = 0, inputLevel: number | null = null
      stats.forEach(report => {
        if ((report.kind ?? report.mediaType) !== 'audio') return
        if (report.type === 'outbound-rtp') sent += report.bytesSent ?? 0
        if (report.type === 'inbound-rtp') received += report.bytesReceived ?? 0
        if (report.type === 'media-source' && typeof report.audioLevel === 'number') inputLevel = report.audioLevel
      })
      const tracks = this.view.local?.getAudioTracks() ?? []
      this.emit({ audio: { sent, received, inputLevel, inputEnabled: tracks.some(t => t.enabled && t.readyState === 'live'), inputMuted: tracks.length === 0 || tracks.every(t => t.muted) } })
    } catch { /* Diagnostics must not interrupt a working call. */ }
  }
  private release() {
    this.cancelMedia?.(); this.cancelMedia = undefined
    this.pc?.close(); this.pc = undefined
    this.view.local?.getTracks().forEach((track) => track.stop())
    this.candidates = []; this.offerSent = false; this.outgoing = Promise.resolve()
    this.emit({ local: null, remote: null, audio: undefined })
  }
  private abandon(call: Call) {
    const action = call.status === 'accepted' ? 'end' : call.caller_id === this.userID ? 'cancel' : 'reject'
    void fetch('/api/calls/' + call.id + '/actions', {
      method: 'POST', credentials: 'include', keepalive: true,
      headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ action }),
    }).catch(() => {})
  }
  dispose() {
    this.disposed = true
    clearInterval(this.timer); this.pusher?.disconnect()
    if (active(this.view.call)) this.abandon(this.view.call!)
    this.release()
  }
}


