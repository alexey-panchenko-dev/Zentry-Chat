import {MFASettings,SecurityHistory} from './MFASettings'
import {useState} from 'react'
import {useMutation,useQuery,useQueryClient} from '@tanstack/react-query'
import {api,errorText} from './api'

type Device={id:string;user_agent:string;created_at:string;last_seen_at:string;expires_at:string;current:boolean}
function deviceName(agent:string){
 const os=/Android/i.test(agent)?'Android':/iPhone|iPad/i.test(agent)?'iOS':/Windows/i.test(agent)?'Windows':/Macintosh/i.test(agent)?'macOS':/Linux/i.test(agent)?'Linux':'Устройство'
 const browser=/Edg\//.test(agent)?'Edge':/Firefox\//.test(agent)?'Firefox':/Chrome\//.test(agent)?'Chrome':/Safari\//.test(agent)?'Safari':''
 return agent?os+(browser?' · '+browser:''):'Устройство неизвестно'
}
const date=(value:string)=>new Date(value).toLocaleString('ru-RU',{dateStyle:'medium',timeStyle:'short'})
export function DeviceSettings(){
 const cache=useQueryClient();const [confirm,setConfirm]=useState<string|null>(null)
 const capability=useQuery({queryKey:['api-health'],queryFn:async()=>(await api.get<{capabilities?:Record<string,boolean>}>('/health')).data,retry:false,staleTime:60_000})
 const supported=capability.data?.capabilities?.session_devices===true
 const devices=useQuery({queryKey:['security-devices'],enabled:supported,queryFn:async()=>(await api.get<{sessions:Device[]}>('/security/sessions')).data.sessions,retry:false})
 const revoke=useMutation({mutationFn:async(id:string)=>{if(id==='others')await api.post('/security/sessions/revoke-others',{confirm:true});else await api.delete('/security/sessions/'+id)},onSuccess:(_,id)=>{setConfirm(null);void cache.invalidateQueries({queryKey:['security-devices']});if(devices.data?.some(device=>device.id===id&&device.current))void cache.invalidateQueries({queryKey:['session']})}})
 return <section className="z-device-settings"><h2>Безопасность</h2><h3>Активные устройства</h3>{capability.isPending?<p role="status">Проверяем доступность…</p>:!supported?<p className="z-muted">Управление устройствами пока недоступно на сервере.{capability.isError&&<button type="button" className="secondary" onClick={()=>void capability.refetch()}>Повторить</button>}</p>:<>{devices.isPending&&<p role="status">Загружаем устройства…</p>}{devices.isError&&<p role="alert" className="notice">{errorText(devices.error)} <button type="button" className="secondary" onClick={()=>void devices.refetch()}>Повторить</button></p>}{devices.data?.map(device=><article key={device.id} className="z-device-row"><strong>{deviceName(device.user_agent)}</strong>{device.current&&<small>Текущее устройство</small>}<span>Последняя активность: {date(device.last_seen_at)}</span><details><summary>Подробности входа</summary><span>Вход: {date(device.created_at)}</span><span>Сессия действует до: {date(device.expires_at)}</span></details><button type="button" className="secondary" disabled={revoke.isPending} onClick={()=>setConfirm(device.id)}>{device.current?'Выйти на этом устройстве':'Завершить сессию'}</button></article>)}{!!devices.data?.some(device=>!device.current)&&<button type="button" className="secondary" disabled={revoke.isPending} onClick={()=>setConfirm('others')}>Завершить все остальные сессии</button>}{confirm&&<div className="notice" role="alert"><p>{confirm==='others'?'Выйти из аккаунта на всех остальных устройствах?':'Завершить выбранную сессию?'}</p><button type="button" className="secondary" disabled={revoke.isPending} onClick={()=>setConfirm(null)}>Отмена</button><button type="button" className="primary" disabled={revoke.isPending} onClick={()=>revoke.mutate(confirm)}>Завершить</button></div>}{revoke.isError&&<p className="notice" role="alert">{errorText(revoke.error)}</p>}</>}<MFASettings/>{supported&&<SecurityHistory/>}</section>
}
