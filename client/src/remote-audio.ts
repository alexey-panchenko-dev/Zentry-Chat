import { acquireCallAudioContext } from './call-audio-context.ts'
export type AudioPlayback = { playing: boolean; blocked: boolean; source: 'waiting' | 'muted' | 'live'; error: boolean; mode?: 'element' | 'webaudio' }

// One binding per call player. Changing video must not pause/recreate audio.
export function bindRemoteAudio(audio: HTMLAudioElement, stream: MediaStream | null, onState: (state: AudioPlayback) => void) {
  let disposed = false, attempt = 0
  let sourceStream: MediaStream | null = null
  let output: MediaStream | null = null
  let mode: 'element' | 'webaudio' = (globalThis.AudioContext || (globalThis as typeof globalThis & {webkitAudioContext?: typeof AudioContext}).webkitAudioContext) ? 'webaudio' : 'element'
  let context: AudioContext | undefined
  let lease: ReturnType<typeof acquireCallAudioContext>
  let graph: MediaStreamAudioSourceNode | undefined
  let state: AudioPlayback = { playing:false, blocked:false, source:'waiting', error:false, mode }
  const tracked = new Set<MediaStreamTrack>()
  const emit = (patch: Partial<AudioPlayback>) => { if (!disposed) {state={...state,...patch};onState(state)} }
  function unwatch(track: MediaStreamTrack) {
    track.removeEventListener('mute',update)
    track.removeEventListener('unmute',retryEvent)
    track.removeEventListener('ended',retryEvent)
    tracked.delete(track)
  }
  function update() {
    if (disposed) return
    const incoming=sourceStream?.getAudioTracks().filter(track=>track.readyState!=='ended')??[]
    const changed=incoming.length!==tracked.size || incoming.some(track=>!tracked.has(track))
    if(changed) {
      attempt++
      for(const track of [...tracked]) unwatch(track)
      for(const track of incoming) {
        tracked.add(track)
        track.addEventListener('mute',update)
        track.addEventListener('unmute',retryEvent)
        track.addEventListener('ended',retryEvent)
      }
      graph?.disconnect();graph=undefined
      // Attach a populated stream, never an empty stream that is filled later.
      output=incoming.length?new MediaStream(incoming):null
      if(mode==='element') audio.srcObject=output
    }
    if(!incoming.length) {audio.pause();emit({source:'waiting',playing:false,blocked:false})}
    else emit({source:incoming.every(track=>track.muted)?'muted':'live'})
  }
  async function retry() {
    if(disposed) return
    update()
    if(!output) return
    const current=++attempt
    try {
      if(mode==='webaudio') {
        lease??=acquireCallAudioContext()
        context=lease?.context
        if(!context) throw new Error('Web Audio unavailable')
        // Called directly from the visible button, before awaiting anything.
        const resume=context.resume()
        audio.pause();audio.muted=true
        if(!graph) {graph=context.createMediaStreamSource(output);graph.connect(context.destination)}
        await resume
        if(disposed||current!==attempt) return
        if(context.state!=='running') throw new DOMException('Playback is suspended','NotAllowedError')
        emit({playing:true,blocked:false,error:false,mode})
      } else {
        audio.muted=false;audio.volume=1
        await audio.play()
        if(!disposed&&current===attempt) emit({playing:!audio.paused,blocked:false,error:false,mode})
      }
    } catch(error) {
      if(!disposed&&current===attempt) emit({playing:false,blocked:(error as {name?:string}).name==='NotAllowedError',error:true,mode})
    }
  }
  function retryEvent() {void retry()}
  function setStream(next: MediaStream | null) {
    if(disposed) return
    sourceStream?.removeEventListener('addtrack',retryEvent)
    sourceStream?.removeEventListener('removetrack',retryEvent)
    sourceStream=next
    sourceStream?.addEventListener('addtrack',retryEvent)
    sourceStream?.addEventListener('removetrack',retryEvent)
    const previous=output
    update()
    if(output && output!==previous) void retry()
  }
  const playing=()=>{if(mode==='element'&&output)emit({playing:true,blocked:false,error:false})}
  const pause=()=>{if(mode==='element')emit({playing:false})}
  const failed=()=>{if(mode==='element')emit({playing:false,error:true})}
  const recover=()=>{if(typeof document==='undefined'||document.visibilityState==='visible')void retry()}
  audio.setAttribute?.('playsinline','')
  audio.autoplay=true;audio.muted=false;audio.volume=1
  audio.addEventListener('playing',playing);audio.addEventListener('pause',pause);audio.addEventListener('error',failed)
  if(typeof window!=='undefined')window.addEventListener('focus',recover)
  setStream(stream)
  return {
    retry, setStream,
    async switchOutput() {
      mode=mode==='element'?'webaudio':'element'
      graph?.disconnect();graph=undefined
      attempt++
      emit({playing:false,error:false,blocked:false,mode})
      if(mode==='element') audio.srcObject=output
      await retry()
    },
    dispose() {
      if(disposed)return
      disposed=true;attempt++
      if(typeof window!=='undefined')window.removeEventListener('focus',recover)
      sourceStream?.removeEventListener('addtrack',retryEvent);sourceStream?.removeEventListener('removetrack',retryEvent)
      for(const track of [...tracked])unwatch(track)
      audio.removeEventListener('playing',playing);audio.removeEventListener('pause',pause);audio.removeEventListener('error',failed)
      graph?.disconnect();lease?.release()
      audio.pause();audio.srcObject=null
    },
  }
}
