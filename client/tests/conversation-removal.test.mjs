import {test} from 'node:test'
import assert from 'node:assert/strict'
import {isRemovalAction,removalCommitted,withoutConversation} from '../src/conversation-controls.ts'
test('deleting chats or removing contacts navigates away only after server confirmation',()=>{
 for(const action of ['delete_chat','delete_everyone','remove_contact']){
  assert.equal(isRemovalAction(action),true)
  assert.equal(removalCommitted(action,{hidden:true}),true)
  assert.equal(removalCommitted(action,{hidden:false}),false)
  assert.equal(removalCommitted(action,{}),false)
 }
 for(const action of ['clear','block','unblock']){
  assert.equal(isRemovalAction(action),false)
  assert.equal(removalCommitted(action,{hidden:true}),false)
 }
})
test('actor list removal preserves other dialogs, pagination and source data',()=>{const pages=[{conversations:[{id:'1'},{id:'2'}],next_cursor:'cursor'},{conversations:[{id:'3'}],next_cursor:''}];const next=withoutConversation(pages,'2');assert.deepEqual(next[0].conversations,[{id:'1'}]);assert.equal(next[0].next_cursor,'cursor');assert.equal(next[1].conversations[0].id,'3');assert.equal(pages[0].conversations.length,2)})
