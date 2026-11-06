import { useEffect, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Phone, X } from 'lucide-react'
import { api, errorText } from '@/api'
import type { Conversation, User } from '@/api'
import { Avatar } from '@/Visual'
type PeerProfile=User&{phone?:string;email?:string}
export function ContactProfile({conversation,onClose,onCall,callBusy=false}:{conversation:Conversation;onClose:()=>void;onCall:()=>void;callBusy?:boolean}) {
 const dialog=useRef<HTMLDialogElement>(null)
 const [enlarged,setEnlarged]=useState(false)
 const [failedPhoto,setFailedPhoto]=useState('')
 const profile=useQuery({queryKey:['contact-profile',conversation.id],queryFn:async({signal})=>(await api.get<{profile:PeerProfile}>(`/conversations/${conversation.id}/profile`,{signal})).data.profile,retry:false})
 useEffect(()=>{const element=dialog.current;element?.showModal();return()=>element?.close()},[])
 const peer:PeerProfile=profile.data??conversation.participant
 const photo=peer.avatar_url?.startsWith('/api/profiles/')&&peer.avatar_url!==failedPhoto?peer.avatar_url:''
 const oldServer=(profile.error as {response?:{status?:number}})?.response?.status===404
 function close(){dialog.current?.close();onClose()}
 return <dialog ref={dialog} className="z-contact-dialog" aria-labelledby="contact-profile-title" onCancel={event=>{event.preventDefault();close()}}><div className="z-contact-dialog-head"><h2 id="contact-profile-title">Профиль</h2><button type="button" className="z-icon" aria-label="Закрыть профиль" autoFocus onClick={close}><X/></button></div>
 <div className="z-contact-profile-body">{photo?<button type="button" className={'z-profile-image-button'+(enlarged?' is-enlarged':'')} aria-label={enlarged?'Уменьшить фото':'Увеличить фото'} aria-expanded={enlarged} onClick={()=>setEnlarged(value=>!value)}><img src={photo} alt={'Фото '+peer.name} onError={()=>setFailedPhoto(photo)}/></button>:<Avatar user={{...peer,avatar_url:''}} large/>}<h3>{peer.name}</h3>
 {profile.isFetching&&<p role="status" className="z-muted">Загружаем профиль…</p>}
 {profile.isError&&<div role="alert"><p className="notice">{oldServer?'Дополнительные сведения профиля пока недоступны.':errorText(profile.error)}</p><button type="button" className="secondary" onClick={()=>void profile.refetch()} disabled={profile.isFetching}>Повторить</button></div>}
 {profile.data&&!profile.isFetching&&!profile.isError&&<><dl className="z-contact-details">{peer.phone&&<><dt>Телефон</dt><dd>{peer.phone}</dd></>}{peer.email&&<><dt>Почта</dt><dd>{peer.email}</dd></>}</dl>{!peer.phone&&!peer.email&&<p className="z-muted">Собеседник не открыл контактные данные.</p>}</>}
 <button type="button" className="primary" disabled={callBusy} onClick={()=>{close();onCall()}}><Phone size={19}/>Позвонить</button>{callBusy&&<p className="z-field-help">Сначала завершите текущий звонок.</p>}
 </div></dialog>
}
