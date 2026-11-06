import { useEffect, useRef, useState } from 'react'
import { Volume2, VolumeX, RotateCw } from 'lucide-react'
import { bindRemoteAudio } from './remote-audio'
import type { AudioPlayback } from './remote-audio'

export function RemoteAudio({ stream }: { stream: MediaStream | null }) {
  const ref = useRef<HTMLAudioElement>(null)
  const retry = useRef<(() => Promise<void>) | null>(null)
  const [state, setState] = useState<AudioPlayback>({ playing: false, blocked: false, source: 'waiting', error: false })
  useEffect(() => {
    if (!ref.current) return
    const binding = bindRemoteAudio(ref.current, stream, setState)
    retry.current = binding.retry
    return () => { retry.current = null; binding.dispose() }
  }, [stream])
  const live = state.source === 'live' && state.playing
  const status = state.source === 'waiting' ? 'Ожидаем аудиодорожку'
    : state.source === 'muted' ? 'Дорожка есть · ожидаем аудиоданные'
    : state.blocked ? 'Браузер просит включить звук'
    : state.playing ? 'Аудиоплеер работает' : 'Звук не воспроизводится'
  return <div className="z-audio-control">
    <audio ref={ref} autoPlay aria-label="Звук собеседника" />
    <div className="z-audio-heading"><span className={'z-audio-symbol'+(live?' is-playing':'')}>{live?<Volume2 size={20}/>:<VolumeX size={20}/>}</span><div><strong>Звук собеседника</strong><span role="status">{status}</span></div></div>
    <button type="button" className="secondary z-audio-retry" onClick={()=>void retry.current?.()} disabled={state.source==='waiting'}><RotateCw size={14}/>{state.playing?'Перезапустить звук':'Включить звук'}</button>
    <details className="z-audio-details"><summary>Если собеседника не слышно</summary><p>Посмотри «Проверку звука» вверху звонка. Счётчик «Получено» показывает приём аудио. Работа плеера сама по себе не подтверждает наличие голоса.</p><p>Проверь громкость устройства и микрофон собеседника.</p></details>
    {state.error&&<p className="notice" role="alert">{state.blocked?'Браузер заблокировал автовоспроизведение. Нажми «Включить звук».':'Не удалось воспроизвести аудио. Попробуй включить звук ещё раз.'}</p>}
  </div>
}
