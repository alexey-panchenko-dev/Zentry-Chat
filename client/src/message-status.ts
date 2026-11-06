export function messageStatus(id:string,delivered='0',read='0'):'sent'|'delivered'|'read'{
 const cursor=(value:string)=>/^\d+$/.test(value)?BigInt(value):0n
 return cursor(id)<=cursor(read)?'read':cursor(id)<=cursor(delivered)?'delivered':'sent'
}
