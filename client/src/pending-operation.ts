export type PendingOperation = {body:string;id:string;replyID?:string}
type StorageLike=Pick<Storage,'getItem'|'setItem'|'removeItem'>
export function pendingOperation(storage:StorageLike,key:string,body:string,replyID:string|undefined,createID:()=>string):PendingOperation {
 let saved:PendingOperation|null=null
 try{const value=JSON.parse(storage.getItem(key)||'null');if(value&&typeof value.id==='string'&&/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(value.id)&&value.body===body&&value.replyID===replyID)saved=value}catch{/* Storage is optional. */}
 const operation=saved??{body,replyID,id:createID()}
 try{storage.setItem(key,JSON.stringify(operation))}catch{/* Caller retains operation until acknowledgement. */}
 return operation
}
export function acknowledgeOperation(storage:StorageLike,key:string,id:string){
 try{const saved=JSON.parse(storage.getItem(key)||'null');if(saved?.id===id)storage.removeItem(key)}catch{/* Never clear a record that cannot be verified. */}
}
