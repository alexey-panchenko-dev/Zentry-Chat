import {test} from 'node:test'
import assert from 'node:assert/strict'
import {photoInputError, photoDimensions} from '../src/profile-photo.ts'
test('phone photo up to 15 MiB is accepted and larger or unsupported input rejected',()=>{
 assert.equal(photoInputError({type:'image/jpeg',size:15*1024*1024}),'')
 assert.ok(photoInputError({type:'image/png',size:15*1024*1024+1}))
 assert.ok(photoInputError({type:'image/svg+xml',size:100}))
 assert.ok(photoInputError({type:'image/jpeg',size:0}))
})
test('phone photo is reduced proportionally without upscaling small images',()=>{
 assert.deepEqual(photoDimensions(6000,4000),[1024,683])
 assert.deepEqual(photoDimensions(3000,6000),[512,1024])
 assert.deepEqual(photoDimensions(200,100),[200,100])
 assert.throws(()=>photoDimensions(0,200))
})
