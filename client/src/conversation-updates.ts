import { subscribeRealtime } from './realtime.ts'

export type ConversationUpdate = { conversation_id?: string; reason: 'conversation'|'message'|'discovery'|'read'|'connected'|'group' }
export function watchConversations(userID: string, refresh: (data: ConversationUpdate) => void, status: (ready: boolean) => void = () => {}) {
  let lastRecovery = 0
  const recover = () => {
    if (typeof document !== 'undefined' && document.visibilityState === 'hidden') return
    if (Date.now()-lastRecovery < 1000) return
    lastRecovery = Date.now()
    refresh({reason:'connected'})
  }
  if (typeof window !== 'undefined') window.addEventListener('focus',recover)
  if (typeof document !== 'undefined') document.addEventListener('visibilitychange',recover)
  const stop = subscribeRealtime(`private-user-${userID}`, {
    'conversation.updated': data => refresh({...data,reason:'conversation'}),
    'conversation.message': data => refresh({...data,reason:'message'}),
    'discovery.updated': () => refresh({reason:'discovery'}),
    'conversation.read': data => refresh({...data,reason:'read'}),
    'group.updated': data => refresh({...data,reason:'group'}),
  }, ready => { status(ready); if (ready) refresh({reason:'connected'}) })
  return () => {
    stop()
    if(typeof window !== 'undefined') window.removeEventListener('focus',recover)
    if(typeof document !== 'undefined') document.removeEventListener('visibilitychange',recover)
  }
}
