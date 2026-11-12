export type PresenceLabel = { label: string; title: string }

export function formatPresence(online: boolean, lastSeen?: string | null, now = new Date()): PresenceLabel {
  if (online) return { label: 'В сети', title: 'Сейчас в сети' }
  if (!lastSeen) return { label: 'Не в сети', title: 'Время последней активности неизвестно' }

  const date = new Date(lastSeen)
  if (!Number.isFinite(date.getTime())) return { label: 'Не в сети', title: 'Время последней активности неизвестно' }

  const time = new Intl.DateTimeFormat('ru-RU', { hour: '2-digit', minute: '2-digit' }).format(date)
  const sameDay = date.getFullYear() === now.getFullYear()
    && date.getMonth() === now.getMonth()
    && date.getDate() === now.getDate()
  const day = new Intl.DateTimeFormat('ru-RU', {
    day: 'numeric',
    month: 'short',
    ...(date.getFullYear() !== now.getFullYear() ? { year: 'numeric' as const } : {}),
  }).format(date).replace('.', '')
  const exact = new Intl.DateTimeFormat('ru-RU', {
    dateStyle: 'long',
    timeStyle: 'short',
  }).format(date)

  return {
    label: sameDay ? `Был(а) в ${time}` : `Был(а) ${day}`,
    title: `Последняя активность: ${exact}`,
  }
}
