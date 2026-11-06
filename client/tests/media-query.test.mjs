import { test } from 'node:test'
import assert from 'node:assert/strict'
import { createMediaQueryStore } from '../src/media-query.ts'
import { VoiceRecorder } from '../src/voice-recorder.ts'
test('resize emits current mobile state and unsubscribe releases listener',()=>{
 let listener;const media={matches:false,addEventListener(event,fn){assert.equal(event,'change');listener=fn},removeEventListener(event,fn){assert.equal(fn,listener);listener=undefined}}
 const store=createMediaQueryStore(media);let observed=store.getSnapshot();const unsubscribe=store.subscribe(()=>observed=store.getSnapshot());media.matches=true;listener();assert.equal(observed,true);unsubscribe();assert.equal(listener,undefined)
})
test('desktop to mobile visibility change stops active recording',async()=>{
 let listener;const media={matches:false,addEventListener(_,fn){listener=fn},removeEventListener(){listener=undefined}}
 const store=createMediaQueryStore(media);const track={stopped:false,stop(){this.stopped=true}}
 class Recorder{static isTypeSupported(){return true};state='inactive';start(){this.state='recording'}stop(){this.state='inactive'}}
 const recorder=new VoiceRecorder(()=>{}, {getMedia:async()=>({getTracks:()=>[track]}),Recorder,createURL:()=>'',revokeURL:()=>{},now:Date.now,setTimer:()=>1,clearTimer:()=>{},uuid:()=>''})
 await recorder.start();const unsubscribe=store.subscribe(()=>{if(store.getSnapshot())recorder.cancel()});media.matches=true;listener();assert.ok(track.stopped);unsubscribe();recorder.dispose()
})
