import { useEffect, useRef, useState } from 'react'
import { X } from 'lucide-react'
import { isChatImageURL } from './image-draft'
export function ImageMessage({url,onOpenChange}:{url?:string;onOpenChange?:(open:boolean)=>void}) {
 const [open,setOpen]=useState(false)
 const [failed,setFailed]=useState(false)
 const [attempt,setAttempt]=useState(0)
 useEffect(()=>{if(open){onOpenChange?.(true);return()=>onOpenChange?.(false)}},[open,onOpenChange])
 if(!isChatImageURL(url))return <p className="notice">Фото недоступно.</p>
 return <div className="z-image-message">{failed?<div role="alert"><p>Не удалось загрузить фото.</p><button type="button" className="secondary" onClick={()=>{setFailed(false);setAttempt(value=>value+1)}}>Повторить</button></div>:<button type="button" className="z-message-image-button" aria-label="Увеличить фото в сообщении" onClick={()=>setOpen(true)}><img key={attempt} src={url} alt="Фото в сообщении" loading="lazy" onError={()=>setFailed(true)}/></button>}{open&&<ImageViewer url={url} onClose={()=>setOpen(false)}/>}</div>
}
function ImageViewer({url,onClose}:{url:string;onClose:()=>void}) {
 const dialog=useRef<HTMLDialogElement>(null)
 const [failed,setFailed]=useState(false)
 useEffect(()=>{const element=dialog.current;element?.showModal();return()=>element?.close()},[])
 return <dialog ref={dialog} className="z-chat-image-dialog" aria-label="Фото в сообщении" onCancel={event=>{event.preventDefault();onClose()}}><button type="button" className="z-icon z-close-image" aria-label="Закрыть фото" autoFocus onClick={onClose}><X/></button>{failed?<p className="notice" role="alert">Не удалось загрузить фото. Закройте просмотр и повторите.</p>:<img src={url} alt="Фото в сообщении в полном размере" onError={()=>setFailed(true)}/>}</dialog>
}
