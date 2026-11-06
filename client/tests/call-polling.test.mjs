import { test } from 'node:test'
import assert from 'node:assert/strict'
import { registerHooks } from 'node:module'

registerHooks({resolve(specifier,context,next){
 if(specifier==='@/api')return {url:'data:text/javascript,export const api=globalThis.pollAPI;export const errorText=e=>e.message',shortCircuit:true}
 if(specifier==='pusher-js')return {url:'data:text/javascript,export default globalThis.PollPusher',shortCircuit:true}
 return next(specifier,context)
}})
let client,callReads=0
globalThis.pollAPI={async get(url){if(url==='/realtime/config')return{data:{enabled:true,key:'key',cluster:'eu'}};if(url==='/calls'){callReads++;return{data:{calls:[]}}};throw Error(url)}}
globalThis.PollPusher=class {
 handlers={}
 connection={bind:(_,fn)=>{this.state=fn},unbind(){}}
 constructor(){client=this}
 subscribe(){return{bind:(event,fn)=>this.handlers[event]=fn,unbind(){}}}
 unsubscribe(){}
 disconnect(){this.disconnected=true}
}
const {CallController}=await import('../src/calls.ts')
test('healthy Pusher eliminates idle calls polling, disconnect enables slower recovery',async()=>{
 const original=globalThis.setInterval,originalClear=globalThis.clearInterval
 let tick,interval,cleared=false
 globalThis.setInterval=(fn,ms)=>{tick=fn;interval=ms;return 123}
 globalThis.clearInterval=id=>{assert.equal(id,123);cleared=true}
 const controller=new CallController('me',()=>{})
 try{
  await controller.connect()
  assert.equal(interval,30000)
  client.handlers['pusher:subscription_succeeded']()
  await new Promise(resolve=>setImmediate(resolve))
  const baseline=callReads
  for(let i=0;i<10;i++)tick()
  assert.equal(callReads,baseline)
  client.state({current:'disconnected'})
  tick()
  await new Promise(resolve=>setImmediate(resolve))
  assert.equal(callReads,baseline+1)
  client.handlers['call.updated']()
  await new Promise(resolve=>setImmediate(resolve))
  assert.equal(callReads,baseline+2)
 }finally{controller.dispose();globalThis.setInterval=original;globalThis.clearInterval=originalClear}
 assert.equal(cleared,true)
 assert.equal(client.disconnected,true)
})
