import { useEffect, useRef } from 'react'
import type { RefObject } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { api } from '@/api'
import { observeReadReceipt } from './read-visibility.ts'

// Only acknowledge the latest rendered message when its marker is actually in
// the viewport of a focused, visible conversation. Loading history is not a read.
export function useReadReceipt(id: string, latest: string | undefined, visible: boolean, marker: RefObject<HTMLDivElement | null>) {
  const cache = useQueryClient()
  const acknowledged = useRef({value:'0'})
  useEffect(() => {
    if (!visible || !latest || !marker.current) return
    return observeReadReceipt(latest, marker.current, acknowledged.current,
      latest => api.post(`/conversations/${id}/read`, {last_message_id:latest}),
      () => {void cache.invalidateQueries({queryKey:['conversations']});void cache.invalidateQueries({queryKey:['unread']})})
  }, [id,latest,visible,marker,cache])
}
