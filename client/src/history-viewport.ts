import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import type { RefObject } from 'react'

const positions = new Map<string, { top: number; bottom: boolean }>()
export function useHistoryViewport(key: string, root: RefObject<HTMLDivElement | null>, latest?: string, target?: string) {
  const nearBottom = useRef(true)
  const interacted = useRef(false)
  const previous = useRef<string | undefined>(undefined)
  const [newCount, setNewCount] = useState(0)
  const [away, setAway] = useState(false)
  const anchor = useRef<{height:number;top:number}|null>(null)
  useLayoutEffect(() => {
    const element = root.current
    if (!element) return
    const saved = positions.get(key)
    nearBottom.current = saved?.bottom ?? true
    interacted.current = saved ? !saved.bottom : false
    if (saved) element.scrollTop = saved.top
    const scroll = () => {
      nearBottom.current = !interacted.current || element.scrollHeight-element.scrollTop-element.clientHeight < 90
      setAway(!nearBottom.current)
      if (nearBottom.current) setNewCount(0)
      positions.set(key,{top:element.scrollTop,bottom:nearBottom.current})
    }
    const interact=()=>{interacted.current=true}
    for(const event of ['wheel','touchmove','pointerdown','keydown'])element.addEventListener(event,interact,{passive:true})
    element.addEventListener('scroll',scroll,{passive:true})
    return () => { positions.set(key,{top:element.scrollTop,bottom:nearBottom.current}); element.removeEventListener('scroll',scroll);for(const event of ['wheel','touchmove','pointerdown','keydown'])element.removeEventListener(event,interact) }
  },[key,root])
  useLayoutEffect(() => {
    const element=root.current
    if (!element || !latest || target) return
    if (previous.current===latest) return
    const initial=!previous.current
    previous.current=latest
    if (nearBottom.current) {element.scrollTop=element.scrollHeight;setNewCount(0)}
    else if (!initial) setNewCount(value=>value+1)
  },[latest,target,root])
  useEffect(() => {
    const element=root.current
    if (!element) return
    const resize=new ResizeObserver(()=>{if(nearBottom.current&&!target)element.scrollTop=element.scrollHeight})
    const observe=()=>{resize.observe(element);for(const child of element.children)resize.observe(child)}
    observe()
    const mutations=new MutationObserver(observe);mutations.observe(element,{childList:true})
    return()=>{resize.disconnect();mutations.disconnect()}
  },[root,target])
  return {away,newCount,toLatest(){const element=root.current;if(element){nearBottom.current=true;element.scrollTo({top:element.scrollHeight,behavior:'smooth'});setNewCount(0)}},async older(load:()=>Promise<unknown>){const element=root.current;if(element)anchor.current={height:element.scrollHeight,top:element.scrollTop};await load();requestAnimationFrame(()=>{if(element&&anchor.current){element.scrollTop=anchor.current.top+element.scrollHeight-anchor.current.height;anchor.current=null}})}}
}
export function messageDay(value:string, now=new Date()) {
 const date=new Date(value)
 const today=new Date(now.getFullYear(),now.getMonth(),now.getDate())
 const yesterday=new Date(today);yesterday.setDate(yesterday.getDate()-1)
 const day=new Date(date.getFullYear(),date.getMonth(),date.getDate())
 return day.getTime()===today.getTime()?'Сегодня':day.getTime()===yesterday.getTime()?'Вчера':date.toLocaleDateString('ru-RU',{day:'numeric',month:'long',year:'numeric'})
}
