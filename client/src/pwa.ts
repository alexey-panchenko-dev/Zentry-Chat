export let pwaRegistration:ServiceWorkerRegistration|undefined
export function startPwa(){
 if(!('serviceWorker' in navigator))return
 const register=()=>{void navigator.serviceWorker.register('/sw.js',{updateViaCache:'none'}).then(value=>{pwaRegistration=value;window.dispatchEvent(new Event('zently-pwa-ready'))}).catch(()=>{})}
 if(document.readyState==='complete')register();else window.addEventListener('load',register,{once:true})
}
