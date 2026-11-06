import { test } from 'node:test'
import assert from 'node:assert/strict'
class Track extends EventTarget {kind='audio';enabled=true;muted=false;readyState='live';stop(){this.stopped=true}}
class Stream extends EventTarget {
 constructor(items=[]){super();this.items=items}
 getTracks(){return [...this.items]}
 getAudioTracks(){return this.items.filter(t=>t.kind==='audio')}
 addTrack(t){this.items.push(t);this.dispatchEvent(new Event('addtrack'))}
 removeTrack(t){this.items=this.items.filter(other=>other!==t);this.dispatchEvent(new Event('removetrack'))}
}
class Audio extends EventTarget {
 paused=true;muted=true;volume=0;srcObject=null;reject=false
 async play(){if(this.reject)throw Object.assign(new Error('blocked'),{name:'NotAllowedError'});this.paused=false;this.dispatchEvent(new Event('playing'))}
 pause(){this.paused=true;this.dispatchEvent(new Event('pause'))}
}
globalThis.MediaStream=Stream
const {bindRemoteAudio}=await import('../src/remote-audio.ts')
const tick=()=>new Promise(resolve=>setImmediate(resolve))
test('Late audio track is attached and playback can recover after autoplay rejection',async()=>{
 const source=new Stream(),audio=new Audio();let state
 const binding=bindRemoteAudio(audio,source,value=>{state=value})
 assert.equal(state.source,'waiting')
 audio.reject=true;const track=new Track();source.addTrack(track);await tick()
 assert.equal(state.blocked,true);assert.equal(state.playing,false)
 audio.reject=false;await binding.retry()
 assert.equal(state.playing,true);assert.equal(state.blocked,false)
 assert.deepEqual(audio.srcObject.getAudioTracks(),[track])
 binding.dispose();assert.equal(audio.srcObject,null);assert.equal(track.stopped,undefined)
})
test('Ended/removed audio cannot remain reported as live or playing',async()=>{
 const track=new Track(),source=new Stream([track]),audio=new Audio();let state
 const binding=bindRemoteAudio(audio,source,value=>{state=value});await tick()
 track.muted=true;track.dispatchEvent(new Event('mute'));assert.equal(state.source,'muted')
 track.readyState='ended';track.dispatchEvent(new Event('ended'));await tick()
 assert.equal(state.playing,false);assert.equal(state.source,'waiting')
 binding.dispose()
})
test('Late play promise cannot update state after disposal',async()=>{
 const track=new Track(),audio=new Audio();let resolvePlay,calls=0
 audio.play=()=>new Promise(resolve=>{resolvePlay=resolve})
 const binding=bindRemoteAudio(audio,new Stream([track]),()=>{calls++})
 binding.dispose();const after=calls;resolvePlay();await tick()
 assert.equal(calls,after);assert.equal(audio.srcObject,null);assert.equal(track.stopped,undefined)
})
