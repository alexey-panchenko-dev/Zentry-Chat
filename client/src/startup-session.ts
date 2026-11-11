import type { User } from './api'

let pending: Promise<User | null> | undefined

// Start authentication while the browser is still downloading React and the UI.
// Only this page's in-memory promise is reused; private data is never persisted.
export function startSessionRequest() {
  if (!pending) {
    pending = fetch('/api/auth/me', { credentials: 'same-origin', cache: 'no-store', signal: AbortSignal.timeout(15_000) })
      .then(async response => {
        if (response.status === 401) return null
        if (!response.ok) throw new Error('Не удалось проверить вход. Попробуйте ещё раз.')
        return ((await response.json()) as { user: User }).user
      })
    void pending.catch(() => {})
  }
  return pending
}

export function takeStartupSession() {
  const request = pending
  pending = undefined
  return request
}
