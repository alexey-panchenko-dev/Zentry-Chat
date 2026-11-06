import { useEffect, useRef, useState } from 'react'
import { ImagePlus, Send, X } from 'lucide-react'
import { api, errorText } from '@/api'
import type { Message } from '@/api'
import { ImageDraftController } from './image-draft'
import type { ImageDraftState } from './image-draft'
import { prepareChatPhoto } from './profile-photo'
type UploadResult={message:Message;realtime:string}
export function ImageComposer({conversationID,disabled,onSent,onActiveChange}:{conversationID:string;disabled:boolean;onSent:(result:UploadResult)=>void;onActiveChange:(active:boolean)=>void}) {
 const [state,setState]=useState<ImageDraftState>({phase:'idle'})
 const input=useRef<HTMLInputElement>(null)
 const controller=useRef<ImageDraftController<UploadResult>|null>(null)
 const sent=useRef(onSent)
 useEffect(()=>{sent.current=onSent},[onSent])
 useEffect(()=>{
  const draft=new ImageDraftController<UploadResult>(setState,{prepare:prepareChatPhoto,createURL:URL.createObjectURL,revokeURL:URL.revokeObjectURL,uuid:()=>crypto.randomUUID(),errorText,upload:async(image,signal)=>{const data=new FormData();data.append('photo',image.file);data.append('client_message_id',image.id);return (await api.post<UploadResult>(`/conversations/${conversationID}/images`,data,{headers:{'Content-Type':undefined},signal,timeout:60000})).data}},result=>sent.current(result))
  controller.current=draft;const close=()=>draft.cancel();window.addEventListener('pagehide',close)
  return()=>{window.removeEventListener('pagehide',close);draft.dispose();controller.current=null}
 },[conversationID])
 useEffect(()=>{if(disabled)controller.current?.cancel()},[disabled])
 const active=state.phase!=='idle'
 useEffect(()=>{onActiveChange(active);return()=>onActiveChange(false)},[active,onActiveChange])
 const pending=state.phase==='sending'
 return <div className="z-image-composer" data-active={active} data-error={!!state.error}>
 <input ref={input} hidden type="file" accept="image/jpeg,image/png" aria-label="Выбрать фото для сообщения" disabled={disabled||pending} onChange={event=>{const file=event.target.files?.[0];event.target.value='';if(file&&!disabled)void controller.current?.select(file)}}/>
 {!active&&<button type="button" className="z-icon" aria-label="Добавить фото" title="Добавить JPG или PNG до 15 МБ" disabled={disabled} onClick={()=>input.current?.click()}><ImagePlus size={21}/></button>}
 {state.phase==='preparing'&&<><span role="status">Подготавливаем фото…</span><button type="button" className="z-icon" aria-label="Отменить фото" onClick={()=>controller.current?.cancel()}><X/></button></>}
 {state.draft&&<><img className="z-chat-photo-preview" src={state.draft.url} alt="Предпросмотр фото перед отправкой"/><div className="z-image-draft-actions"><button type="button" className="z-icon" aria-label={pending?'Отменить отправку фото':'Убрать фото'} onClick={()=>controller.current?.cancel()}><X/></button><button type="button" className="z-send" disabled={pending||disabled} aria-label={state.error?'Повторить отправку фото':'Отправить фото'} onClick={()=>{if(!disabled)void controller.current?.send()}}><Send size={20}/></button></div>{pending&&<span role="status">Отправляем фото…</span>}</>}
 {state.error&&<p className="notice" role="alert">{state.error}</p>}
 </div>
}
