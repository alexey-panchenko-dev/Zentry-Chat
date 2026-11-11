const rememberedAccountsKey = 'zently-remembered-accounts'
export function rememberedAccounts(): string[] {
  try {
    const value: unknown = JSON.parse(localStorage.getItem(rememberedAccountsKey) ?? '[]')
    return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string' && item.includes('@')) : []
  } catch { return [] }
}
export function rememberAccount(email: string) {
  const normalized = email.trim().toLocaleLowerCase()
  localStorage.setItem(rememberedAccountsKey, JSON.stringify([normalized, ...rememberedAccounts().filter(item => item !== normalized)].slice(0, 5)))
}
