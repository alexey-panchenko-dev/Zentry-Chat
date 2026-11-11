import { api } from '@/api'

export type RealtimeConfig = { enabled: boolean; key: string; cluster: string; media?: Record<string, boolean> }
let pending: Promise<RealtimeConfig> | undefined

// Capabilities and realtime subscribers often start in the same render. Share
// their in-flight read, without keeping account data between sign-ins.
export function getRealtimeConfig(): Promise<RealtimeConfig> {
  if (!pending) {
    pending = api.get<RealtimeConfig>('/realtime/config').then(response => response.data)
      .finally(() => { pending = undefined })
  }
  return pending
}
