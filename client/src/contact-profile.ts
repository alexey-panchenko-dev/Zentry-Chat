export function normalizeContactPhone(value:string):string {
 const phone=value.trim().replace(/[\s()-]/g,'')
 if(phone!==''&&!/^\+[1-9][0-9]{7,14}$/.test(phone))throw new Error('Введите номер в международном формате: + и от 8 до 15 цифр.')
 return phone
}
export type ProfileCallRequest={id:string;conversationID:string}
export class CallRequestGate {
 private consumed=new Set<string>()
 consume(request:ProfileCallRequest|null|undefined,busy:boolean):string|null {
  if(!request||this.consumed.has(request.id))return null
  this.consumed.add(request.id)
  return busy?null:request.conversationID
 }
}
