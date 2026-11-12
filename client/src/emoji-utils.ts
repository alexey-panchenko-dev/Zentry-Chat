import keys from './emoji-data/keys.json' with { type: 'json' }

const supported = new Set(keys)
const segmenter = new Intl.Segmenter('ru', { granularity: 'grapheme' })
export function emojiKey(value: string): string | null {
  if (value.includes('\ufe0e')) return null
  const key = Array.from(value).filter(part => part !== '\ufe0f').map(part => part.codePointAt(0)!.toString(16)).join('-')
  return supported.has(key) ? key : null
}
export function emojiParts(text: string): { text: string; key: string | null }[] {
  const parts: { text: string; key: string | null }[] = []
  for (const { segment } of segmenter.segment(text)) {
    const key = emojiKey(segment)
    const previous = parts.at(-1)
    if (!key && previous && !previous.key) previous.text += segment
    else parts.push({ text: segment, key })
  }
  return parts
}

export function isSingleEmoji(text: string): boolean {
  const parts = emojiParts(text.trim())
  return parts.length === 1 && parts[0].key !== null
}

