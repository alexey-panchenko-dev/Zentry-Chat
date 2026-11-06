type ContextLease = { context: AudioContext; release: () => void }
let shared: {context: AudioContext; users: number} | undefined

export function acquireCallAudioContext(): ContextLease | undefined {
  const Context = globalThis.AudioContext ?? (globalThis as typeof globalThis & {webkitAudioContext?:typeof AudioContext}).webkitAudioContext
  if (!Context) return undefined
  if (!shared || shared.context.state === 'closed') shared = {context:new Context(),users:0}
  const current = shared
  current.users++
  let released = false
  return {context:current.context, release:() => {
    if (released) return
    released = true
    if (--current.users === 0) {
      if (shared === current) shared = undefined
      void current.context.close().catch(() => {})
    }
  }}
}
