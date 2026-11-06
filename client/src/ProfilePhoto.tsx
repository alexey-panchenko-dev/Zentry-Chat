import { useEffect, useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { api, errorText } from '@/api'
import type { User } from '@/api'
export function ProfilePhoto({user}:{user:User}) {
 const cache=useQueryClient()
 const input=useRef<HTMLInputElement>(null)
 const [file,setFile]=useState<File|null>(null)
 const [preview,setPreview]=useState('')
 const previewRef=useRef('')
 const [error,setError]=useState('')
 const [pending,setPending]=useState(false)
 const abort=useRef<AbortController|null>(null)
 const busy=useRef(false)
 function selectFile(next:File|null){if(previewRef.current)URL.revokeObjectURL(previewRef.current);const url=next?URL.createObjectURL(next):'';previewRef.current=url;setPreview(url);setFile(next)}
 useEffect(()=>()=>{if(previewRef.current)URL.revokeObjectURL(previewRef.current)},[])
 useEffect(()=>()=>abort.current?.abort(),[])
 async function save(remove=false){if(busy.current)return;busy.current=true;setPending(true);setError('');const request=new AbortController();abort.current=request;try{let avatar_url='';if(remove){avatar_url=(await api.delete<{avatar_url:string}>('/profile/photo',{signal:request.signal})).data.avatar_url}else{if(!file)return;const data=new FormData();data.append('photo',file);avatar_url=(await api.post<{avatar_url:string}>('/profile/photo',data,{headers:{'Content-Type':undefined},signal:request.signal,timeout:60000})).data.avatar_url}if(!request.signal.aborted){cache.setQueryData<User>(['session'],old=>old?{...old,avatar_url}:old);void cache.invalidateQueries({queryKey:['session']});void cache.invalidateQueries({queryKey:['conversations']});void cache.invalidateQueries({queryKey:['users']});selectFile(null)}}catch(e){if(!request.signal.aborted)setError(errorText(e))}finally{busy.current=false;if(!request.signal.aborted)setPending(false)}}
 return <div className="z-profile-photo">{preview&&<img className="z-photo-preview" src={preview} alt="Предпросмотр нового фото"/>}<p className="z-field-help" id="photo-help">JPG или PNG, до 2 МБ.</p><input ref={input} type="file" accept="image/jpeg,image/png" aria-label="Выбрать фото профиля" aria-describedby="photo-help" disabled={pending} onChange={event=>{const next=event.target.files?.[0];event.target.value='';if(!next)return;if(!['image/jpeg','image/png'].includes(next.type)||next.size>2*1024*1024){setError('Выберите JPG или PNG размером до 2 МБ.');return}setError('');selectFile(next)}}/>{file&&<div className="z-photo-actions"><button type="button" className="primary" disabled={pending} onClick={()=>void save()}>Сохранить фото</button><button type="button" className="secondary" disabled={pending} onClick={()=>{selectFile(null);setError('')}}>Отмена</button></div>}{user.avatar_url&&!file&&<button type="button" className="secondary" disabled={pending} onClick={()=>void save(true)}>Удалить фото</button>}{pending&&<p role="status">Сохраняем фото…</p>}{error&&<p className="notice" role="alert">{error}</p>}</div>
}
