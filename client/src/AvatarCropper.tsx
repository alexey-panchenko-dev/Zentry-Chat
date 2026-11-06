import { useEffect, useRef, useState } from 'react'
import type { PointerEvent } from 'react'
import { X } from 'lucide-react'
import { errorText } from '@/api'
import { moveAvatarCrop, renderAvatarCrop, squareAvatarCrop } from './avatar-crop'
export function AvatarCropper({sourceURL,onApply,onCancel}:{sourceURL:string;onApply:(file:File)=>void;onCancel:()=>void}) {
 const dialog=useRef<HTMLDialogElement>(null)
 const image=useRef<HTMLImageElement>(null)
 const [dimensions,setDimensions]=useState<{width:number;height:number}|null>(null)
 const [position,setPosition]=useState({x:.5,y:.5})
 const [zoom,setZoom]=useState(1)
 const [error,setError]=useState('')
 const [pending,setPending]=useState(false)
 const mounted=useRef(false)
 const operation=useRef(0)
 const drag=useRef<{id:number;clientX:number;clientY:number;x:number;y:number;size:number}|null>(null)
 useEffect(()=>{const element=dialog.current;const photo=image.current;mounted.current=true;if(photo&&photo.getAttribute('src')!==sourceURL)photo.src=sourceURL;element?.showModal();return()=>{mounted.current=false;drag.current=null;if(photo)photo.src='';element?.close()}},[sourceURL])
 const crop=dimensions?squareAvatarCrop(dimensions.width,dimensions.height,zoom,position.x,position.y):null
 function cancel(){++operation.current;onCancel()}
 function pointerDown(event:PointerEvent<HTMLDivElement>){
  if(!dimensions||pending||event.button!==0)return
  event.preventDefault();event.currentTarget.setPointerCapture(event.pointerId)
  drag.current={id:event.pointerId,clientX:event.clientX,clientY:event.clientY,x:position.x,y:position.y,size:event.currentTarget.getBoundingClientRect().width}
 }
 function pointerMove(event:PointerEvent<HTMLDivElement>){
  const start=drag.current;if(!start||start.id!==event.pointerId||!dimensions||pending)return
  setPosition(moveAvatarCrop(dimensions.width,dimensions.height,zoom,start.x,start.y,event.clientX-start.clientX,event.clientY-start.clientY,start.size))
 }
 function pointerEnd(event:PointerEvent<HTMLDivElement>){if(drag.current?.id!==event.pointerId)return;drag.current=null;if(event.currentTarget.hasPointerCapture(event.pointerId))event.currentTarget.releasePointerCapture(event.pointerId)}
 async function apply(){
  if(!image.current||!crop||pending)return
  const current=++operation.current;setPending(true);setError('')
  try{const file=await renderAvatarCrop(image.current,crop);if(mounted.current&&current===operation.current)onApply(file)}catch(e){if(mounted.current&&current===operation.current)setError(errorText(e))}finally{if(mounted.current&&current===operation.current)setPending(false)}
 }
 return <dialog ref={dialog} className="z-contact-dialog z-avatar-crop-dialog" aria-labelledby="avatar-crop-title" aria-describedby="avatar-crop-help" onCancel={event=>{event.preventDefault();cancel()}}>
 <div className="z-contact-dialog-head"><h2 id="avatar-crop-title">Выберите область фото</h2><button className="z-icon" type="button" aria-label="Отменить обрезку фото" autoFocus onClick={cancel}><X/></button></div>
 <p id="avatar-crop-help" className="z-field-help">Перемещайте фото или используйте ползунки. В аватар попадёт квадратная область.</p>
 <div className="z-avatar-crop-viewport" aria-label="Предпросмотр квадратного аватара" onPointerDown={pointerDown} onPointerMove={pointerMove} onPointerUp={pointerEnd} onPointerCancel={pointerEnd} onLostPointerCapture={()=>{drag.current=null}}>
 <img ref={image} src={sourceURL} alt="Выбранное фото для аватара" draggable={false} onLoad={event=>{const photo=event.currentTarget;if(photo.naturalWidth>0&&photo.naturalHeight>0)setDimensions({width:photo.naturalWidth,height:photo.naturalHeight});else setError('Не удалось прочитать размер фото.')}} onError={()=>setError('Не удалось открыть фото. Выберите другой JPG или PNG.')} style={dimensions&&crop?{width:`${dimensions.width/crop.size*100}%`,height:`${dimensions.height/crop.size*100}%`,left:`${-crop.x/crop.size*100}%`,top:`${-crop.y/crop.size*100}%`}:{visibility:'hidden'}}/>
 {!dimensions&&!error&&<span role="status">Открываем фото…</span>}<span className="z-avatar-crop-guide" aria-hidden="true"/>
 </div>
 <div className="z-avatar-crop-sliders"><label>Масштаб <output>{Math.round(zoom*100)}%</output><input type="range" min={1} max={3} step={.01} value={zoom} disabled={!dimensions||pending} onChange={event=>setZoom(Number(event.target.value))}/></label><label>По горизонтали<input type="range" min={0} max={100} step={1} value={Math.round(position.x*100)} disabled={!dimensions||pending||!!crop&&dimensions.width===crop.size} onChange={event=>setPosition(value=>({...value,x:Number(event.target.value)/100}))}/></label><label>По вертикали<input type="range" min={0} max={100} step={1} value={Math.round(position.y*100)} disabled={!dimensions||pending||!!crop&&dimensions.height===crop.size} onChange={event=>setPosition(value=>({...value,y:Number(event.target.value)/100}))}/></label></div>
 {error&&<p className="notice" role="alert">{error}</p>}{pending&&<p role="status">Подготавливаем аватар…</p>}
 <div className="z-photo-actions"><button type="button" className="secondary" onClick={cancel}>Отмена</button><button type="button" className="primary" disabled={!crop||pending} onClick={()=>void apply()}>Применить</button></div>
 </dialog>
}
