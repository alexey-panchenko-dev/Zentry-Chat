import { AppearanceSettings, defaultAppearance, useIncomingNotices, useMobileViewport } from './Appearance'
import { ChatRow, FavoritesPanel, useAccountSetting } from './ChatWorkspace'
import type { ChatOptions, Favorite } from './ChatWorkspace'
import { GroupCreate, GroupView } from './Groups'
import type { Group } from './Groups'
import { useDeliveryReceipts } from './useDeliveryReceipts'
import { messageStatus } from './message-status'
import { useChatDraft, useComposerHeight, useMessageGesture, useMessageJump } from './chat-input'
import type { RemovalAction } from './conversation-controls'
import { Attachments } from './Attachments'
import { PinnedMessageBar } from './ChatDetails'
import type { DetailsTab } from './ChatDetails'
import { ImageMessage } from '@/ImageMessage'
import { isChatImageURL } from './image-draft'
import { parseBirthDate, birthDatesSupported } from './birthdays'
import { BirthDateWheel } from './BirthDateWheel'
import { useConversationControls } from './useConversationControls'
import type { ConversationControlView } from './useConversationControls'
import { blockNotice, messageVisible } from './conversation-controls'
import { ContactProfile } from '@/ContactProfile'
import type { PeerProfile } from '@/ContactProfile'
import { ContactSettings } from '@/ContactSettings'
import type { ProfileCallRequest } from './contact-profile'
import { MediaAvailability } from '@/MediaAvailability'
import { watchConversations } from '@/conversation-updates'
import { createMediaQueryStore } from './media-query'
import { VoiceComposer } from '@/VoiceComposer'
import { VoiceMessage } from '@/VoiceMessage'
import { ProfilePhoto } from '@/ProfilePhoto'
import { CallPanel } from '@/CallPanel'
import { MessageActions } from './MessageTools'
import { lazy, Suspense, useEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react'
import type { FormEvent } from 'react'
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { subscribeRealtime } from './realtime.ts'
import { useReadReceipt } from './read-receipt.ts'
import { LogOut, MessageCircle, RefreshCw, Search, Send, Settings, Phone, ArrowLeft, Plus, Users, Check, CheckCheck, Heart, ChevronsLeft } from 'lucide-react'
import { Brand, Avatar } from '@/Visual'
import { api, errorText, visibleUsername } from '@/api'
import type { Conversation, Message, MessagePage, ReadState, User } from '@/api'

const Discovery = lazy(() => import('./Discovery').then(module => ({ default: module.Discovery })))
const ProfileEditor = lazy(() => import('./Discovery').then(module => ({ default: module.ProfileEditor })))
const CallHistory = lazy(() => import('./CallHistory').then(module => ({ default: module.CallHistory })))

const rememberedAccountsKey = 'zently-remembered-accounts'
function rememberedAccounts(): string[] {
  try {
    const value: unknown = JSON.parse(localStorage.getItem(rememberedAccountsKey) ?? '[]')
    return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string' && item.includes('@')) : []
  } catch { return [] }
}
function rememberAccount(email: string) {
  const normalized = email.trim().toLocaleLowerCase()
  localStorage.setItem(rememberedAccountsKey, JSON.stringify([normalized, ...rememberedAccounts().filter(item => item !== normalized)].slice(0, 5)))
}

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
      : session.data ? <Chat key={session.data.id} user={session.data} onSessionViewChanged={sessionViewChanged} /> : <Auth />}
  </div>
}

function Auth() {
  const cache = useQueryClient()
  const [mode, setMode] = useState<'login'|'register'|'verify'|'forgot'|'reset'>('login')
  const register = mode === 'register'
  const [name, setName] = useState('')
  const [email, setEmail] = useState(() => {
    const next = sessionStorage.getItem('zently-next-login-email') ?? ''
    sessionStorage.removeItem('zently-next-login-email')
    return next
  })
  const [remember, setRemember] = useState(true)
  const [savedAccounts] = useState(rememberedAccounts)
  const [password, setPassword] = useState('')
  const [code, setCode] = useState('')
  const [authNotice, setAuthNotice] = useState('')
  const [resendSeconds, setResendSeconds] = useState(0)
  const [birthDate,setBirthDate]=useState('')
  const registrationHealth=useQuery({queryKey:['registration-capabilities'],enabled:register,queryFn:async()=>(await api.get<unknown>('/health')).data,retry:false,staleTime:60_000,refetchInterval:query=>register&&!birthDatesSupported(query.state.data)?30_000:false})
  const registrationReady=birthDatesSupported(registrationHealth.data)
  const [validation,setValidation]=useState<{field:string;message:string}|null>(null)
  useEffect(()=>{if(resendSeconds<=0)return;const timer=window.setTimeout(()=>setResendSeconds(seconds=>seconds-1),1000);return()=>window.clearTimeout(timer)},[resendSeconds])
  const finishLogin = (user:User) => { if (remember) rememberAccount(email); cache.setQueryData(['session'], user); window.history.replaceState(window.history.state, '', '#chats') }
  const auth = useMutation({
    mutationFn: async () => (await api.post<{ user: User }>('/auth/login', { email, password })).data.user,
    onSuccess: finishLogin,
    onError: error => {
      if ((error as {response?:{data?:{error?:string}}}).response?.data?.error === 'email_not_verified') {
        setAuthNotice('Почта ещё не подтверждена. Получите код и завершите проверку.'); setResendSeconds(0); setMode('verify')
      }
    },
  })
  const registerMutation = useMutation({
    mutationFn: () => api.post<{delivery_sent:boolean}>('/auth/register', { name, email, password, birth_date:parseBirthDate(birthDate) }),
    onSuccess: response => { setCode(''); setAuthNotice(response.data.delivery_sent ? '' : 'Аккаунт создан, но письмо пока не отправлено. Попробуйте отправить код ещё раз.'); setResendSeconds(60); setMode('verify') },
  })
  const verifyMutation = useMutation({
    mutationFn: async () => { await api.post('/auth/verify-email',{email,code}); return (await api.post<{user:User}>('/auth/login',{email,password})).data.user },
    onSuccess: finishLogin,
  })
  const resendMutation = useMutation({
    mutationFn: () => api.post('/auth/resend-verification',{email}),
    onSuccess: () => { setResendSeconds(60); setAuthNotice('Новый код отправлен.') },
  })
  const resetRequestMutation = useMutation({
    mutationFn: () => api.post('/auth/password-reset/request',{email}),
    onSuccess: () => { setCode(''); setAuthNotice('Если аккаунт с такой почтой существует, код отправлен.'); setResendSeconds(60); setMode('reset') },
  })
  const resetMutation = useMutation({
    mutationFn: () => api.post('/auth/password-reset/confirm',{email,code,password}),
    onSuccess: () => { setPassword(''); setCode(''); setAuthNotice('Пароль изменён. Теперь войдите с новым паролем.'); setMode('login') },
  })
  const activeMutation = mode==='register'?registerMutation:mode==='verify'?verifyMutation:mode==='forgot'?resetRequestMutation:mode==='reset'?resetMutation:auth
  function submitAuth(event:FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if(activeMutation.isPending||(register&&!registrationReady))return
    const form=event.currentTarget
    const fields=Array.from(form.querySelectorAll<HTMLInputElement>('input:not([type="hidden"])'))
    for(const field of fields){
      let message=''
      if(field.name==='name'&&!/^@?[a-z][a-z0-9_]{2,39}$/i.test(field.value.trim()))message='Придумайте @юзер: 3–40 символов, латинские буквы, цифры и _.'
      else if(field.name==='email'&&!field.checkValidity())message='Введите корректный адрес почты, например you@example.com.'
      else if(field.name==='password'&&(!field.checkValidity()||field.value.length<12||new TextEncoder().encode(field.value).length>72))message='Введите пароль от 12 символов, не более 72 байт.'
      else if(field.name==='code'&&!/^\d{6}$/.test(field.value))message='Введите шестизначный код из письма.'
      if(message){setValidation({field:field.name,message});field.focus();return}
    }
    if(register){try{parseBirthDate(birthDate)}catch(e){setValidation({field:'birth_date',message:errorText(e)});form.querySelector<HTMLElement>('[role="spinbutton"]')?.focus();return}}
    setValidation(null);setAuthNotice('')
    if(mode==='register')registerMutation.mutate()
    else if(mode==='verify')verifyMutation.mutate()
    else if(mode==='forgot')resetRequestMutation.mutate()
    else if(mode==='reset')resetMutation.mutate()
    else auth.mutate()
  }
  const modeTitle = mode==='register'?'Создай профиль и будь на связи.':mode==='verify'?'Проверь почту и введи код.':mode==='forgot'?'Восстановим доступ к аккаунту.':mode==='reset'?'Задай новый пароль.':'Войди, чтобы продолжить разговор.'
  const requestError = activeMutation.isError ? errorText(activeMutation.error)
    : mode==='verify'&&resendMutation.isError ? errorText(resendMutation.error)
    : mode==='reset'&&resetRequestMutation.isError ? errorText(resetRequestMutation.error) : ''
  return <section className="z-auth-card">
    <Brand/><p className="z-eyebrow">БЛИЖЕ ДРУГ К ДРУГУ</p><h1>{mode==='register' ? <>Твой круг.<br/>Твои разговоры.</> : mode==='verify'?<>Подтверди<br/>свою почту.</>:mode==='forgot'||mode==='reset'?<>Вернёмся<br/>в Zently.</>:<>С возвращением<br/>в Zently.</>}</h1>
    <p className="z-muted">{modeTitle}</p>
    <form noValidate className="z-auth-form" onSubmit={submitAuth}>
      {mode==='register' && <label>Ник / @юзер<input name="name" aria-invalid={validation?.field==='name'} aria-describedby={validation?.field==='name'?'auth-validation':undefined} required maxLength={41} value={name} onChange={(e) => setName(e.target.value)} autoComplete="username" placeholder="Например, byliver" /></label>}
      {mode==='register'&&<div className="z-auth-birth-date"><label>Дата рождения</label><BirthDateWheel value={birthDate} onChange={setBirthDate} invalid={validation?.field==='birth_date'} describedBy={validation?.field==='birth_date'?'auth-validation':undefined}/></div>}
      {mode!=='verify'&&<label>Почта<input name="email" aria-invalid={validation?.field==='email'} aria-describedby={validation?.field==='email'?'auth-validation':undefined} type="email" required value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="email" placeholder="you@example.com" readOnly={mode==='reset'||mode==='forgot'} /></label>}
      {mode==='verify'&&<p className="z-field-help">Код подтверждения для <strong>{email}</strong> действует 10 минут.</p>}
      {mode==='login' && savedAccounts.length > 0 && <div className="z-saved-accounts" aria-label="Запомненные аккаунты">{savedAccounts.map(account => <button type="button" className="secondary" key={account} onClick={() => setEmail(account)}>{account}</button>)}</div>}
      {(mode==='login'||mode==='register'||mode==='reset')&&<label>{mode==='reset'?'Новый пароль':'Пароль'}<input name="password" aria-invalid={validation?.field==='password'} aria-describedby={validation?.field==='password'?'auth-validation':undefined} type="password" required minLength={12} value={password} onChange={(e) => setPassword(e.target.value)} autoComplete={register||mode==='reset' ? 'new-password' : 'current-password'} placeholder="Не менее 12 символов" /></label>}
      {(mode==='verify'||mode==='reset')&&<label>Код из письма<input name="code" aria-invalid={validation?.field==='code'} aria-describedby={validation?.field==='code'?'auth-validation':undefined} inputMode="numeric" autoComplete="one-time-code" maxLength={6} pattern="[0-9]{6}" required value={code} onChange={e=>setCode(e.target.value.replace(/\D/g,'').slice(0,6))} placeholder="000000" /></label>}
      {mode==='login'&&<label className="z-remember-account"><input type="checkbox" checked={remember} onChange={event => setRemember(event.target.checked)}/> Запомнить почту на этом устройстве</label>}
      {(mode==='login'||mode==='register'||mode==='reset')&&<p className="z-field-help">Пароль — от 12 до 72 байт.</p>}
      {validation&&<p id="auth-validation" className="notice" role="alert">{validation.message}</p>}
      {requestError && <p className="notice" role="alert">{requestError}</p>}
      {authNotice && <p className="notice" role="status">{authNotice}</p>}
      {register&&!registrationReady&&<p className="notice" role="status">{registrationHealth.isPending?'Проверяем доступность регистрации…':'Регистрация с датой рождения временно недоступна. Попробуйте позже.'}</p>}
      <button className="primary" disabled={activeMutation.isPending||(register&&!registrationReady)}>{activeMutation.isPending ? 'Подождите…' : mode==='register'?'Продолжить':mode==='verify'?'Подтвердить почту':mode==='forgot'?'Отправить код':mode==='reset'?'Сохранить пароль':'Войти'}<ArrowLeft className="z-arrow-forward" size={18}/></button>
    </form>
    {mode==='verify'&&<button className="z-auth-switch" disabled={resendSeconds>0||resendMutation.isPending} onClick={()=>resendMutation.mutate()}>{resendSeconds>0?`Отправить код ещё раз через ${resendSeconds} сек.`:resendMutation.isPending?'Отправляем…':'Отправить код ещё раз'}</button>}
    {mode==='reset'&&<button className="z-auth-switch" disabled={resendSeconds>0||resetRequestMutation.isPending} onClick={()=>resetRequestMutation.mutate()}>{resendSeconds>0?`Повторить запрос через ${resendSeconds} сек.`:'Отправить новый код'}</button>}
    {mode==='login'&&<button className="z-auth-switch" onClick={()=>{setMode('forgot');setValidation(null);setAuthNotice('')}}>Забыли пароль?</button>}
    {mode==='verify'&&<button className="z-auth-switch" onClick={()=>{setMode('register');setValidation(null);setAuthNotice('')}}>Изменить данные регистрации</button>}
    {mode==='reset'&&<button className="z-auth-switch" onClick={()=>{setMode('login');setValidation(null);setAuthNotice('')}}>Вернуться ко входу</button>}
    {(mode==='login'||mode==='register')&&<button className="z-auth-switch" onClick={() => { setMode(register?'login':'register'); setValidation(null); setAuthNotice(''); auth.reset();registerMutation.reset() }}>{register ? 'Уже есть аккаунт? Войти' : 'Нет аккаунта? Создать профиль'}</button>}
  </section>
}

const defaultChatOptions: Record<string, ChatOptions> = {}

function Chat({ user, onSessionViewChanged }: { user: User; onSessionViewChanged:()=>void }) {
  const cache = useQueryClient()
  useEffect(()=>{const touch=()=>{if(document.visibilityState==='visible')void api.post('/presence',{}).catch(()=>{})};touch();const timer=window.setInterval(touch,25_000);document.addEventListener('visibilitychange',touch);return()=>{window.clearInterval(timer);document.removeEventListener('visibilitychange',touch)}},[])
  useDeliveryReceipts(user.id)
  const [selected, setSelectedState] = useState<Conversation | null>(null)
  const [selectedGroup,setSelectedGroup]=useState<Group|null>(null)
  const [groupCreateOpen,setGroupCreateOpen]=useState(false)
  const groupCapability=useQuery({queryKey:['media-capabilities'],queryFn:async()=> (await api.get<{media?:{groups?:boolean}}>('/realtime/config')).data,retry:false,staleTime:5*60_000})
  const groupsSupported=groupCapability.data?.media?.groups===true
  const groups=useQuery({enabled:groupsSupported,queryKey:['groups'],queryFn:async()=> (await api.get<{groups:Group[]}>('/groups')).data.groups,retry:false,refetchInterval:15000})
  const selectedRef=useRef<Conversation|null>(null)
  const [removalNotice,setRemovalNotice]=useState('')
  function setSelected(value:Conversation|null){selectedRef.current=value;setSelectedState(value)}
  function conversationRemoved(id:string,action:RemovalAction){if(selectedRef.current?.id!==id)return;setSelected(null);setMobileChat(false);setRemovalNotice(action==='delete_chat'?'Чат удалён только у вас.':'Контакт удалён из вашего списка. История сохранена.')}
  const [search, setSearch] = useState('')
  const [query, setQuery] = useState('')
  const [searchError,setSearchError]=useState('')
  const [uiSection, setUiSection] = useState<'chats'|'calls'|'settings'|'discovery'|'profile'|'favorites'>('chats')
  const [chatOptions, setChatOptions] = useAccountSetting(user.id, 'chat-options', defaultChatOptions)
  const [appearance,setAppearance]=useAccountSetting(user.id,'appearance',defaultAppearance)
  const [shortcutHelp,setShortcutHelp]=useState(false)
  const searchInput=useRef<HTMLInputElement>(null)
  const [filter, setFilter] = useState<'all'|'personal'|'group'|'unread'>('all')
  const [jumpRequest, setJumpRequest] = useState<{ chatID: string; messageID: string } | null>(null)
  function changeChatOption(id: string, option: keyof ChatOptions) {
    setChatOptions(previous => ({ ...previous, [id]: { ...previous[id], [option]: !previous[id]?.[option] } }))
  }
  function openChat(id: string, dialog: Conversation | null, group: Group | null, messageID?: string) {
    setSelected(dialog)
    setSelectedGroup(group)
    setJumpRequest(messageID ? { chatID: id, messageID } : null)
    setUiSection('chats')
    setMobileChat(true)
    if (chatOptions[id]?.unread) setChatOptions(previous => ({ ...previous, [id]: { ...previous[id], unread: false } }))
  }
  const [callActive,setCallActive]=useState(false)
  const [requestCall,setRequestCall]=useState<ProfileCallRequest|null>(null)
  const [compact, setCompact] = useState(false)
  const [searchOpen, setSearchOpen] = useState(false)
  const [mobileChat, setMobileChat] = useState(false)
  const [mobileLayout] = useState(()=>createMediaQueryStore(window.matchMedia('(max-width: 720px)')))
  const isMobile = useSyncExternalStore(mobileLayout.subscribe,mobileLayout.getSnapshot)
  const shell=useMobileViewport(isMobile&&mobileChat)
  const [soundsEnabled,setSoundsEnabled] = useState(()=>localStorage.getItem('zently-call-sounds')!=='0')
  const [realtimeReady, setRealtimeReady] = useState(false)
  const controls=useConversationControls(selected?.id,realtimeReady)
  useEffect(() => watchConversations(user.id, event => {
    if(event.reason==='group'||event.reason==='connected'){void cache.invalidateQueries({queryKey:['groups']});if(event.reason==='group'){void cache.invalidateQueries({queryKey:['group-messages']});void cache.invalidateQueries({queryKey:['group-message-reactions']});void cache.invalidateQueries({queryKey:['group-pins']});return}}
    if(event.reason==='discovery') {
      void cache.invalidateQueries({queryKey:['discovery']})
      return
    }
    void cache.invalidateQueries({ queryKey: ['conversations'] })
    void cache.invalidateQueries({ queryKey: ['unread'] })
    if(event.reason==='connected') {
      void cache.invalidateQueries({queryKey:['chat-controls']})
      void cache.invalidateQueries({queryKey:['discovery']})
      void cache.invalidateQueries({queryKey:['messages']})
      void cache.invalidateQueries({queryKey:['read-state']})
    }
    if(event.conversation_id && event.reason==='conversation') void cache.invalidateQueries({queryKey:['chat-controls',event.conversation_id]})
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
  const allDialogs = useMemo(()=>dialogs.data?.pages.flatMap(page => page.conversations) ?? [],[dialogs.data])
  const {notice,dismiss}=useIncomingNotices(allDialogs,chatOptions,uiSection==='chats'&&(!isMobile||mobileChat)?selected?.id:undefined,appearance.sound,dialogs.isSuccess)
  const ordered = <T extends {id:string}>(items:T[]) => items.slice().sort((a,b)=>Number(!!chatOptions[b.id]?.pinned)-Number(!!chatOptions[a.id]?.pinned))
  async function openFavorite(item: Favorite) {
    if (item.scope === 'group') {
      const group = groups.data?.find(group => group.id === item.chat_id)
      if (!group) throw new Error('group unavailable')
      openChat(group.id, null, group, item.id)
      return
    }
    let dialog = allDialogs.find(chat => chat.id === item.chat_id)
    let more = dialogs.hasNextPage
    while (!dialog && more) {
      const page = await dialogs.fetchNextPage()
      if (page.isError) break
      dialog = page.data?.pages.at(-1)?.conversations.find(chat => chat.id === item.chat_id)
      more = page.hasNextPage
    }
    if (!dialog) throw new Error('chat unavailable')
    openChat(dialog.id, dialog, null, item.id)
  }
  const users = useQuery({
    queryKey: ['users', query], enabled: query.length >= 2,
    queryFn: async () => (await api.get<{ users: User[] }>('/users', { params: { query } })).data.users,
  })
  const create = useMutation({
    mutationFn: async (id: string) => (await api.post<{ conversation: Conversation }>('/conversations', { user_id: id })).data.conversation,
    onSuccess: (dialog) => { setSelectedGroup(null); setSelected(dialog); setUiSection('chats'); setMobileChat(true); setSearchOpen(false); setQuery(''); setSearch(''); void cache.invalidateQueries({ queryKey: ['conversations'] }) },
  })
  const logout = useMutation({
    mutationFn: () => api.post('/auth/logout', {}),
    onSuccess: () => { cache.clear(); cache.setQueryData(['session'], null); onSessionViewChanged() },
  })
  const [switchEmail, setSwitchEmail] = useState('')
  function switchAccount(email: string) {
    sessionStorage.setItem('zently-next-login-email', email)
    setSwitchEmail(email)
    logout.mutate()
  }

  useEffect(()=>{
    function shortcut(event:KeyboardEvent){
      if(event.key==='Escape'){
        document.querySelector<HTMLDetailsElement>('.z-message-menu[open]')?.removeAttribute('open')
        setShortcutHelp(false)
        return
      }
      if(!event.altKey||!event.shiftKey||event.metaKey||event.ctrlKey||event.repeat||event.isComposing||document.querySelector('dialog[open]'))return
      if((event.target as HTMLElement|null)?.matches('input,textarea,select,[contenteditable]'))return
      switch(event.code){
        case 'KeyF': setUiSection('chats');setMobileChat(false);setSearchOpen(true);requestAnimationFrame(()=>searchInput.current?.focus());break
        case 'KeyC': setUiSection('chats');setMobileChat(false);break
        case 'KeyI': document.querySelector<HTMLTextAreaElement>('.z-message-view:not([hidden]) .z-composer-field textarea')?.focus();break
        case 'KeyS': setUiSection('settings');setMobileChat(true);break
        case 'ArrowUp':
        case 'ArrowDown': {
          const entries=ordered(allDialogs)
          const index=entries.findIndex(item=>item.id===selected?.id)
          const next=entries[index<0?0:index+(event.code==='ArrowUp'?-1:1)]
          if(next)openChat(next.id,next,null)
          break
        }
        default:return
      }
      event.preventDefault()
    }
    window.addEventListener('keydown',shortcut)
    return()=>window.removeEventListener('keydown',shortcut)
  })

  return <div ref={shell} data-ui-section={uiSection} data-nav-side={appearance.navSide==='left'?'left':'right'} data-text={appearance.text} data-density={appearance.density} data-motion={appearance.motion} data-backdrop={appearance.backdrop} className={'z-shell'+(compact?' is-compact':'')+(mobileChat?' has-mobile-chat':'')}>
    <aside className="z-sidebar"><div className="z-sidebar-head"><Brand/></div>
      <nav aria-label="Основная навигация">
        <button className={uiSection==='chats'?'is-active':''} title="Чаты" aria-label="Чаты" onClick={()=>{setUiSection('chats');setMobileChat(false)}}><MessageCircle/><span>Чаты</span>{!!unread.data?.unread_chats&&<b className="z-unread" aria-label={`Непрочитанных чатов: ${unread.data.unread_chats}`}>{unread.data.unread_chats}</b>}</button>
        <button className={uiSection==='discovery'?'is-active':''} title="Знакомства" aria-label="Знакомства" onClick={()=>{setUiSection('discovery');setMobileChat(true)}}><Heart/><span>Знакомства</span></button>
        <button className={uiSection==='calls'?'is-active':''} title="Звонки" aria-label="Звонки" onClick={()=>{setUiSection('calls');setMobileChat(true)}}><Phone/><span>Звонки</span></button>
        <button className={uiSection==='favorites'?'is-active':''} title="Избранное" aria-label="Избранное" onClick={()=>{setUiSection('favorites');setMobileChat(true)}}><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M6 4h12v16l-6-4-6 4z"/></svg><span>Избранное</span></button>
      </nav>
      <div className="z-sidebar-bottom"><button type="button" className="z-icon z-collapse" aria-label={compact?'Развернуть меню':'Свернуть меню'} aria-expanded={!compact} onClick={()=>setCompact(!compact)}><ChevronsLeft size={18}/></button>
      <button className={'z-settings-link'+(uiSection==='settings'?' is-active':'')} title="Настройки" aria-label="Настройки" onClick={()=>{setUiSection('settings');setMobileChat(true)}}><Settings/><span>Настройки</span></button>
      <button className="z-account" aria-label="Мой профиль" onClick={()=>{setUiSection('profile');setMobileChat(true)}}><Avatar user={user}/><span><strong>{user.name}</strong><small>Мой профиль</small></span></button></div>
    </aside>
    <aside className="z-dialogs"><div className="z-list-top"><span>Чаты</span><div><button className="z-icon" aria-label="Обновить переписки" title="Обновить переписки" onClick={()=>void dialogs.refetch()}><RefreshCw size={16}/></button>{groupsSupported&&<button className="z-icon" aria-label="Создать группу" title="Создать группу" onClick={()=>setGroupCreateOpen(true)}><Users size={20}/></button>}<button className="z-icon" aria-label="Найти человека" title="Найти человека" onClick={()=>setSearchOpen(!searchOpen)}><Plus/></button></div></div>
      <form noValidate className="z-search" onSubmit={(e)=>{e.preventDefault();const input=e.currentTarget.querySelector('input')!;const text=search.trim();if(!input.checkValidity()||text.length<2||text.length>100){setSearchError('Введите @юзер, имя или почту от 2 до 100 символов.');input.focus();return}setSearchError('');setQuery(text);setSearchOpen(true)}}><Search size={18}/><input aria-invalid={!!searchError} aria-describedby={searchError?'search-validation':undefined} ref={searchInput} aria-label="Поиск пользователя" placeholder="@юзер, имя или почта" value={search} onChange={e=>{setSearch(e.target.value);setSearchError('')}} minLength={2} maxLength={100} required/>{search && <button type="button" className="z-clear" aria-label="Очистить поиск" onClick={()=>{setSearch('');setQuery('');setSearchError('')}}>×</button>}<button className="z-icon" aria-label="Найти"><ArrowLeft size={16} className="z-arrow-forward"/></button></form>
      {searchError&&<p id="search-validation" className="notice" role="alert">{searchError}</p>}
      {searchOpen && <div className="z-search-results">{!query&&<p className="z-field-help">Введи @юзер, имя профиля или точную почту и нажми стрелку.</p>}{users.isFetching&&<p role="status">Поиск…</p>}{users.isError&&<p className="notice">{errorText(users.error)}</p>}{query&&users.data?.length===0&&<p>Никого не найдено.</p>}{query&&users.data?.map(peer=><button className="z-contact" key={peer.id} disabled={create.isPending} onClick={()=>create.mutate(peer.id)}><Avatar user={peer}/><span>{peer.name}<small>{visibleUsername(peer)}</small></span><MessageCircle size={17}/></button>)}</div>}
      {removalNotice&&<p className="z-field-help" role="status">{removalNotice}</p>}
      {create.isError&&<p className="notice">{errorText(create.error)}</p>}
      <div className="z-mobile-hub"><span>Быстрый доступ</span><button type="button" onClick={()=>{setUiSection('calls');setMobileChat(true)}}>Звонки</button><button type="button" onClick={()=>{setUiSection('discovery');setMobileChat(true)}}>Знакомства</button></div>
      <div className="z-dialog-filters" aria-label="Фильтр чатов">
        {([['all','Все'],['personal','Личные'],['group','Группы'],['unread','Непрочитанные']] as const).map(([value,label])=><button key={value} type="button" aria-pressed={filter===value} onClick={()=>setFilter(value)}>{label}</button>)}
      </div>
      <div className="z-conversation-list">
        {filter!=='personal'&&ordered(groups.data??[]).filter(group=>filter!=='unread'||chatOptions[group.id]?.unread).map(group=><ChatRow key={group.id} label={group.name} selected={selectedGroup?.id===group.id} options={chatOptions[group.id]} onChange={option=>changeChatOption(group.id,option)} onOpen={()=>openChat(group.id,null,group)}><span className="z-group-avatar" aria-hidden="true">{group.avatar_url?<img src={group.avatar_url} alt=""/>:<Users size={20}/>}</span><span><strong>{group.name}</strong><small>{group.member_count} участника</small></span>{chatOptions[group.id]?.pinned&&<small title="Закреплён">◆</small>}{chatOptions[group.id]?.unread&&<b className="z-unread" aria-label="Помечен непрочитанным">•</b>}</ChatRow>)}
        {groups.isError&&<p className="notice">{errorText(groups.error)}</p>}{dialogs.isError&&<p className="notice">{errorText(dialogs.error)}</p>}{dialogs.isPending&&<p className="z-empty-copy" role="status">Загрузка…</p>}
        {filter!=='group'&&ordered(allDialogs).filter(dialog=>filter!=='unread'||!!dialog.unread_count||chatOptions[dialog.id]?.unread).map(dialog=><ChatRow key={dialog.id} label={dialog.display_name||dialog.participant.name} selected={selected?.id===dialog.id} options={chatOptions[dialog.id]} onChange={option=>changeChatOption(dialog.id,option)} onFirstUnread={dialog.first_unread_id?()=>openChat(dialog.id,dialog,null,dialog.first_unread_id):undefined} onOpen={()=>openChat(dialog.id,dialog,null)}><Avatar user={dialog.participant}/><span><strong>{dialog.display_name||dialog.participant.name}</strong><small>{dialog.last_message||'Пока нет сообщений'}</small></span>{chatOptions[dialog.id]?.pinned&&<small title="Закреплён">◆</small>}{(!!dialog.unread_count||chatOptions[dialog.id]?.unread)&&<b className="z-unread" aria-label="Непрочитанные сообщения">{dialog.unread_count||'•'}</b>}</ChatRow>)}
        {dialogs.data?.pages[0].conversations.length===0&&<div className="z-empty-copy"><MessageCircle/><p>Твой круг начинается здесь.</p><small>Найди человека по имени или почте.</small></div>}
        {dialogs.hasNextPage&&<button className="secondary" disabled={dialogs.isFetchingNextPage} onClick={()=>void dialogs.fetchNextPage()}>Ещё переписки</button>}
      </div>
    </aside>
    <div className="z-workspace">
      <button className="z-mobile-back z-icon" aria-label="Назад к чатам" onClick={()=>{setMobileChat(false);setUiSection('chats')}}><ArrowLeft size={20}/></button>
      {selectedGroup&&<div className="z-message-view" hidden={uiSection!=='chats'}><GroupView enterSends={appearance.enter} targetMessage={jumpRequest?.chatID===selectedGroup.id?jumpRequest.messageID:undefined} key={selectedGroup.id} group={selectedGroup} user={user} visible={uiSection==='chats' && (mobileChat || !isMobile)} onBack={()=>setMobileChat(false)} onGroupChanged={updated=>{setSelectedGroup(updated);cache.setQueryData<Group[]>(['groups'],old=>old?.map(item=>item.id===updated.id?updated:item))}}/></div>}
      {selected&&<div className="z-message-view" hidden={uiSection!=='chats'}><Messages enterSends={appearance.enter} targetMessage={jumpRequest?.chatID===selected.id?jumpRequest.messageID:undefined} key={selected.id} conversation={selected} user={user} visible={uiSection==='chats' && (mobileChat || !isMobile)} callActive={callActive} realtimeReady={realtimeReady} controls={controls} onRemoved={conversationRemoved} onProfileCall={()=>setRequestCall({id:crypto.randomUUID(),conversationID:selected.id})}/></div>}
      {uiSection==='profile'?<section className="z-settings z-own-profile"><h1>Мой профиль</h1><div className="z-profile"><ProfilePhoto user={user}/><h2>{user.name}</h2><p className="z-muted">Личный профиль</p></div><Suspense fallback={<p role="status">Загружаем профиль…</p>}><ProfileEditor user={user} onSaved={()=>setUiSection('discovery')}/></Suspense><section className="z-profile-accounts"><h2>Аккаунты</h2><p className="z-muted">Сохраняются только адреса почты. Пароль вводится при каждом переключении.</p>{rememberedAccounts().map(account=><button className="secondary z-account-switch" key={account} disabled={logout.isPending} onClick={()=>switchAccount(account)}>{account}{switchEmail===account&&logout.isPending?' · Выходим…':' · Переключить'}</button>)}<button className="secondary" disabled={logout.isPending} onClick={()=>switchAccount('')}>＋ Добавить аккаунт</button>{logout.isError&&<p className="notice">{errorText(logout.error)}</p>}<button className="secondary z-danger" disabled={logout.isPending} onClick={()=>switchAccount('')}><LogOut size={17}/>Выйти из аккаунта</button></section></section>:uiSection==='settings'?<section className="z-settings"><h1>Настройки</h1><AppearanceSettings value={appearance} update={setAppearance}/><button className="secondary" type="button" aria-expanded={shortcutHelp} onClick={()=>setShortcutHelp(!shortcutHelp)}>Горячие клавиши</button>{shortcutHelp&&<ul className="z-shortcuts"><li>Alt + Shift + F — поиск людей</li><li>Alt + Shift + C — список чатов</li><li>Alt + Shift + ↑ / ↓ — предыдущий или следующий чат</li><li>Alt + Shift + I — поле сообщения</li><li>Alt + Shift + S — настройки</li><li>Esc — закрыть меню сообщения</li></ul>}<MediaAvailability feature="contact_profiles"><ContactSettings/></MediaAvailability><label className="z-setting-row">Звуки звонков<input type="checkbox" checked={soundsEnabled} onChange={e=>{setSoundsEnabled(e.target.checked);localStorage.setItem("zently-call-sounds",e.target.checked?"1":"0")}}/></label></section>
      :uiSection==='discovery'?<Suspense fallback={<p role="status">Загружаем знакомства…</p>}><Discovery fallbackInterval={realtimeReady?false:60_000} onSettings={()=>setUiSection('profile')}/></Suspense>
      :uiSection==='calls'?<Suspense fallback={<p role="status">Загружаем звонки…</p>}><CallHistory/></Suspense>
      :uiSection==='favorites'?<FavoritesPanel onOpen={openFavorite}/>
      :!selected&&!selectedGroup?<div className="z-empty"><MessageCircle size={46}/><h1>На одной волне</h1><p>Выбери разговор или найди человека,<br/>с которым хочешь быть на связи.</p></div>:null}
      <CallPanel showLaunch={uiSection==='chats'&&!selectedGroup} contactBlocked={controls.blocked||controls.unavailable} requestCall={requestCall} onActiveChange={setCallActive} user={user} selected={selected} soundsEnabled={soundsEnabled} conversations={dialogs.data?.pages.flatMap(page=>page.conversations)||[]}/>
    </div>
    {notice&&!chatOptions[notice.chat.id]?.muted&&<aside className="z-message-notice" role="status"><button type="button" onClick={()=>{openChat(notice.chat.id,notice.chat,null);dismiss()}}><strong>{notice.chat.display_name||notice.chat.participant.name}</strong><span>{appearance.preview&&notice.chat.last_message?notice.chat.last_message:`Новых сообщений: ${notice.count}`}</span></button><button type="button" className="z-icon" aria-label="Закрыть уведомление" onClick={dismiss}>×</button></aside>}
    {groupCreateOpen&&<GroupCreate onClose={()=>setGroupCreateOpen(false)} onCreated={group=>{setGroupCreateOpen(false);setSelectedGroup(group);setSelected(null);setUiSection('chats');setMobileChat(true);void cache.invalidateQueries({queryKey:['groups']})}}/>}
  </div>
}

function Messages({ enterSends,targetMessage, conversation, user, visible, callActive, realtimeReady, onProfileCall, controls, onRemoved }: { enterSends:boolean;targetMessage?:string;conversation: Conversation; user: User; visible:boolean;callActive:boolean;realtimeReady:boolean;onProfileCall:()=>void;controls:ConversationControlView;onRemoved:(id:string,action:RemovalAction)=>void }) {
  const cache = useQueryClient()
  const { body, setBody, reply: replyTo, setReply: setReplyTo, clearSent } = useChatDraft(user.id, conversation.id)
  const messageInput = useComposerHeight(body)
  const messageGesture = useMessageGesture<Message>(setReplyTo)
  const [imageActive,setImageActive]=useState(false)
  const [voiceActive,setVoiceActive]=useState(false)
  const [imageViewing,setImageViewing]=useState(false)
  const [profileOpen,setProfileOpen]=useState(false)
  const [profileTab,setProfileTab]=useState<DetailsTab>('info')
  const [live, setLive] = useState<Record<string, Message>>({})
  const pending = useRef<{ body: string; id: string; replyID?: string } | null>(null)
  const bottom = useRef<HTMLDivElement>(null)
  const contactDisabled=controls.blocked||controls.unavailable
  const history = useInfiniteQuery({
    enabled: visible,
    queryKey: ['messages', conversation.id], initialPageParam: '',
    queryFn: async ({ pageParam }) => (await api.get<MessagePage>(`/conversations/${conversation.id}/messages`, { params: { before: pageParam || undefined, limit: 50 } })).data,
    getNextPageParam: (page) => page.next_cursor || undefined,
    refetchInterval: visible && !realtimeReady ? 60_000 : false,
  })

  const readState = useQuery({enabled:visible, queryKey:['read-state',conversation.id], queryFn:async()=>(await api.get<ReadState>(`/conversations/${conversation.id}/read`)).data, refetchInterval:visible?5000:false})
   const peerProfile=useQuery({enabled:visible,queryKey:['contact-profile',conversation.id],queryFn:async({signal})=>(await api.get<{profile:PeerProfile}>(`/conversations/${conversation.id}/profile`,{signal})).data.profile,retry:false,refetchInterval:visible?20_000:false})
  useEffect(() => subscribeRealtime(`private-conversation-${conversation.id}`, {
    'message.created': ({message}: {message: Message}) => {
      if(message.conversation_id!==conversation.id) return
      setLive(old=>({...old,[message.id]:message}))
      void cache.invalidateQueries({queryKey:['contact-profile',conversation.id]})
    },
    'message.feature.updated': () => {
      void cache.invalidateQueries({queryKey:['messages',conversation.id]})
      void cache.invalidateQueries({queryKey:['message-reactions']})
      void cache.invalidateQueries({queryKey:['pinned-messages',conversation.id]})
    },
   }, ready => {
    if(ready) {
      void cache.invalidateQueries({queryKey:['messages',conversation.id]})
      void cache.invalidateQueries({queryKey:['read-state',conversation.id]})
    }
  }), [cache,conversation.id])

  const send = useMutation({
    mutationFn: async (input: {text:string;replyTo?:{id:string}|null}) => {
      if (pending.current?.body !== input.text || pending.current.replyID !== input.replyTo?.id) pending.current = { body: input.text, id: crypto.randomUUID(), replyID: input.replyTo?.id }
      return (await api.post<{ message: Message; realtime: string }>(`/conversations/${conversation.id}/messages`, {
        body: input.text, client_message_id: pending.current.id,reply_to_id:input.replyTo?.id,
      })).data
    },
    onSuccess: (result, input) => {
      setLive((old) => ({ ...old, [result.message.id]: result.message }))
      clearSent(input.text, input.replyTo?.id); pending.current = null
       void cache.invalidateQueries({queryKey:['contact-profile',conversation.id]})
    },
  })
  useEffect(() => {
    if(!visible||contactDisabled||send.isPending||callActive||profileOpen||imageViewing||imageActive||voiceActive)return
    const frame=window.requestAnimationFrame(()=>messageInput.current?.focus({preventScroll:true}))
    return()=>window.cancelAnimationFrame(frame)
  },[conversation.id,visible,contactDisabled,send.isPending,callActive,profileOpen,imageViewing,imageActive,voiceActive])
  const merged = new Map<string, Message>()
  history.data?.pages.forEach((page) => page.messages.forEach((message) => merged.set(message.id, message)))
  Object.values(live).forEach((message) => { if (!merged.has(message.id)) merged.set(message.id, message) })
  const messages = [...merged.values()].filter(message=>messageVisible(message.id,controls.state?.cleared_through)).sort((a, b) => BigInt(a.id) < BigInt(b.id) ? -1 : BigInt(a.id) > BigInt(b.id) ? 1 : 0)
  const photoMessages=messages.filter(message=>message.kind==='image'&&isChatImageURL(message.image_url))
  const photoGallery=photoMessages.map(message=>message.image_url).filter((url):url is string=>isChatImageURL(url))
  const latest = messages.at(-1)?.id
  useReadReceipt(conversation.id, latest, visible && !callActive && !profileOpen && !imageViewing, bottom)
  useEffect(() => { if(!targetMessage)bottom.current?.scrollIntoView({ block: 'nearest' }) }, [latest,targetMessage])
  function submit(event: FormEvent) { event.preventDefault(); if (body.trim() && !send.isPending && !contactDisabled) send.mutate({text:body.trim(),replyTo}) }
  const historyRoot=useRef<HTMLDivElement>(null)
  const {jump:jumpToMessage,status:jumpStatus}=useMessageJump(historyRoot,history.fetchNextPage,history.hasNextPage)
  const lastJump=useRef('')
  useEffect(()=>{
    if(visible&&targetMessage&&history.isSuccess&&lastJump.current!==targetMessage){
      lastJump.current=targetMessage
      void jumpToMessage(targetMessage)
    }
    if(!targetMessage)lastJump.current=''
  },[visible,targetMessage,history.isSuccess,jumpToMessage])
  const [unreadJumpError,setUnreadJumpError]=useState('')
  async function jumpToUnread(){
    setUnreadJumpError('')
    try{
      const result=await api.get<{message_id:string}>('/conversations/'+conversation.id+'/first-unread')
      if(result.data.message_id)await jumpToMessage(result.data.message_id)
      else setUnreadJumpError('Непрочитанных сообщений больше нет.')
    }catch(e){setUnreadJumpError(errorText(e))}
  }

  return <section className="z-messages">
    {profileOpen&&visible&&<ContactProfile initialTab={profileTab} onJump={jumpToMessage} conversation={conversation} onClose={()=>setProfileOpen(false)} onCall={onProfileCall} callBusy={callActive} controls={controls} onRemoved={onRemoved}/>}
    <header className="z-chat-header"><button type="button" className="z-header-profile" aria-label={'Открыть профиль: '+conversation.participant.name} onClick={()=>{setProfileTab('info');setProfileOpen(true)}}><Avatar user={conversation.participant}/><span><strong>{peerProfile.data?.display_name||conversation.display_name||conversation.participant.name}</strong><small>{peerProfile.isPending?'Проверяем статус…':peerProfile.isError||typeof peerProfile.data?.online!=='boolean'?'Статус недоступен':peerProfile.data.online?'В сети':'Не в сети'}</small></span></button><button type="button" className="z-icon" title="Поиск в переписке" aria-label="Поиск в переписке" onClick={()=>{setProfileTab('search');setProfileOpen(true)}}><Search size={18}/></button><button className="z-icon z-history" title="Обновить историю" aria-label="Обновить историю" onClick={()=>{void history.refetch();void peerProfile.refetch()}} disabled={history.isFetching}><RefreshCw size={18}/></button></header>
    <PinnedMessageBar scope="personal" chatID={conversation.id} onJump={jumpToMessage} onOpen={()=>{setProfileTab('pins');setProfileOpen(true)}}/>
    <div className="z-message-scroll" ref={historyRoot}>{(jumpStatus||unreadJumpError)&&<p role="status">{jumpStatus||unreadJumpError}</p>}{!!readState.data?.unread_count&&<button type="button" className="secondary z-older" onClick={()=>void jumpToUnread()}>К первому непрочитанному</button>}
      {history.hasNextPage&&<button className="secondary z-older" disabled={history.isFetchingNextPage} onClick={()=>void history.fetchNextPage()}>Более ранние сообщения</button>}
      {history.isPending&&<p className="z-empty-copy" role="status">Загружаем историю…</p>}{history.isError&&<p className="notice">{errorText(history.error)}</p>}
      {!history.isPending&&!history.isError&&messages.length===0&&<div className="z-empty-copy"><p>Начни разговор</p><small>Напиши первое сообщение.</small></div>}
      {messages.map(message=><div key={message.id} data-message-id={message.id} {...messageGesture(message)} className={'z-bubble-row'+(message.kind==='system'?' is-system':message.sender_id===user.id?' is-own':'')}><div className={'z-bubble'+(message.kind==='image'?' is-image-bubble':'')} onDoubleClick={()=>{if(message.kind!=='system')void api.post(`/messages/${message.id}/reactions`,{emoji:'❤️'}).then(()=>cache.invalidateQueries({queryKey:['messages',conversation.id]})).catch(()=>{})}}>
        {message.reply_to_id&&<button type="button" className="z-reply-quote" onClick={()=>jumpToMessage(message.reply_to_id!)}><span>{message.reply_sender_name&&<strong>{message.reply_sender_name}: </strong>}{message.reply_body||'Сообщение удалено'}</span></button>}
        {message.forwarded_from_id&&<small className="z-forward-label">Переслано</small>}
        {message.kind==='image'?<><ImageMessage url={message.image_url} gallery={photoGallery} index={photoMessages.findIndex(photo=>photo.id===message.id)} onOpenChange={setImageViewing}/>{message.body.trim()&&<p className="z-image-caption">{message.body}</p>}</>:message.kind==='voice'&&message.audio_url?.startsWith('/api/messages/')?<VoiceMessage url={message.audio_url} duration={message.duration_ms}/>:message.kind==='file'&&message.attachment_mime?.startsWith('video/')?<video className="z-video-attachment" controls preload="metadata" src={message.attachment_url||`/api/messages/${message.id}/file`}/>:message.kind==='file'?<a className="z-file-message" href={message.attachment_url||`/api/messages/${message.id}/file`}><span>{message.attachment_name||'Вложение'}</span><small>Скачать файл · {message.attachment_mime||'файл'}</small></a>:<p>{message.body}</p>}
        <div className="z-message-footer"><div className="z-message-meta"><time>{new Date(message.created_at).toLocaleTimeString('ru-RU',{hour:'2-digit',minute:'2-digit'})}</time>{message.edited_at&&<span>Изменено</span>}{message.sender_id===user.id&&(messageStatus(message.id,readState.data?.peer_last_delivered_id,readState.data?.peer_last_read_id)==='read'?<CheckCheck size={16} aria-label="Прочитано" className="z-read-check"/>:messageStatus(message.id,readState.data?.peer_last_delivered_id,readState.data?.peer_last_read_id)==='delivered'?<CheckCheck size={16} aria-label="Доставлено" className="z-dark-check"/>:<Check size={16} aria-label="Отправлено" className="z-dark-check"/>)}</div>
        <MessageActions message={message} userID={user.id} conversationID={conversation.id} onReply={message=>{setReplyTo(message);messageInput.current?.focus({preventScroll:true})}} onChanged={()=>{void cache.invalidateQueries({queryKey:['messages',conversation.id]});void cache.invalidateQueries({queryKey:['pinned-messages',conversation.id]})}}/></div>
      </div></div>)}<div ref={bottom} style={{minHeight:1}}/>
    </div>
    <form noValidate className="z-composer-region" onSubmit={submit}>{controls.state&&blockNotice(controls.state)&&<p className="notice" role="status">{blockNotice(controls.state)}</p>}{controls.loading&&<p role="status">Проверяем переписку…</p>}{controls.error&&<p className="notice" role="alert">Не удалось проверить переписку. <button type="button" className="secondary" onClick={()=>void controls.refetch()}>Повторить</button></p>}{send.isError&&<p className="notice" role="alert">{errorText(send.error)} Повторите отправку: прежний идентификатор сохранён.</p>}
      {replyTo&&<div className="z-reply-composer"><span><small>Ответ на сообщение</small>{replyTo.body||merged.get(replyTo.id)?.body||'Исходное сообщение'}</span><button type="button" aria-label="Отменить ответ" onClick={()=>setReplyTo(null)}>×</button></div>}<div className="z-composer"><div className="z-composer-field"><textarea ref={messageInput} className="resize-none" aria-label="Сообщение" placeholder="Сообщение…" rows={1} maxLength={1000} value={body} readOnly={send.isPending} disabled={contactDisabled} onChange={e=>setBody(e.target.value)} onKeyDown={e=>{if(e.key==='Enter'&&enterSends&&!e.shiftKey&&!e.nativeEvent.isComposing){e.preventDefault();e.currentTarget.form?.requestSubmit()}}}/></div><Attachments conversationID={conversation.id} disabled={contactDisabled||!visible} photoDisabled={voiceActive} fileDisabled={imageActive||voiceActive} onPhotoActive={setImageActive} onPhotoSent={result=>{setLive(old=>({...old,[result.message.id]:result.message}));void cache.invalidateQueries({queryKey:['contact-profile',conversation.id]})}} onFileSent={message=>{setLive(old=>({...old,[message.id]:message}));void cache.invalidateQueries({queryKey:['messages',conversation.id]})}}/><MediaAvailability feature="voice"><VoiceComposer conversationID={conversation.id} disabled={callActive||!visible||contactDisabled||imageActive} onActiveChange={setVoiceActive} onSent={result=>{setLive(old=>({...old,[result.message.id]:result.message}));void cache.invalidateQueries({queryKey:['contact-profile',conversation.id]})}}/></MediaAvailability><button className="z-send z-text-send" aria-label="Отправить сообщение" disabled={!body.trim()||send.isPending||contactDisabled}><Send size={23}/></button></div>
    </form>
  </section>
}

