import { ReportButton } from './ReportButton'
import { createPortal } from 'react-dom'
import { ChatDetails } from './ChatDetails'
import type { DetailsTab } from './ChatDetails'
import { birthdayLabel } from './birthdays'
import { blockNotice, isRemovalAction, removalCommitted } from './conversation-controls'
import type { ControlAction, RemovalAction } from './conversation-controls'
import type { ConversationControlView } from './useConversationControls'
import { useEffect, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { MoreHorizontal, Phone, X } from 'lucide-react'
import { api, errorText, visibleUsername } from '@/api'
import type { Conversation, User } from '@/api'
import { Avatar } from '@/Visual'
import { formatPresence } from './presence'
export type PeerProfile=User&{phone?:string;birthday?:string;sent_count?:number;received_count?:number;presence_hidden?:boolean;online?:boolean;last_seen?:string|null;display_name?:string}
const actionLabel:Record<ControlAction,string>={clear:'Очистить историю',delete_chat:'Удалить только у меня',delete_everyone:'Удалить у всех',remove_contact:'Удалить из моих чатов',block:'Заблокировать',unblock:'Разблокировать'}
const actionDescription:Record<ControlAction,string>={
 clear:'Очистить историю этой переписки только у вас? У собеседника сообщения сохранятся.',
 delete_chat:'Удалить чат только у вас? История будет очищена, а переписка исчезнет из вашего списка. У собеседника сообщения сохранятся.',
 delete_everyone:'Удалить переписку у вас и собеседника? Текущие сообщения исчезнут у обоих. Если кто-то из вас напишет снова, переписка появится с новым сообщением.',
 remove_contact:'Убрать человека из вашего списка чатов и контактов? История останется доступна при повторном открытии через поиск. Новое сообщение вернёт чат в список; чтобы запретить сообщения и звонки, выберите «Заблокировать».',
 block:'Заблокировать контакт? Сообщения и звонки между вами станут недоступны до разблокировки.',
 unblock:'Разблокировать контакт и снова разрешить сообщения и звонки?',
}
export function ContactProfile({conversation,onClose,onCall,onRemoved,callBusy=false,controls,onJump,initialTab='info'}:{onJump:(id:string)=>void;initialTab?:DetailsTab;conversation:Conversation;onClose:()=>void;onCall:()=>void;onRemoved:(id:string,action:RemovalAction)=>void;callBusy?:boolean;controls:ConversationControlView}) {
 const cache=useQueryClient()
 const dialog=useRef<HTMLDialogElement>(null)
 const menu=useRef<HTMLDetailsElement>(null)
 const backdropPress=useRef(false)
 const [confirmation,setConfirmation]=useState<ControlAction|null>(null)
 const [completed,setCompleted]=useState('')
 const previousConfirmation=useRef<ControlAction|null>(null)
 const [enlarged,setEnlarged]=useState(false)
 const [failedPhoto,setFailedPhoto]=useState('')
 const [aliasDraft,setAliasDraft]=useState<string|null>(null)
 const [editingContact,setEditingContact]=useState(false)
 const profile=useQuery({queryKey:['contact-profile',conversation.id],queryFn:async({signal})=>(await api.get<{profile:PeerProfile}>(`/conversations/${conversation.id}/profile`,{signal})).data.profile,retry:false})
 const alias=aliasDraft??profile.data?.display_name??conversation.display_name??''
 const saveAlias=useMutation({mutationFn:async()=> (await api.put<{alias:string}>(`/conversations/${conversation.id}/alias`,{alias:alias.trim()})).data.alias,onSuccess:next=>{setAliasDraft(null);setEditingContact(false);cache.setQueryData<PeerProfile>(['contact-profile',conversation.id],old=>old?{...old,display_name:next}:old);void cache.invalidateQueries({queryKey:['conversations']})}})
 useEffect(()=>{const element=dialog.current;element?.showModal();if(initialTab==='search')element?.querySelector<HTMLInputElement>('[aria-label="Поиск по переписке"]')?.focus();return()=>element?.close()},[initialTab])
 useEffect(()=>{
  const outside=(event:PointerEvent)=>{const element=menu.current;if(element?.open&&event.target instanceof Node&&!element.contains(event.target))element.open=false}
  document.addEventListener('pointerdown',outside)
  return()=>document.removeEventListener('pointerdown',outside)
 },[])
 useEffect(()=>{
  if(confirmation){previousConfirmation.current=confirmation;return}
  if(previousConfirmation.current){const target=menu.current?.querySelector<HTMLElement>('summary')??dialog.current?.querySelector<HTMLButtonElement>('[aria-label="Закрыть профиль"]');target?.focus();previousConfirmation.current=null}
 },[confirmation])
 const peer:PeerProfile=profile.data??conversation.participant
 const presence=profile.data&&typeof profile.data.online==='boolean'?(profile.data.presence_hidden?{label:'Статус скрыт',title:'Пользователь скрыл время активности'}:formatPresence(profile.data.online,profile.data.last_seen)):null
 const photo=peer.avatar_url?.startsWith('/api/profiles/')&&peer.avatar_url!==failedPhoto?peer.avatar_url:''
 const birthday=profile.data&&!profile.isFetching&&!profile.isError?birthdayLabel(peer.birthday?.length===10?peer.birthday.slice(5):peer.birthday):null
 const notice=controls.state?blockNotice(controls.state):''
 const oldServer=(profile.error as {response?:{status?:number}})?.response?.status===404
 function close(){dialog.current?.close();onClose()}
 function choose(action:ControlAction){if(menu.current)menu.current.open=false;controls.action.reset();setConfirmation(action)}
 async function confirmAction(){
  if(!confirmation||controls.action.isPending)return
  const action=confirmation;const id=conversation.id
  try{const state=await controls.action.mutateAsync({value:action,id});void cache.invalidateQueries({queryKey:['contact-profile',id]});if(isRemovalAction(action)&&removalCommitted(action,state)){close();onRemoved(id,action);return}setCompleted(action==='block'?'Контакт заблокирован.':action==='unblock'?'Контакт разблокирован.':'Изменения сохранены.');setConfirmation(null)}catch{/* Keep the confirmation and its request error for retry. */}
 }
 return createPortal(<dialog ref={dialog} className="z-contact-dialog z-peer-panel" aria-labelledby="contact-profile-title" onPointerDown={event=>{const rect=event.currentTarget.getBoundingClientRect();backdropPress.current=event.target===event.currentTarget&&(event.clientX<rect.left||event.clientX>rect.right||event.clientY<rect.top||event.clientY>rect.bottom)}} onPointerCancel={()=>{backdropPress.current=false}} onClick={event=>{const startedOutside=backdropPress.current;backdropPress.current=false;if(!startedOutside||event.target!==event.currentTarget||controls.action.isPending)return;const rect=event.currentTarget.getBoundingClientRect();if(event.clientX<rect.left||event.clientX>rect.right||event.clientY<rect.top||event.clientY>rect.bottom)close()}} onCancel={event=>{
  event.preventDefault();if(controls.action.isPending)return
  if(confirmation){setConfirmation(null);return}
  if(menu.current?.open){menu.current.open=false;menu.current.querySelector<HTMLElement>('summary')?.focus();return}close()
 }}>
 <div className="z-contact-dialog-head"><h2 id="contact-profile-title">{confirmation?'Подтвердите действие':'Профиль'}</h2><div className="z-contact-top-actions">
 {controls.supported&&!confirmation&&<details ref={menu} className="z-contact-more" onBlur={event=>{if(!event.currentTarget.contains(event.relatedTarget as Node|null))event.currentTarget.open=false}}><summary className="z-icon" aria-label="Действия с контактом" title="Действия с контактом"><MoreHorizontal/></summary><div className="z-contact-menu">
 {controls.removalSupported&&<><button type="button" disabled={controls.loading||!!controls.error} onClick={()=>choose('delete_everyone')}>Удалить у всех</button><button type="button" disabled={controls.loading||!!controls.error} onClick={()=>choose('delete_chat')}>Удалить только у меня</button><button type="button" disabled={controls.loading||!!controls.error} onClick={()=>choose('remove_contact')}>Удалить из моих чатов</button></>}
 <button type="button" disabled={controls.loading||!!controls.error} onClick={()=>choose(controls.state?.blocked_by_me?'unblock':'block')}>{controls.state?.blocked_by_me?'Разблокировать':'Заблокировать'}</button>
 <ReportButton userID={conversation.participant.id}/>
 </div></details>}
 <button type="button" className="z-icon" aria-label="Закрыть профиль" autoFocus disabled={controls.action.isPending} onClick={close}><X/></button></div></div>
 {confirmation?<div className="z-contact-confirm"><p>{actionDescription[confirmation]}</p>{controls.action.isError&&<p className="notice" role="alert">{errorText(controls.action.error)}</p>}<button type="button" className="secondary" autoFocus disabled={controls.action.isPending} onClick={()=>setConfirmation(null)}>Отмена</button><button type="button" className="primary" disabled={controls.action.isPending} onClick={()=>void confirmAction()}>{controls.action.isPending?'Сохраняем…':actionLabel[confirmation]}</button></div>:<ChatDetails scope="personal" chatID={conversation.id} initialTab={initialTab} onJump={id=>{close();onJump(id)}} info={<div className="z-contact-profile-body">
 {photo?<button type="button" className={'z-profile-image-button'+(enlarged?' is-enlarged':'')} aria-label={enlarged?'Уменьшить фото':'Увеличить фото'} aria-expanded={enlarged} onClick={()=>setEnlarged(value=>!value)}><img src={photo} alt={'Фото '+peer.name} onError={()=>setFailedPhoto(photo)}/></button>:<Avatar user={{...peer,avatar_url:''}} large/>}<h3>{peer.display_name||peer.name}</h3><div className="z-contact-identity-row"><p className="z-contact-identity">{visibleUsername(peer)}</p>{presence&&<p className="z-contact-presence" title={presence.title}><span className={profile.data?.online?'is-online':''}/>{presence.label}</p>}</div>
 {profile.isFetching&&<p role="status" className="z-muted">Загружаем профиль…</p>}
 {profile.isError&&<div role="alert"><p className="notice">{oldServer?'Дополнительные сведения профиля пока недоступны.':errorText(profile.error)}</p><button type="button" className="secondary" onClick={()=>void profile.refetch()} disabled={profile.isFetching}>Повторить</button></div>}
 {profile.data&&!profile.isFetching&&!profile.isError&&<><dl className="z-contact-details">{peer.phone&&<><dt>Телефон</dt><dd>{peer.phone}</dd></>}</dl>{!peer.phone&&<p className="z-muted">Собеседник не открыл контактные данные.</p>}</>}
 {profile.data?.username&&(editingContact?<form className="z-contact-alias" onSubmit={event=>{event.preventDefault();if(!saveAlias.isPending&&alias.trim())saveAlias.mutate()}}><label>Как сохранить контакт<input autoFocus value={alias} maxLength={80} placeholder={peer.name} onChange={event=>setAliasDraft(event.target.value)}/></label><div><button type="submit" className="primary" disabled={saveAlias.isPending||!alias.trim()}>{saveAlias.isPending?'Сохраняем…':'Сохранить контакт'}</button><button type="button" className="secondary" disabled={saveAlias.isPending} onClick={()=>{setEditingContact(false);setAliasDraft(null);saveAlias.reset()}}>Отмена</button></div>{saveAlias.isError&&<p className="notice" role="alert">{errorText(saveAlias.error)}</p>}</form>:<button type="button" className="secondary" onClick={()=>{setAliasDraft(profile.data?.display_name||peer.name);saveAlias.reset();setEditingContact(true)}}>{profile.data.display_name?'Изменить имя':'Добавить в контакты'}</button>)}
 {profile.data&&!profile.isFetching&&!profile.isError&&<dl className="z-contact-details"><dt>День рождения</dt><dd>{birthday?birthday.label+(peer.birthday?.length===10?' '+peer.birthday.slice(0,4):''):'Не указан'}{birthday?.today&&<strong className="z-birthday-today"> · Сегодня 🎉</strong>}</dd></dl>}
 {profile.data&&!profile.isError&&typeof peer.sent_count==='number'&&typeof peer.received_count==='number'&&<div className="z-message-counts"><div><strong>{peer.sent_count}</strong><span>Вы отправили</span></div><div><strong>{peer.received_count}</strong><span>Вам отправили</span></div></div>}

 {notice&&<p className="notice" role="status">{notice}</p>}
 <button type="button" className="primary" disabled={callBusy||controls.blocked||controls.unavailable} onClick={()=>{close();onCall()}}><Phone size={19}/>Позвонить</button>{callBusy&&<p className="z-field-help">Сначала завершите текущий звонок.</p>}
 {controls.loading&&<p role="status">Проверяем переписку…</p>}{controls.error&&<div><p className="notice" role="alert">{errorText(controls.error)}</p><button className="secondary" type="button" onClick={()=>void controls.refetch()}>Повторить</button></div>}{completed&&<p role="status">{completed}</p>}
 </div>}/>}</dialog>,document.body)
}

