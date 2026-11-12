import {test} from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import vm from 'node:vm'
const script=readFileSync(new URL('../public/sw.js',import.meta.url),'utf8')
function worker(fetcher=async()=>({ok:true})){const listeners=new Map(),writes=[];const cache={addAll:async()=>{},put:async(...args)=>writes.push(args),keys:async()=>[]};const sandbox={URL,fetch:fetcher,caches:{open:async()=>cache,match:async()=>({offline:true}),keys:async()=>[]},self:{location:{origin:'https://zently.test'},addEventListener:(type,fn)=>listeners.set(type,fn),clients:{claim:async()=>{}},skipWaiting(){}}};vm.runInNewContext(script,sandbox);return{listeners,writes}}
test('service worker never handles private API, cross-origin resources or write requests',()=>{
 const w=worker();for(const [url,method] of [['https://zently.test/api/messages/123/image','GET'],['https://other.test/assets/a.js','GET'],['https://zently.test/api/auth/login','POST']]){let intercepted=false;w.listeners.get('fetch')({request:{url,method},respondWith(){intercepted=true}});assert.equal(intercepted,false)}
})
test('navigation uses a generic offline page and never stores authenticated HTML',async()=>{
 const w=worker(async()=>{throw Error('offline')});let response;w.listeners.get('fetch')({request:{url:'https://zently.test/',method:'GET',mode:'navigate'},respondWith(value){response=value}});assert.equal((await response).offline,true);assert.equal(w.writes.length,0)
})
