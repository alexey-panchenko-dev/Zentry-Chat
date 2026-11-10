type DraftStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

export function messageDraftKey(userID: string, conversationID: string) {
  return `zently-message-draft:${encodeURIComponent(userID)}:${encodeURIComponent(conversationID)}`
}

export function readMessageDraft(storage: DraftStorage, userID: string, conversationID: string) {
  try {
    return storage.getItem(messageDraftKey(userID, conversationID))?.slice(0, 1000) ?? ''
  } catch {
    return ''
  }
}

export function writeMessageDraft(storage: DraftStorage, userID: string, conversationID: string, body: string) {
  const key = messageDraftKey(userID, conversationID)
  try {
    if (body) storage.setItem(key, body.slice(0, 1000))
    else storage.removeItem(key)
  } catch {
    // Storage can be unavailable in private browsing or when the device is full.
  }
}

export function replaceDraftRecord(storage: DraftStorage, key: string, expected: string | null, next: string) {
  if (storage.getItem(key) !== expected) return false
  storage.setItem(key, next)
  return true
}
