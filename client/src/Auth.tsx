import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, Eye, EyeOff } from 'lucide-react'
import { api, errorText } from './api'
import type { User } from './api'
import { Brand } from './Visual'
import { BirthDateWheel } from './BirthDateWheel'
import { parseBirthDate, birthDatesSupported } from './birthdays'
import { rememberedAccounts, rememberAccount } from './accounts'

const newPasswordIsValid = (value: string, minimum: number) =>
  value.length >= minimum && value.length <= 16 &&
  /^[A-Za-z0-9&*$%]+$/.test(value) &&
  /[a-z]/.test(value) && /[A-Z]/.test(value) && /[0-9]/.test(value) && /[&*$%]/.test(value)

export default function Auth() {
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
  const [showPassword, setShowPassword] = useState(false)
  const [code, setCode] = useState('')
  const [authNotice, setAuthNotice] = useState('')
  const [resendSeconds, setResendSeconds] = useState(0)
  const [birthDate,setBirthDate]=useState('')
  const registrationHealth=useQuery({queryKey:['registration-capabilities'],enabled:register||mode==='reset',queryFn:async()=>(await api.get<unknown>('/health')).data,retry:false,staleTime:60_000,refetchInterval:query=>register&&!birthDatesSupported(query.state.data)?30_000:false})
  const registrationReady=birthDatesSupported(registrationHealth.data)
  const passwordMinimum=(registrationHealth.data as {capabilities?:{short_passwords?:boolean}}|undefined)?.capabilities?.short_passwords===true?6:12
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
    onError: error => { if ((error as {response?:{data?:{error?:string}}}).response?.data?.error==='registration_pending') { setMode('verify'); setCode(''); setResendSeconds(0); setAuthNotice('Регистрация уже начата. Введите код из письма или запросите новый.'); } },
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
      else if(field.name==='password'&&mode!=='login'&&!newPasswordIsValid(field.value,passwordMinimum))message=`Пароль: ${passwordMinimum}–16 символов, латинские строчные и заглавные буквы, цифра и один из символов & * $ %.`
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
    <Brand/>
    {(mode==='login'||mode==='register')&&<div className="z-auth-mode-tabs" role="group" aria-label="Выбор действия">
      <button type="button" aria-pressed={mode==='login'} onClick={()=>{setMode('login');setPassword('');setShowPassword(false);setValidation(null);setAuthNotice('');auth.reset();registerMutation.reset()}}>Войти</button>
      <button type="button" aria-pressed={mode==='register'} onClick={()=>{setMode('register');setPassword('');setShowPassword(false);setValidation(null);setAuthNotice('');auth.reset();registerMutation.reset()}}>Зарегистрироваться</button>
    </div>}
    <p className="z-eyebrow">БЛИЖЕ ДРУГ К ДРУГУ</p><h1>{mode==='register' ? <>Твой круг.<br/>Твои разговоры.</> : mode==='verify'?<>Подтверди<br/>свою почту.</>:mode==='forgot'||mode==='reset'?<>Вернёмся<br/>в Zently.</>:<>С возвращением<br/>в Zently.</>}</h1>
    <p className="z-muted">{modeTitle}</p>
    <form noValidate className="z-auth-form" onSubmit={submitAuth}>
      {mode==='register' && <label>Ник / @юзер<input name="name" aria-invalid={validation?.field==='name'} aria-describedby={validation?.field==='name'?'auth-validation':undefined} required maxLength={41} value={name} onChange={(e) => setName(e.target.value)} autoComplete="username" placeholder="Например, byliver" /></label>}
      {mode==='register'&&<div className="z-auth-birth-date"><label>Дата рождения</label><BirthDateWheel value={birthDate} onChange={setBirthDate} invalid={validation?.field==='birth_date'} describedBy={validation?.field==='birth_date'?'auth-validation':undefined}/></div>}
      {mode!=='verify'&&<label>Почта<input name="email" aria-invalid={validation?.field==='email'} aria-describedby={validation?.field==='email'?'auth-validation':undefined} type="email" required value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="email" placeholder="you@example.com" readOnly={mode==='reset'} /></label>}
      {mode==='verify'&&<p className="z-field-help">Код подтверждения для <strong>{email}</strong> действует 10 минут.</p>}
      {mode==='login' && savedAccounts.length > 0 && <div className="z-saved-accounts" aria-label="Запомненные аккаунты">{savedAccounts.map(account => <button type="button" className="secondary" key={account} onClick={() => setEmail(account)}>{account}</button>)}</div>}
      {(mode==='login'||mode==='register'||mode==='reset')&&<label>{mode==='reset'?'Новый пароль':'Пароль'}<span className="z-password-wrap"><input name="password" aria-invalid={validation?.field==='password'} aria-describedby={validation?.field==='password'?'auth-validation':undefined} type={showPassword?'text':'password'} required minLength={mode==='login'?1:passwordMinimum} maxLength={mode==='login'?72:16} value={password} onChange={(e) => setPassword(e.target.value)} autoComplete={register||mode==='reset' ? 'new-password' : 'current-password'} placeholder={mode==='login'?'Введите пароль':`${passwordMinimum}–16 символов, A–Z, a–z, 0–9, & * $ %`} /><button className="z-password-visibility" type="button" aria-label={showPassword?'Скрыть пароль':'Показать пароль'} aria-pressed={showPassword} onClick={()=>setShowPassword(visible=>!visible)}>{showPassword?<EyeOff size={18} aria-hidden="true"/>:<Eye size={18} aria-hidden="true"/>}</button></span></label>}
      {(mode==='verify'||mode==='reset')&&<label>Код из письма<input name="code" aria-invalid={validation?.field==='code'} aria-describedby={validation?.field==='code'?'auth-validation':undefined} inputMode="numeric" autoComplete="one-time-code" maxLength={6} pattern="[0-9]{6}" required value={code} onChange={e=>setCode(e.target.value.replace(/\D/g,'').slice(0,6))} placeholder="000000" /></label>}
      {mode==='login'&&<label className="z-remember-account"><input type="checkbox" checked={remember} onChange={event => setRemember(event.target.checked)}/> Запомнить почту на этом устройстве</label>}
      {(mode==='register'||mode==='reset')&&<p className="z-field-help">{passwordMinimum}–16 символов: латинские строчные и заглавные буквы, цифра и один из символов &amp; * $ %.</p>}
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
  </section>
}
