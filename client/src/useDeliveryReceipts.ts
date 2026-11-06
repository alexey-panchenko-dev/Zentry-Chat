import { useEffect } from 'react'
import { api } from './api'
import type { Message } from './api'
// Delivery is acknowledged only after this browser receives the message batch.
// Reading is separate and remains gated by the focused chat viewport.
export function useDeliveryReceipts(userID:string) {
 useEffect(()=>{
  const abort=new AbortController();let disposed=false;const timer:{value:ReturnType<typeof setTimeout>|undefined}={value:undefined}
  async function sync(){
   let delay=5000
   try{const {messages}=(await api.get<{messages:Message[]}>('/delivery',{signal:abort.signal})).data
    if(!disposed&&messages.length){await api.post('/delivery',{message_ids:messages.map(message=>message.id)},{signal:abort.signal});if(messages.length===100)delay=100}
   }catch{delay=15000}
   if(!disposed)timer.value=setTimeout(()=>void sync(),delay)
  }
  void sync();return()=>{disposed=true;abort.abort();clearTimeout(timer.value)}
 },[userID])
}
