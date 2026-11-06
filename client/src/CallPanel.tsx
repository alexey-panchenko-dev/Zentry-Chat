import { Avatar } from '@/Visual'
import { CallSounds } from './call-sounds.js'
import { RemoteAudio } from '@/RemoteAudio'
import { useEffect, useRef, useState } from 'react'
import { Mic, MicOff, Phone, PhoneOff, Video, VideoOff } from 'lucide-react'
import { CallController } from '@/calls'
import type { CallView } from '@/calls'
import type { Conversation, User } from '@/api'

function VideoStream({ stream, muted = false, label, user }: { stream: MediaStream | null; muted?: boolean; label: string; user: User }) {
  const ref = useRef<HTMLVideoElement>(null)
  const [hasVideo, setHasVideo] = useState(false)
  useEffect(() => {
    const tracks = stream?.getVideoTracks() ?? []
    const update = () => setHasVideo(tracks.some(track => track.enabled && !track.muted && track.readyState !== 'ended'))
    update()
    for (const track of tracks) for (const event of ['mute','unmute','ended']) track.addEventListener(event, update)
    return () => { for (const track of tracks) for (const event of ['mute','unmute','ended']) track.removeEventListener(event, update) }
  }, [stream])
  useEffect(() => {
    const video = ref.current
    if (!video) return
    video.srcObject = stream
    const play = () => { if (stream) void video.play().catch(() => {}) }
    play()
    stream?.addEventListener('addtrack', play)
    return () => { stream?.removeEventListener('addtrack', play); video.srcObject = null }
  }, [stream])
  return <div className={"z-video " + (muted ? "is-self" : "is-remote")} style={muted&&!hasVideo?{display:'none'}:undefined}>
    <div className="relative">
      <video ref={ref} autoPlay playsInline muted style={!hasVideo?{visibility:'hidden'}:undefined} className="aspect-video w-full object-contain" />
      {!hasVideo&&<div className="z-call-audio-avatar"><Avatar user={user} large/><strong>{user.name}</strong><p>Камера выключена</p></div>}
      {muted&&hasVideo&&<span className="absolute left-3 top-2 rounded bg-black/60 px-2 py-1 text-xs">{label}</span>}
    </div>
    {!muted && <details className="z-remote-audio"><summary>Звук собеседника</summary><RemoteAudio stream={stream} /></details>}
  </div>
}
export function CallPanel({ user, selected, conversations = [], soundsEnabled = true, onActiveChange }: { onActiveChange?:(active:boolean)=>void; user: User; selected: Conversation | null; conversations?: Conversation[]; soundsEnabled?:boolean }) {
  const callDialog = useRef<HTMLDialogElement>(null)
  const [sounds] = useState(()=>new CallSounds({enabled:soundsEnabled}))
  const [soundBlocked,setSoundBlocked] = useState(false)

  useEffect(()=>{const unlock=()=>{void sounds.unlock()};document.addEventListener('pointerdown',unlock,{once:true});document.addEventListener('keydown',unlock,{once:true});return()=>{document.removeEventListener('pointerdown',unlock);document.removeEventListener('keydown',unlock);sounds.stop()}},[sounds])
  useEffect(()=>{void sounds.setEnabled(soundsEnabled)},[soundsEnabled,sounds])
  const controller = useRef<CallController | null>(null)
  const [view, setView] = useState<CallView>({ call: null, local: null, remote: null, connection: 'Подключаем звонки…', error: '', busy: false })
  useEffect(() => {
    const connection: {client?: CallController} = {}
    const open = () => {
      if (connection.client) return
      connection.client = new CallController(user.id, setView)
      controller.current = connection.client
      void connection.client.connect()
    }
    open()
    const close = () => {connection.client?.dispose();connection.client=undefined;controller.current=null}
    const restore = (event: PageTransitionEvent) => {if(event.persisted) open()}
    window.addEventListener('pagehide', close)
    window.addEventListener('pageshow',restore)
    return () => { window.removeEventListener('pagehide', close);window.removeEventListener('pageshow',restore);close() }
  }, [user.id])
  const call = view.call
  const ongoing = call?.status === 'ringing' || call?.status === 'accepted'
  useEffect(()=>{onActiveChange?.(!!ongoing || view.busy);return()=>onActiveChange?.(false)},[ongoing,view.busy,onActiveChange])
  const incoming = call?.callee_id === user.id
  useEffect(()=>{let active=true;if(call?.status==='ringing'&&soundsEnabled){const audible=sounds.play(incoming?'incoming':'outgoing');void Promise.resolve(audible).then(ok=>{if(active)setSoundBlocked(!ok)})}else sounds.stop();return()=>{active=false;sounds.stop()}},[call?.id,call?.status,incoming,soundsEnabled,sounds])
  const callPeer = call ? conversations.find(c => c.id === call.conversation_id)?.participant : selected?.participant
  useEffect(() => { const dialog=callDialog.current; if(ongoing && dialog && !dialog.open)dialog.showModal(); if(!ongoing && dialog?.open)dialog.close() },[ongoing])
  const names: Record<string, string> = { ringing: incoming ? 'Входящий звонок' : 'Вызываем собеседника…', accepted: 'Звонок', ended: 'Звонок завершён', cancelled: 'Вызов отменён', rejected: 'Вызов отклонён', expired: 'Время звонка истекло' }
  return <section className={'z-call-panel '+(ongoing?'is-ongoing':'is-idle')+(!selected&&!ongoing?' no-peer':'')} aria-label="Звонки">
    {!ongoing && <><button className="z-call-launch z-icon" title={selected?'Позвонить: '+selected.participant.name:'Выберите переписку'} aria-label="Позвонить" disabled={!selected||view.busy} onClick={()=>selected&&void controller.current?.start(selected.id)}><Phone size={23}/></button>{view.error&&<p className="notice z-call-idle-error" role="alert">{view.error}</p>}</>}
    {ongoing&&<dialog className="z-call-dialog" aria-label="Звонок" ref={callDialog} onCancel={e=>e.preventDefault()}><div className="z-call-stage"><div className="z-call-top"><Avatar user={callPeer||{id:'caller',name:incoming?'Входящий звонок':user.name}}/><div><h2>{call?names[call.status]??call.status:'Звонок'}</h2><p role="status">{view.connection}</p></div></div>
      <div className="z-call-feedback">
      {view.error&&<p className="notice" role="alert">{view.error}</p>}
      {view.cameraError&&<p className="notice" role="status">{view.cameraError}</p>}
      {call.status==='accepted'&&view.audio&&<details className="z-audio-diagnostics"><summary>Проверка звука</summary><p>Микрофон: {!view.audio.inputEnabled?'выключен':view.audio.inputMuted?'не получает данные':'включён'}</p><p>Уровень голоса: {view.audio.inputLevel===null?'недоступен':Math.round(view.audio.inputLevel*100)+'%'}</p><p>Передано аудио: {Math.round(view.audio.sent/1024)} КБ · Получено: {Math.round(view.audio.received/1024)} КБ</p><small>Счётчики должны расти во время разговора. Если получено 0 КБ — звук не доходит от собеседника.</small></details>}
      </div>
      {call.status==='accepted'&&!view.local&&<p className="notice z-call-recovery">Медиа соединение не восстановлено после перезагрузки. Завершите звонок и позвоните заново.</p>}
      {(view.local||view.remote)?<div className="z-video-stage"><VideoStream stream={view.remote} user={callPeer||{id:'caller',name:'Собеседник'}} label="Собеседник"/><VideoStream stream={view.local} user={user} muted label="Вы"/></div>:<div className="z-call-wait"><Avatar user={callPeer||{id:'caller',name:'Входящий звонок'}} large/><h2>{incoming?'Тебе звонят':'Вызываем собеседника…'}</h2><p>Для ответа нужен только микрофон.<br/>Камеру можно включить во время звонка.</p></div>}
      {soundBlocked&&call.status==="ringing"&&soundsEnabled&&<button className="secondary z-unlock-ring" onClick={()=>void sounds?.unlock().then(ok=>{if(ok)setSoundBlocked(false)})}>Включить звук звонка</button>}<div className="z-call-dock">{incoming&&call.status==='ringing'&&<button className="primary" disabled={view.busy} onClick={()=>void controller.current?.action('accept')}><Phone size={20}/>Ответить</button>}
        {view.local&&<><button className="z-icon" title="Микрофон" aria-label="Переключить микрофон" aria-pressed={!!view.local.getAudioTracks()[0]?.enabled} onClick={()=>controller.current?.toggle('audio')}>{view.local.getAudioTracks()[0]?.enabled?<Mic size={22}/>:<MicOff size={22}/>}</button><button className="z-icon" title={view.cameraBusy?'Подключаем камеру…':view.local.getVideoTracks().length?'Выключить камеру':'Включить камеру'} aria-label="Переключить камеру" aria-pressed={!!view.local.getVideoTracks().length} disabled={view.cameraBusy} onClick={()=>void controller.current?.toggleCamera()}>{view.local.getVideoTracks().length?<Video size={22}/>:<VideoOff size={22}/>}</button></>}
        <button className="z-icon z-end-call" aria-label={call.status==='accepted'?'Завершить звонок':incoming?'Отклонить звонок':'Отменить звонок'} onClick={()=>void controller.current?.finish()}><PhoneOff size={23}/></button>
      </div>
    </div></dialog>}
  </section>
}
