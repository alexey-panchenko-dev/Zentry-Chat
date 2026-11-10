import { useEffect, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, errorText } from './api'

export type Favorite = { id: string; chat_id: string; scope: 'personal' | 'group'; title: string; body: string; kind: string; saved_at: string }

export function useFavorites() {
  return useQuery({
    queryKey: ['favorites'],
    queryFn: async () => (await api.get<{ favorites: Favorite[] }>('/favorites')).data.favorites,
    staleTime: 30_000,
    retry: false,
  })
}

export function useFavoriteChange() {
  const cache = useQueryClient()
  return useMutation({
    mutationFn: ({ scope, id, save }: { scope: Favorite['scope']; id: string; save: boolean }) =>
      save ? api.put('/favorites/' + scope + '/' + id) : api.delete('/favorites/' + scope + '/' + id),
    onSuccess: () => cache.invalidateQueries({ queryKey: ['favorites'] }),
  })
}

export function FavoritesPanel({ onOpen }: { onOpen: (item: Favorite) => Promise<void> }) {
  const items = useFavorites()
  const change = useFavoriteChange()
  const [error, setError] = useState('')
  return <section className="z-settings z-favorites">
    <h1>Избранные сообщения</h1>
    <p className="z-muted">Эти сообщения видны только вам.</p>
    {items.isPending && <p role="status">Загружаем избранное…</p>}
    {items.isError && <p role="alert" className="notice">{errorText(items.error)}</p>}
    {items.data?.length === 0 && <p>Сохраните сообщение через меню рядом с ним.</p>}
    {items.data?.map(item => <div className="z-favorite-item" key={item.scope + item.id}>
      <button type="button" onClick={() => { setError(''); void onOpen(item).catch(() => setError('Исходная переписка больше не доступна.')) }}>
        <strong>{item.title}</strong><span>{item.body || 'Вложение'}</span>
      </button>
      <button type="button" className="z-icon" aria-label="Убрать из избранного" disabled={change.isPending} onClick={() => change.mutate({ ...item, save: false })}>×</button>
    </div>)}
    {(error || change.isError) && <p className="notice" role="alert">{error || errorText(change.error)}</p>}
  </section>
}

export function useAccountSetting<T>(userID: string, name: string, initial: T) {
  const key = 'zently:' + userID + ':' + name
  const [value, setValue] = useState<T>(() => {
    try { const stored = localStorage.getItem(key); return stored ? JSON.parse(stored) as T : initial }
    catch { return initial }
  })
  const current = useRef(value)
  useEffect(() => {
    const read = (event: StorageEvent) => {
      if (event.key !== key) return
      try { current.current = event.newValue ? JSON.parse(event.newValue) as T : initial; setValue(current.current) } catch { /* Ignore invalid settings. */ }
    }
    window.addEventListener('storage', read)
    return () => window.removeEventListener('storage', read)
  }, [key, initial])
  function update(next: T | ((previous: T) => T)) {
    const result = typeof next === 'function' ? (next as (previous: T) => T)(current.current) : next
    current.current = result
    setValue(result)
    try { localStorage.setItem(key, JSON.stringify(result)) } catch { /* Settings still work for this session. */ }
  }
  return [value, update] as const
}

export type ChatOptions = { pinned?: boolean; unread?: boolean; muted?: boolean }
export function ChatRow({ label, selected, options, onOpen, onFirstUnread, onChange, children }: {
  label: string; selected: boolean; options?: ChatOptions; onOpen: () => void; onFirstUnread?: () => void;
  onChange: (option: keyof ChatOptions) => void; children: React.ReactNode
}) {
  const [open, setOpen] = useState(false)
  const start = useRef<{ x: number; y: number; id: number } | null>(null)
  return <div className="z-chat-row" data-pinned={!!options?.pinned} onPointerDown={event => {
    if (event.pointerType === 'touch' && event.isPrimary) start.current = { x: event.clientX, y: event.clientY, id: event.pointerId }
  }} onPointerCancel={() => { start.current = null }} onPointerUp={event => {
    const origin = start.current
    start.current = null
    if (origin && origin.id === event.pointerId && origin.x - event.clientX > 56 && Math.abs(origin.y - event.clientY) < 18) setOpen(true)
  }}>
    <button type="button" className={'z-conversation' + (selected ? ' is-selected' : '')} onClick={() => { if (open) setOpen(false); else onOpen() }}>{children}</button>
    <button type="button" className="z-chat-row-menu z-icon" aria-label={'Действия: ' + label} aria-expanded={open} onClick={() => setOpen(!open)}>···</button>
    {open && <div className="z-chat-quick-actions">{onFirstUnread&&<button type="button" onClick={()=>{setOpen(false);onFirstUnread()}}>К первому непрочитанному</button>}
      <button type="button" onClick={() => { onChange('pinned'); setOpen(false) }}>{options?.pinned ? 'Открепить' : 'Закрепить чат'}</button>
      <button type="button" onClick={() => { onChange('unread'); setOpen(false) }}>{options?.unread ? 'Снять пометку' : 'Пометить непрочитанным'}</button>
      <button type="button" onClick={() => { onChange('muted'); setOpen(false) }}>{options?.muted ? 'Включить уведомления' : 'Отключить уведомления'}</button>
    </div>}
  </div>
}
