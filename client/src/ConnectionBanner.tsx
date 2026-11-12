import { useEffect, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { api } from './api'

export function ConnectionBanner() {
 const [connecting,setConnecting]=useState(!navigator.onLine)
 const cache=useQueryClient()
 useEffect(()=>{
  const offline=()=>setConnecting(true)
  const online=()=>{setConnecting(false);void cache.invalidateQueries({predicate:query=>query.isActive()})}
  window.addEventListener('offline',offline);window.addEventListener('online',online)
  const interceptor=api.interceptors.response.use(response=>{setConnecting(false);return response},error=>{if(!error.response&&error.code!=='ERR_CANCELED')setConnecting(true);return Promise.reject(error)})
  return()=>{window.removeEventListener('offline',offline);window.removeEventListener('online',online);api.interceptors.response.eject(interceptor)}
 },[cache])
 return connecting?<div className="z-connection-banner" role="status">Подключаемся…</div>:null
}
