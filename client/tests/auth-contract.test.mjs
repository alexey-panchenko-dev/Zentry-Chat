import {test} from 'node:test'
import assert from 'node:assert/strict'
import {parseLoginResponse,requireLoginUser} from '../src/auth-contract.ts'
test('MFA challenge can never be mistaken for an authenticated account',()=>{const value={mfa_required:true,challenge:'a'.repeat(64)};assert.equal(parseLoginResponse(value).kind,'mfa');assert.throws(()=>requireLoginUser(value))})
test('malformed successful responses cannot open a session view',()=>{for(const value of [{},{user:{}},{mfa_required:true,challenge:'short'},{user:{id:'x',name:'Name'}},null])assert.throws(()=>parseLoginResponse(value))})
test('existing user response stays compatible',()=>{const user={id:'11111111-1111-4111-8111-111111111111',name:'Fixture'};assert.deepEqual(requireLoginUser({user}),user)})
