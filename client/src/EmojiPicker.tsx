import { useEffect, useRef, useState } from 'react'
import { Smile, X, Grid2X2, Heart, ThumbsUp, PartyPopper, PawPrint, Apple, Plane, Shapes, Flag } from 'lucide-react'
import { Emoji } from './Emoji'

const emojis = ['😀','😃','😄','😁','😆','😅','😂','🙂','😉','😊','😍','🥰','😘','😎','🤔','🙃','😭','😡','👍','👎','❤️','🔥','🎉','🙏','🫶','👏','💯','✨']

export function EmojiPicker({ value, onChange, inputRef, disabled = false }: {
  value: string
  onChange: (value: string) => void
  inputRef: React.RefObject<HTMLTextAreaElement | null>
  disabled?: boolean
}) {
  const [open, setOpen] = useState(false)
  const [usage, setUsage] = useState<Record<string, number>>(() => {
    try { const saved = JSON.parse(localStorage.getItem('zently-emoji-usage') || '{}'); return saved && typeof saved === 'object' && !Array.isArray(saved) ? saved : {} } catch { return {} }
  })
  const [catalogue, setCatalogue] = useState<{ emoji: string; name: string; key: string }[]>([])
  const [category,setCategory]=useState('all')
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

  const categories=[['all','Все',Grid2X2],['серд|любов|поцел','Сердечки',Heart],['рук|пальц|ладон|кулак|жест','Жесты',ThumbsUp],['празд|конфетти|подар|салют|вечерин','Праздники',PartyPopper],['сме|улыб|лицо|плач|груст|зли|счаст|смущ|испуг','Лица',Smile],['кот|собак|цвет|дерев|птиц|живот|растен','Природа',PawPrint],['еда|фрукт|яблок|торт|хлеб|мяс|напит|овощ','Еда',Apple],['машин|автомоб|самолет|поезд|город|дом|кораб','Транспорт',Plane],['знак|символ|кнопка|инструмент|предмет','Символы',Shapes],['флаг','Флаги',Flag]] as const
  const matches = catalogue.length ? catalogue.filter(item => (category==='all'||!!search.trim()||new RegExp(category,'i').test(item.name))).filter(item => !search.trim() || item.name.toLocaleLowerCase('ru').includes(search.trim().toLocaleLowerCase('ru')) || item.emoji === search.trim()) : emojis.map(emoji => ({ emoji, name: emoji, key: emoji }))

  function insert(emoji: string) {
    const input = inputRef.current
    const start = input?.selectionStart ?? value.length
    const end = input?.selectionEnd ?? start
    const next = value.slice(0, start) + emoji + value.slice(end)
    if (input && input.maxLength > 0 && next.length > input.maxLength) return
    onChange(next)
    const updated = { ...usage, [emoji]: (Number(usage[emoji]) || 0) + 1 }
    setUsage(updated)
    try { localStorage.setItem('zently-emoji-usage', JSON.stringify(updated)) } catch { /* Works without persistent storage. */ }
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
    {open && <div className="z-emoji-panel" style={{right:root.current?root.current.getBoundingClientRect().right-(root.current.closest('.z-workspace')?.getBoundingClientRect().right??root.current.getBoundingClientRect().right)+16:16}} role="group" aria-label="Выберите смайлик">
      <nav className="z-emoji-categories" aria-label="Категории смайликов">{categories.map(([id,label,Icon])=><button type="button" key={id} title={label} aria-label={label} aria-pressed={category===id} onClick={()=>{setCategory(id);setSearch('');setLimit(84);root.current?.querySelector('.z-emoji-panel>.z-emoji-grid')?.scrollTo({top:0})}}><Icon size={18}/></button>)}</nav>
      <input aria-label="Найти смайлик" placeholder="Поиск смайликов…" value={search} onChange={event => { setSearch(event.target.value); setLimit(84) }} onKeyDown={event => { if (event.key === 'Enter') event.preventDefault() }}/>
      {!search.trim() && category==='all' && <section className="z-emoji-frequent" aria-label="Часто используемые"><small>Часто используемые</small><div className="z-emoji-grid">{[...new Set([...Object.keys(usage).sort((a,b)=>(Number(usage[b])||0)-(Number(usage[a])||0)),...emojis])].slice(0,14).map(emoji=><button type="button" key={emoji} aria-label={`Вставить ${emoji}`} onPointerDown={event=>{if(event.pointerType==='mouse')event.preventDefault()}} onClick={()=>insert(emoji)}><Emoji emoji={emoji}/></button>)}</div></section>}
      <div className="z-emoji-grid" onScroll={event => {
        const grid = event.currentTarget
        if (grid.scrollHeight - grid.scrollTop - grid.clientHeight < 100 && limit < matches.length) {
          setLimit(current => Math.min(matches.length, current + 84))
        }
      }}>{matches.slice(0, limit).map(({ emoji, name, key }) => <button type="button" key={key} aria-label={`Вставить ${emoji}`} title={name} onPointerDown={event => { if (event.pointerType === 'mouse') event.preventDefault() }} onClick={() => insert(emoji)}><Emoji emoji={emoji}/></button>)}</div>
      {!matches.length && <small>Ничего не найдено.</small>}
      {loadError && <small role="status">Полный набор не загрузился. Закройте и откройте панель ещё раз.</small>}
    </div>}
  </div>
}
