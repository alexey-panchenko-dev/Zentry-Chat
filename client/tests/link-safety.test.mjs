import { test } from 'node:test'
import assert from 'node:assert/strict'
import { externalHttpURL,externalLinkHost,linkRiskReason } from '../src/link-safety.ts'

test('ordinary secure links open without an extra warning', () => {
  assert.equal(linkRiskReason('https://example.com/path?q=1'), null)
})

test('only web schemes can open and the display host excludes misleading credentials',()=>{
 for(const value of ['javascript:alert(1)','data:text/html,test','file:///C:/test','vbscript:msgbox(1)','not-a-url'])assert.equal(externalHttpURL(value),null)
 assert.equal(externalLinkHost('https://trusted.example@evil.example/path'),'evil.example')
 assert.match(externalLinkHost('https://пример.рф'),/xn--/)
 assert.equal(externalHttpURL('https://example.com'),'https://example.com/')
})

test('plaintext, address literals, disguised domains, shorteners and embedded credentials warn', () => {
  assert.match(linkRiskReason('http://example.com'), /HTTPS/)
  assert.match(linkRiskReason('https://192.168.1.1/login'), /IP-адрес/)
  assert.match(linkRiskReason('https://xn--e1afmkfd.example'), /международные символы/)
  assert.match(linkRiskReason('https://bit.ly/secret'), /сокращатель/)
  assert.match(linkRiskReason('https://user:pass@example.com'), /встроенные данные/)
  assert.match(linkRiskReason('not-a-url'), /не удалось проверить/)
})
