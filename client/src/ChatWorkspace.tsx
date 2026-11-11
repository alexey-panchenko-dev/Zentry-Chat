import { useEffect, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, errorText } from './api'
import { Bookmark } from 'lucide-react'
import { ActionMenu } from './ActionMenu'

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
    {items.data?.length === 0 && <div className="z-favorites-empty"><Bookmark size={30}/><h2>Важное — под рукой</h2><p>Откройте меню сообщения и выберите «В избранное», чтобы сохранить его здесь.</p></div>}
    {items.data?.map(item => <div className="z-favorite-item" key={item.scope + item.id}>
      <button type="button" onClick={() => { setError(''); void onOpen(item).catch(() => setError('Исходная переписка больше не доступна.')) }}>
        <strong>{item.title}</strong><span>{item.body || 'Вложение'}</span><time dateTime={item.saved_at}>{new Date(item.saved_at).toLocaleDateString('ru-RU', {dateStyle:'long'})}</time>
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
  const row = useRef<HTMLDivElement>(null)
  const gesture = useRef<{ x: number; y: number; id: number; timer: number } | null>(null)
  const suppressClick = useRef(false)

  function stopGesture() {
    if (gesture.current) window.clearTimeout(gesture.current.timer)
    gesture.current = null
  }
  function showMenu() {
    const menu = row.current?.querySelector<HTMLDetailsElement>('details')
    if (menu) menu.open = true
  }
  useEffect(() => stopGesture, [])
  return <div ref={row} className="z-chat-row" data-pinned={!!options?.pinned} onContextMenu={event => {
    event.preventDefault()
    showMenu()
  }} onPointerDown={event => {
    if (event.pointerType !== 'touch' || !event.isPrimary || (event.target as Element).closest('summary')) return
    stopGesture()
    suppressClick.current = false
    const timer = window.setTimeout(() => { suppressClick.current = true; showMenu() }, 460)
    gesture.current = { x: event.clientX, y: event.clientY, id: event.pointerId, timer }
  }} onPointerMove={event => {
    const start = gesture.current
    if (start && (Math.abs(event.clientX - start.x) > 10 || Math.abs(event.clientY - start.y) > 10)) stopGesture()
  }} onPointerCancel={stopGesture} onPointerUp={event => {
    const start = gesture.current
    stopGesture()
    if (start && start.id === event.pointerId && start.x - event.clientX > 56 && Math.abs(start.y - event.clientY) < 18) {
      suppressClick.current = true
      showMenu()
    }
  }}>
    <button type="button" className={'z-conversation' + (selected ? ' is-selected' : '')} onClick={() => {
      if (suppressClick.current) { suppressClick.current = false; return }
      onOpen()
    }}>{children}</button>
    <ActionMenu className="z-chat-row-menu" label={'Действия: ' + label}>
      {onFirstUnread && <button type="button" onClick={onFirstUnread}>К первому непрочитанному</button>}
      <button type="button" onClick={() => onChange('pinned')}>{options?.pinned ? 'Открепить' : 'Закрепить чат'}</button>
      <button type="button" onClick={() => onChange('unread')}>{options?.unread ? 'Снять пометку' : 'Пометить непрочитанным'}</button>
      <button type="button" onClick={() => onChange('muted')}>{options?.muted ? 'Включить уведомления' : 'Отключить уведомления'}</button>
    </ActionMenu>
  </div>
}
