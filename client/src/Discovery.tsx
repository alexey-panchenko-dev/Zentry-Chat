import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, X, Heart } from 'lucide-react'
import { api, errorText } from '@/api'
import type { Profile, User } from '@/api'
import { Avatar } from '@/Visual'
import '@/discovery.css'

function useProfile() {
  return useQuery({ queryKey: ['profile'], queryFn: async () => (await api.get<{profile: Profile}>('/profile')).data.profile, staleTime: 60_000 })
}

export function ProfileEditor({ user }: { user: User }) {
  const profile = useProfile()
  if (profile.isPending) return <p role="status">Загружаем профиль…</p>
  if (profile.isError) return <p className="notice">{errorText(profile.error)}</p>
  return <ProfileForm key={user.id} profile={profile.data ?? {...user, age: null, bio: '', city: ''}} />
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
function ProfileForm({ profile }: {profile: Profile}) {
  const cache = useQueryClient()
  const [name, setName] = useState(profile.name)
  const [age, setAge] = useState(profile.age === null ? '' : String(profile.age))
  const [bio, setBio] = useState(profile.bio)
  const [city, setCity] = useState(profile.city)
  const save = useMutation({
    mutationFn: async () => {
      if (!name.trim() || Array.from(name).length > 100 || Array.from(bio).length > 100 || Array.from(city).length > 100 || (age !== '' && (!/^\d+$/.test(age) || !Number.isSafeInteger(Number(age)) || Number(age)>2147483647))) throw new Error('Проверьте имя, возраст и длину описания.')
      return (await api.put<{profile: Profile}>('/profile', {name, age: age === '' ? null : Number(age), bio, city})).data.profile
    },
    onSuccess: next => {
      cache.setQueryData(['profile'], next)
      cache.setQueryData(['session'], (old: User) => ({...old, name: next.name}))
      void cache.invalidateQueries({queryKey: ['discovery']})
    },
  })
  return <form className="z-profile-form" onSubmit={event => {event.preventDefault();save.mutate()}}>
    <label>Имя<input required maxLength={100} value={name} onChange={e=>setName(e.target.value)} /></label>
    <label>Возраст<input type="text" inputMode="numeric" pattern="[0-9]*" value={age} onChange={e=>setAge(e.target.value)} placeholder="Полных лет" /></label>
    <label>О себе<textarea value={bio} onChange={e=>setBio(e.target.value)} rows={3} aria-describedby="bio-help" /><small id="bio-help">{Array.from(bio).length}/100 символов</small></label>
    <label>Город <small>необязательно</small><input maxLength={100} value={city} onChange={e=>setCity(e.target.value)} /></label>
    <p className="z-field-help">Возраст и описание нужны для знакомств. Чаты доступны с любым заполнением профиля.</p>
    {save.isError&&<p className="notice" role="alert">{errorText(save.error)}</p>}
    {save.isSuccess&&<p role="status">Профиль сохранён.</p>}
    <button className="primary" disabled={save.isPending}>Сохранить профиль</button>
  </form>
}

export function Discovery({ onSettings, fallbackInterval }: { onSettings: () => void; fallbackInterval: number | false }) {
  const cache = useQueryClient()
  const profile = useProfile()
  const ready = profile.data?.age != null && !!profile.data.bio.trim()
  const [tab, setTab] = useState<'cards'|'likes'>('cards')
  const [expanded, setExpanded] = useState<Profile|null>(null)
  const [match, setMatch] = useState('')
  const cards = useQuery({ queryKey: ['discovery','cards'], enabled: ready && tab==='cards', queryFn: async () => (await api.get<{profiles: Profile[]}>('/discovery')).data.profiles, refetchInterval: fallbackInterval })
  const likes = useQuery({ queryKey: ['discovery','likes'], enabled: ready && tab==='likes', queryFn: async () => (await api.get<{profiles: Profile[]}>('/discovery/likes')).data.profiles, refetchInterval: fallbackInterval })
  const vote = useMutation({
    mutationFn: async ({id,liked}: {id:string;liked:boolean}) => (await api.post<{matched:boolean;conversation_id:string}>('/discovery/votes',{user_id:id,liked})).data,
    onSuccess: (result, variables) => {
      setExpanded(null)
      if(result.matched) setMatch('Вы стали друзьями! Переписка появилась в чатах.')
      // Remove the acted-on card immediately, including when its page is inactive.
      for (const key of ['cards','likes']) cache.setQueryData<Profile[]>(['discovery',key],old=>old?.filter(p=>p.id!==variables.id))
      void cache.invalidateQueries({queryKey:['discovery']})
      void cache.invalidateQueries({queryKey:['conversations']})
      void cache.invalidateQueries({queryKey:['unread']})
    },
  })
  const actions = (person: Profile) => <div className="z-discovery-actions"><button className="z-skip" disabled={vote.isPending} aria-label="Не подходит" onClick={()=>vote.mutate({id:person.id,liked:false})}><X size={30}/></button><button className="z-like" disabled={vote.isPending} aria-label="Нравится" onClick={()=>vote.mutate({id:person.id,liked:true})}><Check size={30}/></button></div>
  const card = (person: Profile) => <article className="z-discovery-card"><DiscoveryPhoto key={person.id} user={person}/><div className="z-discovery-details"><h2>{person.name}, {person.age}</h2>{person.city&&<p className="z-muted">{person.city}</p>}<p className="z-discovery-bio">{person.bio}</p></div>{actions(person)}</article>
  const active = tab==='cards'?cards:likes
  return <section className="z-discovery"><header className="z-discovery-header"><h1><Heart/> Знакомства</h1>
    {ready&&<div className="z-discovery-tabs"><button className={tab==='cards'?'is-active':''} onClick={()=>{setTab('cards');setExpanded(null)}}>Люди</button><button className={tab==='likes'?'is-active':''} onClick={()=>{setTab('likes');setExpanded(null)}}>Входящие лайки</button></div>}
    </header><div className="z-discovery-content">
    {profile.isPending?<p role="status">Загружаем профиль…</p>:profile.isError?<p className="notice">{errorText(profile.error)}</p>:!ready?<div className="z-discovery-empty"><Heart size={32} aria-hidden="true"/><h2>Расскажи немного о себе</h2><p>Добавь возраст и описание,<br/>чтобы знакомиться.</p><button className="primary" onClick={onSettings}>Заполнить профиль</button></div>:<>
      {match&&<p role="status">{match}</p>}{vote.isError&&<p className="notice" role="alert">{errorText(vote.error)}</p>}
      {active.isPending&&<p role="status">Загружаем карточки…</p>}{active.isError&&<p className="notice">{errorText(active.error)}</p>}
      {active.data?.length===0&&<p className="z-empty-copy">{tab==='likes'?'Входящих лайков пока нет.':'Пока больше никого нет. Загляни позже.'}</p>}
      {tab==='cards'&&cards.data?.[0]&&card(cards.data[0])}
      {tab==='likes'&&(expanded?<><button className="secondary" onClick={()=>setExpanded(null)}>Назад к лайкам</button>{card(expanded)}</>:<div className="z-likes-grid">{likes.data?.map(person=><button key={person.id} className="z-like-preview" onClick={()=>setExpanded(person)}><Avatar user={person} large/><strong>{person.name}, {person.age}</strong>{person.city&&<small>{person.city}</small>}</button>)}</div>)}
    </>}
    </div>
  </section>
}
