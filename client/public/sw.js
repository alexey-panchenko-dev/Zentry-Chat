const CACHE='zently-public-1.1';
const OFFLINE=['/offline.html','/offline.css','/assets/logo.svg'];
self.addEventListener('install',event=>event.waitUntil(caches.open(CACHE).then(cache=>cache.addAll(OFFLINE))));
self.addEventListener('activate',event=>event.waitUntil((async()=>{for(const name of await caches.keys())if(name.startsWith('zently-public-')&&name!==CACHE)await caches.delete(name);await self.clients.claim()})()));
self.addEventListener('message',event=>{if(event.data?.type==='ACTIVATE_UPDATE')self.skipWaiting()});
self.addEventListener('fetch',event=>{
 const request=event.request,url=new URL(request.url);
 if(request.method!=='GET'||url.origin!==self.location.origin||url.pathname.startsWith('/api/'))return;
 if(request.mode==='navigate'){event.respondWith(fetch(request).catch(()=>caches.match('/offline.html')));return}
 if(!OFFLINE.includes(url.pathname)&&!/^\/assets\/[\w.-]+\.(js|css|woff2?|ttf|svg)$/.test(url.pathname))return;
 event.respondWith((async()=>{const cached=await caches.match(request);if(cached)return cached;const response=await fetch(request);if(response.ok){const cache=await caches.open(CACHE);await cache.put(request,response.clone());const keys=await cache.keys();if(keys.length>80)for(const key of keys.slice(0,keys.length-80))if(!OFFLINE.includes(new URL(key.url).pathname))await cache.delete(key)}return response})());
});
self.addEventListener('push',event=>event.waitUntil((async()=>{let data;try{data=event.data?.json()}catch{return}if(!data||typeof data.chat_id!=='string'||!/^[0-9a-f-]{36}$/i.test(data.chat_id)||!['personal','group'].includes(data.scope))return;await self.registration.showNotification('Zently',{body:typeof data.body==='string'?data.body:'Новое сообщение',icon:'/assets/icon-192.png',badge:'/assets/icon-192.png',tag:data.tag,data:{chat_id:data.chat_id,scope:data.scope}})})()));
self.addEventListener('notificationclick',event=>{event.notification.close();const data=event.notification.data;event.waitUntil((async()=>{const windows=await self.clients.matchAll({type:'window',includeUncontrolled:true});const target=windows.find(client=>new URL(client.url).origin===self.location.origin);if(target){await target.focus();target.postMessage({type:'OPEN_PUSH_CHAT',chat_id:data.chat_id,scope:data.scope})}else await self.clients.openWindow('/?chat='+encodeURIComponent(data.chat_id)+'&scope='+encodeURIComponent(data.scope))})())});
