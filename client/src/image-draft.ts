export type ImageDraft={file:File;url:string;id:string}
export type ImageDraftState={phase:'idle'|'preparing'|'preview'|'sending';draft?:ImageDraft;error?:string}
type Environment<T>={prepare:(file:File,signal:AbortSignal)=>Promise<File>;upload:(draft:ImageDraft,signal:AbortSignal)=>Promise<T>;createURL:(file:File)=>string;revokeURL:(url:string)=>void;uuid:()=>string;errorText:(error:unknown)=>string}
export class ImageDraftController<T>{
 private state:ImageDraftState={phase:'idle'}
 private generation=0
 private disposed=false
 private request?:AbortController
 private preparation?:AbortController
 private notify:(state:ImageDraftState)=>void
 private environment:Environment<T>
 private sent:(result:T)=>void
 constructor(notify:(state:ImageDraftState)=>void,environment:Environment<T>,sent:(result:T)=>void){this.notify=notify;this.environment=environment;this.sent=sent}
 private emit(state:ImageDraftState){this.state=state;if(!this.disposed)this.notify(state)}
 async select(file:File){
  if(this.disposed||this.state.phase==='sending')return
  this.cancel();const generation=++this.generation;const preparation=new AbortController();this.preparation=preparation;this.emit({phase:'preparing'})
  try{const prepared=await this.environment.prepare(file,preparation.signal);if(this.disposed||generation!==this.generation)return;this.emit({phase:'preview',draft:{file:prepared,url:this.environment.createURL(prepared),id:this.environment.uuid()}})}catch(error){if(!this.disposed&&generation===this.generation)this.emit({phase:'idle',error:this.environment.errorText(error)})}
 }
 async send(){
  const draft=this.state.draft;if(this.disposed||!draft||this.state.phase==='sending')return
  const generation=this.generation;const request=new AbortController();this.request=request;this.emit({phase:'sending',draft})
  try{const result=await this.environment.upload(draft,request.signal);if(this.disposed||generation!==this.generation||request.signal.aborted)return;this.sent(result);this.cancel()}catch(error){if(!this.disposed&&generation===this.generation&&!request.signal.aborted)this.emit({phase:'preview',draft,error:this.environment.errorText(error)+' Фото сохранено для повторной отправки.'})}
 }
 cancel(){++this.generation;this.preparation?.abort();this.preparation=undefined;this.request?.abort();this.request=undefined;if(this.state.draft)this.environment.revokeURL(this.state.draft.url);this.emit({phase:'idle'})}
 dispose(){this.disposed=true;this.cancel()}
}
export function isChatImageURL(value?:string):value is string{return !!value&&/^\/api\/messages\/[0-9]+\/image(?:\?[^#]*)?$/.test(value)}
