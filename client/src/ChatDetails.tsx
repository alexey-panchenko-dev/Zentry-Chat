import { useId, useState } from 'react'
import type { ReactNode } from 'react'
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ChevronRight, Pin, Search, X } from 'lucide-react'
import { api, errorText } from './api'
import { MediaCenter } from './MediaCenter'

export type DetailsTab = 'info' | 'media' | 'search' | 'pins'
type ChatMessage = { id: string; body: string; kind?: string; image_url?: string; attachment_name?: string; created_at: string }
type Scope = 'personal' | 'group'
const pinKey = (scope: Scope, chatID: string) => [scope === 'group' ? 'group-pins' : 'pinned-messages', chatID]
const endpoint = (scope: Scope, chatID: string) => (scope === 'group' ? '/groups/' : '/conversations/') + chatID

function messagePreview(message: ChatMessage) {
  return message.body || message.attachment_name || ({ image: 'Фотография', voice: 'Голосовое сообщение', file: 'Файл' }[message.kind ?? ''] ?? 'Сообщение')
}

function usePins(scope: Scope, chatID: string) {
  return useQuery({
    queryKey: pinKey(scope, chatID),
    queryFn: async ({ signal }) => (await api.get<{ messages: ChatMessage[] }>(endpoint(scope, chatID) + '/pins', { signal })).data.messages,
    retry: false,
  })
}

export function PinnedMessageBar({ scope, chatID, onJump, onOpen }: {
  scope: Scope; chatID: string; onJump: (id: string) => void; onOpen: () => void
}) {
  const pins = usePins(scope, chatID)
  if (!pins.data?.length) return null
  return <div className="z-pinned-bar">
    <button type="button" onClick={() => onJump(pins.data![0].id)} title="Перейти к закреплённому сообщению">
      <Pin size={16}/><span><small>Закреплённое сообщение</small><span>{messagePreview(pins.data[0])}</span></span><ChevronRight size={16}/>
    </button>
    {pins.data.length > 1 && <button type="button" className="z-pin-count" onClick={onOpen} aria-label="Все закреплённые сообщения">{pins.data.length} закреплённых</button>}
  </div>
}

export function ChatDetails({ scope, chatID, initialTab = 'info', info, onJump, canManagePins = true }: {
  scope: Scope; chatID: string; initialTab?: DetailsTab; info: ReactNode; onJump: (id: string) => void; canManagePins?: boolean
}) {
  const [tab, setTab] = useState(initialTab)
  const [draft, setDraft] = useState('')
  const [query, setQuery] = useState('')
  const [type, setType] = useState('')
  const tabsID = useId()
  const cache = useQueryClient()
  const pins = usePins(scope, chatID)
  const unpin = useMutation({
    mutationFn: (id: string) => api.post(endpoint(scope, chatID) + '/pins', { message_id: id, pinned: false }),
    onSuccess: () => cache.invalidateQueries({ queryKey: pinKey(scope, chatID) }),
  })
  const results = useInfiniteQuery({
    queryKey: ['details-search', scope, chatID, query, type],
    enabled: tab === 'search' && (!!query || !!type),
    initialPageParam: '',
    queryFn: async ({ pageParam, signal }) => (await api.get<{ messages: ChatMessage[]; next_cursor: string }>(
      endpoint(scope, chatID) + '/search', { signal, params: { q: query, type, limit: 25, before: pageParam || undefined } },
    )).data,
    getNextPageParam: page => page.next_cursor || undefined,
    retry: false,
  })
  const filters = [['', 'Все'], ['messages', 'Текст'], ['photos', 'Фото'], ['videos', 'Видео'], ...(scope === 'personal' ? [['voice', 'Голосовые']] : []), ['files', 'Файлы'], ['links', 'Ссылки']]
  return <div className="z-chat-details">
    <div className="z-details-tabs" role="tablist" aria-label="Разделы профиля" onKeyDown={event => {
      if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
      const buttons = Array.from(event.currentTarget.querySelectorAll<HTMLButtonElement>('[role=tab]'))
      const index = buttons.indexOf(document.activeElement as HTMLButtonElement)
      const next = event.key === 'Home' ? 0 : event.key === 'End' ? buttons.length - 1 :
        (index + (event.key === 'ArrowRight' ? 1 : -1) + buttons.length) % buttons.length
      event.preventDefault()
      buttons[next]?.click()
      buttons[next]?.focus()
    }}>
      {([['info', 'Информация'], ['media', 'Медиа'], ['search', 'Поиск'], ['pins', 'Закреплённые']] as const).map(([value, label]) =>
        <button key={value} type="button" role="tab" id={tabsID + value} aria-controls={tabsID + '-panel'} aria-selected={tab === value} tabIndex={tab === value ? 0 : -1}
          onClick={() => setTab(value)}>{label}</button>)}
    </div>
    <div id={tabsID + '-panel'} role="tabpanel" aria-labelledby={tabsID + tab}>
      {tab === 'info' && info}
      {tab === 'media' && <MediaCenter scope={scope} chatID={chatID} onJump={onJump}/>}
      {tab === 'search' && <section className="z-details-search">
        <form onSubmit={event => { event.preventDefault(); setQuery(draft.trim()) }}>
          <label className="z-detail-search-field"><Search size={17}/><input autoFocus maxLength={80} aria-label="Поиск по переписке" placeholder="Найти сообщение…" value={draft} onChange={event => setDraft(event.target.value)}/></label>
          <button type="submit" className="secondary">Найти</button>
        </form>
        <div className="z-search-filters" aria-label="Тип сообщения">{filters.map(([value, label]) =>
          <button key={value} type="button" aria-pressed={type === value} onClick={() => { setType(value); setQuery(draft.trim()) }}>{label}</button>)}</div>
        {!query && !type ? <p className="z-muted">Введите текст или выберите тип вложения.</p> : <>
          {results.isPending && <p role="status">Ищем сообщения…</p>}
          {results.isError && <p className="notice" role="alert">{errorText(results.error)} <button type="button" onClick={() => void results.refetch()}>Повторить</button></p>}
          {results.data?.pages.flatMap(page => page.messages).map(message =>
            <button type="button" className="z-tool-result z-search-result-row" key={message.id} onClick={() => onJump(message.id)}>
              {message.kind==='image'&&<img className="z-search-photo-thumb" src={(scope==='group'?`/api/groups/${chatID}/messages/${message.id}/image`:`/api/messages/${message.id}/image`)+'?preview=1'} alt="Миниатюра фотографии" loading="lazy" decoding="async"/>}
              <span className="z-search-result-copy"><span>{messagePreview(message)}</span><small>{new Date(message.created_at).toLocaleString('ru-RU')}</small></span>
            </button>)}
          {results.isSuccess && !results.data.pages[0].messages.length && <p className="z-muted">Ничего не найдено.</p>}
          {results.hasNextPage && <button type="button" className="secondary" disabled={results.isFetchingNextPage} onClick={() => void results.fetchNextPage()}>Показать ещё</button>}
        </>}
      </section>}
      {tab === 'pins' && <section className="z-details-pins">
        {pins.isPending && <p role="status">Загружаем закрепления…</p>}
        {pins.isError && <p className="notice" role="alert">{errorText(pins.error)} <button type="button" onClick={() => void pins.refetch()}>Повторить</button></p>}
        {pins.isSuccess && !pins.data.length && <p className="z-muted">В этой переписке пока нет закреплений.</p>}
        {pins.data?.map(message => <div className="z-pin-row" key={message.id}>
          <button type="button" onClick={() => onJump(message.id)}>{messagePreview(message)}</button>
          {canManagePins && <button type="button" disabled={unpin.isPending} title="Открепить сообщение" aria-label="Открепить сообщение" onClick={() => unpin.mutate(message.id)}><X size={16}/></button>}
        </div>)}
        {unpin.isError && <p className="notice" role="alert">{errorText(unpin.error)}</p>}
      </section>}
    </div>
  </div>
}
