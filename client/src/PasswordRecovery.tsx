import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { ArrowLeft, Eye, EyeOff, Mail, ShieldCheck } from 'lucide-react'
import { api } from '@/api'
import { Brand } from '@/Visual'

function recoveryError(error: unknown) {
  const code = (error as {response?:{data?:{error?:string}}}).response?.data?.error
  const labels: Record<string,string> = {
    recovery_unavailable: 'Отправка писем ещё не подключена. Попробуй позже.',
    recovery_busy: 'Сервис восстановления занят. Попробуй через минуту.',
    reset_expired: 'Ссылка устарела или уже использована. Запроси новую.',
    invalid_reset_input: 'Проверь ссылку и пароль: от 12 до 72 байт.',
    rate_limit_exceeded: 'Слишком много попыток. Подожди минуту.',
  }
  return labels[code??''] ?? 'Не удалось связаться с сервером. Попробуй ещё раз.'
}

export function RecoveryRequest({onBack}: {onBack:()=>void}) {
  const [email,setEmail] = useState('')
  const [invalid,setInvalid] = useState(false)
  const request = useMutation({mutationFn:()=>api.post('/auth/forgot-password',{email:email.trim()})})
  return <section className="z-recovery">
    <span className="z-recovery-icon"><Mail size={23}/></span><h2>Восстановить доступ</h2>
    <p className="z-muted">Пришлём одноразовую ссылку на почту твоего аккаунта. Старый пароль не нужен.</p>
    {request.isSuccess?<div role="status" className="z-recovery-result"><ShieldCheck/><p>Если аккаунт с этой почтой существует, письмо придёт в ближайшие минуты. Проверь также папку «Спам».</p><small>Ссылка действует 15 минут. Если письма нет, повтори запрос через минуту.</small></div>:<form className="z-auth-form" noValidate onSubmit={e=>{e.preventDefault();const input=e.currentTarget.elements.namedItem('recovery-email') as HTMLInputElement;if(!input.validity.valid){setInvalid(true);input.focus();return}setInvalid(false);request.mutate()}}>
      <label htmlFor="recovery-email">Почта аккаунта</label><input id="recovery-email" name="recovery-email" type="email" autoComplete="email" required maxLength={254} value={email} onChange={e=>setEmail(e.target.value)} aria-invalid={invalid} aria-describedby={invalid?'recovery-error':undefined} placeholder="you@example.com"/>
      {invalid&&<p id="recovery-error" className="notice">Введи полную почту аккаунта.</p>}
      {request.isError&&<p role="alert" className="notice">{recoveryError(request.error)}</p>}
      <button className="primary" disabled={request.isPending}>{request.isPending?'Отправляем…':'Получить ссылку'}</button>
    </form>}
    <button className="z-auth-switch" onClick={onBack}><ArrowLeft size={15}/>Назад</button>
  </section>
}

export function PasswordReset({token,onDone}: {token:string;onDone:()=>void}) {
  const [password,setPassword] = useState('')
  const [confirm,setConfirm] = useState('')
  const [visible,setVisible] = useState(false)
  const [validation,setValidation] = useState('')
  const reset = useMutation({mutationFn:()=>api.post('/auth/reset-password',{token,password})})
  return <section className="z-auth-card"><Brand/><h1>Новый пароль</h1>
    {reset.isSuccess?<div className="z-recovery-result" role="status"><ShieldCheck/><h2>Пароль изменён</h2><p>Старые сеансы завершены. Войди с новым паролем.</p><button className="primary" onClick={onDone}>Перейти ко входу</button></div>:<><p className="z-muted">Придумай новый пароль для своего аккаунта Zently.</p><form className="z-auth-form" noValidate onSubmit={e=>{e.preventDefault();const length=new TextEncoder().encode(password).length;if(length<12||length>72){setValidation('Пароль должен содержать от 12 до 72 байт.');(e.currentTarget.elements.namedItem('new-password') as HTMLInputElement).focus();return}if(password!==confirm){setValidation('Пароли не совпадают.');(e.currentTarget.elements.namedItem('confirm-password') as HTMLInputElement).focus();return}setValidation('');reset.mutate()}}>
      <label htmlFor="new-password">Новый пароль</label><div className="z-password-field"><input id="new-password" name="new-password" autoComplete="new-password" type={visible?'text':'password'} value={password} onChange={e=>setPassword(e.target.value)} aria-describedby="reset-validation"/><button type="button" aria-label={visible?'Скрыть пароль':'Показать пароль'} onClick={()=>setVisible(!visible)}>{visible?<EyeOff size={18}/>:<Eye size={18}/>}</button></div>
      <label htmlFor="confirm-password">Повтори пароль</label><input id="confirm-password" name="confirm-password" autoComplete="new-password" type={visible?'text':'password'} value={confirm} onChange={e=>setConfirm(e.target.value)} aria-describedby="reset-validation"/>
      <p id="reset-validation" className={validation?'notice':'z-field-help'}>{validation||'От 12 до 72 байт. Можно вставить пароль из менеджера.'}</p>
      {reset.isError&&<p role="alert" className="notice">{recoveryError(reset.error)}</p>}
      <button className="primary" disabled={reset.isPending}>{reset.isPending?'Сохраняем…':'Сохранить пароль'}</button>
    </form><button className="z-auth-switch" onClick={onDone}>Вернуться ко входу</button></>}
  </section>
}
