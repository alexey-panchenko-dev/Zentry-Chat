import { test } from 'node:test'
import assert from 'node:assert/strict'
import { normalizeContactPhone, CallRequestGate } from '../src/contact-profile.ts'
test('phone normalization allows international formatting and removal',()=>{assert.equal(normalizeContactPhone('+7 (701) 123-45-67'),'+77011234567');assert.equal(normalizeContactPhone('  '),'');assert.equal(normalizeContactPhone('+12345678'),'+12345678')})
test('phone validation rejects local, short, long and letter values',()=>{for(const value of ['87011234567','+01234567','+123','+1234567890123456','+7abc12345678'])assert.throws(()=>normalizeContactPhone(value))})
test('call requests execute once and busy requests never queue for later',()=>{const gate=new CallRequestGate();assert.equal(gate.consume({id:'1',conversationID:'chat'},false),'chat');assert.equal(gate.consume({id:'1',conversationID:'chat'},false),null);assert.equal(gate.consume({id:'2',conversationID:'chat'},true),null);assert.equal(gate.consume({id:'2',conversationID:'chat'},false),null);assert.equal(gate.consume({id:'3',conversationID:'next'},false),'next')})
