import { useState } from 'react'
import type { User } from '@/api'
export function Brand(){return <span className="z-brand"><img src="/assets/logo.svg" alt=""/><span>zently</span></span>}
export function Avatar({user,large=false}:{user:User;large?:boolean}) {
 const [failed,setFailed]=useState('')
 const account=(user as User & {username?:string}).username || user.name
 const url=user.avatar_url?.startsWith('/api/profiles/')?user.avatar_url:''
 return <span className={'z-avatar'+(large?' is-large':'')} aria-hidden="true">{url&&failed!==url?<img src={url} alt="" onError={()=>setFailed(url)}/>:Array.from(account.replace(/^@/,'')).slice(0,2).join('').toLocaleUpperCase('ru')}</span>
}
