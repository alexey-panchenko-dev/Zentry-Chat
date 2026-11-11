import type Pusher from 'pusher-js'
import { api, errorText } from '@/api'
import { getRealtimeConfig } from './realtime-config.ts'

type Hub = {
  users: number; client?: Pusher; connecting: boolean; lastAttempt: number
  channels: Map<string, number>; listeners: Set<(client?: Pusher) => void>
  retry?: ReturnType<typeof setTimeout>; removeFocus?: () => void
}
let hub: Hub | undefined

async function connect(current: Hub) {
  if (!current.users || current.client || current.connecting) return
  current.connecting = true
  current.lastAttempt = Date.now()
  try {
    const data = await getRealtimeConfig()
    if (!current.users) return
    if (!data.enabled) { for (const listener of current.listeners) listener(); return }
    const { default: Pusher } = await import('pusher-js')
    if (!current.users) return
    current.client = new Pusher(data.key, {
      cluster: data.cluster, forceTLS: true,
      channelAuthorization: { customHandler: ({ socketId, channelName }, callback) => {
        void api.post('/pusher/auth', { socket_id: socketId, channel_name: channelName })
          .then(({ data }) => callback(null, data))
          .catch((error: unknown) => callback(new Error(errorText(error)), null))
      } },
    })
    for (const listener of current.listeners) listener(current.client)
  } catch {
    for (const listener of current.listeners) listener()
    // A sleeping API may time out on the first config read. Retry only while
    // the app is visible; focus also recovers it, without polling a healthy API.
    if (current.users) current.retry = setTimeout(() => {
      if (typeof document === 'undefined' || document.visibilityState !== 'hidden') void connect(current)
    }, 60_000)
  } finally { current.connecting = false }
}

// All realtime features share one WebSocket, config read, and private subscription.
export function subscribeRealtime(name: string, events: Record<string, (data: any) => void>, status: (ready: boolean) => void = () => {}) {
  if (!hub) {
    const next: Hub = {users:0, connecting:false, lastAttempt:0, channels:new Map(), listeners:new Set()}
    if (typeof window !== 'undefined') {
      const recover = () => { if (Date.now()-next.lastAttempt >= 60_000) { clearTimeout(next.retry); void connect(next) } }
      window.addEventListener('focus', recover)
      next.removeFocus = () => window.removeEventListener('focus', recover)
    }
    hub = next
  }
  const current = hub
  current.users++
  let disposed = false
  let detach = () => {}
  const attach = (client?: Pusher) => {
    if (disposed) return
    if (!client) { status(false); return }
    current.channels.set(name, (current.channels.get(name) ?? 0) + 1)
    const channel = client.subscribe(name)
    const subscribed = () => { if (!disposed) status(true) }
    const unavailable = () => { if (!disposed) status(false) }
    const stateChange = ({ current }: { current: string }) => { if (current !== 'connected') unavailable() }
    const wrapped = Object.entries(events).map(([event, handler]) => {
      const guarded = (data: unknown) => { if (!disposed) handler(data) }
      channel.bind(event, guarded)
      return [event, guarded] as const
    })
    channel.bind('pusher:subscription_succeeded', subscribed)
    channel.bind('pusher:subscription_error', unavailable)
    client.connection.bind('state_change', stateChange)
    if (channel.subscribed) subscribed()
    detach = () => {
      for (const [event, handler] of wrapped) channel.unbind(event, handler)
      channel.unbind('pusher:subscription_succeeded', subscribed)
      channel.unbind('pusher:subscription_error', unavailable)
      client.connection.unbind('state_change', stateChange)
      const remaining = (current.channels.get(name) ?? 1) - 1
      if (remaining) current.channels.set(name, remaining)
      else { current.channels.delete(name); client.unsubscribe(name) }
    }
  }
  current.listeners.add(attach)
  if (current.client) attach(current.client)
  else void connect(current)
  return () => {
    if (disposed) return
    disposed = true
    current.listeners.delete(attach)
    detach()
    if (--current.users === 0) {
      clearTimeout(current.retry)
      current.removeFocus?.()
      current.client?.disconnect()
      if (hub === current) hub = undefined
    }
  }
}
