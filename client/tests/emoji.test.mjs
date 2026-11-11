import { test } from 'node:test'
import assert from 'node:assert/strict'
import { emojiKey, emojiParts } from '../src/emoji-utils.ts'

test('emoji artwork resolves selectors, flags, skin tones and joined sequences', () => {
  assert.equal(emojiKey('❤️'), '2764')
  assert.equal(emojiKey('👍🏻'), '1f44d-1f3fb')
  assert.equal(emojiKey('🇰🇿'), '1f1f0-1f1ff')
  assert.ok(emojiKey('👩‍💻'))
  assert.equal(emojiKey('hello'), null)
  assert.equal(emojiKey('❤︎'), null)
})
test('message text keeps ordinary text and complete emoji sequences intact', () => {
  const parts = emojiParts('Привет, <script>! 👩‍💻 и 👍🏻')
  assert.deepEqual(parts.filter(part => part.key).map(part => part.text), ['👩‍💻','👍🏻'])
  assert.equal(parts.map(part => part.text).join(''), 'Привет, <script>! 👩‍💻 и 👍🏻')
  assert.equal(parts[0].key, null)
})
