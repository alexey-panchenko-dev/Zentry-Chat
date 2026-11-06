import { useQuery } from '@tanstack/react-query'
import { api } from '@/api'
import { mediaCapability } from './media-capabilities'
import type { MediaFeature } from './media-capabilities'
export function useMediaAvailability(feature:MediaFeature) {
 const config=useQuery({
  queryKey:['media-capabilities'],
  queryFn:async()=> (await api.get<unknown>('/realtime/config')).data,
  retry:false,
  staleTime: 5 * 60_000,
 })
 return {...config,supported:mediaCapability(config.data,feature)}
}
