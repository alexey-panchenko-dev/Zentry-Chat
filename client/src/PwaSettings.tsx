import {useEffect,useState} from 'react'
import {pwaRegistration} from './pwa'
export function PwaSettings(){
 const [waiting,setWaiting]=useState<ServiceWorker|null>(null)
 useEffect(()=>{const registration=()=>{const current=pwaRegistration;if(!current)return;setWaiting(current.waiting);current.addEventListener('updatefound',()=>{current.installing?.addEventListener('statechange',()=>setWaiting(current.waiting))})};registration();window.addEventListener('zently-pwa-ready',registration);return()=>window.removeEventListener('zently-pwa-ready',registration)},[])
 function update(){if(!waiting)return;navigator.serviceWorker.addEventListener('controllerchange',()=>window.location.reload(),{once:true});waiting.postMessage({type:'ACTIVATE_UPDATE'})}
 return <section className="z-pwa-settings"><h2>Приложение Zently</h2><details><summary>Как установить на устройство</summary><p className="z-muted">Откройте меню браузера и выберите «Установить приложение». На iPhone: Safari → «Поделиться» → «На экран Домой».</p></details>{waiting&&<button type="button" className="secondary" onClick={update}>Установить обновление</button>}</section>
}
