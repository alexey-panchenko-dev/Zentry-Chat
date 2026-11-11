import { useEffect, useId, useLayoutEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { ChevronDown, MoreHorizontal } from 'lucide-react'

/** A shared top-layer menu for chat rows, messages and attachments. */
export function ActionMenu({ label, children, className = '', icon, disabled = false }: {
  label: string
  children: ReactNode
  className?: string
  icon?: ReactNode
  disabled?: boolean
}) {
  const id = useId()
  const trigger = useRef<HTMLDetailsElement>(null)
  const popup = useRef<HTMLDivElement>(null)
  const [open, setOpen] = useState(false)
  const [position, setPosition] = useState({ left: 0, top: 0 })

  function close(restoreFocus = false) {
    if (trigger.current) trigger.current.open = false
    setOpen(false)
    if (restoreFocus) trigger.current?.querySelector('summary')?.focus()
  }

  useLayoutEffect(() => {
    if (!open || !popup.current || !trigger.current) return
    const anchor = trigger.current.getBoundingClientRect()
    const menu = popup.current.getBoundingClientRect()
    const viewport = window.visualViewport
    const left = viewport?.offsetLeft ?? 0
    const top = viewport?.offsetTop ?? 0
    const width = viewport?.width ?? window.innerWidth
    const height = viewport?.height ?? window.innerHeight
    const below = anchor.bottom + 6
    setPosition({
      left: Math.max(left + 8, Math.min(anchor.right - menu.width, left + width - menu.width - 8)),
      top: Math.max(top + 8, Math.min(below + menu.height > top + height - 8 ? anchor.top - menu.height - 6 : below, top + height - menu.height - 8)),
    })
    popup.current.querySelector<HTMLElement>('button:not(:disabled), input')?.focus({ preventScroll: true })
  }, [open])

  useEffect(() => {
    if (!open) return
    function outside(event: PointerEvent) {
      const target = event.target as Node
      if (!trigger.current?.contains(target) && !popup.current?.contains(target)) close()
    }
    function key(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        event.preventDefault()
        event.stopPropagation()
        close(true)
      }
    }
    function reposition(event: Event) {
      if (!popup.current?.contains(event.target as Node)) close()
    }
    document.addEventListener('pointerdown', outside)
    document.addEventListener('keydown', key, true)
    window.addEventListener('resize', reposition)
    document.addEventListener('scroll', reposition, true)
    return () => {
      document.removeEventListener('pointerdown', outside)
      document.removeEventListener('keydown', key, true)
      window.removeEventListener('resize', reposition)
      document.removeEventListener('scroll', reposition, true)
    }
  }, [open])

  return <details ref={trigger} className={'z-action-menu ' + className} onToggle={event => {
    if (disabled) { event.currentTarget.open = false; return }
    setOpen(event.currentTarget.open)
  }}>
    <summary role="button" tabIndex={disabled ? -1 : 0} aria-label={label} title={label} aria-expanded={open} aria-controls={id} aria-disabled={disabled}
      onClick={event => { if (disabled) event.preventDefault() }}>
      {icon ?? (className.includes('z-message-menu') ? <ChevronDown size={18}/> : <MoreHorizontal size={18}/>)}
    </summary>
    {open && createPortal(<div id={id} ref={popup} className="z-action-popup" role="group" aria-label={label}
      style={position} onClick={event => {
        if ((event.target as Element).closest('button:not(:disabled)')) {
          close()
          queueMicrotask(() => {
            if (document.activeElement === document.body) trigger.current?.querySelector('summary')?.focus({ preventScroll: true })
          })
        }
      }} onBlur={event => {
        if (event.relatedTarget && !event.currentTarget.contains(event.relatedTarget as Node) && !trigger.current?.contains(event.relatedTarget as Node)) close()
      }} onKeyDown={event => {
        if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
        const buttons = Array.from(event.currentTarget.querySelectorAll<HTMLButtonElement>('button:not(:disabled)'))
        const index = buttons.indexOf(document.activeElement as HTMLButtonElement)
        const next = event.key === 'Home' ? 0 : event.key === 'End' ? buttons.length - 1 :
          (index + (event.key === 'ArrowDown' ? 1 : -1) + buttons.length) % buttons.length
        event.preventDefault()
        buttons[next]?.focus()
      }}>{children}</div>, document.body)}
  </details>
}
