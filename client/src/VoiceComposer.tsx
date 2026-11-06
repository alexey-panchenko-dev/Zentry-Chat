import { useEffect, useRef, useState } from 'react'
import { Mic, Square, X, Send } from 'lucide-react'
import { api, errorText } from '@/api'
import type { Message } from '@/api'
import { VoiceRecorder } from './voice-recorder'
import type { RecorderState } from './voice-recorder'
import { VoiceMessage } from './VoiceMessage'
export function VoiceComposer({conversationID,disabled,onSent,onActiveChange}:{conversationID:string;disabled:boolean;onSent:(result:{message:Message;realtime:string})=>void;onActiveChange?:(active:boolean)=>void}) {
 const [state,setState]=useState<RecorderState>({phase:'idle'})
 const controller=useRef<VoiceRecorder|null>(null)
 const [pending,setPending]=useState(false)
 const [error,setError]=useState('')
 const abort=useRef<AbortController|null>(null)
 const submitting=useRef(false)
 useEffect(()=>{const recorder=new VoiceRecorder(setState);controller.current=recorder;const close=()=>recorder.cancel();window.addEventListener('pagehide',close);return()=>{window.removeEventListener('pagehide',close);recorder.dispose();abort.current?.abort();controller.current=null}},[])
 useEffect(()=>{if(disabled)controller.current?.cancel()},[disabled])
 const active=state.phase==='requesting'||state.phase==='recording'||state.phase==='preview'
 useEffect(()=>{onActiveChange?.(active);return()=>onActiveChange?.(false)},[active,onActiveChange])
 async function send(){if(!state.draft||submitting.current)return;submitting.current=true;setPending(true);setError('');const draft=state.draft;const data=new FormData();data.append('audio',draft.blob,draft.blob.type.includes('mp4')?'voice.mp4':draft.blob.type.includes('ogg')?'voice.ogg':'voice.webm');data.append('duration_ms',String(draft.duration));data.append('client_message_id',draft.id);const request=new AbortController();abort.current=request;try{const result=await api.post<{message:Message;realtime:string}>(`/conversations/${conversationID}/voice`,data,{headers:{'Content-Type':undefined},signal:request.signal,timeout:60000});if(!request.signal.aborted){onSent(result.data);controller.current?.cancel()}}catch(e){if(!request.signal.aborted)setError(errorText(e)+' Запись сохранена для повторной отправки.')}finally{if(!request.signal.aborted)setPending(false);submitting.current=false}}
 return <div className="z-voice-composer" data-phase={state.phase} data-active={state.phase==='requesting'||state.phase==='recording'||state.phase==='preview'}>
 {state.phase==='idle'||state.phase==='error'?<button type="button" className="z-icon" aria-label="Записать голосовое сообщение" title={disabled?'Запись недоступна во время звонка':'Записать голосовое сообщение (до 2 минут)'} disabled={disabled} onClick={()=>{setError('');void controller.current?.start()}}><Mic size={21}/></button>:null}
 {state.phase==='requesting'&&<><span role="status">Разрешите доступ к микрофону…</span><button type="button" className="z-icon" aria-label="Отменить запись" onClick={()=>controller.current?.cancel()}><X/></button></>}
 {state.phase==='recording'&&<><span role="status" className="z-recording">Идёт запись · до 2 минут</span><button type="button" className="z-icon" aria-label="Остановить запись" onClick={()=>controller.current?.stop()}><Square size={19}/></button><button type="button" className="z-icon" aria-label="Отменить запись" onClick={()=>controller.current?.cancel()}><X/></button></>}
 {state.draft&&<><VoiceMessage url={state.draft.url} duration={state.draft.duration}/><button type="button" className="z-icon" aria-label="Удалить запись" disabled={pending} onClick={()=>{controller.current?.cancel();setError('')}}><X/></button><button type="button" className="z-send" aria-label={error?'Повторить отправку записи':'Отправить запись'} disabled={pending} onClick={()=>void send()}><Send size={20}/></button>{pending&&<span role="status">Отправляем запись…</span>}</>}
 {(state.error||error)&&<p className="notice" role="alert">{state.error||error}</p>}
 </div>
}
