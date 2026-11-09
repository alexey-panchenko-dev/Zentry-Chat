import { useQuery } from '@tanstack/react-query'
import { Phone, PhoneIncoming, PhoneMissed, PhoneOutgoing } from 'lucide-react'
import { api, errorText } from './api'

type CallRecord = {
  id: string
  peer_id: string
  peer_name: string
  direction: 'incoming' | 'outgoing'
  status: 'ended' | 'rejected' | 'cancelled' | 'expired' | string
  created_at: string
  duration_ms: number
}

function durationLabel(duration: number) {
  const total = Math.max(0, Math.floor(duration / 1000))
  const minutes = Math.floor(total / 60)
  const seconds = total % 60
  return minutes ? `${minutes}:${String(seconds).padStart(2, '0')}` : `${seconds} сек.`
}

function callStatus(status: string) {
  if (status === 'ended') return 'Завершён'
  if (status === 'rejected') return 'Отклонён'
  if (status === 'cancelled') return 'Отменён'
  return 'Нет ответа'
}

export function CallHistory() {
  const history = useQuery({
    queryKey: ['call-history'],
    queryFn: async () => (await api.get<{ calls: CallRecord[] }>('/calls/history')).data.calls,
    refetchInterval: 30_000,
  })

  return <section className="z-call-lobby z-call-history" aria-labelledby="call-history-title">
    <header className="z-call-history-heading"><Phone size={27}/><div><h1 id="call-history-title">Звонки</h1><p>История недавних вызовов</p></div></header>
    {history.isPending && <p className="z-muted" role="status">Загружаем звонки…</p>}
    {history.isError && <div className="z-call-history-error"><p className="notice" role="alert">{errorText(history.error)}</p><button type="button" className="secondary" onClick={()=>void history.refetch()}>Повторить</button></div>}
    {history.data?.length===0 && <div className="z-call-history-empty"><Phone size={34}/><strong>Пока нет звонков</strong><span>Здесь появятся время, собеседник и длительность вызовов.</span></div>}
    {!!history.data?.length && <div className="z-call-history-list">{history.data.map(call=>{
      const missed = call.status !== 'ended'
      const Icon = missed ? PhoneMissed : call.direction === 'incoming' ? PhoneIncoming : PhoneOutgoing
      const date = new Date(call.created_at)
      return <article className={'z-call-history-row'+(missed?' is-missed':'')} key={call.id}>
        <span className="z-call-history-icon"><Icon size={19}/></span>
        <div className="z-call-history-person"><strong>{call.peer_name}</strong><span>{call.direction==='incoming'?'Входящий':'Исходящий'} · {callStatus(call.status)}</span></div>
        <div className="z-call-history-meta"><time dateTime={call.created_at}>{date.toLocaleDateString('ru-RU',{day:'numeric',month:'short'})} · {date.toLocaleTimeString('ru-RU',{hour:'2-digit',minute:'2-digit'})}</time><span>{call.duration_ms>0?durationLabel(call.duration_ms):'—'}</span></div>
      </article>
    })}</div>}
  </section>
}
