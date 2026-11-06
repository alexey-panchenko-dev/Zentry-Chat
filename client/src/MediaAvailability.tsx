import type { ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api'
import { mediaCapability } from './media-capabilities'
import type { MediaFeature } from './media-capabilities'
export function MediaAvailability({feature,children}:{feature:MediaFeature;children:ReactNode}) {
 const config=useQuery({
  queryKey:['media-capabilities'],
  queryFn:async()=> (await api.get<unknown>('/realtime/config')).data,
  retry:false,
  staleTime: 5 * 60_000,
 })
 if(mediaCapability(config.data,feature))return <>{children}</>
 return <span className="z-media-unavailable" role="status">{config.isPending?'Проверяем доступность…':feature==='voice'?'Голосовые сообщения пока недоступны.':feature==='profile_photos'?'Фото профиля пока недоступно.':'Контактные данные пока недоступны.'}</span>
}
