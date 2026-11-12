import {test} from 'node:test'
import assert from 'node:assert/strict'
import {messageDay} from '../src/history-viewport.ts'
test('date dividers follow local calendar days across month and year boundaries',()=>{
 const now=new Date(2026,0,1,12)
 assert.equal(messageDay(new Date(2026,0,1,0).toISOString(),now),'Сегодня')
 assert.equal(messageDay(new Date(2025,11,31,23).toISOString(),now),'Вчера')
 assert.match(messageDay(new Date(2025,11,30,12).toISOString(),now),/30.*2025/)
})
