import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api, errorText } from '@/api'
import { normalizeContactPhone } from './contact-profile'
type Contact={phone:string;show_phone:boolean;show_email:boolean;email:string}
export function ContactSettings(){
 const contact=useQuery({queryKey:['own-contact'],queryFn:async({signal})=>(await api.get<Contact>('/profile/contact',{signal})).data,retry:false})
 if(contact.isPending)return <p role="status">Загружаем контактные данные…</p>
 if(contact.isError)return <div><p className="notice" role="alert">{(contact.error as {response?:{status:number}})?.response?.status===404?'Контактные данные пока недоступны.':errorText(contact.error)}</p><button className="secondary" type="button" onClick={()=>void contact.refetch()}>Повторить</button></div>
 return <ContactForm initial={contact.data}/>
}
function ContactForm({initial}:{initial:Contact}){
 const cache=useQueryClient()
 const [phone,setPhone]=useState(initial.phone??'')
 const [showPhone,setShowPhone]=useState(initial.show_phone===true)
 const [showEmail,setShowEmail]=useState(initial.show_email===true)
 const [pending,setPending]=useState(false)
 const [error,setError]=useState('')
 const [phoneInvalid,setPhoneInvalid]=useState(false)
 const [success,setSuccess]=useState('')
 const phoneInput=useRef<HTMLInputElement>(null)
 const abort=useRef<AbortController|null>(null)
 const busy=useRef(false)
 useEffect(()=>()=>abort.current?.abort(),[])
 async function save(remove=false){
  if(busy.current)return
  let normalized='';try{normalized=normalizeContactPhone(remove?'':phone)}catch(e){setError(errorText(e));setPhoneInvalid(true);phoneInput.current?.focus();return}
  busy.current=true;setPending(true);setError('');setSuccess('');setPhoneInvalid(false)
  const request=new AbortController();abort.current=request
  try{const result=(await api.post<Contact>('/profile/contact',{phone:normalized,show_phone:!!normalized&&showPhone,show_email:showEmail},{signal:request.signal})).data;if(!request.signal.aborted){setPhone(result.phone??'');setShowPhone(result.show_phone===true);setShowEmail(result.show_email===true);cache.setQueryData(['own-contact'],result);void cache.invalidateQueries({queryKey:['contact-profile']});setSuccess(remove?'Номер удалён.':'Контактные данные сохранены.')}}catch(e){if(!request.signal.aborted)setError(errorText(e))}finally{busy.current=false;if(!request.signal.aborted)setPending(false)}
 }
 function submit(event:FormEvent){event.preventDefault();void save()}
 return <form noValidate className="z-contact-settings" onSubmit={submit}><h2>Контактные данные</h2><label>Телефон<input ref={phoneInput} type="tel" autoComplete="tel" placeholder="+7 701 123 45 67" maxLength={40} value={phone} disabled={pending} aria-invalid={phoneInvalid} aria-describedby={phoneInvalid?'contact-settings-error':'contact-phone-help'} onChange={event=>{setPhone(event.target.value);setPhoneInvalid(false);setError('');setSuccess('');if(!event.target.value.trim())setShowPhone(false)}}/></label><p id="contact-phone-help" className="z-field-help">Международный формат, от 8 до 15 цифр. Номер не подтверждён.</p><label className="z-setting-row">Показывать телефон собеседникам<input type="checkbox" checked={showPhone} disabled={pending||!phone.trim()} onChange={event=>setShowPhone(event.target.checked)}/></label><label>Почта для входа<input type="email" readOnly value={initial.email??''}/></label><label className="z-setting-row">Показывать почту собеседникам<input type="checkbox" checked={showEmail} disabled={pending} onChange={event=>setShowEmail(event.target.checked)}/></label><p className="z-field-help">По умолчанию телефон и почта скрыты. Открытые данные видны участникам ваших переписок.</p>{error&&<p id="contact-settings-error" className="notice" role="alert">{error}</p>}{success&&<p role="status">{success}</p>}{pending&&<p role="status">Сохраняем…</p>}<div className="z-photo-actions"><button className="primary" disabled={pending}>Сохранить</button>{(phone||initial.phone)&&<button type="button" className="secondary" disabled={pending} onClick={()=>void save(true)}>Удалить номер</button>}</div></form>
}
