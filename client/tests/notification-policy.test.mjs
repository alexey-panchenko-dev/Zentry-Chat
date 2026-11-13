import test from 'node:test'
import assert from 'node:assert/strict'
import {notificationDecision} from '../src/notification-policy.ts'

const base={sound_messages:true,push_enabled:true,sound_calls:true,mentions_enabled:true,dnd_until:null,dnd_mute_message_sound:true,dnd_suppress_push:true,dnd_mute_call_sound:true,dnd_suppress_mentions:false,quiet_enabled:false,quiet_days:127,quiet_start:'23:00',quiet_end:'09:00',timezone:'UTC'}
test('chat silent keeps push and visual notice but mutes sound',()=>{assert.deepEqual(notificationDecision(base,'silent',null,'message','personal',new Date()),{push:true,visual:true,messageSound:false,callSound:false,mention:false})})
test('DND silences selected channels without affecting message delivery or visual in-app notice',()=>{assert.deepEqual(notificationDecision({...base,dnd_until:'2030-01-01T00:00:00Z'},'all',null,'message','personal',new Date('2029-01-01T00:00:00Z')),{push:false,visual:true,messageSound:false,callSound:false,mention:false})})
test('chat mute expires and stops suppressing the next message',()=>{assert.equal(notificationDecision(base,'off','2020-01-01T00:00:00Z','message','personal',new Date()).push,true)})
test('weekly quiet schedule uses recipient timezone and carries overnight period to selected prior day',()=>{const p={...base,quiet_enabled:true,quiet_days:1,quiet_start:'23:00',quiet_end:'09:00',timezone:'Asia/Qyzylorda'};const d=notificationDecision(p,'all',null,'message','personal',new Date('2025-01-07T00:00:00Z'));assert.equal(d.push,false)})
test('mentions-only keeps mentions and suppresses ordinary group messages',()=>{assert.equal(notificationDecision(base,'mentions',null,'message','group',new Date()).push,false);assert.equal(notificationDecision(base,'mentions',null,'mention','group',new Date()).push,true)})
