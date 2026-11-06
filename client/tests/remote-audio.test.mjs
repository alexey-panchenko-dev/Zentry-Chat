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

test('video-only stream updates preserve audio source and never restart playback',async()=>{
 const track=new Track(),audio=new Audio();let pauses=0,plays=0
 const pause=audio.pause.bind(audio),play=audio.play.bind(audio)
 audio.pause=()=>{pauses++;pause()};audio.play=()=>{plays++;return play()}
 const binding=bindRemoteAudio(audio,new Stream([track]),()=>{});await tick()
 const source=audio.srcObject,before={pauses,plays}
 binding.setStream(new Stream([track]));await tick()
 assert.equal(audio.srcObject,source);assert.deepEqual({pauses,plays},before)
 binding.dispose();assert.equal(track.stopped,undefined)
})

test('Web Audio output shares the gesture-unlocked microphone context and releases its lease',async()=>{
 const original=globalThis.AudioContext,instances=[]
 class Context {
  state='suspended';destination={};sources=[]
  constructor(){instances.push(this)}
  async resume(){this.state='running'}
  createMediaStreamSource(stream){const node={stream,connect(target){this.target=target},disconnect(){this.disconnected=true}};this.sources.push(node);return node}
  createAnalyser(){return{fftSize:1024,getFloatTimeDomainData(values){values.fill(.25)}}}
  async close(){this.state='closed'}
 }
 globalThis.AudioContext=Context
 const {AudioMeter}=await import('../src/audio-meter.ts')
 const meter=new AudioMeter();meter.unlock()
 const track=new Track(),audio=new Audio();let state
 const binding=bindRemoteAudio(audio,new Stream([track]),value=>state=value)
 try{
  await tick()
  assert.equal(instances.length,1);assert.equal(state.mode,'webaudio');assert.equal(state.playing,true)
  assert.equal(audio.paused,true);assert.equal(audio.muted,true)
  assert.equal(instances[0].sources[0].target,instances[0].destination)
  assert.equal(meter.sample(new Stream([new Track()])),.25)
  await binding.switchOutput();assert.equal(state.mode,'element');assert.equal(audio.paused,false)
  assert.equal(instances[0].sources[0].disconnected,true)
  binding.dispose();assert.equal(instances[0].state,'running','player must not close the microphone context')
  assert.equal(track.stopped,undefined)
 }finally{binding.dispose();meter.dispose();globalThis.AudioContext=original}
 assert.equal(instances[0].state,'closed')
})

test('blocked Web Audio can be resumed with the visible playback button',async()=>{
 const original=globalThis.AudioContext
 let blocked=true
 class Context {
  state='suspended';destination={}
  async resume(){if(blocked)throw new DOMException('blocked','NotAllowedError');this.state='running'}
  createMediaStreamSource(){return {connect(){},disconnect(){}}}
  async close(){this.state='closed'}
 }
 globalThis.AudioContext=Context
 let state;const binding=bindRemoteAudio(new Audio(),new Stream([new Track()]),value=>state=value)
 try{await tick();assert.equal(state.blocked,true);assert.equal(state.playing,false);blocked=false;await binding.retry();assert.equal(state.playing,true);assert.equal(state.blocked,false)}
 finally{binding.dispose();globalThis.AudioContext=original}
})
