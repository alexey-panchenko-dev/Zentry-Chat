import { subscribeRealtime } from './realtime.ts'
import { api, errorText } from '@/api'
import { AudioMeter } from './audio-meter.ts'

export type Call = {
  id: string; conversation_id: string; caller_id: string; callee_id: string
  status: string; version: number; expires_at: string
}
export type CallView = {
  call: Call | null; local: MediaStream | null; remote: MediaStream | null
  connection: string; error: string; busy: boolean
  audio?: { sent: number; received: number; inputLevel: number | null; inputEnabled: boolean; inputMuted: boolean }
  cameraBusy?: boolean
  cameraError?: string
}
type Signal = { id: string; type: 'offer' | 'answer' | 'ice'; payload: RTCSessionDescriptionInit & RTCIceCandidateInit }
const active = (call: Call | null) => call?.status === 'ringing' || call?.status === 'accepted'
class MediaCancelled extends Error {}

function deviceError(error: unknown, camera = false) {
  const name = (error as { name?: string })?.name
  const device = camera ? 'камере' : 'микрофону'
  if (name === 'NotAllowedError' || name === 'PermissionDeniedError') return new Error(`Разрешите доступ к ${device} в настройках браузера.`)
  if (name === 'NotFoundError' || name === 'DevicesNotFoundError') return new Error(camera ? 'Камера не найдена. Можно продолжить разговор без видео.' : 'Микрофон не найден. Подключите его и повторите попытку.')
  if (name === 'NotReadableError' || name === 'TrackStartError') return new Error(camera ? 'Камера недоступна или занята другим приложением.' : 'Микрофон недоступен или занят другим приложением.')
  return new Error(camera ? 'Не удалось включить камеру. Можно продолжить разговор без видео.' : 'Не удалось включить микрофон. Проверьте устройство и разрешение браузера.')
}

export class CallController {
  view: CallView = { call: null, local: null, remote: null, connection: 'Подключаем канал звонков…', error: '', busy: false }
  private userID: string
  private update: (view: CallView) => void
  private disposed = false
  private stopRealtime?: () => void
  private realtimeReady = false
  private stopFocus?: () => void
  private timer?: ReturnType<typeof setInterval>
  private pc?: RTCPeerConnection
  private cursor = '0'
  private candidates: RTCIceCandidateInit[] = []
  private syncing = false
  private rerun = false
  private outgoing: Promise<void> = Promise.resolve()
  private offerSent = false
  private cancelMedia?: () => void
  private cancelCamera?: () => void
  private generation = 0
  private videoSender?: RTCRtpSender
  private finishing = false
  private inputMeter = new AudioMeter()
  private audioTimer?: ReturnType<typeof setInterval>

  constructor(userID: string, update: (view: CallView) => void) { this.userID = userID; this.update = update }
  private emit(patch: Partial<CallView>) {
    if (this.disposed) return
    this.view = { ...this.view, ...patch }; this.update(this.view)
  }
  private failure(error: unknown) { this.emit({ error: errorText(error) }) }

  async connect() {
    if (typeof window !== 'undefined') {
      const recover = () => { if (!this.disposed) void this.sync() }
      window.addEventListener('focus', recover)
      this.stopFocus = () => window.removeEventListener('focus', recover)
    }
    this.timer = setInterval(() => {
      if (this.disposed || this.realtimeReady) return
      if (typeof document !== 'undefined' && document.visibilityState === 'hidden' && !active(this.view.call)) return
      void this.sync()
    }, 30_000)
    this.stopRealtime = subscribeRealtime('private-user-' + this.userID, {
      'call.updated': () => void this.sync(),
      'call.signal': () => void this.sync(),
    }, ready => {
      this.realtimeReady = ready
      this.emit({ connection: ready ? 'Канал звонков подключён' : 'Связь восстанавливается. Резервная проверка раз в 30 секунд.' })
      if (ready) void this.sync()
    })
    await this.sync()
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
        const generation = this.generation
        if (active(old)) {
          const { data } = await api.get<{ call: Call }>('/calls/' + old!.id)
          if (this.disposed) return
          if (this.view.call?.id !== old?.id || generation !== this.generation) { this.rerun = true; continue }
          this.apply(data.call)
        } else {
          const { data } = await api.get<{ calls: Call[] }>('/calls')
          if (this.disposed) return
          if (this.view.call?.id !== old?.id || generation !== this.generation) { this.rerun = true; continue }
          if (data.calls[0]) this.apply(data.calls[0])
        }
        const call = this.view.call
        if (call?.status === 'accepted' && this.view.local) {
          const mediaGeneration = this.generation
          try {
            await this.ensurePeer(call)
            await this.receive(call)
            await this.inspectAudio()
          } catch (error) {
            if (mediaGeneration === this.generation) throw error
          }
        }
      } while (this.rerun && !this.disposed)
    } catch (error) {
      // A concurrent end can make the signal endpoint return 409.
      if ((error as { response?: { status: number } }).response?.status !== 409) this.failure(error)
    } finally { this.syncing = false }
  }

  private async media() {
    if (this.view.local) return
    if (!navigator.mediaDevices?.getUserMedia) throw new Error('Микрофон доступен на localhost или HTTPS.')
    const callID = this.view.call?.id
    const generation = this.generation
    let cancelled = false
    let cancel!: () => void
    const cancellation = new Promise<never>((_, reject) => { cancel = () => { cancelled = true; reject(new MediaCancelled()) } })
    this.cancelMedia = cancel
    const acquisition = Promise.resolve().then(() => {
      if (cancelled) throw new MediaCancelled()
      return navigator.mediaDevices.getUserMedia({ audio: true, video: false })
    }).catch(error => { if (error instanceof MediaCancelled) throw error; throw deviceError(error) }).then(stream => {
      if (cancelled || this.disposed || generation !== this.generation || this.view.call?.id !== callID || !active(this.view.call)) {
        stream.getTracks().forEach(track => track.stop())
        throw new MediaCancelled()
      }
      if (!stream.getAudioTracks().some(track => track.readyState !== 'ended')) {
        stream.getTracks().forEach(track => track.stop())
        throw new Error('Микрофон не передаёт звук. Подключите его и повторите попытку.')
      }
      for (const track of stream.getAudioTracks()) track.onended = () => {
        if (generation !== this.generation || this.disposed) return
        this.failure(new Error('Микрофон отключён. Подключите его и позвоните заново.'))
        void this.finish()
      }
      return stream
    })
    try { this.emit({ local: await Promise.race([acquisition, cancellation]) }) }
    finally { if (this.cancelMedia === cancel) this.cancelMedia = undefined }
  }

  async start(conversationID: string) {
    if (this.disposed || this.view.busy || active(this.view.call)) return
    this.inputMeter.unlock()
    this.emit({ busy: true, error: '', cameraError: '' })
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
    if (this.disposed || !call || this.view.busy) return
    if (action === 'accept') this.inputMeter.unlock()
    this.emit({ busy: true, error: '' })
    try {
      if (action === 'accept') await this.media()
      if (this.disposed || this.view.call?.id !== call.id || !active(this.view.call)) return
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
      this.audioTimer = setInterval(() => void this.inspectAudio(), 1000)
      // Node controller tests should not be kept alive by a failed assertion.
      ;(this.audioTimer as unknown as {unref?:()=>void}).unref?.()
      const remote = new MediaStream()
      this.emit({ remote, connection: 'Устанавливаем соединение…' })
      this.view.local.getAudioTracks().forEach((track) => pc.addTrack(track, this.view.local!))
      // Reserve a video section in SDP even for microphone-only calls. This
      // allows replaceTrack() to enable video later without renegotiation.
      if (call.caller_id === this.userID) {
        const video = this.view.local.getVideoTracks()[0]
        this.videoSender = pc.addTransceiver(video ?? 'video', { direction: 'sendrecv', streams: [this.view.local] }).sender
      }
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
        const labels: Record<string, string> = { connected: 'Соединение установлено', connecting: 'Соединяемся…', disconnected: 'Связь прервана. Ждём восстановления…', failed: 'Соединение не удалось. Для разных сетей может понадобиться TURN.' }
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
      const pc = this.pc
      const { data } = await api.get<{ signals: Signal[]; next_cursor: string; has_more: boolean }>('/calls/' + call.id + '/signals', { params: { after: this.cursor } })
      if (this.pc !== pc || this.disposed || this.view.call?.id !== call.id || this.view.call.status !== 'accepted') return
      for (const signal of data.signals) {
        if (signal.type === 'ice') {
          if (pc.remoteDescription) await pc.addIceCandidate(signal.payload)
          else this.candidates.push(signal.payload)
        } else {
          await pc.setRemoteDescription(signal.payload)
          if (this.pc !== pc) return
          for (const candidate of this.candidates) {
            await pc.addIceCandidate(candidate)
            if (this.pc !== pc) return
          }
          this.candidates = []
          if (signal.type === 'offer') {
            // The answerer must use the video transceiver created by the remote
            // offer. Adding one beforehand can leave an extra unnegotiated
            // sender, so its camera never reaches the caller.
            const video = pc.getTransceivers().find(item => item.receiver.track.kind === 'video')
            if (video && this.view.local) {
              video.direction = 'sendrecv'
              this.videoSender = video.sender
              video.sender.setStreams?.(this.view.local)
              await video.sender.replaceTrack(this.view.local.getVideoTracks()[0] ?? null)
              if (this.pc !== pc) return
            }
            await pc.setLocalDescription(await pc.createAnswer())
            if (this.pc !== pc) return
            await this.send(call, 'answer', { type: 'answer', sdp: pc.localDescription!.sdp })
          }
        }
        this.cursor = signal.id
      }
      more = data.has_more
    }
  }

  toggle(kind: 'audio' | 'video') {
    if (kind === 'video') { void this.toggleCamera(); return }
    this.inputMeter.unlock()
    const tracks = this.view.local?.getTracks().filter((track) => track.kind === kind) ?? []
    tracks.forEach((track) => { track.enabled = !track.enabled })
    this.emit({})
  }

  async toggleCamera() {
    const call = this.view.call
    const local = this.view.local
    if (!local || !active(call) || this.disposed || this.view.cameraBusy) return
    const current = local.getVideoTracks()
    if (current.length) {
      this.emit({ cameraBusy: true, cameraError: '' })
      const generation = this.generation
      // Stop capture immediately, even if detaching the sender fails.
      current.forEach(track => { track.onended = null; track.stop() })
      this.emit({ local: new MediaStream(local.getAudioTracks()) })
      try {
        await this.videoSender?.replaceTrack(null)
      } catch { if (generation === this.generation) this.emit({ cameraError: 'Камера выключена, но не удалось обновить видеоканал.' }) }
      finally { if (generation === this.generation) this.emit({cameraBusy:false}) }
      return
    }
    const generation = this.generation
    let cancelled = false
    let cancel!: () => void
    const cancellation = new Promise<never>((_, reject) => { cancel = () => { cancelled = true; reject(new MediaCancelled()) } })
    this.cancelCamera = cancel
    this.emit({ cameraBusy: true, cameraError: '' })
    let acquired: MediaStream | undefined
    const acquisition = Promise.resolve().then(() => {
      if (cancelled) throw new MediaCancelled()
      return navigator.mediaDevices.getUserMedia({audio:false,video:true})
    }).then(stream => {
      if (cancelled || this.disposed || generation !== this.generation || !active(this.view.call)) {
        stream.getTracks().forEach(track => track.stop())
        throw new MediaCancelled()
      }
      acquired = stream
      return stream
    })
    try {
      const stream = await Promise.race([acquisition,cancellation])
      const track = stream.getVideoTracks().find(track => track.readyState !== 'ended')
      if (!track) throw new Error('no video track')
      stream.getTracks().filter(other => other !== track).forEach(other => other.stop())
      await this.videoSender?.replaceTrack(track)
      if (generation !== this.generation || this.disposed) { stream.getTracks().forEach(track => track.stop()); return }
      track.onended = () => {
        if (generation !== this.generation || this.disposed) return
        void this.toggleCamera()
      }
      this.emit({local:new MediaStream([...local.getAudioTracks(),track])})
    } catch (error) {
      acquired?.getTracks().forEach(track => track.stop())
      if (!(error instanceof MediaCancelled) && generation === this.generation) this.emit({cameraError:deviceError(error,true).message})
    } finally {
      if (this.cancelCamera === cancel) this.cancelCamera = undefined
      if (generation === this.generation) this.emit({cameraBusy:false})
    }
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
      if (this.view.local) inputLevel = this.inputMeter.sample(this.view.local) ?? inputLevel
      this.emit({ audio: { sent, received, inputLevel, inputEnabled: tracks.some(t => t.enabled && t.readyState === 'live'), inputMuted: tracks.length === 0 || tracks.every(t => t.muted) } })
    } catch { /* Diagnostics must not interrupt a working call. */ }
  }
  private release() {
    if(this.audioTimer!==undefined) clearInterval(this.audioTimer)
    this.audioTimer=undefined
    this.inputMeter.disconnect()
    this.generation++
    this.cancelMedia?.(); this.cancelMedia = undefined
    this.cancelCamera?.(); this.cancelCamera = undefined
    this.pc?.close(); this.pc = undefined
    this.videoSender = undefined
    this.view.local?.getTracks().forEach((track) => {track.onended=null;track.stop()})
    this.candidates = []; this.offerSent = false; this.outgoing = Promise.resolve()
    this.emit({ local: null, remote: null, audio: undefined, cameraBusy:false, cameraError:'' })
  }
  private abandon(call: Call) {
    const action = call.status === 'accepted' ? 'end' : call.caller_id === this.userID ? 'cancel' : 'reject'
    void fetch('/api/calls/' + call.id + '/actions', {
      method: 'POST', credentials: 'include', keepalive: true,
      headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ action }),
    }).catch(() => {})
  }
  dispose() {
    if (this.disposed) return
    this.disposed = true
    clearInterval(this.timer); this.stopRealtime?.(); this.stopFocus?.()
    if (active(this.view.call)) this.abandon(this.view.call!)
    this.release()
    this.inputMeter.dispose()
  }
}


