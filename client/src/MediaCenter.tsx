import { useEffect, useRef, useState } from 'react'
import { useInfiniteQuery } from '@tanstack/react-query'
import { api, errorText } from './api'
import { ImageMessage } from './ImageMessage'
import { linkRiskReason } from './link-safety'

type Medium = {
  id: string
  created_at: string
  body: string
  image_url?: string
  attachment_url?: string
  attachment_name?: string
  attachment_size?: number
}

const tabs = [['photos','Фото'],['videos','Видео'],['files','Файлы'],['links','Ссылки']] as const

export function MediaCenter({ scope, chatID, onJump }: { scope: 'personal' | 'group'; chatID: string; onJump: (id: string) => void }) {
  const [tab, setTab] = useState<(typeof tabs)[number][0]>('photos')
  const [search, setSearch] = useState('')
  const [query, setQuery] = useState('')
  const [warningLink, setWarningLink] = useState<string | null>(null)
  const warningDialog = useRef<HTMLDialogElement>(null)
  useEffect(() => {
    const dialog = warningDialog.current
    if (warningLink && dialog && !dialog.open) dialog.showModal()
    else if (!warningLink && dialog?.open) dialog.close()
  }, [warningLink])
  const data = useInfiniteQuery({
    queryKey: ['chat-media', scope, chatID, tab, query],
    initialPageParam: '',
    queryFn: async ({ pageParam, signal }) => (await api.get<{ messages: Medium[]; next_cursor: string }>(
      (scope === 'group' ? '/groups/' : '/conversations/') + chatID + '/search',
      { signal, params: { q: query, type: tab, before: pageParam || undefined, limit: 18 } },
    )).data,
    getNextPageParam: page => page.next_cursor || undefined,
    staleTime: 15_000,
  })
  const entries = data.data?.pages.flatMap(page => page.messages) ?? []
  const photos = entries.map(item => item.image_url).filter((url): url is string => !!url)
  const labelDate = (date: string) => new Date(date).toLocaleDateString('ru-RU', { dateStyle: 'long' })
  return <section className="z-media-center">
    <div className="z-search-filters" aria-label="Медиа переписки">
      {tabs.map(([value, label]) => <button type="button" key={value} aria-pressed={tab === value} className={tab === value ? 'is-active' : ''} onClick={() => { setTab(value); setSearch(''); setQuery('') }}>{label}</button>)}
    </div>
    {(tab === 'files' || tab === 'videos') && <form onSubmit={event => { event.preventDefault(); setQuery(search.trim()) }}>
      <input aria-label="Поиск файла" placeholder="Название файла…" maxLength={80} value={search} onChange={event => setSearch(event.target.value)}/>
      <button type="submit" className="secondary">Найти</button>
    </form>}
    {data.isPending && <p role="status">Загружаем…</p>}
    {data.isError && <p role="alert">{errorText(data.error)} <button type="button" onClick={() => void data.refetch()}>Повторить</button></p>}
    {data.isSuccess && !entries.length && <p className="z-muted">Здесь пока ничего нет.</p>}
    {entries.map((entry, index) => <div className="z-media-entry" key={entry.id}>
      {(index === 0 || labelDate(entries[index - 1].created_at) !== labelDate(entry.created_at)) && <h4>{labelDate(entry.created_at)}</h4>}
      {entry.image_url && <ImageMessage thumbnail url={entry.image_url} gallery={photos}/>}
      {entry.attachment_url && <a href={entry.attachment_url} download={entry.attachment_name}>{entry.attachment_name || 'Скачать файл'}</a>}
      {entry.attachment_size ? <small>{new Intl.NumberFormat('ru-RU', { style: 'unit', unit: 'megabyte', maximumFractionDigits: 1 }).format(entry.attachment_size / 1_048_576)}</small> : null}
      {tab === 'links' && entry.body.match(/https?:\/\/[^\s<>"']+/gi)?.map((link, i) => <a key={i} href={link} target="_blank" rel="noopener noreferrer" onClick={event => { if (linkRiskReason(link)) { event.preventDefault(); setWarningLink(link) } }}>{link}{linkRiskReason(link) && <span className="z-link-warning-mark" aria-label="Проверить ссылку"> ⚠</span>}</a>)}
      <button type="button" className="z-media-jump" onClick={() => onJump(entry.id)}>К сообщению</button>
    </div>)}
    {data.hasNextPage && <button type="button" className="secondary" disabled={data.isFetchingNextPage} onClick={() => void data.fetchNextPage()}>{data.isFetchingNextPage ? 'Загружаем…' : 'Показать ещё'}</button>}
    <dialog ref={warningDialog} className="z-link-warning" aria-labelledby="z-link-warning-title" onCancel={event => { event.preventDefault(); setWarningLink(null) }} onClick={event => { if (event.target === event.currentTarget) setWarningLink(null) }}>
      <h2 id="z-link-warning-title">Проверь ссылку</h2>
      <p>Эта ссылка выглядит потенциально рискованной: {warningLink ? linkRiskReason(warningLink) : ''}. Zently не проверяет содержимое страницы.</p>
      {warningLink && <strong className="z-link-warning-url">{warningLink}</strong>}
      <div><button type="button" className="secondary" onClick={() => setWarningLink(null)}>Отмена</button><button type="button" className="primary" onClick={() => { if (warningLink) window.open(warningLink, '_blank', 'noopener,noreferrer'); setWarningLink(null) }}>Открыть ссылку</button></div>
    </dialog>
  </section>
}

