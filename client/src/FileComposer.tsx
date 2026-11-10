import { useEffect, useRef, useState } from 'react'
import { FileUp, Plus, X } from 'lucide-react'
import { api, errorText } from './api'
import type { Message } from './api'

const limit=15*1024*1024
export function FileComposer({conversationID,groupID,disabled,onSent}:{conversationID?:string;groupID?:string;disabled:boolean;onSent:(message:Message)=>void}){
 const root=useRef<HTMLDivElement>(null)
 const input=useRef<HTMLInputElement>(null);const [files,setFiles]=useState<{file:File;id:string}[]>([]);const [caption,setCaption]=useState('');const [busy,setBusy]=useState(false);const [error,setError]=useState('')
 function add(selected:FileList|null){if(!selected)return;const incoming=Array.from(selected);const tooLarge=incoming.find(file=>file.size>limit||file.size===0);if(tooLarge){setError(`${tooLarge.name}: предел — 15 МБ.`);return}setError('');setFiles(old=>[...old,...incoming.map(file=>({file,id:crypto.randomUUID()}))].slice(0,10));if(input.current)input.current.value=''}
 async function send(){if(!files.length||busy||disabled||(!conversationID&&!groupID))return;setBusy(true);setError('');try{for(const {file,id} of files){const form=new FormData();form.append('file',file);form.append('filename',file.name);form.append('caption',caption.trim());form.append('client_message_id',id);const endpoint=groupID?`/groups/${groupID}/files`:`/conversations/${conversationID}/files`;const result=(await api.post<{message:Message}>(endpoint,form,{headers:{'Content-Type':undefined},timeout:60000})).data.message;setFiles(queue=>queue.filter(item=>item.id!==id));onSent(result)}setCaption('')}catch(e){setError(errorText(e))}finally{setBusy(false)}}
 useEffect(()=>{
   const section=root.current?.closest('.z-messages')
   if(!section||disabled||busy)return
   const drag=(event:Event)=>{
     const drag=event as DragEvent
     if(!drag.dataTransfer?.types.includes('Files'))return
     drag.preventDefault()
     drag.dataTransfer.dropEffect='copy'
   }
   const drop=(event:Event)=>{
     const drag=event as DragEvent
     if(event.defaultPrevented||!drag.dataTransfer?.types.includes('Files'))return
     drag.preventDefault()
     add(drag.dataTransfer.files)
   }
   section.addEventListener('dragover',drag)
   section.addEventListener('drop',drop)
   return()=>{section.removeEventListener('dragover',drag);section.removeEventListener('drop',drop)}
 },[disabled,busy])
 return <div ref={root} className="z-file-composer">
  <button type="button" className="z-icon z-file-open" aria-label="Прикрепить файл" title="Перетащите файл сюда или выберите" disabled={disabled||busy} onClick={()=>input.current?.click()}><Plus size={20}/></button><input ref={input} hidden type="file" multiple disabled={disabled} accept=".pdf,.txt,.csv,.zip,.doc,.docx,.xls,.xlsx,.odt,.ods,.mp4,.webm" onChange={event=>add(event.target.files)}/>
  {!!files.length&&<div className="z-file-queue"><header><strong>Вложения · {files.length}/10</strong><button type="button" className="z-icon" aria-label="Закрыть вложения" disabled={busy} onClick={()=>setFiles([])}><X size={16}/></button></header><ul>{files.map(({file,id},index)=><li key={id}><FileUp size={16}/><span>{file.name}<small>{(file.size/1024/1024).toFixed(1)} МБ</small></span><button type="button" disabled={busy} aria-label={`Убрать ${file.name}`} onClick={()=>setFiles(old=>old.filter((_,i)=>i!==index))}><X size={14}/></button></li>)}</ul><input disabled={busy} aria-label="Подпись к файлам" maxLength={1000} value={caption} onChange={event=>setCaption(event.target.value)} placeholder="Подпись к вложению…"/><button type="button" className="primary" disabled={busy} onClick={()=>void send()}>{busy?'Отправляем…':'Отправить файлы'}</button>{error&&<p role="alert" className="notice">{error}</p>}</div>}
 </div>
}
