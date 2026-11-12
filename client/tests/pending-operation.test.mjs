import {test} from 'node:test'
import assert from 'node:assert/strict'
import {pendingOperation,acknowledgeOperation} from '../src/pending-operation.ts'
const a='10000000-0000-4000-8000-000000000001',b='10000000-0000-4000-8000-000000000002'
function memory(){const m=new Map();return {getItem:k=>m.get(k)??null,setItem:(k,v)=>m.set(k,v),removeItem:k=>m.delete(k)}}
test('ambiguous send reuses operation ID after a page reload and clears only its acknowledgement',()=>{
 const storage=memory();const first=pendingOperation(storage,'user/chat','hello','42',()=>a)
 assert.equal(pendingOperation(storage,'user/chat','hello','42',()=>b).id,first.id)
 const newer=pendingOperation(storage,'user/chat','different','42',()=>b)
 acknowledgeOperation(storage,'user/chat',first.id)
 assert.equal(pendingOperation(storage,'user/chat','different','42',()=>a).id,newer.id)
 acknowledgeOperation(storage,'user/chat',newer.id);assert.equal(storage.getItem('user/chat'),null)
})
test('operations cannot leak across accounts, chats or reply targets',()=>{
 const storage=memory();pendingOperation(storage,'one/chat','hello','1',()=>a)
 assert.equal(pendingOperation(storage,'two/chat','hello','1',()=>b).id,b)
 assert.equal(pendingOperation(storage,'one/chat','hello','2',()=>b).id,b)
})
