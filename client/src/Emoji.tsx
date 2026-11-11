import { memo, useState } from 'react'
import { emojiKey, emojiParts } from './emoji-utils'

export const Emoji = memo(function Emoji({ emoji }: { emoji: string }) {
  const key = emojiKey(emoji)
  const [failed, setFailed] = useState<string | null>(null)
  return key && failed !== key ? <img className="z-emoji" src={`/emoji/apple/${key}.png`} width="24" height="24" alt={emoji} loading="lazy" decoding="async" draggable={false} onError={() => setFailed(key)}/> : <>{emoji}</>
})

export const EmojiText = memo(function EmojiText({ text }: { text: string }) {
  return <>{emojiParts(text).map((part, index) => part.key ? <Emoji key={index} emoji={part.text}/> : part.text)}</>
})
