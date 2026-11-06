import { test } from 'node:test'
import assert from 'node:assert/strict'
import { observeReadReceipt } from '../src/read-visibility.ts'

test('read receipt requires focus, visible tab, and an on-screen latest message',async()=>{
 const original={document:globalThis.document,window:globalThis.window,Observer:globalThis.IntersectionObserver}
 const doc=new EventTarget(),win=new EventTarget()
 let focus=true,intersect,posts=0,changes=0
 doc.visibilityState='visible';doc.hasFocus=()=>focus
 globalThis.document=doc;globalThis.window=win
 globalThis.IntersectionObserver=class{constructor(fn){intersect=fn}observe(){}disconnect(){}}
 const acknowledged={value:'0'}
 const stop=observeReadReceipt('10',{},acknowledged,async()=>{posts++},()=>changes++)
 const settle=()=>new Promise(resolve=>setTimeout(resolve,280))
 try{
  // Merely mounting after a history load must not mark anything read.
  await settle();assert.equal(posts,0)
  focus=false;intersect([{isIntersecting:true}]);await settle();assert.equal(posts,0)
  focus=true;doc.visibilityState='hidden';doc.dispatchEvent(new Event('visibilitychange'));await settle();assert.equal(posts,0)
  doc.visibilityState='visible';intersect([{isIntersecting:false}]);win.dispatchEvent(new Event('focus'));await settle();assert.equal(posts,0)
  intersect([{isIntersecting:true}]);await settle();assert.equal(posts,1);assert.equal(changes,1);assert.equal(acknowledged.value,'10')
  win.dispatchEvent(new Event('focus'));await settle();assert.equal(posts,1)
  stop();win.dispatchEvent(new Event('focus'));await settle();assert.equal(posts,1)
 }finally{stop();globalThis.document=original.document;globalThis.window=original.window;globalThis.IntersectionObserver=original.Observer}
})

test('leaving the conversation cancels a scheduled acknowledgement',async()=>{
 const original={document:globalThis.document,window:globalThis.window,Observer:globalThis.IntersectionObserver}
 let intersect,posts=0
 globalThis.document=Object.assign(new EventTarget(),{visibilityState:'visible',hasFocus:()=>true})
 globalThis.window=new EventTarget()
 globalThis.IntersectionObserver=class{constructor(fn){intersect=fn}observe(){}disconnect(){}}
 const stop=observeReadReceipt('2',{}, {value:'0'}, async()=>posts++,()=>{})
 try{intersect([{isIntersecting:true}]);stop();await new Promise(resolve=>setTimeout(resolve,280));assert.equal(posts,0)}
 finally{stop();globalThis.document=original.document;globalThis.window=original.window;globalThis.IntersectionObserver=original.Observer}
})
