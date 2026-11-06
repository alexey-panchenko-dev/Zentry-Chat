import {test} from 'node:test'
import assert from 'node:assert/strict'
import {messageStatus} from '../src/message-status.ts'
test('three receipt states use persisted recipient cursors',()=>{assert.equal(messageStatus('12','0','0'),'sent');assert.equal(messageStatus('12','12','0'),'delivered');assert.equal(messageStatus('12','12','12'),'read');assert.equal(messageStatus('13','12','12'),'sent')})
test('read takes precedence and big message IDs stay exact',()=>{assert.equal(messageStatus('9007199254740993','9007199254740992','0'),'sent');assert.equal(messageStatus('12','0','12'),'read');assert.equal(messageStatus('12',undefined,undefined),'sent')})
