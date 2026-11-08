import {test} from 'node:test'
import assert from 'node:assert/strict'
import {countryForPhone,formatPhoneInput,phoneCountries} from '../src/phone-number.ts'

test('common international prefixes resolve to the longest known calling code',()=>{
 assert.equal(countryForPhone('+380671234567').id,'UA')
 assert.equal(countryForPhone('+420777123456').id,'CZ')
 assert.equal(phoneCountries.some(country=>country.id==='KZ'&&country.code==='+7'),true)
})

test('Kazakhstan and Russia phone numbers are displayed in readable groups',()=>{
 assert.equal(formatPhoneInput('+77777777777','+7'),'+7 777 777 7777')
 assert.equal(formatPhoneInput('7777777777','+7'),'+7 777 777 7777')
})

test('country-specific national formats are preserved',()=>{
 assert.equal(formatPhoneInput('+420777123456','+420'),'+420 777 123 456')
 assert.equal(formatPhoneInput('+380671234567','+380'),'+380 67 123 45 67')
})
