import { test } from 'node:test'
import assert from 'node:assert/strict'
import { VoiceRecorder } from '../src/voice-recorder.ts'
function setup(getMedia) {
 const track={stopped:false,stop(){this.stopped=true}}
 class Recorder { static isTypeSupported(){return true}; state='inactive'; mimeType='audio/webm'; start(){this.state='recording'} requestData(){this.ondataavailable?.({data:new Blob(['voice'])})} stop(){this.state='inactive';this.ondataavailable?.({data:new Blob(['final'])});this.onstop?.()} }
 let timer; let revoked=0; let state
 const env={getMedia:getMedia??(async()=>({getTracks:()=>[track]})),Recorder,createURL:()=> 'blob:test',revokeURL:()=>revoked++,now:()=>1000,setTimer:fn=>{timer=fn;return 1},clearTimer:()=>{},uuid:()=> 'same-id'}
 const recorder=new VoiceRecorder(s=>state=s,env)
 return {recorder,track,env,state:()=>state,expire:()=>timer(),revoked:()=>revoked}
}
test('stop creates preview, cancel releases URL and tracks',async()=>{const x=setup();await x.recorder.start();x.recorder.stop();assert.equal(x.state().phase,'preview');assert.equal(x.state().draft.id,'same-id');assert.ok(x.track.stopped);x.recorder.cancel();assert.equal(x.revoked(),1);assert.equal(x.state().phase,'idle')})
test('denied permission has an actionable error',async()=>{const x=setup(async()=>{throw Error('denied')});await x.recorder.start();assert.equal(x.state().phase,'error');assert.match(x.state().error,/микрофон/i)})
test('late permission after cancel is stopped',async()=>{let resolve;const x=setup(()=>new Promise(r=>resolve=r));const pending=x.recorder.start();x.recorder.cancel();resolve({getTracks:()=>[x.track]});await pending;assert.ok(x.track.stopped);assert.equal(x.state().phase,'idle')})
test('120 second timer automatically stops capture',async()=>{const x=setup();await x.recorder.start();x.expire();assert.equal(x.state().phase,'preview');assert.ok(x.track.stopped)})
test('disposal stops microphone and releases preview',async()=>{const x=setup();await x.recorder.start();x.recorder.stop();x.recorder.dispose();assert.ok(x.track.stopped);assert.equal(x.revoked(),1)})

test('unsupported recorder gives notice without requesting microphone',async()=>{let requested=false;const x=setup(async()=>{requested=true});x.env.Recorder=undefined;await x.recorder.start();assert.equal(requested,false);assert.equal(x.state().phase,'error')})
test('disposing during permission request stops its late tracks',async()=>{let resolve;const x=setup(()=>new Promise(r=>resolve=r));const pending=x.recorder.start();x.recorder.dispose();resolve({getTracks:()=>[x.track]});await pending;assert.ok(x.track.stopped)})
test('oversized recording stops and cannot become a draft',async()=>{const x=setup();class LargeRecorder {static isTypeSupported(){return true};state='inactive';mimeType='audio/webm';start(){this.state='recording';this.ondataavailable({data:new Blob([new Uint8Array(3*1024*1024+1)])})}stop(){this.state='inactive';this.onstop?.()}};x.env.Recorder=LargeRecorder;await x.recorder.start();assert.equal(x.state().draft,undefined);assert.ok(x.track.stopped)})
test('delayed stop from cancelled recorder cannot stop a newer microphone',async()=>{
 const instances=[];const tracks=[];let state
 class AsyncRecorder{static isTypeSupported(){return true};state='inactive';mimeType='audio/webm';constructor(){instances.push(this)}start(){this.state='recording'}stop(){this.state='inactive'}finish(){this.ondataavailable?.({data:new Blob(['old recording'])});this.onstop?.()}}
 const env={getMedia:async()=>{const track={stopped:false,stop(){this.stopped=true}};tracks.push(track);return {getTracks:()=>[track]}},Recorder:AsyncRecorder,createURL:()=> 'blob:test',revokeURL:()=>{},now:()=>1000,setTimer:()=>1,clearTimer:()=>{},uuid:()=> 'id'}
 const recorder=new VoiceRecorder(next=>state=next,env)
 await recorder.start();recorder.stop();recorder.cancel();assert.ok(tracks[0].stopped);await recorder.start();instances[0].finish();assert.equal(tracks[1].stopped,false);assert.equal(state.phase,'recording');recorder.dispose();assert.ok(tracks[1].stopped)
})
test('stopping waits for the final chunk before offering preview',async()=>{
 let instance;let state
 class AsyncRecorder{static isTypeSupported(){return true};state='inactive';mimeType='audio/webm';constructor(){instance=this}start(){this.state='recording'}stop(){this.state='inactive'}}
 const track={stopped:false,stop(){this.stopped=true}}
 const recorder=new VoiceRecorder(next=>state=next,{getMedia:async()=>({getTracks:()=>[track]}),Recorder:AsyncRecorder,createURL:()=> 'blob:test',revokeURL:()=>{},now:()=>1000,setTimer:()=>1,clearTimer:()=>{},uuid:()=> 'id'})
 await recorder.start();recorder.stop();assert.equal(track.stopped,false,'capture must remain available until the recorder flushes its final data');assert.equal(state.draft,undefined);instance.ondataavailable({data:new Blob(['final'])});instance.onstop();assert.ok(track.stopped);assert.equal(state.phase,'preview');assert.equal(await state.draft.blob.text(),'final');recorder.dispose()
})
test('stop requests buffered audio before the recorder flushes its final chunk',async()=>{
 let state
 class BufferedRecorder{static isTypeSupported(){return true};state='inactive';mimeType='audio/webm';start(){this.state='recording'}requestData(){this.ondataavailable?.({data:new Blob(['buffered'])})}stop(){this.state='inactive';this.ondataavailable?.({data:new Blob(['final'])});this.onstop?.()}}
 const track={stopped:false,stop(){this.stopped=true}}
 const recorder=new VoiceRecorder(next=>state=next,{getMedia:async()=>({getTracks:()=>[track]}),Recorder:BufferedRecorder,createURL:()=> 'blob:test',revokeURL:()=>{},now:()=>1000,setTimer:()=>1,clearTimer:()=>{},uuid:()=> 'id'})
 await recorder.start();recorder.stop();assert.equal(state.phase,'preview');assert.equal(await state.draft.blob.text(),'bufferedfinal');assert.ok(track.stopped);recorder.dispose()
})
