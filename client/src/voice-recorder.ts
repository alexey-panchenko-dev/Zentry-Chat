export const MAX_VOICE_BYTES = 3 * 1024 * 1024
export type VoiceDraft = { blob: Blob; url: string; duration: number; id: string }
export type RecorderState = { phase: 'idle'|'requesting'|'recording'|'preview'|'error'; draft?: VoiceDraft; error?: string }
type Environment = { getMedia:()=>Promise<MediaStream>; Recorder:typeof MediaRecorder; createURL:(blob:Blob)=>string; revokeURL:(url:string)=>void; now:()=>number; setTimer:(fn:()=>void,ms:number)=>ReturnType<typeof setTimeout>; clearTimer:(id:ReturnType<typeof setTimeout>)=>void; uuid:()=>string }
export class VoiceRecorder {
 private notify:(state:RecorderState)=>void
 private env:Environment
 private state:RecorderState={phase:'idle'}
 private generation=0
 private disposed=false
 private stream?:MediaStream
 private recorder?:MediaRecorder
 private timer?:ReturnType<typeof setTimeout>
 private started=0
 constructor(notify:(state:RecorderState)=>void, env?:Environment){this.notify=notify;this.env=env??{getMedia:()=>navigator.mediaDevices.getUserMedia({audio:true,video:false}),Recorder:globalThis.MediaRecorder,createURL:URL.createObjectURL,revokeURL:URL.revokeObjectURL,now:Date.now,setTimer:setTimeout,clearTimer:clearTimeout,uuid:()=>crypto.randomUUID()}}
 private emit(state:RecorderState){this.state=state;if(!this.disposed)this.notify(state)}
 private release(){this.stream?.getTracks().forEach(track=>track.stop());this.stream=undefined;if(this.timer)this.env.clearTimer(this.timer);this.timer=undefined}
 async start(){
  if(this.disposed||this.state.phase==='requesting'||this.state.phase==='recording')return
  this.cancel();const generation=++this.generation
  const Recorder=this.env.Recorder
  const mime=Recorder&&['audio/webm;codecs=opus','audio/ogg;codecs=opus','audio/mp4'].find(type=>Recorder.isTypeSupported(type))
  if(!mime){this.emit({phase:'error',error:'Этот браузер не поддерживает запись голоса. Попробуйте другой браузер.'});return}
  this.emit({phase:'requesting'})
  try{
   const stream=await this.env.getMedia()
   if(this.disposed||generation!==this.generation){stream.getTracks().forEach(track=>track.stop());return}
   this.stream=stream
   const recorder=new Recorder(stream,{mimeType:mime,audioBitsPerSecond:96000});this.recorder=recorder
   const chunks:Blob[]=[];let size=0;let oversized=false
   recorder.ondataavailable=event=>{if(generation!==this.generation)return;if(event.data.size){size+=event.data.size;if(size>MAX_VOICE_BYTES){oversized=true;this.stop()}else chunks.push(event.data)}}
   recorder.onerror=()=>{if(generation===this.generation){this.cancel();this.emit({phase:'error',error:'Не удалось записать голос. Проверьте микрофон и повторите.'})}}
   recorder.onstop=()=>{if(this.disposed||generation!==this.generation)return;this.release();const blob=new Blob(chunks,{type:recorder.mimeType||mime});if(oversized||!blob.size){this.emit({phase:'error',error:oversized?'Запись превысила 3 МБ. Запишите более короткое сообщение.':'Запись пустая. Попробуйте ещё раз.'});return}this.emit({phase:'preview',draft:{blob,url:this.env.createURL(blob),duration:Math.min(120000,Math.max(1,this.env.now()-this.started)),id:this.env.uuid()}})}
   this.started=this.env.now();recorder.start(250);this.emit({phase:'recording'});this.timer=this.env.setTimer(()=>this.stop(),120000)
  }catch{if(generation===this.generation&&!this.disposed){this.release();this.emit({phase:'error',error:'Нет доступа к микрофону. Разрешите доступ в настройках браузера и повторите.'})}}
 }
 // stop() flushes its last chunk asynchronously. Keep the source alive until
 // onstop handles that chunk; cancel/dispose still release it immediately.
 stop(){if(this.recorder?.state==='recording')this.recorder.stop()}
 cancel(){++this.generation;this.release();if(this.recorder?.state==='recording')this.recorder.stop();this.recorder=undefined;if(this.state.draft)this.env.revokeURL(this.state.draft.url);this.emit({phase:'idle'})}
 dispose(){this.disposed=true;this.cancel()}
}
