import { useEffect, useRef } from 'react'
import type { PointerEvent, WheelEvent } from 'react'
import { joinBirthDate, limitBirthInput, normalizeBirthInput, splitBirthDate, stepBirthPart } from './birth-wheel'
import type { BirthPart } from './birth-wheel'

const order: BirthPart[] = ['day', 'month', 'year']
const labels: Record<BirthPart, string> = { day: 'День', month: 'Месяц', year: 'Год' }
const widths: Record<BirthPart, number> = { day: 2, month: 2, year: 4 }
const currentYear = new Date().getFullYear()
const maximumBirthYear = currentYear - 14

type WheelDrag = { part: BirthPart; y: number; remainder: number }

export function BirthDateWheel({ value, onChange, invalid = false, describedBy }: {
  value: string
  onChange: (value: string) => void
  invalid?: boolean
  describedBy?: string
}) {
  const parts = splitBirthDate(value)
  const latestParts = useRef(parts)
  const inputs = useRef<Record<BirthPart, HTMLInputElement | null>>({ day: null, month: null, year: null })
  const columns = useRef<Record<BirthPart, HTMLDivElement | null>>({ day: null, month: null, year: null })
  const pointers = useRef<Record<number, WheelDrag>>({})
  const wheelRemainders = useRef<Record<BirthPart, number>>({ day: 0, month: 0, year: 0 })
  const animationTimers = useRef<Record<BirthPart, ReturnType<typeof setTimeout> | null>>({ day: null, month: null, year: null })

  useEffect(() => { latestParts.current = parts }, [parts])
  useEffect(() => () => {
    for (const timer of Object.values(animationTimers.current)) if (timer) clearTimeout(timer)
  }, [])

  function setPart(part: BirthPart, next: string) {
    latestParts.current = { ...latestParts.current, [part]: next }
    onChange(joinBirthDate(latestParts.current))
  }

  function animate(part: BirthPart, direction: 1 | -1) {
    const column = columns.current[part]
    if (!column) return
    column.classList.remove('is-spinning-up', 'is-spinning-down')
    void column.offsetWidth
    column.classList.add(direction > 0 ? 'is-spinning-up' : 'is-spinning-down')
    const previous = animationTimers.current[part]
    if (previous) clearTimeout(previous)
    animationTimers.current[part] = setTimeout(() => {
      column.classList.remove('is-spinning-up', 'is-spinning-down')
      animationTimers.current[part] = null
    }, 420)
  }

  function step(part: BirthPart, direction: 1 | -1) {
    const next = stepBirthPart(latestParts.current[part], part, direction, currentYear)
    setPart(part, part === 'year' && Number(next) > maximumBirthYear ? String(maximumBirthYear) : next)
    animate(part, direction)
  }

  function type(part: BirthPart, text: string) {
    const limited = limitBirthInput(text, part, currentYear)
    const next = part === 'year' && limited.length === widths.year && Number(limited) > maximumBirthYear ? String(maximumBirthYear) : limited
    setPart(part, next)
    if (next.length === widths[part]) {
      const index = order.indexOf(part)
      if (index < order.length - 1) inputs.current[order[index + 1]]?.focus()
    }
  }

  function wheel(event: WheelEvent<HTMLDivElement>, part: BirthPart) {
    event.preventDefault()
    wheelRemainders.current[part] += event.deltaY
    while (Math.abs(wheelRemainders.current[part]) >= 28) {
      const direction = wheelRemainders.current[part] < 0 ? 1 : -1
      step(part, direction)
      wheelRemainders.current[part] += direction > 0 ? 28 : -28
    }
  }

  function pointerDown(event: PointerEvent<HTMLDivElement>, part: BirthPart) {
    // Keep mouse input text selection intact; use the drum surface for mouse drags.
    if (event.pointerType === 'mouse' && event.target instanceof HTMLInputElement) return
    pointers.current[event.pointerId] = { part, y: event.clientY, remainder: 0 }
    event.currentTarget.setPointerCapture(event.pointerId)
  }

  function pointerMove(event: PointerEvent<HTMLDivElement>) {
    const drag = pointers.current[event.pointerId]
    if (!drag) return
    const delta = drag.y - event.clientY
    drag.y = event.clientY
    drag.remainder += delta
    while (Math.abs(drag.remainder) >= 24) {
      const direction = drag.remainder > 0 ? 1 : -1
      step(drag.part, direction)
      drag.remainder -= direction * 24
    }
    if (Math.abs(delta) > 2) event.preventDefault()
  }

  function pointerUp(event: PointerEvent<HTMLDivElement>) {
    delete pointers.current[event.pointerId]
  }

  return <div className="z-birth-wheel" role="group" aria-label="Дата рождения: прокрутите день, месяц и год">
    <input type="hidden" name="birth_date" value={value} />
    {order.map(part => {
      const number = Number(parts[part])
      const maximum = part === 'day' ? 31 : part === 'month' ? 12 : maximumBirthYear
      const shown = parts[part]
        ? String(number).padStart(widths[part], '0')
        : part === 'year' ? 'ГГГГ' : part === 'day' ? 'ДД' : 'ММ'
      const before = parts[part] && (part !== 'year' || number > 1900)
        ? stepBirthPart(parts[part], part, -1, currentYear) : ''
      const after = parts[part] && (part !== 'year' || number < maximum)
        ? stepBirthPart(parts[part], part, 1, currentYear) : ''

      return <div
        key={part}
        ref={node => { columns.current[part] = node }}
        className={`z-birth-wheel-column z-birth-wheel-${part}`}
        onWheel={event => wheel(event, part)}
        onPointerDown={event => pointerDown(event, part)}
        onPointerMove={pointerMove}
        onPointerUp={pointerUp}
        onPointerCancel={pointerUp}
        aria-label={`${labels[part]}: прокрутите колесо или проведите пальцем`}
      >
        <span className="z-birth-wheel-label">{labels[part]}</span>
        <span aria-hidden="true" className="z-birth-wheel-neighbor">{before}</span>
        <input
          ref={node => { inputs.current[part] = node }}
          type="text"
          role="spinbutton"
          inputMode="numeric"
          autoComplete={part === 'day' ? 'bday-day' : part === 'month' ? 'bday-month' : 'bday-year'}
          aria-label={labels[part]}
          aria-valuemin={part === 'year' ? 1900 : 1}
          aria-valuemax={maximum}
          aria-valuenow={parts[part] ? number : undefined}
          aria-invalid={invalid || undefined}
          aria-describedby={describedBy}
          maxLength={widths[part]}
          value={parts[part] || shown}
          onFocus={event => event.currentTarget.select()}
          onBlur={event => setPart(part, normalizeBirthInput(event.currentTarget.value, part))}
          onChange={event => type(part, event.target.value)}
          onKeyDown={event => {
            if (event.key === 'ArrowUp') { event.preventDefault(); step(part, 1) }
            if (event.key === 'ArrowDown') { event.preventDefault(); step(part, -1) }
            if (event.key === 'Enter') {
              const next = order[order.indexOf(part) + 1]
              if (next) { event.preventDefault(); inputs.current[next]?.focus() }
            }
          }}
        />
        <span aria-hidden="true" className="z-birth-wheel-neighbor">{after}</span>
      </div>
    })}
  </div>
}
