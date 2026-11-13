import { useState } from 'react'
import type { User } from '@/api'
export function Brand(){return <span className="z-brand"><img src="/assets/logo.svg" alt=""/><span>zently</span></span>}
export function Avatar({user,large=false,loading='eager'}:{user:User;large?:boolean;loading?:'eager'|'lazy'}) {
 const [failed,setFailed]=useState('')
 const account=(user as User & {username?:string}).username || user.name
 const initials=Array.from(account.replace(/^@/,'')).slice(0,2).join('').toLocaleUpperCase('ru')
 const baseURL=user.avatar_url?.startsWith('/api/profiles/')?user.avatar_url:''
 const url=baseURL?`${baseURL}${baseURL.includes('?')?'&':'?'}size=${large?512:128}`:''
 return <span className={'z-avatar'+(large?' is-large':'')} aria-hidden="true">{url&&failed!==url?<><span className="z-avatar-fallback">{initials}</span><img src={url} alt="" loading={loading} decoding="async" onError={()=>setFailed(url)}/></>:initials}</span>
}
