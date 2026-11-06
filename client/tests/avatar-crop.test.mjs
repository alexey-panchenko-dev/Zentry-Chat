import {test} from 'node:test'
import assert from 'node:assert/strict'
import {squareAvatarCrop,moveAvatarCrop} from '../src/avatar-crop.ts'
test('square avatar crop centers landscape and portrait without stretching',()=>{assert.deepEqual(squareAvatarCrop(4000,2000,1,.5,.5),{x:1000,y:0,size:2000});assert.deepEqual(squareAvatarCrop(2000,4000,1,.5,.5),{x:0,y:1000,size:2000});assert.deepEqual(squareAvatarCrop(512,512,1,.5,.5),{x:0,y:0,size:512})})
test('zoom and position remain inside source bounds',()=>{for(const [width,height] of [[4000,2000],[2000,4000],[512,512]])for(const zoom of [0,1,2,3,99])for(const position of [-10,0,.5,1,10]){const crop=squareAvatarCrop(width,height,zoom,position,position);assert.ok(crop.x>=0&&crop.y>=0);assert.ok(crop.size<=Math.min(width,height));assert.ok(crop.x+crop.size<=width+1e-8);assert.ok(crop.y+crop.size<=height+1e-8)}assert.equal(squareAvatarCrop(3000,2000,2,1,1).size,1000)})
test('drag position matches source movement and clamps edges',()=>{assert.deepEqual(moveAvatarCrop(4000,2000,1,.5,.5,100,0,200),{x:0,y:.5});assert.deepEqual(moveAvatarCrop(4000,2000,1,.5,.5,-9999,9999,200),{x:1,y:.5});assert.deepEqual(moveAvatarCrop(2000,4000,1,.5,.5,0,100,200),{x:.5,y:0})})
test('invalid source dimensions cannot create a crop',()=>{for(const size of [0,-1,NaN,Infinity])assert.throws(()=>squareAvatarCrop(size,100,1,.5,.5))})
