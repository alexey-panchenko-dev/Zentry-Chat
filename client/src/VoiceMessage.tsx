import { useEffect, useRef, useState } from 'react'
export function VoiceMessage({url,duration=0}:{url:string;duration?:number}) {
 const audio=useRef<HTMLAudioElement>(null)
 const [error,setError]=useState(false)
 useEffect(()=>{const element=audio.current;return()=>{element?.pause()}},[])
 return <div className="z-voice-player"><audio ref={audio} controls preload="metadata" src={url} aria-label="Голосовое сообщение" onError={()=>setError(true)}/><small>{Math.ceil(duration/1000)} сек.</small>{error&&<p className="notice" role="alert">Не удалось загрузить запись. <button type="button" className="secondary" onClick={()=>{setError(false);audio.current?.load()}}>Повторить</button></p>}</div>
}
