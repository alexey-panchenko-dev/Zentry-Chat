import { useEffect, useRef, useState } from 'react'
import type { Conversation } from './api'
import type { ChatOptions } from './ChatWorkspace'

export const defaultAppearance = {
  text: 'normal', density: 'normal', motion: 'full', backdrop: 'zently', navSide: 'right',
  preview: true, sound: true, enter: true,
}

type Appearance = typeof defaultAppearance

export function AppearanceSettings({ value, update, showSound=false }: { value: Appearance; update: (value: Appearance) => void; showSound?:boolean }) {
  const options = [
    ['text', 'Размер текста', [['normal','Обычный'],['large','Крупный']]],
    ['density','Интервалы в чате',[['normal','Обычные'],['compact','Компактные']]],
    ['motion','Анимации',[['full','Плавные'],['reduced','Минимальные']]],
    ['backdrop','Фон чата',[['zently','Zently'],['graphite','Графит'],['plum','Слива']]],
    ['navSide','Панель навигации',[['right','Справа'],['left','Слева']]],
  ] as const
  return <section className="z-appearance">
    <h2>Оформление и сообщения</h2>
    {options.map(([key,label,choices]) => <label className="z-setting-row" key={key}>{label}
      <select value={value[key] ?? defaultAppearance[key]} onChange={event => update({ ...value, [key]: event.target.value })}>{choices.map(([id,text]) => <option key={id} value={id}>{text}</option>)}</select>
    </label>)}
    {showSound&&<label className="z-setting-row">Звуки сообщений<input type="checkbox" checked={value.sound} onChange={event=>update({...value,sound:event.target.checked})}/></label>}
    {([['preview','Текст в уведомлениях'],['enter','Enter отправляет сообщение']] as const).map(([key,label]) =>
      <label className="z-setting-row" key={key}>{label}<input type="checkbox" checked={value[key]} onChange={event => update({ ...value, [key]: event.target.checked })}/></label>)}
  </section>
}

type Notice = { chat: Conversation; count: number }
export function useIncomingNotices(dialogs: Conversation[], options: Record<string,ChatOptions>, visibleChat: string | undefined, sound: boolean | ((chat:Conversation)=>boolean), loaded: boolean) {
  const [notice, setNotice] = useState<Notice | null>(null)
  const seen = useRef(new Map<string,string>())
  const initialized = useRef(false)
  const newest = useRef(0n)
  const timer = useRef(0)
  useEffect(() => {
    try { getNotificationAudio() } catch { /* Sound remains optional when the browser has no Web Audio support. */ }
    const unlock = () => {
      const context = getNotificationAudio()
      if (context?.state === 'suspended') void context.resume().catch(() => {})
    }
    window.addEventListener('pointerdown', unlock, true)
    window.addEventListener('keydown', unlock, true)
    return () => {
      window.removeEventListener('pointerdown', unlock, true)
      window.removeEventListener('keydown', unlock, true)
    }
  }, [])
  useEffect(() => () => window.clearTimeout(timer.current), [])
  useEffect(() => {
    if (!loaded) return
    const previousNewest = newest.current
    const updates: Notice[] = []
    for (const chat of dialogs) {
      if (!chat.last_message_id) continue
      const latestID = BigInt(chat.last_message_id)
      if (latestID > newest.current) newest.current = latestID
      const previous = seen.current.get(chat.id)
      if (previous && BigInt(previous) >= BigInt(chat.last_message_id)) continue
      seen.current.set(chat.id, chat.last_message_id)
      if (initialized.current && (previous || latestID > previousNewest) && chat.last_message_incoming && chat.unread_count && visibleChat !== chat.id && !options[chat.id]?.muted) {
        updates.push({chat,count:chat.unread_count})
      }
    }
    initialized.current = true
    const latest = updates.sort((a,b) => BigInt(a.chat.last_message_id!) > BigInt(b.chat.last_message_id!) ? -1 : 1)[0]
    if (!latest) return
    setNotice(latest)
    window.clearTimeout(timer.current)
    timer.current = window.setTimeout(() => setNotice(null), 6500)
    if ((typeof sound==='function'?sound(latest.chat):sound) && document.visibilityState === 'visible') playChime()
  }, [dialogs, options, visibleChat, sound, loaded])
  return { notice, dismiss: () => setNotice(null) }
}

let notificationAudio: AudioContext | undefined
function getNotificationAudio() {
  if (!notificationAudio && typeof window !== 'undefined' && window.AudioContext) notificationAudio = new window.AudioContext()
  return notificationAudio
}

function playChime() {
  try {
    const context = getNotificationAudio()
    if (!context || context.state !== 'running') return
    const now = context.currentTime
    for (const [frequency, start, peak] of [[659.25, 0, .012], [783.99, .075, .008]] as const) {
      const tone = context.createOscillator()
      const volume = context.createGain()
      const began = now + start
      tone.type = 'sine'
      tone.frequency.setValueAtTime(frequency, began)
      volume.gain.setValueAtTime(.0001, began)
      volume.gain.linearRampToValueAtTime(peak, began + .025)
      volume.gain.exponentialRampToValueAtTime(.0001, began + .27)
      tone.connect(volume).connect(context.destination)
      tone.start(began)
      tone.stop(began + .28)
    }
  } catch { /* Some browsers require a user gesture before playing audio. */ }
}

export function useMobileViewport(enabled: boolean) {
  const root = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const viewport = window.visualViewport
    const element = root.current
    if (!enabled || !viewport || !element) return
    function update() {
      if (viewport && element && viewport.scale === 1) element.style.setProperty('--z-viewport-height', Math.round(viewport.height) + 'px')
    }
    update()
    viewport.addEventListener('resize', update)
    return () => {
      viewport.removeEventListener('resize', update)
      element.style.removeProperty('--z-viewport-height')
    }
  }, [enabled])
  return root
}
