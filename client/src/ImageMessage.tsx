import { useEffect, useRef, useState } from 'react'
import type { CSSProperties } from 'react'
import { ChevronLeft, ChevronRight, X } from 'lucide-react'
import { isChatImageURL } from './image-draft'
export function ImageMessage({url,onOpenChange,gallery=[],index=0,thumbnail:_thumbnail=false}:{url?:string;onOpenChange?:(open:boolean)=>void;gallery?:string[];index?:number;thumbnail?:boolean}) {
 const [open,setOpen]=useState(false)
 const [failedURL,setFailedURL]=useState<string|null>(null)
 const [attempt,setAttempt]=useState(0)
 const [ratio,setRatio]=useState(1)
 useEffect(()=>{if(open){onOpenChange?.(true);return()=>onOpenChange?.(false)}},[open,onOpenChange])
 if(!isChatImageURL(url))return <p className="notice">Фото недоступно.</p>
 const viewerGallery=gallery.filter(isChatImageURL)
 const viewerIndex=viewerGallery.indexOf(url)
 return <div className="z-image-message" style={{'--photo-ratio':ratio} as CSSProperties}>{failedURL===url?<div role="alert"><p>Не удалось загрузить фото.</p><button type="button" className="secondary" onClick={()=>{setFailedURL(null);setAttempt(value=>value+1)}}>Повторить</button></div>:<button type="button" className="z-message-image-button" aria-label="Увеличить фото в сообщении" onClick={()=>setOpen(true)}><img key={attempt} src={url+(url.includes("?")?"&":"?")+"preview=1"} alt="Фото в сообщении" loading="lazy" decoding="async" fetchPriority="low" onLoad={event=>setRatio(event.currentTarget.naturalWidth/event.currentTarget.naturalHeight)} onError={()=>setFailedURL(url)}/></button>}{open&&<ImageViewer url={url} gallery={viewerGallery.length?viewerGallery:[url]} index={viewerIndex>=0?viewerIndex:index} onClose={()=>setOpen(false)}/>}</div>
}
function ImageViewer({url,gallery,index,onClose}:{url:string;gallery:string[];index:number;onClose:()=>void}) {
 const dialog=useRef<HTMLDialogElement>(null)
 const [failedURL,setFailedURL]=useState<string|null>(null)
 const [activeIndex,setActiveIndex]=useState(index)
 const [slideDirection,setSlideDirection]=useState<'next'|'previous'>('next')
 const touchStart=useRef<number|null>(null)
 const activeURL=gallery[activeIndex]||url
 const showPhoto=(nextIndex:number)=>{
  if(nextIndex<0||nextIndex>=gallery.length||nextIndex===activeIndex)return
  setSlideDirection(nextIndex>activeIndex?'next':'previous')
  setFailedURL(null)
  setActiveIndex(nextIndex)
 }
 useEffect(()=>{const element=dialog.current;element?.showModal();return()=>element?.close()},[])
 useEffect(()=>{const key=(event:KeyboardEvent)=>{if(event.key==='ArrowLeft'&&activeIndex>0){event.preventDefault();setSlideDirection('previous');setFailedURL(null);setActiveIndex(activeIndex-1)}if(event.key==='ArrowRight'&&activeIndex<gallery.length-1){event.preventDefault();setSlideDirection('next');setFailedURL(null);setActiveIndex(activeIndex+1)}};window.addEventListener('keydown',key);return()=>window.removeEventListener('keydown',key)},[activeIndex,gallery.length])
 const swipe=(end:number)=>{const start=touchStart.current;touchStart.current=null;if(start===null)return;const delta=end-start;if(Math.abs(delta)<45)return;if(delta<0)showPhoto(activeIndex+1);if(delta>0)showPhoto(activeIndex-1)}
 return <dialog ref={dialog} className="z-chat-image-dialog" aria-label="Фото в сообщении" onClick={event=>{if(event.target===event.currentTarget)onClose()}} onCancel={event=>{event.preventDefault();onClose()}}><button type="button" className="z-icon z-close-image" aria-label="Закрыть фото" autoFocus onClick={onClose}><X/></button>{failedURL===activeURL?<p className="notice" role="alert">Не удалось загрузить фото. Закройте просмотр и повторите.</p>:<div className="z-image-viewer" onClick={event=>{if(event.target===event.currentTarget)onClose()}} onTouchStart={event=>{touchStart.current=event.touches[0]?.clientX??null}} onTouchEnd={event=>swipe(event.changedTouches[0]?.clientX??0)}><img key={activeIndex} className={`z-gallery-photo z-gallery-photo--${slideDirection}`} src={activeURL} alt="Фото в сообщении в полном размере" onError={()=>setFailedURL(activeURL)}/>{gallery.length>1&&<><button type="button" className="z-icon z-photo-arrow is-previous" aria-label="Предыдущее фото" disabled={activeIndex===0} onClick={()=>showPhoto(activeIndex-1)}><ChevronLeft/></button><button type="button" className="z-icon z-photo-arrow is-next" aria-label="Следующее фото" disabled={activeIndex===gallery.length-1} onClick={()=>showPhoto(activeIndex+1)}><ChevronRight/></button><span className="z-photo-count" aria-live="polite">{activeIndex+1} / {gallery.length}</span></>}</div>}</dialog>
}
