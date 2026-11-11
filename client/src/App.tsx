import { lazy, Suspense, useEffect, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api, errorText } from './api'
import type { User } from './api'
import { Brand } from './Visual'
import { takeStartupSession } from './startup-session'

const Chat = lazy(() => import('./Chat'))
const Auth = lazy(() => import('./Auth'))

export default function App() {
  const cache = useQueryClient()
  const [, refreshSessionView] = useState(0)
  const sessionViewChanged = () => refreshSessionView(value => value + 1)
  const session = useQuery({
    queryKey: ['session'],
    queryFn: async () => {
      try { return await (takeStartupSession() ?? api.get<{ user: User }>('/auth/me').then(response=>response.data.user)) }
      catch (error) {
        if ((error as { response?: { status: number } }).response?.status === 401) return null
        throw error
      }
    },
  })

  useEffect(() => {
    const interceptor = api.interceptors.response.use(undefined, (error) => {
      if (error.response?.status === 401 && !error.config?.url?.startsWith('/auth/')) {
        cache.clear()
        cache.setQueryData(['session'], null)
      }
      return Promise.reject(error)
    })
    return () => api.interceptors.response.eject(interceptor)
  }, [cache])

  return <div className={session.data ? 'z-root' : 'z-auth-page'}>
    {session.isPending ? <div className="z-loading" role="status"><Brand/><p>Открываем Zently…</p></div>
      : session.isError ? <div className="z-auth-card"><Brand/><p className="notice">{errorText(session.error)}</p><button className="secondary" onClick={() => void session.refetch()}>Повторить</button></div>
      : session.data ? <Suspense fallback={<div className="z-loading" role="status"><Brand/><p>Открываем чаты…</p></div>}><Chat key={session.data.id} user={session.data} onSessionViewChanged={sessionViewChanged}/></Suspense> : <Suspense fallback={<div className="z-loading" role="status"><Brand/><p>Открываем вход…</p></div>}><Auth/></Suspense>}
  </div>
}

