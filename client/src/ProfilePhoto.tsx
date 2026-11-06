import { useEffect, useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { api, errorText } from '@/api'
import type { User } from '@/api'
import { Avatar } from '@/Visual'
import { useMediaAvailability } from './useMediaAvailability'
import { prepareProfilePhoto } from './profile-photo'
export function ProfilePhoto({user}:{user:User}) {
 const cache=useQueryClient()
 const availability=useMediaAvailability('profile_photos')
 const input=useRef<HTMLInputElement>(null)
 const [file,setFile]=useState<File|null>(null)
 const [preview,setPreview]=useState('')
 const previewRef=useRef('')
 const [error,setError]=useState('')
 const [pending,setPending]=useState(false)
 const [preparing,setPreparing]=useState(false)
 const preparation=useRef(0)
 const abort=useRef<AbortController|null>(null)
 const busy=useRef(false)
 function selectFile(next:File|null){if(previewRef.current)URL.revokeObjectURL(previewRef.current);const url=next?URL.createObjectURL(next):'';previewRef.current=url;setPreview(url);setFile(next)}
 useEffect(()=>()=>{if(previewRef.current)URL.revokeObjectURL(previewRef.current)},[])
 useEffect(()=>()=>{preparation.current++;abort.current?.abort()},[])
 async function choosePhoto(next:File){const current=++preparation.current;setPreparing(true);setError('');selectFile(null);try{const prepared=await prepareProfilePhoto(next);if(current===preparation.current)selectFile(prepared)}catch(e){if(current===preparation.current)setError(errorText(e))}finally{if(current===preparation.current)setPreparing(false)}}
 async function save(){
  if(busy.current||!file||!availability.supported)return
  busy.current=true;setPending(true);setError('')
  const request=new AbortController();abort.current=request
  try{
   const data=new FormData();data.append('photo',file)
   const {avatar_url}=(await api.post<{avatar_url:string}>('/profile/photo',data,{headers:{'Content-Type':undefined},signal:request.signal,timeout:60000})).data
   if(!request.signal.aborted){cache.setQueryData<User>(['session'],old=>old?{...old,avatar_url}:old);for(const key of ['session','conversations','users','profile','discovery','contact-profile'])void cache.invalidateQueries({queryKey:[key]});selectFile(null)}
  }catch(e){if(!request.signal.aborted)setError(errorText(e))}finally{busy.current=false;if(!request.signal.aborted)setPending(false)}
 }
 return <div className="z-profile-photo">
 <button type="button" className="z-photo-picker" aria-label={user.avatar_url||file?'Изменить фото профиля':'Добавить фото профиля'} aria-describedby="photo-help" disabled={pending||preparing||!availability.supported} onClick={()=>input.current?.click()}>{preview?<img className="z-photo-preview" src={preview} alt="Предпросмотр нового фото"/>:<Avatar user={user} large/>}<span className="z-photo-plus" aria-hidden="true"><Plus size={24}/></span></button>
 <input ref={input} hidden type="file" accept="image/jpeg,image/png" aria-label="Выбрать фото профиля" disabled={pending||preparing||!availability.supported} onChange={event=>{const next=event.target.files?.[0];event.target.value='';if(next)void choosePhoto(next)}}/>
 <p className="z-field-help" id="photo-help">Нажмите на фото, чтобы выбрать JPG или PNG до 15 МБ. Фото уменьшается автоматически.</p>
 {!availability.supported&&<p className="z-field-help" role="status">{availability.isPending?'Проверяем доступность фото…':'Загрузка фото пока недоступна.'}</p>}
 {file&&<div className="z-photo-actions"><button type="button" className="primary" disabled={pending||preparing} onClick={()=>void save()}>Сохранить фото</button><button type="button" className="secondary" disabled={pending||preparing} onClick={()=>{selectFile(null);setError('')}}>Отмена</button></div>}
 {preparing&&<p role="status">Подготавливаем фото…</p>}{pending&&<p role="status">Сохраняем фото…</p>}{error&&<p className="notice" role="alert">{error}</p>}
 </div>
}
