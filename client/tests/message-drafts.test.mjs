import {test} from 'node:test'
import assert from 'node:assert/strict'
import {messageDraftKey,readMessageDraft,writeMessageDraft,replaceDraftRecord} from '../src/message-drafts.ts'

function memoryStorage(){
  const values=new Map()
  return {getItem:key=>values.get(key)??null,setItem:(key,value)=>values.set(key,value),removeItem:key=>values.delete(key),values}
}

test('drafts are isolated by account and conversation and restore after remount',()=>{
  const storage=memoryStorage()
  writeMessageDraft(storage,'user one','chat/1','черновик')
  assert.equal(readMessageDraft(storage,'user one','chat/1'),'черновик')
  assert.equal(readMessageDraft(storage,'user one','chat/2'),'')
  assert.equal(readMessageDraft(storage,'user two','chat/1'),'')
  assert.notEqual(messageDraftKey('user one','chat/1'),messageDraftKey('user two','chat/1'))
})

test('clearing a sent draft removes it and storage failures are harmless',()=>{
  const storage=memoryStorage()
  writeMessageDraft(storage,'user','chat','готово')
  writeMessageDraft(storage,'user','chat','')
  assert.equal(readMessageDraft(storage,'user','chat'),'')
  const unavailable={getItem(){throw Error('blocked')},setItem(){throw Error('blocked')},removeItem(){throw Error('blocked')}}
  assert.equal(readMessageDraft(unavailable,'user','chat'),'')
  assert.doesNotThrow(()=>writeMessageDraft(unavailable,'user','chat','draft'))
})

test('draft persistence respects the message length limit',()=>{
  const storage=memoryStorage()
  writeMessageDraft(storage,'user','chat','x'.repeat(1200))
  assert.equal(readMessageDraft(storage,'user','chat').length,1000)
})

test('late confirmation cannot erase a newer draft or its reply reference',()=>{
  const storage=memoryStorage()
  const key='draft'
  const old=JSON.stringify({body:'sent',replyID:'31'})
  const newer=JSON.stringify({body:'newer',replyID:'32'})
  assert.equal(replaceDraftRecord(storage,key,null,old),true)
  assert.equal(replaceDraftRecord(storage,key,old,newer),true)
  assert.equal(replaceDraftRecord(storage,key,old,''),false)
  assert.equal(storage.getItem(key),newer)
})
