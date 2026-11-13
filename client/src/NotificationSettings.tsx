import {useEffect,useState} from 'react'
import {useMutation,useQuery,useQueryClient} from '@tanstack/react-query'
import {api,errorText} from './api'

type Global={sound_messages:boolean;push_enabled:boolean;sound_calls:boolean;mentions_enabled:boolean;dnd_until:string|null;dnd_mute_message_sound:boolean;dnd_suppress_push:boolean;dnd_mute_call_sound:boolean;dnd_suppress_mentions:boolean;quiet_enabled:boolean;quiet_days:number;quiet_start:string;quiet_end:string;timezone:string}
type Exception={contact_id:string;allow_dnd_push:boolean;allow_call_sound:boolean;important:boolean}
type User={id:string;name:string;username:string}
type Settings={global:Global;chats:{scope:string;chat_id:string;mode:string;muted_until:string|null}[];exceptions:Exception[]}
const defaults:Global={sound_messages:true,push_enabled:true,sound_calls:true,mentions_enabled:true,dnd_until:null,dnd_mute_message_sound:true,dnd_suppress_push:true,dnd_mute_call_sound:true,dnd_suppress_mentions:false,quiet_enabled:false,quiet_days:127,quiet_start:'23:00',quiet_end:'09:00',timezone:Intl.DateTimeFormat().resolvedOptions().timeZone||'UTC'}
const days=['Пн','Вт','Ср','Чт','Пт','Сб','Вс']
export function useNotificationSupport(){
 const health=useQuery({queryKey:['api-health'],queryFn:async()=>(await api.get<{capabilities?:Record<string,boolean>}>('/health')).data,retry:false,staleTime:60_000})
 return {supported:health.data?.capabilities?.notification_preferences===true,isPending:health.isPending}
}
export function useNotificationSettings(){
 const support=useNotificationSupport()
 const query=useQuery({queryKey:['notification-settings'],enabled:support.supported,queryFn:async()=>(await api.get<{settings:Settings}>('/notifications')).data.settings,retry:false})
 return {...query,supported:support.supported,supportPending:support.isPending}
}
export function NotificationSettings(){
 const cache=useQueryClient(),query=useNotificationSettings(),[global,setGlobal]=useState<Global>(defaults),[search,setSearch]=useState(''),[selected,setSelected]=useState<User|null>(null),[dndChoice,setDndChoice]=useState('off'),[error,setError]=useState('')
 useEffect(()=>{if(query.data)setGlobal(query.data.global)},[query.data])
 const users=useQuery({queryKey:['notification-contact-search',search],enabled:search.trim().length>=2,queryFn:async()=>(await api.get<{users:User[]}>('/users',{params:{query:search.trim()}})).data.users,retry:false})
 const save=useMutation({mutationFn:(value:Global)=>api.put('/notifications',value),onSuccess:()=>{setError('');void cache.invalidateQueries({queryKey:['notification-settings']})},onError:e=>setError(errorText(e))})
 const exception=useMutation({mutationFn:({id,value}:{id:string;value:Exception|null})=>value?api.put('/notifications/exceptions/'+id,value):api.delete('/notifications/exceptions/'+id),onSuccess:()=>{setSelected(null);setSearch('');void cache.invalidateQueries({queryKey:['notification-settings']})},onError:e=>setError(errorText(e))})
 const update=(next:Global)=>{setGlobal(next);save.mutate(next)}
 if(!query.supported)return <section><h2>Уведомления</h2><p className="z-muted" role="status">{query.supportPending?'Проверяем доступность…':'Расписание тишины и настройки уведомлений для всех устройств станут доступны после обновления сервера.'}</p></section>
 if(query.isPending)return <section><h2>Уведомления</h2><p role="status">Загружаем настройки…</p></section>
 if(query.isError)return <section><h2>Уведомления</h2><p className="notice" role="alert">{errorText(query.error)}</p></section>
 const current=query.data?.global??defaults
 const setException=(contact:Exception,patch:Partial<Exception>)=>exception.mutate({id:contact.contact_id,value:{...contact,...patch}})
 const toggleDay=(index:number)=>update({...global,quiet_days:global.quiet_days^(1<<index)})
 return <section className="z-notification-settings"><h2>Уведомления</h2>
  <label className="z-setting-row">Звуки сообщений<input type="checkbox" checked={global.sound_messages} onChange={e=>update({...global,sound_messages:e.target.checked})}/></label>
  <label className="z-setting-row">Push-уведомления<input type="checkbox" checked={global.push_enabled} onChange={e=>update({...global,push_enabled:e.target.checked})}/></label>
  <label className="z-setting-row">Звук звонков<input type="checkbox" checked={global.sound_calls} onChange={e=>update({...global,sound_calls:e.target.checked})}/></label>
  <label className="z-setting-row">Уведомления об упоминаниях<input type="checkbox" checked={global.mentions_enabled} onChange={e=>update({...global,mentions_enabled:e.target.checked})}/></label>
  <h3>Не беспокоить</h3><p className="z-muted">Сообщения продолжат приходить. DND управляет только уведомлениями.</p>
  <label className="z-setting-row">Режим<input aria-label="Не беспокоить" type="checkbox" checked={!!current.dnd_until&&Date.parse(current.dnd_until)>Date.now()} onChange={e=>update({...global,dnd_until:e.target.checked?new Date(Date.now()+8*3600000).toISOString():null})}/></label>
  <label className="z-setting-row">Срок режима<select aria-label="Срок не беспокоить" value={dndChoice} onChange={e=>{const value=e.target.value;setDndChoice(value);const duration:Record<string,number>={'1h':3600000,'8h':8*3600000,'1d':86400000,'1w':7*86400000,forever:100*365*86400000};update({...global,dnd_until:value==='off'?null:new Date(Date.now()+(duration[value]??8*3600000)).toISOString()})}}><option value="off">Выключен</option><option value="1h">На 1 час</option><option value="8h">На 8 часов</option><option value="1d">На сутки</option><option value="1w">На неделю</option><option value="forever">Навсегда</option></select></label>
  <label className="z-setting-row">Во время DND отключать звук сообщений<input type="checkbox" checked={global.dnd_mute_message_sound} onChange={e=>update({...global,dnd_mute_message_sound:e.target.checked})}/></label>
  <label className="z-setting-row">Во время DND скрывать push<input type="checkbox" checked={global.dnd_suppress_push} onChange={e=>update({...global,dnd_suppress_push:e.target.checked})}/></label>
  <label className="z-setting-row">Во время DND отключать звук звонков<input type="checkbox" checked={global.dnd_mute_call_sound} onChange={e=>update({...global,dnd_mute_call_sound:e.target.checked})}/></label>
  <label className="z-setting-row">Во время DND скрывать уведомления об упоминаниях<input type="checkbox" checked={global.dnd_suppress_mentions} onChange={e=>update({...global,dnd_suppress_mentions:e.target.checked})}/></label>
  <h3>Расписание тишины</h3><label className="z-setting-row">Включить расписание<input type="checkbox" checked={global.quiet_enabled} onChange={e=>update({...global,quiet_enabled:e.target.checked})}/></label>
  <div className="z-setting-row"><span>Дни</span><div>{days.map((day,index)=><label key={day}><input type="checkbox" aria-label={day} checked={(global.quiet_days&(1<<index))!==0} onChange={()=>toggleDay(index)}/>{day} </label>)}</div></div>
  <label className="z-setting-row">С<input type="time" value={global.quiet_start} onChange={e=>update({...global,quiet_start:e.target.value})}/></label><label className="z-setting-row">До<input type="time" value={global.quiet_end} onChange={e=>update({...global,quiet_end:e.target.value})}/></label>
  <label className="z-setting-row">Часовой пояс<input aria-label="Часовой пояс расписания" value={global.timezone} onChange={e=>setGlobal({...global,timezone:e.target.value})} onBlur={()=>save.mutate(global)}/></label>
  <h3>Исключения</h3><p className="z-muted">Только выбранные тобой контакты могут присылать push или звонить со звуком во время DND.</p>
  {selected?<div><p>{selected.name} · @{selected.username}</p><label><input type="checkbox" onChange={e=>exception.mutate({id:selected.id,value:{contact_id:selected.id,allow_dnd_push:e.target.checked,allow_call_sound:false,important:false}})}/>Разрешить push во время DND</label><button type="button" className="secondary" onClick={()=>setSelected(null)}>Готово</button></div>:<><label className="z-setting-row">Добавить контакт<input value={search} onChange={e=>setSearch(e.target.value)} placeholder="Имя или @ник"/></label>{users.data?.map(person=><button type="button" className="secondary" key={person.id} onClick={()=>setSelected(person)}>{person.name} · @{person.username}</button>)}</>}
  {query.data?.exceptions.map(item=><div className="z-setting-row" key={item.contact_id}><span>{item.contact_id.slice(0,8)} · {item.allow_dnd_push?'Push при DND':'Исключение'}</span><label>Звонок<input type="checkbox" checked={item.allow_call_sound} onChange={e=>setException(item,{allow_call_sound:e.target.checked})}/></label><label>Важные<input type="checkbox" checked={item.important} onChange={e=>setException(item,{important:e.target.checked})}/></label><button type="button" className="secondary" onClick={()=>exception.mutate({id:item.contact_id,value:null})}>Убрать</button></div>)}
  {(error||save.isError||exception.isError)&&<p className="notice" role="alert">{error}</p>}{save.isPending&&<p role="status">Сохраняем…</p>}
 </section>
}

export function ChatNotificationControls({scope,chatID,onSaved,onToggleLocal,muted}:{scope:'personal'|'group';chatID:string;onSaved:()=>void;onToggleLocal?:()=>void;muted?:boolean}){
 const query=useNotificationSettings(),[mode,setMode]=useState('all'),[duration,setDuration]=useState('forever'),[error,setError]=useState('')
 useEffect(()=>{const found=query.data?.chats.find(x=>x.scope===scope&&x.chat_id===chatID);setMode(found?.mode??'all')},[query.data,scope,chatID])
 const save=useMutation({mutationFn:()=>api.put('/notifications/chats',{scope,chat_id:chatID,mode,duration:mode==='off'?duration:''}),onSuccess:()=>{setError('');void query.refetch();onSaved()},onError:e=>setError(errorText(e))})
 if(!query.supported)return onToggleLocal?<button type="button" onClick={onToggleLocal}>{muted?'Включить уведомления':'Отключить уведомления'}</button>:null
 return <div className="z-chat-notification-controls"><label>Уведомления<select aria-label="Режим уведомлений чата" value={mode} onChange={e=>setMode(e.target.value)}><option value="all">Все уведомления</option><option value="silent">Без звука</option>{scope==='group'&&<option value="mentions">Только упоминания</option>}<option value="off">Полностью отключить</option></select></label>{mode==='off'&&<label>Срок<select aria-label="Срок отключения чата" value={duration} onChange={e=>setDuration(e.target.value)}><option value="1h">На 1 час</option><option value="8h">На 8 часов</option><option value="1w">На неделю</option><option value="forever">Навсегда</option></select></label>}<button type="button" className="secondary" disabled={save.isPending} onClick={()=>save.mutate()}>Сохранить</button>{error&&<p className="notice" role="alert">{error}</p>}</div>
}
