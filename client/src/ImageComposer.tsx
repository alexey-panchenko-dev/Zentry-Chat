import { useEffect, useImperativeHandle, useRef, useState } from 'react'
import type { RefObject } from 'react'
import { Plus, Send, X } from 'lucide-react'
import { api, errorText } from '@/api'
import type { Message } from '@/api'
import { ImageDraftController } from './image-draft'
import type { ImageDraftState } from './image-draft'
import { prepareChatPhoto } from './profile-photo'
type UploadResult={message:Message;realtime:string}
export function ImageComposer({conversationID,groupID,disabled,onSent,onActiveChange,pickerRef,hideTrigger=false}:{conversationID?:string;groupID?:string;disabled:boolean;onSent:(result:UploadResult)=>void;onActiveChange:(active:boolean)=>void;pickerRef?:RefObject<HTMLInputElement|null>;hideTrigger?:boolean}) {
 const [state,setState]=useState<ImageDraftState>({phase:'idle'})
 const [caption,setCaption]=useState('')
 const captionRef=useRef('')
 const root=useRef<HTMLDivElement>(null)
 const input=useRef<HTMLInputElement>(null)
 useImperativeHandle(pickerRef,()=>input.current!,[])
 const controller=useRef<ImageDraftController<UploadResult>|null>(null)
 const sent=useRef(onSent)
 useEffect(()=>{sent.current=onSent},[onSent])
 useEffect(()=>{
  const draft=new ImageDraftController<UploadResult>(setState,{prepare:prepareChatPhoto,createURL:URL.createObjectURL,revokeURL:URL.revokeObjectURL,uuid:()=>crypto.randomUUID(),errorText,upload:async(image,signal)=>{const data=new FormData();data.append('photo',image.file);data.append('client_message_id',image.id);data.append('caption',captionRef.current);return (await api.post<UploadResult>(groupID?`/groups/${groupID}/images`:`/conversations/${conversationID}/images`,data,{headers:{'Content-Type':undefined},signal,timeout:60000})).data}},result=>{setCaption('');captionRef.current='';sent.current(result)})
  controller.current=draft;const close=()=>draft.cancel();window.addEventListener('pagehide',close)
  return()=>{window.removeEventListener('pagehide',close);draft.dispose();controller.current=null}
 },[conversationID,groupID])
 useEffect(()=>{if(disabled)controller.current?.cancel()},[disabled])
 useEffect(()=>{
  const section=root.current?.closest('.z-messages')
  if(!section||disabled||state.phase==='sending')return
  function paste(event:Event){
   const file=Array.from((event as ClipboardEvent).clipboardData?.files??[]).find(item=>/^image\/(png|jpeg)$/.test(item.type))
   if(!file||!(event.target as Element).closest('.z-composer-field'))return
   event.preventDefault()
   void controller.current?.select(file)
  }
  function drop(event:Event){
   const files=Array.from((event as DragEvent).dataTransfer?.files??[])
   if(files.length!==1||!/^image\/(png|jpeg)$/.test(files[0].type))return
   event.preventDefault()
   void controller.current?.select(files[0])
  }
  section.addEventListener('drop',drop,{capture:true})
  section.addEventListener('paste',paste)
  return()=>{section.removeEventListener('paste',paste);section.removeEventListener('drop',drop,{capture:true})}
 },[disabled,state.phase])
 const active=state.phase!=='idle'
 useEffect(()=>{onActiveChange(active);return()=>onActiveChange(false)},[active,onActiveChange])
 const pending=state.phase==='sending'
 return <div ref={root} className="z-image-composer" data-active={active} data-error={!!state.error}>
 <input ref={input} hidden type="file" accept="image/jpeg,image/png" aria-label="Выбрать фото для сообщения" disabled={disabled||pending} onChange={event=>{const file=event.target.files?.[0];event.target.value='';if(file&&!disabled)void controller.current?.select(file)}}/>
 {!active&&!hideTrigger&&<button type="button" className="z-icon z-attach-photo" aria-label="Добавить фото" title="Добавить JPG или PNG до 15 МБ" disabled={disabled} onClick={()=>input.current?.click()}><Plus size={20}/></button>}
 {state.phase==='preparing'&&<><span role="status">Подготавливаем фото…</span><button type="button" className="z-icon" aria-label="Отменить фото" onClick={()=>controller.current?.cancel()}><X/></button></>}
 {state.draft&&<><img className="z-chat-photo-preview" src={state.draft.url} alt="Предпросмотр фото перед отправкой"/><textarea className="z-image-caption-input" aria-label="Подпись к фото" placeholder="Подпись к фото…" rows={2} maxLength={1000} disabled={pending||disabled||!!state.error} value={caption} onChange={event=>{setCaption(event.target.value);captionRef.current=event.target.value}}/><div className="z-image-draft-actions"><button type="button" className="z-icon" aria-label={pending?'Отменить отправку фото':'Убрать фото'} onClick={()=>{controller.current?.cancel();setCaption('');captionRef.current=''}}><X/></button><button type="button" className="z-send" disabled={pending||disabled} aria-label={state.error?'Повторить отправку фото':'Отправить фото'} onClick={()=>{if(!disabled)void controller.current?.send()}}><Send size={20}/></button></div>{pending&&<span role="status">Отправляем фото…</span>}</>}
 {state.error&&<p className="notice" role="alert">{state.error}</p>}
 </div>
}
