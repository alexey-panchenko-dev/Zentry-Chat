import {useState} from 'react'
import {useMutation,useQuery,useQueryClient} from '@tanstack/react-query'
import {api,errorText} from './api'
type Privacy={messages:string;calls:string;presence:string;search:string;birthday:string}
const fields:Array<[keyof Privacy,string]>=[['messages','Кто может писать первым'],['calls','Кто может звонить'],['presence','Кто видит время активности'],['search','Кто может найти меня в поиске'],['birthday','Кто видит дату рождения']]
export function PrivacySettings(){
 const cache=useQueryClient();const [draft,setDraft]=useState<Privacy|null>(null)
 const query=useQuery({queryKey:['privacy'],queryFn:async()=> (await api.get<Privacy>('/profile/privacy')).data,retry:false})
 const save=useMutation({mutationFn:async()=> (await api.put<Privacy>('/profile/privacy',draft)).data,onSuccess:value=>{cache.setQueryData(['privacy'],value);setDraft(null)}})
 const value=draft??query.data
 return <section className="z-privacy-settings"><h2>Конфиденциальность</h2>{query.isPending?<p role="status">Загружаем настройки…</p>:query.isError?<p className="notice" role="alert">Настройки недоступны. <button type="button" className="secondary" onClick={()=>void query.refetch()}>Повторить</button></p>:value&&<form onSubmit={e=>{e.preventDefault();if(draft&&!save.isPending)save.mutate()}}>{fields.map(([key,label])=><label className="z-setting-row" key={key}><span>{label}</span><select aria-label={label} value={value[key]} disabled={save.isPending} onChange={e=>{save.reset();setDraft({...value,[key]:e.target.value})}}><option value="everyone">Все</option>{key==='messages'&&<option value="requests">Через запросы</option>}<option value="contacts">Мои контакты</option><option value="nobody">Никто</option></select></label>)}<p className="z-muted">Контакты — люди, которых вы сохранили. Незнакомые люди могут писать через запросы. Уже существующие переписки сохраняются.</p><button type="submit" className="primary" disabled={!draft||save.isPending}>{save.isPending?'Сохраняем…':'Сохранить приватность'}</button>{save.isError&&<p className="notice" role="alert">{errorText(save.error)}</p>}{save.isSuccess&&!draft&&<p role="status">Настройки сохранены.</p>}</form>}</section>
}
