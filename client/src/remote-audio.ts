export type AudioPlayback = { playing: boolean; blocked: boolean; source: 'waiting' | 'muted' | 'live'; error: boolean }

// The received tracks belong to CallController. This binding owns only the
// output element and its listeners; it must never stop the remote tracks.
export function bindRemoteAudio(audio: HTMLAudioElement, stream: MediaStream | null, onState: (state: AudioPlayback) => void) {
  let disposed = false
  let attempt = 0
  let state: AudioPlayback = { playing: false, blocked: false, source: 'waiting', error: false }
  const output = new MediaStream()
  const tracked = new Set<MediaStreamTrack>()
  const emit = (patch: Partial<AudioPlayback>) => { if (!disposed) { state = { ...state, ...patch }; onState(state) } }
  const unwatch = (track: MediaStreamTrack) => {
    track.removeEventListener('mute', update)
    track.removeEventListener('unmute', retryEvent)
    track.removeEventListener('ended', update)
    tracked.delete(track)
  }
  function update() {
    if (disposed) return
    const incoming = stream?.getAudioTracks().filter(track => track.readyState !== 'ended') ?? []
    for (const track of tracked) if (!incoming.includes(track)) { output.removeTrack(track); unwatch(track) }
    for (const track of incoming) if (!tracked.has(track)) {
      tracked.add(track); output.addTrack(track)
      track.addEventListener('mute', update)
      track.addEventListener('unmute', retryEvent)
      track.addEventListener('ended', update)
    }
    if (!incoming.length) {
      attempt++ // Ignore an earlier play() that resolves after the track ended.
      audio.pause()
      emit({ source: 'waiting', playing: false, blocked: false })
    } else emit({ source: incoming.every(track => track.muted) ? 'muted' : 'live' })
  }
  async function retry() {
    if (disposed) return
    update()
    if (!output.getAudioTracks().length) return
    const current = ++attempt
    audio.muted = false; audio.volume = 1
    try {
      await audio.play()
      if (!disposed && current === attempt) emit({ playing: !audio.paused, blocked: false, error: false })
    } catch (error) {
      if (!disposed && current === attempt) emit({ playing: false, blocked: (error as {name?:string}).name === 'NotAllowedError', error: true })
    }
  }
  function retryEvent() { void retry() }
  const playing = () => { if (output.getAudioTracks().length) emit({ playing: true, blocked: false, error: false }) }
  const pause = () => emit({ playing: false })
  const failed = () => emit({ playing: false, error: true })
  audio.srcObject = output
  audio.muted = false; audio.volume = 1
  audio.addEventListener('playing', playing)
  audio.addEventListener('pause', pause)
  audio.addEventListener('error', failed)
  stream?.addEventListener('addtrack', retryEvent)
  stream?.addEventListener('removetrack', update)
  void retry()
  return {
    retry,
    dispose() {
      if (disposed) return
      disposed = true; attempt++
      stream?.removeEventListener('addtrack', retryEvent)
      stream?.removeEventListener('removetrack', update)
      for (const track of tracked) unwatch(track)
      audio.removeEventListener('playing', playing)
      audio.removeEventListener('pause', pause)
      audio.removeEventListener('error', failed)
      audio.pause(); audio.srcObject = null
    },
  }
}
