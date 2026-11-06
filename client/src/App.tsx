import { ContactProfile } from '@/ContactProfile'
import { ContactSettings } from '@/ContactSettings'
import type { ProfileCallRequest } from './contact-profile'
import { MediaAvailability } from '@/MediaAvailability'
import { watchConversations } from '@/conversation-updates'
import { createMediaQueryStore } from './media-query'
import { VoiceComposer } from '@/VoiceComposer'
import { VoiceMessage } from '@/VoiceMessage'
import { ProfilePhoto } from '@/ProfilePhoto'
import { CallPanel } from '@/CallPanel'
import { useEffect, useRef, useState, useSyncExternalStore } from 'react'
import type { FormEvent } from 'react'
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { subscribeRealtime } from './realtime.ts'
import { Discovery, ProfileEditor } from '@/Discovery'
import { useReadReceipt } from './read-receipt.ts'
import { LogOut, MessageCircle, RefreshCw, Search, Send, Wifi, Settings, Phone, ArrowLeft, Plus, Check, CheckCheck, Heart, ChevronsLeft } from 'lucide-react'
import { Brand, Avatar } from '@/Visual'
import { api, errorText } from '@/api'
import type { Conversation, Message, MessagePage, ReadState, User } from '@/api'

export default function App() {
  const cache = useQueryClient()
  const [, refreshSessionView] = useState(0)
  const sessionViewChanged = () => refreshSessionView(value => value + 1)
  const session = useQuery({
    queryKey: ['session'],
    queryFn: async () => {
      try { return (await api.get<{ user: User }>('/auth/me')).data.user }
      catch (error) {
        if ((error as { response?: { status: number } }).response?.status === 401) return null
        throw error
      }
    },
  })

  useEffect(() => {
    const interceptor = api.interceptors.response.use(undefined, (error) => {
      if (error.response?.status === 401 && !error.config?.url?.startsWith('/auth/')) {
        cache.clear()
        cache.setQueryData(['session'], null)
      }
      return Promise.reject(error)
    })
    return () => api.interceptors.response.eject(interceptor)
  }, [cache])

  return <div className={session.data ? 'z-root' : 'z-auth-page'}>
    {session.isPending ? <div className="z-loading" role="status"><Brand/><p>Открываем Zently…</p></div>
      : session.isError ? <div className="z-auth-card"><Brand/><p className="notice">{errorText(session.error)}</p><button className="secondary" onClick={() => void session.refetch()}>Повторить</button></div>
      : session.data ? <Chat key={session.data.id} user={session.data} onSessionViewChanged={sessionViewChanged} /> : <Auth onSessionViewChanged={sessionViewChanged} />}
  </div>
}

function Auth({onSessionViewChanged}:{onSessionViewChanged:()=>void}) {
  const cache = useQueryClient()
  const [register, setRegister] = useState(false)
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [validation,setValidation]=useState<{field:string;message:string}|null>(null)
  const auth = useMutation({
    mutationFn: async () => {
      if (register) await api.post('/auth/register', { name, email, password })
      return (await api.post<{ user: User }>('/auth/login', { email, password })).data.user
    },
    onSuccess: (user) => { cache.clear(); cache.setQueryData(['session'], user); onSessionViewChanged() },
  })
  function submitAuth(event:FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if(auth.isPending)return
    const form=event.currentTarget
    const fields=Array.from(form.querySelectorAll('input'))
    for(const field of fields){
      let message=''
      if(field.name==='name'&&(!field.value.trim()||field.value.length>100))message='Введите имя профиля от 1 до 100 символов.'
      else if(field.name==='email'&&!field.checkValidity())message='Введите корректный адрес почты, например you@example.com.'
      else if(field.name==='password'&&(!field.checkValidity()||field.value.length<12||new TextEncoder().encode(field.value).length>72))message='Введите пароль от 12 символов, не более 72 байт.'
      if(message){setValidation({field:field.name,message});field.focus();return}
    }
    setValidation(null);auth.mutate()
  }
  return <section className="z-auth-card">
    <Brand/><p className="z-eyebrow">БЛИЖЕ ДРУГ К ДРУГУ</p><h1>{register ? <>Твой круг.<br/>Твои разговоры.</> : <>С возвращением<br/>в Zently.</>}</h1>
    <p className="z-muted">{register ? 'Создай профиль и будь на связи.' : 'Войди, чтобы продолжить разговор.'}</p>
    <form noValidate className="z-auth-form" onSubmit={submitAuth}>
      {register && <label>Имя профиля<input name="name" aria-invalid={validation?.field==='name'} aria-describedby={validation?.field==='name'?'auth-validation':undefined} required maxLength={100} value={name} onChange={(e) => setName(e.target.value)} autoComplete="name" placeholder="Как к тебе обращаться" /></label>}
      <label>Почта<input name="email" aria-invalid={validation?.field==='email'} aria-describedby={validation?.field==='email'?'auth-validation':undefined} type="email" required value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="email" placeholder="you@example.com" /></label>
      <label>Пароль<input name="password" aria-invalid={validation?.field==='password'} aria-describedby={validation?.field==='password'?'auth-validation':undefined} type="password" required minLength={12} value={password} onChange={(e) => setPassword(e.target.value)} autoComplete={register ? 'new-password' : 'current-password'} placeholder="Не менее 12 символов" /></label>
      <p className="z-field-help">От 12 до 72 байт. Латинские буквы и цифры.</p>
      {validation&&<p id="auth-validation" className="notice" role="alert">{validation.message}</p>}
      {auth.isError && <p className="notice" role="alert">{errorText(auth.error)}{register && ' Если аккаунт уже создался, переключитесь на вход.'}</p>}
      <button className="primary" disabled={auth.isPending}>{auth.isPending ? 'Подождите…' : register ? 'Создать аккаунт' : 'Войти'}<ArrowLeft className="z-arrow-forward" size={18}/></button>
    </form>
    <button className="z-auth-switch" onClick={() => { setRegister(!register); setValidation(null); auth.reset() }}>{register ? 'Уже есть аккаунт? Войти' : 'Нет аккаунта? Создать профиль'}</button>
  </section>
}

function Chat({ user, onSessionViewChanged }: { user: User; onSessionViewChanged:()=>void }) {
  const cache = useQueryClient()
  const [selected, setSelected] = useState<Conversation | null>(null)
  const [search, setSearch] = useState('')
  const [query, setQuery] = useState('')
  const [searchError,setSearchError]=useState('')
  const [uiSection, setUiSection] = useState<'chats'|'calls'|'settings'|'discovery'>('chats')
  const [callActive,setCallActive]=useState(false)
  const [requestCall,setRequestCall]=useState<ProfileCallRequest|null>(null)
  const [compact, setCompact] = useState(false)
  const [searchOpen, setSearchOpen] = useState(false)
  const [mobileChat, setMobileChat] = useState(false)
  const [mobileLayout] = useState(()=>createMediaQueryStore(window.matchMedia('(max-width: 720px)')))
  const isMobile = useSyncExternalStore(mobileLayout.subscribe,mobileLayout.getSnapshot)
  const [soundsEnabled,setSoundsEnabled] = useState(()=>localStorage.getItem('zently-call-sounds')!=='0')
  const [realtimeReady, setRealtimeReady] = useState(false)
  useEffect(() => watchConversations(user.id, event => {
    if(event.reason==='discovery') {
      void cache.invalidateQueries({queryKey:['discovery']})
      return
    }
    void cache.invalidateQueries({ queryKey: ['conversations'] })
    void cache.invalidateQueries({ queryKey: ['unread'] })
    if(event.reason==='connected') {
      void cache.invalidateQueries({queryKey:['discovery']})
      void cache.invalidateQueries({queryKey:['messages']})
      void cache.invalidateQueries({queryKey:['read-state']})
    }
    if(event.conversation_id && event.reason!=='message') void cache.invalidateQueries({queryKey:['read-state',event.conversation_id]})
    if(event.conversation_id && event.reason==='conversation') void cache.invalidateQueries({queryKey:['messages',event.conversation_id]})
  }, setRealtimeReady), [cache, user.id])
  const unread = useQuery({queryKey:['unread'], queryFn:async()=>(await api.get<{unread_chats:number}>('/unread')).data, refetchInterval:realtimeReady?false:60_000})
  const dialogs = useInfiniteQuery({
    queryKey: ['conversations'], initialPageParam: '',
    refetchInterval: realtimeReady ? false : 60_000,
    queryFn: async ({ pageParam }) => (await api.get<{ conversations: Conversation[]; next_cursor: string }>('/conversations', { params: { limit: 30, before: pageParam || undefined } })).data,
    getNextPageParam: (page) => page.next_cursor || undefined,
  })
  const users = useQuery({
    queryKey: ['users', query], enabled: query.length >= 2,
    queryFn: async () => (await api.get<{ users: User[] }>('/users', { params: { query } })).data.users,
  })
  const create = useMutation({
    mutationFn: async (id: string) => (await api.post<{ conversation: Conversation }>('/conversations', { user_id: id })).data.conversation,
    onSuccess: (dialog) => { setSelected(dialog); setUiSection('chats'); setMobileChat(true); setSearchOpen(false); setQuery(''); setSearch(''); void cache.invalidateQueries({ queryKey: ['conversations'] }) },
  })
  const logout = useMutation({
    mutationFn: () => api.post('/auth/logout', {}),
    onSuccess: () => { cache.clear(); cache.setQueryData(['session'], null); onSessionViewChanged() },
  })

  return <div data-ui-section={uiSection} className={'z-shell'+(compact?' is-compact':'')+(mobileChat?' has-mobile-chat':'')}>
    <aside className="z-sidebar"><div className="z-sidebar-head"><Brand/></div>
      <nav aria-label="Основная навигация">
        <button className={uiSection==='chats'?'is-active':''} title="Чаты" onClick={()=>{setUiSection('chats');setMobileChat(false)}}><MessageCircle/><span>Чаты</span>{!!unread.data?.unread_chats&&<b className="z-unread" aria-label={`Непрочитанных чатов: ${unread.data.unread_chats}`}>{unread.data.unread_chats}</b>}</button>
        <button className={uiSection==='discovery'?'is-active':''} title="Знакомства" onClick={()=>{setUiSection('discovery');setMobileChat(true)}}><Heart/><span>Знакомства</span></button>
        <button className={uiSection==='calls'?'is-active':''} title="Звонки" onClick={()=>{setUiSection('calls');setMobileChat(true)}}><Phone/><span>Звонки</span></button>
        <button className={uiSection==='settings'?'is-active':''} title="Настройки" onClick={()=>{setUiSection('settings');setMobileChat(true)}}><Settings/><span>Настройки</span></button>
      </nav>
      <button type="button" className="z-icon z-collapse" aria-label={compact?'Развернуть меню':'Свернуть меню'} aria-expanded={!compact} onClick={()=>setCompact(!compact)}><ChevronsLeft size={18}/></button>
      <button className="z-account" aria-label="Мой профиль" onClick={()=>{setUiSection('settings');setMobileChat(true)}}><Avatar user={user}/><span><strong>{user.name}</strong><small>Мой профиль</small></span></button>
    </aside>
    <aside className="z-dialogs"><div className="z-list-top"><span>Чаты</span><div><button className="z-icon" aria-label="Обновить переписки" onClick={()=>void dialogs.refetch()}><RefreshCw size={16}/></button><button className="z-icon" aria-label="Найти человека" onClick={()=>setSearchOpen(!searchOpen)}><Plus/></button></div></div>
      <form noValidate className="z-search" onSubmit={(e)=>{e.preventDefault();const input=e.currentTarget.querySelector('input')!;const text=search.trim();if(!input.checkValidity()||text.length<2||text.length>100){setSearchError('Введите имя или почту от 2 до 100 символов.');input.focus();return}setSearchError('');setQuery(text);setSearchOpen(true)}}><Search size={18}/><input aria-invalid={!!searchError} aria-describedby={searchError?'search-validation':undefined} aria-label="Поиск пользователя" placeholder="Имя или точная почта" value={search} onChange={e=>{setSearch(e.target.value);setSearchError('')}} minLength={2} maxLength={100} required/>{search && <button type="button" className="z-clear" aria-label="Очистить поиск" onClick={()=>{setSearch('');setQuery('');setSearchError('')}}>×</button>}<button className="z-icon" aria-label="Найти"><ArrowLeft size={16} className="z-arrow-forward"/></button></form>
      {searchError&&<p id="search-validation" className="notice" role="alert">{searchError}</p>}
      {searchOpen && <div className="z-search-results">{!query&&<p className="z-field-help">Введи имя профиля или точную почту и нажми стрелку.</p>}{users.isFetching&&<p role="status">Поиск…</p>}{users.isError&&<p className="notice">{errorText(users.error)}</p>}{query&&users.data?.length===0&&<p>Никого не найдено.</p>}{query&&users.data?.map(peer=><button className="z-contact" key={peer.id} disabled={create.isPending} onClick={()=>create.mutate(peer.id)}><Avatar user={peer}/><span>{peer.name}</span><MessageCircle size={17}/></button>)}</div>}
      {create.isError&&<p className="notice">{errorText(create.error)}</p>}
      <div className="z-conversation-list">{dialogs.isError&&<p className="notice">{errorText(dialogs.error)}</p>}{dialogs.isPending&&<p className="z-empty-copy" role="status">Загрузка…</p>}
        {dialogs.data?.pages.flatMap(page=>page.conversations).map(dialog=><button key={dialog.id} onClick={()=>{setSelected(dialog);setUiSection('chats');setMobileChat(true)}} className={'z-conversation'+(selected?.id===dialog.id?' is-selected':'')}><Avatar user={dialog.participant}/><span><strong>{dialog.participant.name}</strong><small>Открыть переписку</small></span>{!!dialog.unread_count&&<b className="z-unread" aria-label={`Непрочитанных сообщений: ${dialog.unread_count}`}>{dialog.unread_count}</b>}</button>)}
        {dialogs.data?.pages[0].conversations.length===0&&<div className="z-empty-copy"><MessageCircle/><p>Твой круг начинается здесь.</p><small>Найди человека по имени или почте.</small></div>}
        {dialogs.hasNextPage&&<button className="secondary" disabled={dialogs.isFetchingNextPage} onClick={()=>void dialogs.fetchNextPage()}>Ещё переписки</button>}
      </div>
    </aside>
    <div className="z-workspace">
      <button className="z-mobile-back z-icon" aria-label="Назад к чатам" onClick={()=>{setMobileChat(false);setUiSection('chats')}}><ArrowLeft size={20}/></button>
      {selected&&<div className="z-message-view" hidden={uiSection!=='chats'}><Messages key={selected.id} conversation={selected} user={user} visible={uiSection==='chats' && (mobileChat || !isMobile)} callActive={callActive} realtimeReady={realtimeReady} onProfileCall={()=>setRequestCall({id:crypto.randomUUID(),conversationID:selected.id})}/></div>}
      {uiSection==='settings'?<section className="z-settings"><h1>Настройки</h1><div className="z-profile"><Avatar user={user} large/><h2>{user.name}</h2><p className="z-muted">Личный профиль</p><MediaAvailability feature="profile_photos"><ProfilePhoto user={user}/></MediaAvailability></div><ProfileEditor user={user}/><MediaAvailability feature="contact_profiles"><ContactSettings/></MediaAvailability><label className="z-setting-row">Компактное меню<input type="checkbox" checked={compact} onChange={e=>setCompact(e.target.checked)}/></label><label className="z-setting-row">Звуки звонков<input type="checkbox" checked={soundsEnabled} onChange={e=>{setSoundsEnabled(e.target.checked);localStorage.setItem("zently-call-sounds",e.target.checked?"1":"0")}}/></label><p className="z-muted">Вход в аккаунт — по почте и паролю.</p>{logout.isError&&<p className="notice">{errorText(logout.error)}</p>}<button className="secondary z-danger" disabled={logout.isPending} onClick={()=>logout.mutate()}><LogOut size={17}/>Выйти из аккаунта</button></section>
      :uiSection==='discovery'?<Discovery fallbackInterval={realtimeReady?false:60_000} onSettings={()=>setUiSection('settings')}/>
      :uiSection==='calls'?<section className="z-call-lobby"><Phone size={40}/><h1>Ближе голосом</h1><p className="z-muted">{selected?'Начни видеозвонок с '+selected.participant.name:'Выбери переписку, чтобы позвонить.'}</p><p className="z-field-help">Кнопка звонка появится, когда выберешь собеседника.</p></section>
      :!selected?<div className="z-empty"><MessageCircle size={46}/><h1>На одной волне</h1><p>Выбери разговор или найди человека,<br/>с которым хочешь быть на связи.</p></div>:null}
    </div>
    <CallPanel requestCall={requestCall} onActiveChange={setCallActive} user={user} selected={selected} soundsEnabled={soundsEnabled} conversations={dialogs.data?.pages.flatMap(page=>page.conversations)||[]}/>
  </div>
}

function Messages({ conversation, user, visible, callActive, realtimeReady, onProfileCall }: { conversation: Conversation; user: User; visible:boolean;callActive:boolean;realtimeReady:boolean;onProfileCall:()=>void }) {
  const cache = useQueryClient()
  const [body, setBody] = useState('')
  const [profileOpen,setProfileOpen]=useState(false)
  const [status, setStatus] = useState('Подключаемся…')
  const [events, setEvents] = useState(0)
  const [live, setLive] = useState<Record<string, Message>>({})
  const [delivery, setDelivery] = useState('')
  const pending = useRef<{ body: string; id: string } | null>(null)
  const bottom = useRef<HTMLDivElement>(null)
  const history = useInfiniteQuery({
    enabled: visible,
    queryKey: ['messages', conversation.id], initialPageParam: '',
    queryFn: async ({ pageParam }) => (await api.get<MessagePage>(`/conversations/${conversation.id}/messages`, { params: { before: pageParam || undefined, limit: 50 } })).data,
    getNextPageParam: (page) => page.next_cursor || undefined,
    refetchInterval: visible && !realtimeReady ? 60_000 : false,
  })

  const readState = useQuery({enabled:visible, queryKey:['read-state',conversation.id], queryFn:async()=>(await api.get<ReadState>(`/conversations/${conversation.id}/read`)).data, refetchInterval:realtimeReady?false:60_000})
  useEffect(() => subscribeRealtime(`private-conversation-${conversation.id}`, {
    'message.created': ({message}: {message: Message}) => {
      if(message.conversation_id!==conversation.id) return
      setLive(old=>({...old,[message.id]:message}))
      setEvents(count=>count+1)
    },
  }, ready => {
    setStatus(ready?'Pusher: подписка активна':'Связь восстанавливается…')
    if(ready) {
      void cache.invalidateQueries({queryKey:['messages',conversation.id]})
      void cache.invalidateQueries({queryKey:['read-state',conversation.id]})
    }
  }), [cache,conversation.id])

  const send = useMutation({
    mutationFn: async (text: string) => {
      if (pending.current?.body !== text) pending.current = { body: text, id: crypto.randomUUID() }
      return (await api.post<{ message: Message; realtime: string }>(`/conversations/${conversation.id}/messages`, {
        body: text, client_message_id: pending.current.id,
      })).data
    },
    onSuccess: (result) => {
      setLive((old) => ({ ...old, [result.message.id]: result.message }))
      setBody(''); pending.current = null
      setDelivery(result.realtime === 'published' ? 'Сохранено, событие принято Pusher.' : result.realtime === 'failed' ? 'Сохранено в БД, но Pusher не принял событие. Собеседнику нужно обновить историю.' : 'Сохранено в БД. Pusher отключён.')
    },
  })
  const merged = new Map<string, Message>()
  history.data?.pages.forEach((page) => page.messages.forEach((message) => merged.set(message.id, message)))
  Object.values(live).forEach((message) => merged.set(message.id, message))
  const messages = [...merged.values()].sort((a, b) => BigInt(a.id) < BigInt(b.id) ? -1 : BigInt(a.id) > BigInt(b.id) ? 1 : 0)
  const latest = messages.at(-1)?.id
  useReadReceipt(conversation.id, latest, visible && !callActive && !profileOpen, bottom)
  useEffect(() => { bottom.current?.scrollIntoView({ behavior: 'smooth', block: 'nearest' }) }, [latest])
  function submit(event: FormEvent) { event.preventDefault(); if (body.trim() && !send.isPending) send.mutate(body.trim()) }

  return <section className="z-messages">
    {profileOpen&&visible&&<ContactProfile conversation={conversation} onClose={()=>setProfileOpen(false)} onCall={onProfileCall} callBusy={callActive}/>}
    <header className="z-chat-header"><button type="button" className="z-header-profile" aria-label={'Открыть профиль: '+conversation.participant.name} onClick={()=>setProfileOpen(true)}><Avatar user={conversation.participant}/><span><strong>{conversation.participant.name}</strong><small>{status.includes('подписка активна')?'Соединение установлено':'Личная переписка'}</small></span></button><button className="z-icon z-history" title="Обновить историю" aria-label="Обновить историю" onClick={()=>void history.refetch()} disabled={history.isFetching}><RefreshCw size={18}/></button></header>
    <div className="z-message-scroll">
      {history.hasNextPage&&<button className="secondary z-older" disabled={history.isFetchingNextPage} onClick={()=>void history.fetchNextPage()}>Более ранние сообщения</button>}
      {history.isPending&&<p className="z-empty-copy" role="status">Загружаем историю…</p>}{history.isError&&<p className="notice">{errorText(history.error)}</p>}
      {!history.isPending&&!history.isError&&messages.length===0&&<div className="z-empty-copy"><p>Начни разговор</p><small>Напиши первое сообщение.</small></div>}
      {messages.map(message=><div key={message.id} className={'z-bubble-row'+(message.kind==='system'?' is-system':message.sender_id===user.id?' is-own':'')}><div className="z-bubble"><>{message.kind==='voice'&&message.audio_url?.startsWith('/api/messages/')?<VoiceMessage url={message.audio_url} duration={message.duration_ms}/>:<p>{message.body}</p>}</><div className="z-message-meta"><time>{new Date(message.created_at).toLocaleTimeString('ru-RU',{hour:'2-digit',minute:'2-digit'})}</time>{message.sender_id===user.id&&(BigInt(message.id)<=BigInt(readState.data?.peer_last_read_id??'0')?<CheckCheck size={16} aria-label="Прочитано" className="z-read-check"/>:<Check size={16} aria-label="Сохранено на сервере"/>)}</div></div></div>)}<div ref={bottom} style={{minHeight:1}}/>
    </div>
    <form noValidate className="z-composer-region" onSubmit={submit}>{send.isError&&<p className="notice" role="alert">{errorText(send.error)} Повторите отправку: прежний идентификатор сохранён.</p>}
      <div className="z-composer"><div className="z-composer-field"><textarea className="resize-none" aria-label="Сообщение" placeholder="Сообщение…" rows={1} maxLength={1000} value={body} disabled={send.isPending} onChange={e=>setBody(e.target.value)} onKeyDown={e=>{if(e.key==='Enter'&&!e.shiftKey&&!e.nativeEvent.isComposing){e.preventDefault();e.currentTarget.form?.requestSubmit()}}}/></div><MediaAvailability feature="voice"><VoiceComposer conversationID={conversation.id} disabled={callActive||!visible} onSent={result=>{setLive(old=>({...old,[result.message.id]:result.message}));setDelivery(result.realtime==='published'?'Голосовое сообщение сохранено.':'Запись сохранена. Собеседнику может понадобиться обновить историю.')}}/></MediaAvailability><button className="z-send z-text-send" aria-label="Отправить сообщение" disabled={!body.trim()||send.isPending}><Send size={23}/></button></div>
      <details className="z-connection"><summary><Wifi size={11}/>{send.isPending?'Отправляем…':'Состояние соединения'}</summary><p role="status">{status}</p><p>Получено событий: {events}. {delivery||'До 1000 символов. Enter — отправить, Shift+Enter — новая строка.'}</p></details>
    </form>
  </section>
}
