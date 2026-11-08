import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { InfiniteData } from '@tanstack/react-query'
import { api } from '@/api'
import type { Conversation, MessagePage } from '@/api'
import { mediaCapability } from './media-capabilities'
import { messageVisible, removalCommitted, withoutConversation } from './conversation-controls'
import type { ConversationControls, ControlAction } from './conversation-controls'
export function useConversationControls(conversationID?:string,realtimeReady=true) {
 const cache=useQueryClient()
 const capability=useQuery({queryKey:['media-capabilities'],queryFn:async()=>(await api.get<unknown>('/realtime/config')).data,retry:false,staleTime:5*60_000})
 const supported=mediaCapability(capability.data,'chat_controls')
 const removalSupported=mediaCapability(capability.data,'chat_removal')
 const query=useQuery({queryKey:['chat-controls',conversationID],enabled:supported&&!!conversationID,queryFn:async({signal})=>(await api.get<ConversationControls>(`/conversations/${conversationID}/controls`,{signal})).data,retry:false,refetchInterval:supported&&!realtimeReady?60_000:false})
 const action=useMutation({mutationFn:async({value,id}:{value:ControlAction;id:string})=>(await api.post<ConversationControls>(`/conversations/${id}/controls`,{action:value})).data,onSuccess:async(state,{value,id})=>{
  await cache.cancelQueries({queryKey:['chat-controls',id]})
  cache.setQueryData(['chat-controls',id],state)
  if(value==='clear'||value==='delete_chat'||value==='delete_everyone'){
   await cache.cancelQueries({queryKey:['messages',id]})
   cache.setQueryData<InfiniteData<MessagePage>>(['messages',id],old=>old?{...old,pages:old.pages.map(page=>({...page,messages:page.messages.filter(message=>messageVisible(message.id,state.cleared_through))}))}:old)
  }
  if(removalCommitted(value,state)){
   await cache.cancelQueries({queryKey:['conversations']})
   cache.setQueryData<InfiniteData<{conversations:Conversation[];next_cursor:string}>>(['conversations'],old=>old?{...old,pages:withoutConversation(old.pages,id)}:old)
  }
  for(const key of [['messages',id],['read-state',id],['conversations'],['unread'],['discovery']])void cache.invalidateQueries({queryKey:key})
 }})
 const blocked=!!query.data&&(query.data.blocked_by_me||query.data.blocked_me)
 return {supported,removalSupported,state:query.data,blocked,unavailable:supported&&(!query.data||query.isError),loading:supported&&query.isPending,error:query.error,refetch:query.refetch,action}
}
export type ConversationControlView=ReturnType<typeof useConversationControls>
