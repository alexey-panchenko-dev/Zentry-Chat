import { useLayoutEffect, useRef, useState } from 'react'
import type { PointerEvent as TouchEvent } from 'react'
import { messageDraftKey, readMessageDraft, replaceDraftRecord } from './message-drafts'

type Reply = { id: string; body: string }
type Draft = { body: string; reply: Reply | null }

export function useChatDraft(userID: string, chatID: string) {
  const key = messageDraftKey(userID, chatID) + ':v2'
  const storedRecord = useRef<string | null>(null)
  const [draft, setDraft] = useState<Draft>(() => {
    try {
      const raw = sessionStorage.getItem(key)
      storedRecord.current = raw
      if (raw) {
        const saved = JSON.parse(raw)
        return {
          body: typeof saved.body === 'string' ? saved.body.slice(0, 1000) : '',
          reply: typeof saved.replyID === 'string' && /^\d+$/.test(saved.replyID) ? { id: saved.replyID, body: '' } : null,
        }
      }
      return { body: readMessageDraft(localStorage, userID, chatID), reply: null }
    } catch { return { body: '', reply: null } }
  })
  const current = useRef(draft)
  function update(next: Draft) {
    current.current = next
    setDraft(next)
    try {
      const record = JSON.stringify({ body: next.body, replyID: next.reply?.id ?? null })
      if (replaceDraftRecord(sessionStorage, key, storedRecord.current, record)) {
        storedRecord.current = record
        localStorage.removeItem(messageDraftKey(userID, chatID))
      }
    } catch { /* Keep the draft in memory if storage is unavailable. */ }
  }
  return {
    ...draft,
    setBody: (body: string) => update({ ...current.current, body }),
    setReply: (reply: Reply | null) => update({ ...current.current, reply }),
    clearSent: (body: string, replyID?: string) => {
      if (current.current.body.trim() === body && current.current.reply?.id === replyID) update({ body: '', reply: null })
    },
  }
}

export function useComposerHeight(body: string) {
  const ref = useRef<HTMLTextAreaElement>(null)
  useLayoutEffect(() => {
    const input = ref.current
    if (!input) return
    const resize = () => {
      input.style.height = 'auto'
      input.style.height = Math.min(input.scrollHeight, 132) + 'px'
    }
    let width = 0
    let frame = 0
    const observer = new ResizeObserver(entries => {
      const nextWidth = entries[0]?.borderBoxSize?.[0]?.inlineSize ?? entries[0]?.contentRect.width ?? 0
      if (Math.abs(nextWidth - width) < 1) return
      width = nextWidth
      window.cancelAnimationFrame(frame)
      frame = window.requestAnimationFrame(resize)
    })
    if (input.parentElement) observer.observe(input.parentElement, { box: 'border-box' })
    resize()
    return () => { observer.disconnect(); window.cancelAnimationFrame(frame) }
  }, [body])
  return ref
}

export function useMessageGesture<T>(onReply: (message: T) => void) {
  const gesture = useRef<{ x: number; y: number; id: number; timer: number } | null>(null)
  function cancel() {
    if (gesture.current) window.clearTimeout(gesture.current.timer)
    gesture.current = null
  }
  useLayoutEffect(() => cancel, [])
  return (message: T) => ({
    onDoubleClick(event: React.MouseEvent<HTMLDivElement>) {
      if ((event.target as Element).closest("button,a,video,audio,input,textarea,summary")) return
      onReply(message)
    },
    onContextMenu(event: React.MouseEvent<HTMLDivElement>) {
      if ((event.target as Element).closest('input,textarea,a,video,audio')) return
      const menu = event.currentTarget.querySelector<HTMLDetailsElement>('.z-message-menu')
      if (menu) { event.preventDefault(); menu.open = true }
    },
    onPointerDown(event: TouchEvent<HTMLDivElement>) {
      if (event.pointerType !== 'touch' || !event.isPrimary || (event.target as Element).closest('button,a,video,audio,input,textarea,summary,.z-message-menu-items')) return
      cancel()
      const row = event.currentTarget
      const timer = window.setTimeout(() => {
        cancel()
        row.querySelector<HTMLDetailsElement>('.z-message-menu')?.setAttribute('open', '')
      }, 460)
      gesture.current = { x: event.clientX, y: event.clientY, id: event.pointerId, timer }
    },
    onPointerMove(event: TouchEvent<HTMLDivElement>) {
      const start = gesture.current
      if (!start || start.id !== event.pointerId) return
      const dx = event.clientX - start.x
      const dy = Math.abs(event.clientY - start.y)
      if (Math.abs(dx) > 10 || dy > 10) window.clearTimeout(start.timer)
      if (dy > 18 || dx > 16) cancel()
    },
    onPointerUp(event: TouchEvent<HTMLDivElement>) {
      const start = gesture.current
      if (!start || start.id !== event.pointerId) return
      cancel()
      if (start.x - event.clientX >= 54 && Math.abs(event.clientY - start.y) < 18) onReply(message)
    },
    onPointerCancel: cancel,
    onLostPointerCapture: cancel,
  })
}

export function useMessageJump<T extends { id: string }>(
  root: React.RefObject<HTMLElement | null>,
  load: () => Promise<{ data?: { pages: { messages: T[] }[] }; isError: boolean }>,
  hasMore: boolean,
) {
  const [target, setTarget] = useState('')
  const [status, setStatus] = useState('')
  const request = useRef(0)
  const appliedTarget = useRef('')
  useLayoutEffect(() => () => { request.current++ }, [])
  const find = (id: string) => root.current?.querySelector<HTMLElement>('[data-message-id="' + id + '"]')
  useLayoutEffect(() => {
    if (!target || target === appliedTarget.current) return
    const row = find(target)
    if (row) {
      row.scrollIntoView({ block: 'center' })
      appliedTarget.current = target
    }
  })
  async function jump(id: string) {
    if (!/^\d+$/.test(id)) return
    const version = ++request.current
    appliedTarget.current = ''
    const existing = find(id)
    if (existing) { existing.scrollIntoView({ block: 'center', behavior: 'smooth' }); setStatus(''); return }
    if (!hasMore) { setStatus('Сообщение больше не доступно.'); return }
    setStatus('Ищем сообщение в истории…')
    let cursor = ''
    try {
      while (version === request.current) {
        const result = await load()
        if (version !== request.current) return
        if (result.isError) throw new Error('history')
        const pages = result.data?.pages
        if (pages?.some(page => page.messages.some(message => message.id === id))) {
          setTarget(id)
          setStatus('')
          return
        }
        const next = pages?.at(-1)?.messages.at(-1)?.id
        if (!next || next === cursor || BigInt(next) < BigInt(id)) break
        cursor = next
      }
      setStatus('Сообщение больше не доступно.')
    } catch { if (version === request.current) setStatus('Не удалось загрузить историю. Повторите попытку.') }
  }
  return { jump, status }
}
