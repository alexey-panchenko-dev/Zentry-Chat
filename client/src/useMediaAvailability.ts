import { getRealtimeConfig } from './realtime-config'
import { useQuery } from '@tanstack/react-query'
import { mediaCapability } from './media-capabilities'
import type { MediaFeature } from './media-capabilities'
export function useMediaAvailability(feature:MediaFeature) {
 const config=useQuery({
  queryKey:['media-capabilities'],
  queryFn:getRealtimeConfig,
  retry:false,
  staleTime: 5 * 60_000,
 })
 return {...config,supported:mediaCapability(config.data,feature)}
}
