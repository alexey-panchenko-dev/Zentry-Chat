import type { ReactNode } from 'react'
import type { MediaFeature } from './media-capabilities'
import { useMediaAvailability } from './useMediaAvailability'
export function MediaAvailability({feature,children}:{feature:MediaFeature;children:ReactNode}) {
 const config=useMediaAvailability(feature)
 if(config.supported)return <>{children}</>
 return <span className="z-media-unavailable" role="status">{config.isPending?'Проверяем доступность…':feature==='voice'?'Голосовые сообщения пока недоступны.':feature==='profile_photos'?'Фото профиля пока недоступно.':feature==='chat_images'?'Отправка фото пока недоступна.':'Контактные данные пока недоступны.'}</span>
}
