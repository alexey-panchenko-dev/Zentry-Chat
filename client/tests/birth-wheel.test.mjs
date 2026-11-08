import test from 'node:test'
import assert from 'node:assert/strict'
import {joinBirthDate,limitBirthInput,normalizeBirthInput,splitBirthDate,stepBirthPart} from '../src/birth-wheel.ts'

test('birth date wheel keeps day, month and year fields separately',()=>{
 const parts=splitBirthDate('17.12.2007')
 assert.deepEqual(parts,{day:'17',month:'12',year:'2007'})
 assert.equal(joinBirthDate(parts),'17.12.2007')
})
test('day and month drums wrap around, and an empty field starts at a useful date',()=>{
 assert.equal(stepBirthPart('','day',1,2026),'01')
 assert.equal(stepBirthPart('31','day',1,2026),'01')
 assert.equal(stepBirthPart('01','day',-1,2026),'31')
 assert.equal(stepBirthPart('01','month',-1,2026),'12')
 assert.equal(stepBirthPart('12','month',1,2026),'01')
 assert.equal(stepBirthPart('2007','year',-1,2026),'2006')
 assert.equal(stepBirthPart('2026','year',1,2026),'2026')
})
test('manual date input accepts digits only and clamps invalid day or month values',()=>{
 assert.equal(limitBirthInput('3a2','day',2026),'31')
 assert.equal(limitBirthInput('13','month',2026),'12')
 assert.equal(limitBirthInput('202','year',2026),'202')
})
test('leaving a one-digit day or month completes it to the visible two-digit format',()=>{
 assert.equal(normalizeBirthInput('7','day'),'07')
 assert.equal(normalizeBirthInput('3','month'),'03')
 assert.equal(normalizeBirthInput('7','year'),'0007')
 assert.equal(normalizeBirthInput('','day'),'')
})
