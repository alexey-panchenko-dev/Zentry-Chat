import { formatBirthDate, parseBirthDate, ageFromBirthDate, birthdayLabel, birthDatesSupported } from './birthdays'
import { BirthDateWheel } from './BirthDateWheel'
import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, X, Heart } from 'lucide-react'
import { api, errorText } from '@/api'
import type { Profile, User } from '@/api'
import { Avatar } from '@/Visual'
import '@/discovery.css'

function useProfile() {
  return useQuery({ queryKey: ['profile'], queryFn: async () => (await api.get<{profile: Profile}>('/profile')).data.profile, staleTime: 60_000 })
}

export function ProfileEditor({ user,onSaved }: { user: User;onSaved?:()=>void }) {
  const profile = useProfile()
  if (profile.isPending) return <p role="status">Загружаем профиль…</p>
  if (profile.isError) return <p className="notice">{errorText(profile.error)}</p>
  return <>{(profile.data?.username||user.username)&&<UsernameEditor username={profile.data?.username||user.username||''}/>}<ProfileForm key={user.id+':'+(profile.data?.username||user.username)} profile={profile.data ?? {...user, age: null, bio: '', city: ''}} onSaved={onSaved} /></>
}

function UsernameEditor({username}:{username:string}){
  const cache=useQueryClient()
  const [draft,setDraft]=useState(/^u_[0-9a-f]{32}$/.test(username)?'':username)
  const [saved,setSaved]=useState(username)
  const save=useMutation({mutationFn:async()=> (await api.put<{username:string}>('/profile/username',{username:draft})).data.username,onSuccess:next=>{setDraft(next);setSaved(next);cache.setQueryData<User>(['session'],old=>old?{...old,username:next,name:next}:old);cache.setQueryData<Profile>(['profile'],old=>old?{...old,username:next,name:next}:old);void cache.invalidateQueries({queryKey:['conversations']})}})
  return <form className="z-username-editor" onSubmit={event=>{event.preventDefault();if(!save.isPending&&draft.trim()!==saved)save.mutate()}}><label>Мой ник / @юзер<input value={draft} maxLength={41} autoComplete="username" onChange={event=>setDraft(event.target.value)} placeholder="Например, byliver"/></label><button type="submit" className="secondary" disabled={save.isPending||draft.trim()===saved}>Сохранить ник</button><p className="z-field-help">Это одно имя, по которому вас найдут. Латинские буквы, цифры и _.</p>{save.isError&&<p className="notice" role="alert">{errorText(save.error)}</p>}{save.isSuccess&&<p role="status">Ник сохранён.</p>}</form>
}

function DiscoveryPhoto({ user }: { user: User }) {
  const [failed, setFailed] = useState('')
  const url = user.avatar_url?.startsWith('/api/profiles/') ? user.avatar_url : ''
  return <div className="z-discovery-photo">
    {url && failed !== url
      ? <img src={url} alt={`Фото ${user.name}`} onError={() => setFailed(url)} />
      : <span className="z-discovery-initials" aria-hidden="true">{Array.from(user.name).slice(0,2).join('').toLocaleUpperCase('ru')}</span>}
  </div>
}
function ProfileForm({ profile,onSaved }: {profile: Profile;onSaved?:()=>void}) {
  const cache = useQueryClient()
  const health=useQuery({queryKey:['registration-capabilities'],queryFn:async()=>(await api.get<unknown>('/health')).data,retry:false,staleTime:60_000,refetchInterval:query=>birthDatesSupported(query.state.data)?false:30_000})
  const birthDatesReady=birthDatesSupported(health.data)
  const [birthDate,setBirthDate]=useState(formatBirthDate(profile.birth_date))
  const [bio, setBio] = useState(profile.bio)
  const [validation,setValidation]=useState<{field:string;message:string}|null>(null)
  const form=useRef<HTMLFormElement>(null)
  let birthISO='';try{birthISO=parseBirthDate(birthDate)}catch{/* An incomplete draft is validated on save. */}
  const age=birthISO?ageFromBirthDate(birthISO):profile.age
  const birthday=birthISO?birthdayLabel(birthISO.slice(5)):null
  const save = useMutation({
    mutationFn: async () => (await api.put<{profile: Profile}>('/profile', {name:profile.name,birth_date:parseBirthDate(birthDate),bio,city:profile.city})).data.profile,
    onSuccess: next => {
      cache.setQueryData(['profile'], next)
      cache.setQueryData(['session'], (old: User) => ({...old, name: next.name}))
      void cache.invalidateQueries({queryKey: ['discovery']})
      void cache.invalidateQueries({queryKey: ['conversations']})
      onSaved?.()
    },
  })
  function submit(event:FormEvent<HTMLFormElement>){
    event.preventDefault();if(save.isPending||!birthDatesReady)return
    let invalid:null|{field:string;message:string}=null
    if(!invalid){try{parseBirthDate(birthDate)}catch(e){invalid={field:'birth_date',message:errorText(e)}}}
    if(!invalid&&Array.from(bio).length>100)invalid={field:'bio',message:'Описание должно быть не длиннее 100 символов.'}
    setValidation(invalid);if(invalid){form.current?.querySelector<HTMLElement>(invalid.field==='birth_date'?'[role="spinbutton"]':'[name="'+invalid.field+'"]')?.focus();return}save.mutate()
  }
  return <form ref={form} noValidate className="z-profile-form" onSubmit={submit}>
    <div className="z-birth-date-field"><div><label>Дата рождения</label><BirthDateWheel value={birthDate} onChange={setBirthDate} invalid={validation?.field==='birth_date'} describedBy={validation?.field==='birth_date'?'profile-validation':'birth-date-help'}/></div><button className="secondary" type="submit" disabled={save.isPending||!birthDatesReady||birthDate===formatBirthDate(profile.birth_date)}>Подтвердить дату</button></div>
    <p id="birth-date-help" className="z-field-help">Дата рождения, включая год, видна собеседникам. Возраст рассчитывается автоматически.</p>
    {age!=null&&<p className="z-muted">Возраст: {age}{birthday&&<> · День рождения: {birthday.label}{birthday.today?' · Сегодня 🎉':''}</>}</p>}
    <label>О себе<textarea name="bio" className="resize-none" value={bio} onChange={e=>setBio(e.target.value)} rows={3} aria-invalid={validation?.field==='bio'} aria-describedby={validation?.field==='bio'?'profile-validation':'bio-help'} /><small id="bio-help">{Array.from(bio).length}/100 символов</small></label>
    {validation&&<p id="profile-validation" className="notice" role="alert">{validation.message}</p>}
    {save.isError&&<p className="notice" role="alert">{errorText(save.error)}</p>}
    {save.isSuccess&&<p role="status">Профиль сохранён.</p>}
    {!birthDatesReady&&<p className="notice" role="status">{health.isPending?'Проверяем доступность сохранения…':'Сохранение профиля с датой рождения временно недоступно. Попробуйте позже.'}</p>}
    <button className="primary" disabled={save.isPending||!birthDatesReady}>Сохранить профиль</button>
  </form>
}

function DiscoveryCity({profile}:{profile:Profile}) {
  const cache=useQueryClient()
  const [city,setCity]=useState(profile.city)
  const save=useMutation({mutationFn:async()=> (await api.put<{profile:Profile}>('/profile',{name:profile.name,birth_date:profile.birth_date,bio:profile.bio,city:city.trim()})).data.profile,onSuccess:next=>{cache.setQueryData(['profile'],next);void cache.invalidateQueries({queryKey:['discovery']})}})
  return <form className="z-discovery-city" onSubmit={event=>{event.preventDefault();if(!save.isPending)save.mutate()}}><label>Мой город<input value={city} maxLength={100} placeholder="Указать город" onChange={event=>setCity(event.target.value)}/></label><button className="secondary" disabled={save.isPending||city.trim()===profile.city} type="submit">Сохранить</button>{save.isError&&<p className="notice" role="alert">{errorText(save.error)}</p>}{save.isSuccess&&<p role="status">Город сохранён.</p>}</form>
}

export function Discovery({ onSettings, fallbackInterval }: { onSettings: () => void; fallbackInterval: number | false }) {
  const cache = useQueryClient()
  const profile = useProfile()
  const ready = profile.data?.age != null && !!profile.data.bio.trim()
  const [tab, setTab] = useState<'cards'|'likes'>('cards')
  const [expanded, setExpanded] = useState<Profile|null>(null)
  const [match, setMatch] = useState('')
  const [swipe,setSwipe]=useState<'like'|'skip'|null>(null)
  const swipeTimer=useRef<number|null>(null)
  useEffect(()=>()=>{if(swipeTimer.current!==null)window.clearTimeout(swipeTimer.current)},[])
  const cards = useQuery({ queryKey: ['discovery','cards'], enabled: ready && tab==='cards', queryFn: async () => (await api.get<{profiles: Profile[]}>('/discovery')).data.profiles, refetchInterval: fallbackInterval })
  const likes = useQuery({ queryKey: ['discovery','likes'], enabled: ready && tab==='likes', queryFn: async () => (await api.get<{profiles: Profile[]}>('/discovery/likes')).data.profiles, refetchInterval: fallbackInterval })
  const vote = useMutation({
    mutationFn: async ({id,liked}: {id:string;liked:boolean}) => (await api.post<{matched:boolean;conversation_id:string}>('/discovery/votes',{user_id:id,liked})).data,
    onSuccess: (result, variables) => {
      setSwipe(null)
      setExpanded(null)
      if(result.matched) setMatch('Вы стали друзьями! Переписка появилась в чатах.')
      // Remove the acted-on card immediately, including when its page is inactive.
      for (const key of ['cards','likes']) cache.setQueryData<Profile[]>(['discovery',key],old=>old?.filter(p=>p.id!==variables.id))
      void cache.invalidateQueries({queryKey:['discovery']})
      void cache.invalidateQueries({queryKey:['conversations']})
      void cache.invalidateQueries({queryKey:['unread']})
    },
    onError:()=>setSwipe(null),
  })
  const castVote=(person:Profile,liked:boolean)=>{if(vote.isPending||swipe||swipeTimer.current!==null)return;setSwipe(liked?'like':'skip');swipeTimer.current=window.setTimeout(()=>{swipeTimer.current=null;vote.mutate({id:person.id,liked})},420)}
  const actions = (person: Profile) => <div className="z-discovery-actions"><button className="z-skip" disabled={vote.isPending||!!swipe} aria-label="Не подходит" onClick={()=>castVote(person,false)}><X size={30}/></button><button className="z-like" disabled={vote.isPending||!!swipe} aria-label="Нравится" onClick={()=>castVote(person,true)}><Check size={30}/></button></div>
  const card = (person: Profile) => <article key={person.id} className={'z-discovery-card'+(swipe?' is-leaving-'+swipe:'')}><DiscoveryPhoto key={person.id} user={person}/><div className="z-discovery-details"><h2>{person.name}, {person.age}</h2>{person.city&&<p className="z-muted">{person.city}</p>}<p className="z-discovery-bio">{person.bio}</p></div>{actions(person)}</article>
  const active = tab==='cards'?cards:likes
  return <section className="z-discovery"><header className="z-discovery-header"><h1><Heart/> Знакомства</h1>
    {ready&&<div className="z-discovery-tabs"><button className={tab==='cards'?'is-active':''} onClick={()=>{setTab('cards');setExpanded(null);setSwipe(null)}}>Люди</button><button className={tab==='likes'?'is-active':''} onClick={()=>{setTab('likes');setExpanded(null);setSwipe(null)}}>Входящие лайки</button></div>}
    </header><div className="z-discovery-content">
    {profile.data&&<DiscoveryCity key={profile.data.id} profile={profile.data}/>}
    {profile.isPending?<p role="status">Загружаем профиль…</p>:profile.isError?<p className="notice">{errorText(profile.error)}</p>:!ready?<div className="z-discovery-empty"><Heart size={32} aria-hidden="true"/><h2>Расскажи немного о себе</h2><p>Добавь дату рождения и описание,<br/>чтобы знакомиться.</p><button className="primary" onClick={onSettings}>Заполнить профиль</button></div>:<>
      {match&&<p role="status">{match}</p>}{vote.isError&&<p className="notice" role="alert">{errorText(vote.error)}</p>}
      {active.isPending&&<p role="status">Загружаем карточки…</p>}{active.isError&&<p className="notice">{errorText(active.error)}</p>}
      {active.data?.length===0&&<p className="z-empty-copy">{tab==='likes'?'Входящих лайков пока нет.':'Пока больше никого нет. Загляни позже.'}</p>}
      {tab==='cards'&&cards.data?.[0]&&card(cards.data[0])}
      {tab==='likes'&&(expanded?<><button className="secondary" onClick={()=>setExpanded(null)}>Назад к лайкам</button>{card(expanded)}</>:<div className="z-likes-grid">{likes.data?.map(person=><button key={person.id} className="z-like-preview" onClick={()=>setExpanded(person)}><Avatar user={person} large/><strong>{person.name}, {person.age}</strong>{person.city&&<small>{person.city}</small>}</button>)}</div>)}
    </>}
    </div>
  </section>
}

