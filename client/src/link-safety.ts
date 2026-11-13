const shortenerHosts = new Set([
  'bit.ly', 't.co', 'tinyurl.com', 'tiny.cc', 'cutt.ly', 'is.gd', 'rb.gy', 'shorturl.at',
])

export function externalHttpURL(value:string):string|null{
  try{const url=new URL(value);return url.protocol==='https:'||url.protocol==='http:'?url.href:null}catch{return null}
}
export function externalLinkHost(value:string):string{
  try{return new URL(value).host}catch{return 'Неизвестный адрес'}
}

function isIPAddress(host: string): boolean {
  const value = host.replace(/^\[|\]$/g, '')
  if (value.includes(':')) return true
  return /^\d{1,3}(?:\.\d{1,3}){3}$/.test(value)
}

export function linkRiskReason(value: string): string | null {
  let url: URL
  try { url = new URL(value) } catch { return 'адрес не удалось проверить' }
  if (url.protocol !== 'https:') return 'соединение не использует HTTPS'
  if (url.username || url.password) return 'адрес содержит встроенные данные для входа'
  const host = url.hostname.toLowerCase().replace(/\.$/, '')
  if (isIPAddress(host)) return 'ссылка ведёт напрямую на IP-адрес'
  if (host.split('.').some(label => label.startsWith('xn--'))) return 'домен использует международные символы, которые могут выглядеть как латиница'
  if (shortenerHosts.has(host)) return 'сокращатель скрывает конечный адрес страницы'
  return null
}
