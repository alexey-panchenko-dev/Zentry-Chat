import { useEffect, useRef, useState } from 'react'
import { Smile, X } from 'lucide-react'
import { Emoji } from './Emoji'

const emojis = ['😀','😃','😄','😁','😆','😅','😂','🙂','😉','😊','😍','🥰','😘','😎','🤔','🙃','😭','😡','👍','👎','❤️','🔥','🎉','🙏','🫶','👏','💯','✨']

export function EmojiPicker({ value, onChange, inputRef, disabled = false }: {
  value: string
  onChange: (value: string) => void
  inputRef: React.RefObject<HTMLTextAreaElement | null>
  disabled?: boolean
}) {
  const [open, setOpen] = useState(false)
  const [catalogue, setCatalogue] = useState<{ emoji: string; name: string; key: string }[]>([])
  const [search, setSearch] = useState('')
  const [limit, setLimit] = useState(84)
  const [loadError, setLoadError] = useState(false)
  const root = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    let active = true
    void import('./emoji-data/catalogue.json').then(module => { if (active) { setCatalogue(module.default); setLoadError(false) } }).catch(() => { if (active) setLoadError(true) })
    const outside = (event: PointerEvent) => {
      if (!root.current?.contains(event.target as Node)) setOpen(false)
    }
    const key = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false)
    }
    document.addEventListener('pointerdown', outside)
    document.addEventListener('keydown', key)
    return () => {
      active = false
      document.removeEventListener('pointerdown', outside)
      document.removeEventListener('keydown', key)
    }
  }, [open])

  const matches = catalogue.length ? catalogue.filter(item => !search.trim() || item.name.toLocaleLowerCase('ru').includes(search.trim().toLocaleLowerCase('ru')) || item.emoji === search.trim()) : emojis.map(emoji => ({ emoji, name: emoji, key: emoji }))

  function insert(emoji: string) {
    const input = inputRef.current
    const start = input?.selectionStart ?? value.length
    const end = input?.selectionEnd ?? start
    const next = value.slice(0, start) + emoji + value.slice(end)
    if (input && input.maxLength > 0 && next.length > input.maxLength) return
    onChange(next)
    setOpen(false)
    requestAnimationFrame(() => {
      input?.focus({ preventScroll: true })
      input?.setSelectionRange(start + emoji.length, start + emoji.length)
    })
  }

  return <div className="z-emoji-picker" ref={root}>
    <button type="button" className="z-emoji-toggle" aria-label={open ? 'Закрыть смайлики' : 'Добавить смайлик'} aria-expanded={open} title="Добавить смайлик" disabled={disabled} onClick={() => setOpen(current => !current)}>
      {open ? <X size={18}/> : <Smile size={19}/>}
    </button>
    {open && <div className="z-emoji-panel" role="group" aria-label="Выберите смайлик">
      <strong>Смайлики Apple{catalogue.length ? ` · ${catalogue.length}` : ''}</strong>
      <input aria-label="Найти смайлик" placeholder="Поиск смайликов…" value={search} onChange={event => { setSearch(event.target.value); setLimit(84) }} onKeyDown={event => { if (event.key === 'Enter') event.preventDefault() }}/>
      <div className="z-emoji-grid">{matches.slice(0, limit).map(({ emoji, name, key }) => <button type="button" key={key} aria-label={`Вставить ${emoji}`} title={name} onPointerDown={event => { if (event.pointerType === 'mouse') event.preventDefault() }} onClick={() => insert(emoji)}><Emoji emoji={emoji}/></button>)}</div>
      {matches.length > limit && <button type="button" className="z-emoji-more" onClick={() => setLimit(value => value + 84)}>Ещё смайлики</button>}
      {!matches.length && <small>Ничего не найдено.</small>}
      {loadError && <small role="status">Полный набор не загрузился. Закройте и откройте панель ещё раз.</small>}
    </div>}
  </div>
}
