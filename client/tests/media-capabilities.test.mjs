import { test } from 'node:test'
import assert from 'node:assert/strict'
import { mediaCapability } from '../src/media-capabilities.ts'
test('old server and absent flags keep media disabled',()=>{assert.equal(mediaCapability(undefined,'voice'),false);assert.equal(mediaCapability({enabled:true},'profile_photos'),false);assert.equal(mediaCapability({media:{}},'voice'),false)})
test('new server explicitly enables each capability',()=>{assert.equal(mediaCapability({media:{voice:true,profile_photos:true}},'voice'),true);assert.equal(mediaCapability({media:{voice:true,profile_photos:true}},'profile_photos'),true)})
test('partial or malformed flags never enable other features',()=>{assert.equal(mediaCapability({media:{voice:true}},'voice'),true);assert.equal(mediaCapability({media:{voice:true}},'profile_photos'),false);assert.equal(mediaCapability({media:{voice:'true'}},'voice'),false)})
