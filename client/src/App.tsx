import { CallPanel } from '@/CallPanel'
import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import Pusher from 'pusher-js'
import { LogOut, MessageCircle, RefreshCw, Search, Send, Wifi, Settings, Phone, ArrowLeft, Plus, Check, ChevronsLeft } from 'lucide-react'
import { RecoveryRequest, PasswordReset } from '@/PasswordRecovery'
import { Bookmark } from 'lucide-react'
import { Brand, Avatar } from '@/Visual'
import { api, errorText } from '@/api'
import type { Conversation, Message, MessagePage, User } from '@/api'

export default function App() {
  const [resetToken,setResetToken] = useState(() => { const match = window.location.hash.match(/^#reset-password=([a-f0-9]{64})$/); return match?.[1] ?? '' })
  useEffect(() => { if (resetToken) window.history.replaceState(null,'',window.location.pathname+window.location.search) },[resetToken])
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

  if (resetToken) return <div className="z-auth-page"><PasswordReset token={resetToken} onDone={()=>{setResetToken('');cache.clear();cache.setQueryData(['session'],null);sessionViewChanged()}}/></div>

  return <div className={session.data ? 'z-root' : 'z-auth-page'}>
    {session.isPending ? <div className="z-loading" role="status"><Brand/><p>Открываем Zently…</p></div>
      : session.isError ? <div className="z-auth-card"><Brand/><p className="notice">{errorText(session.error)}</p><button className="secondary" onClick={() => void session.refetch()}>Повторить</button></div>
      : session.data ? <Chat key={session.data.id} user={session.data} onSessionViewChanged={sessionViewChanged} /> : <Auth onSessionViewChanged={sessionViewChanged} />}
  </div>
}

function Auth({onSessionViewChanged}:{onSessionViewChanged:()=>void}) {
  const cache = useQueryClient()
  const [recover,setRecover] = useState(false)
  const [register, setRegister] = useState(false)
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const auth = useMutation({
    mutationFn: async () => {
      if (register) await api.post('/auth/register', { name, email, password })
      return (await api.post<{ user: User }>('/auth/login', { email, password })).data.user
    },
    onSuccess: (user) => { cache.clear(); cache.setQueryData(['session'], user); onSessionViewChanged() },
  })
  if (recover) return <section className="z-auth-card"><Brand/><RecoveryRequest onBack={()=>setRecover(false)}/></section>
  return <section className="z-auth-card">
    <Brand/><p className="z-eyebrow">БЛИЖЕ ДРУГ К ДРУГУ</p><h1>{register ? <>Твой круг.<br/>Твои разговоры.</> : <>С возвращением<br/>в Zently.</>}</h1>
    <p className="z-muted">{register ? 'Создай профиль и будь на связи.' : 'Войди, чтобы продолжить разговор.'}</p>
    <form className="z-auth-form" onSubmit={(event) => { event.preventDefault(); auth.mutate() }}>
      {register && <label>Имя профиля<input required maxLength={100} value={name} onChange={(e) => setName(e.target.value)} autoComplete="name" placeholder="Как к тебе обращаться" /></label>}
      <label>Почта<input type="email" required value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="email" placeholder="you@example.com" /></label>
      <label>Пароль<input type="password" required minLength={12} value={password} onChange={(e) => setPassword(e.target.value)} autoComplete={register ? 'new-password' : 'current-password'} placeholder="Не менее 12 символов" /></label>
      <p className="z-field-help">От 12 до 72 байт. Латинские буквы и цифры.</p>
      {auth.isError && <p className="notice" role="alert">{errorText(auth.error)}{register && ' Если аккаунт уже создался, переключитесь на вход.'}</p>}
      <button className="primary" disabled={auth.isPending}>{auth.isPending ? 'Подождите…' : register ? 'Создать аккаунт' : 'Войти'}<ArrowLeft className="z-arrow-forward" size={18}/></button>
    </form>
    {!register&&<button className="z-auth-switch" onClick={()=>setRecover(true)}>Забыли пароль?</button>}
    <button className="z-auth-switch" onClick={() => { setRegister(!register); auth.reset() }}>{register ? 'Уже есть аккаунт? Войти' : 'Нет аккаунта? Создать профиль'}</button>
  </section>
}

function Chat({ user, onSessionViewChanged }: { user: User; onSessionViewChanged:()=>void }) {
  const cache = useQueryClient()
  const [selected, setSelected] = useState<Conversation | null>(null)
  const [search, setSearch] = useState('')
  const [query, setQuery] = useState('')
  const [uiSection, setUiSection] = useState<'chats'|'calls'|'settings'>('chats')
  const [compact, setCompact] = useState(false)
  const [searchOpen, setSearchOpen] = useState(false)
  const [mobileChat, setMobileChat] = useState(false)
  const [recoveryOpen,setRecoveryOpen] = useState(false)
  const saved = useMutation({mutationFn:async()=>(await api.post<{conversation:Conversation}>('/conversations/saved',{})).data.conversation,onSuccess:dialog=>{setSelected(dialog);setUiSection('chats');setMobileChat(true);void cache.invalidateQueries({queryKey:['conversations']})}})
  const [soundsEnabled,setSoundsEnabled] = useState(()=>localStorage.getItem('zently-call-sounds')!=='0')
  const dialogs = useInfiniteQuery({
    queryKey: ['conversations'], initialPageParam: '',
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
        <button className={uiSection==='chats'?'is-active':''} title="Чаты" onClick={()=>{setUiSection('chats');setMobileChat(false)}}><MessageCircle/><span>Чаты</span></button>
        <button className={uiSection==='calls'?'is-active':''} title="Звонки" onClick={()=>{setUiSection('calls');setMobileChat(true)}}><Phone/><span>Звонки</span></button>
        <button className={uiSection==='settings'?'is-active':''} title="Настройки" onClick={()=>{setUiSection('settings');setMobileChat(true)}}><Settings/><span>Настройки</span></button>
      </nav>
      <button className="z-account" aria-label="Мой профиль" onClick={()=>{setUiSection('settings');setMobileChat(true)}}><Avatar user={user}/><span><strong>{user.name}</strong><small>Мой профиль</small></span></button>
    <button className="z-icon z-collapse" aria-expanded={!compact} aria-label={compact?'Развернуть меню':'Свернуть меню'} onClick={()=>setCompact(!compact)}><ChevronsLeft size={18}/></button></aside>
    <aside className="z-dialogs"><div className="z-list-top"><span>Чаты</span><div><button className="z-icon" aria-label="Обновить переписки" onClick={()=>void dialogs.refetch()}><RefreshCw size={16}/></button><button className="z-icon" aria-label="Найти человека" onClick={()=>setSearchOpen(!searchOpen)}><Plus/></button></div></div>
      <form className="z-search" onSubmit={(e)=>{e.preventDefault();setQuery(search.trim());setSearchOpen(true)}}><Search size={18}/><input aria-label="Поиск пользователя" placeholder="Имя или точная почта" value={search} onChange={e=>setSearch(e.target.value)} minLength={2} maxLength={100} required/>{search && <button type="button" className="z-clear" aria-label="Очистить поиск" onClick={()=>{setSearch('');setQuery('')}}>×</button>}<button className="z-icon" aria-label="Найти"><ArrowLeft size={16} className="z-arrow-forward"/></button></form>
      {searchOpen && <div className="z-search-results">{!query&&<p className="z-field-help">Введи имя профиля или точную почту и нажми стрелку.</p>}{users.isFetching&&<p role="status">Поиск…</p>}{users.isError&&<p className="notice">{errorText(users.error)}</p>}{query&&users.data?.length===0&&<p>Никого не найдено.</p>}{query&&users.data?.map(peer=><button className="z-contact" key={peer.id} disabled={create.isPending} onClick={()=>create.mutate(peer.id)}><Avatar user={peer}/><span>{peer.name}</span><MessageCircle size={17}/></button>)}</div>}
      {create.isError&&<p className="notice">{errorText(create.error)}</p>}
      <div className="z-conversation-list"><button className={'z-conversation z-saved'+(selected?.participant.id===user.id?' is-selected':'')} disabled={saved.isPending} onClick={()=>saved.mutate()}><span className="z-avatar z-saved-avatar"><Bookmark size={22}/></span><span><strong>Избранное</strong><small>{saved.isPending?'Открываем…':'Только для тебя'}</small></span></button>{saved.isError&&<p className="notice" role="alert">Не удалось открыть Избранное. {errorText(saved.error)}</p>}{dialogs.isError&&<p className="notice">{errorText(dialogs.error)}</p>}{dialogs.isPending&&<p className="z-empty-copy" role="status">Загрузка…</p>}
        {dialogs.data?.pages.flatMap(page=>page.conversations).filter(dialog=>dialog.participant.id!==user.id).map(dialog=><button key={dialog.id} onClick={()=>{setSelected(dialog);setUiSection('chats');setMobileChat(true)}} className={'z-conversation'+(selected?.id===dialog.id?' is-selected':'')}><Avatar user={dialog.participant}/><span><strong>{dialog.participant.name}</strong><small>Открыть переписку</small></span></button>)}
        {dialogs.data?.pages[0].conversations.length===0&&<div className="z-empty-copy"><MessageCircle/><p>Твой круг начинается здесь.</p><small>Найди человека по имени или почте.</small></div>}
        {dialogs.hasNextPage&&<button className="secondary" disabled={dialogs.isFetchingNextPage} onClick={()=>void dialogs.fetchNextPage()}>Ещё переписки</button>}
      </div>
    </aside>
    <div className="z-workspace">
      <button className="z-mobile-back z-icon" aria-label="Назад к чатам" onClick={()=>{setMobileChat(false);setUiSection('chats')}}><ArrowLeft size={20}/></button>
      {selected&&<div className="z-message-view" hidden={uiSection!=='chats'}><Messages key={selected.id} conversation={selected} user={user}/></div>}
      {uiSection==='settings'?<section className="z-settings"><h1>Настройки</h1><div className="z-profile"><Avatar user={user} large/><h2>{user.name}</h2><p className="z-muted">Личный профиль</p></div><label className="z-setting-row">Звуки звонков<input type="checkbox" checked={soundsEnabled} onChange={e=>{setSoundsEnabled(e.target.checked);localStorage.setItem("zently-call-sounds",e.target.checked?"1":"0")}}/></label><p className="z-muted">Вход в аккаунт — по почте и паролю.</p><button className="secondary" onClick={()=>setRecoveryOpen(!recoveryOpen)} aria-expanded={recoveryOpen}>Изменить пароль</button>{recoveryOpen&&<RecoveryRequest onBack={()=>setRecoveryOpen(false)}/>}{logout.isError&&<p className="notice">{errorText(logout.error)}</p>}<button className="secondary z-danger" disabled={logout.isPending} onClick={()=>logout.mutate()}><LogOut size={17}/>Выйти из аккаунта</button></section>
      :uiSection==='calls'?<section className="z-call-lobby"><Phone size={40}/><h1>Ближе голосом</h1><p className="z-muted">{selected&&selected.participant.id!==user.id?'Начни видеозвонок с '+selected.participant.name:'Выбери переписку, чтобы позвонить.'}</p><p className="z-field-help">Кнопка звонка появится, когда выберешь собеседника.</p></section>
      :!selected?<div className="z-empty"><MessageCircle size={46}/><h1>На одной волне</h1><p>Выбери разговор или найди человека,<br/>с которым хочешь быть на связи.</p></div>:null}
    </div>
    <CallPanel user={user} selected={selected?.participant.id===user.id?null:selected} soundsEnabled={soundsEnabled} conversations={dialogs.data?.pages.flatMap(page=>page.conversations)||[]}/>
  </div>
}

function Messages({ conversation, user }: { conversation: Conversation; user: User }) {
  const cache = useQueryClient()
  const [body, setBody] = useState('')
  const [status, setStatus] = useState('Подключаемся…')
  const [events, setEvents] = useState(0)
  const [live, setLive] = useState<Record<string, Message>>({})
  const [delivery, setDelivery] = useState('')
  const pending = useRef<{ body: string; id: string } | null>(null)
  const bottom = useRef<HTMLDivElement>(null)
  const history = useInfiniteQuery({
    queryKey: ['messages', conversation.id], initialPageParam: '',
    queryFn: async ({ pageParam }) => (await api.get<MessagePage>(`/conversations/${conversation.id}/messages`, { params: { before: pageParam || undefined, limit: 50 } })).data,
    getNextPageParam: (page) => page.next_cursor || undefined,
  })

  useEffect(() => {
    let disposed = false
    const connection: { client?: Pusher } = {}
    void (async () => {
      try {
        const { data: config } = await api.get<{ enabled: boolean; key: string; cluster: string }>('/realtime/config')
        if (disposed) return
        if (!config.enabled) { setStatus('Pusher отключён: заполните .env'); return }
        const pusher = new Pusher(config.key, {
          cluster: config.cluster, forceTLS: true,
          channelAuthorization: {
            customHandler: ({ socketId, channelName }, callback) => {
              void api.post('/pusher/auth', { socket_id: socketId, channel_name: channelName })
                .then(({ data }) => { if (!disposed) callback(null, data) })
                .catch((error: unknown) => { if (!disposed) callback(new Error(errorText(error)), null) })
            },
          },
        })
        connection.client = pusher
        pusher.connection.bind('state_change', ({ current }: { current: string }) => {
          if (!disposed && current !== 'connected') setStatus(`Соединение: ${current}`)
        })
        pusher.connection.bind('error', () => { if (!disposed) setStatus('Ошибка соединения с Pusher') })
        const channel = pusher.subscribe(`private-conversation-${conversation.id}`)
        channel.bind('pusher:subscription_succeeded', () => {
          if (disposed) return
          setStatus('Pusher: подписка активна')
          // Re-fetch all loaded pages after each subscription/reconnection.
          void cache.invalidateQueries({ queryKey: ['messages', conversation.id] })
        })
        channel.bind('pusher:subscription_error', () => { if (!disposed) setStatus('Ошибка авторизации канала Pusher') })
        channel.bind('message.created', ({ message }: { message: Message }) => {
          if (disposed || message.conversation_id !== conversation.id) return
          setLive((old) => ({ ...old, [message.id]: message }))
          setEvents((count) => count + 1)
        })
      } catch (error) { if (!disposed) setStatus(errorText(error)) }
    })()
    return () => { disposed = true; connection.client?.disconnect() }
  }, [cache, conversation.id])

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
  useEffect(() => { bottom.current?.scrollIntoView({ behavior: 'smooth', block: 'nearest' }) }, [latest])
  function submit(event: FormEvent) { event.preventDefault(); if (body.trim() && !send.isPending) send.mutate(body.trim()) }

  return <section className="z-messages">
    <header className="z-chat-header">{conversation.participant.id===user.id?<span className="z-avatar z-saved-avatar"><Bookmark size={22}/></span>:<Avatar user={conversation.participant}/>}<div><h2>{conversation.participant.id===user.id?'Избранное':conversation.participant.name}</h2><p>{conversation.participant.id===user.id?'Личные заметки · Только для тебя':status.includes('подписка активна')?'Соединение установлено':'Личная переписка'}</p></div><button className="z-icon z-history" title="Обновить историю" aria-label="Обновить историю" onClick={()=>void history.refetch()} disabled={history.isFetching}><RefreshCw size={18}/></button></header>
    <div className="z-message-scroll">
      {history.hasNextPage&&<button className="secondary z-older" disabled={history.isFetchingNextPage} onClick={()=>void history.fetchNextPage()}>Более ранние сообщения</button>}
      {history.isPending&&<p className="z-empty-copy" role="status">Загружаем историю…</p>}{history.isError&&<p className="notice">{errorText(history.error)}</p>}
      {!history.isPending&&!history.isError&&messages.length===0&&<div className="z-empty-copy"><p>{conversation.participant.id===user.id?'Всё важное — здесь':'Начни разговор'}</p><small>{conversation.participant.id===user.id?'Сохраняй заметки и ссылки. Их видишь только ты.':'Напиши первое сообщение.'}</small></div>}
      {messages.map(message=><div key={message.id} className={'z-bubble-row'+(message.sender_id===user.id?' is-own':'')}><div className="z-bubble"><p>{message.body}</p><div className="z-message-meta"><time>{new Date(message.created_at).toLocaleTimeString('ru-RU',{hour:'2-digit',minute:'2-digit'})}</time>{message.sender_id===user.id&&<Check size={16} aria-label="Сохранено на сервере"/>}</div></div></div>)}<div ref={bottom}/>
    </div>
    <form className="z-composer-region" onSubmit={submit}>{send.isError&&<p className="notice" role="alert">{errorText(send.error)} Повторите отправку: прежний идентификатор сохранён.</p>}
      <div className="z-composer"><div className="z-composer-field"><textarea aria-label="Сообщение" placeholder="Сообщение…" rows={1} maxLength={1000} value={body} disabled={send.isPending} onChange={e=>setBody(e.target.value)} onKeyDown={e=>{if(e.key==='Enter'&&!e.shiftKey&&!e.nativeEvent.isComposing){e.preventDefault();e.currentTarget.form?.requestSubmit()}}}/></div><button className="z-send" aria-label="Отправить сообщение" disabled={!body.trim()||send.isPending}><Send size={23}/></button></div>
      <details className="z-connection"><summary><Wifi size={11}/>{send.isPending?'Отправляем…':'Состояние соединения'}</summary><p role="status">{status}</p><p>Получено событий: {events}. {delivery||'До 1000 символов. Enter — отправить, Shift+Enter — новая строка.'}</p></details>
    </form>
  </section>
}
