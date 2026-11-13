import assert from 'node:assert/strict'
import {chromium} from 'playwright'

const base=process.env.ZENTLY_E2E_URL??'http://127.0.0.1:5189'
if(!['127.0.0.1','localhost'].includes(new URL(base).hostname))throw Error('This fixture test only permits a local Zently server')
const user={id:'11111111-1111-4111-8111-111111111111',name:'Fixture',username:'fixture'}
const peer={id:'22222222-2222-4222-8222-222222222222',name:'Friend',username:'friend'}
const cid='33333333-3333-4333-8333-333333333333',gid='44444444-4444-4444-8444-444444444444',pid='55555555-5555-4555-8555-555555555555'
const now=new Date().toISOString()
const group={id:gid,name:'Test Group',owner_id:user.id,role:'owner',member_count:3,created_at:now,updated_at:now,message_count:1}
const browser=await chromium.launch({headless:true})
const failures=[]
let debugPage
async function harness(legacy=false,authenticated=false){
 const context=await browser.newContext({viewport:{width:1280,height:900},serviceWorkers:'block'})
 const page=await context.newPage();page.setDefaultTimeout(12000);page.on('pageerror',error=>failures.push(error.message))
 const state={authenticated,enabled:false,scheduled:[],calls:[],polls:new Map(),messages:[{id:'1',group_id:gid,sender_id:peer.id,sender_name:peer.name,client_message_id:pid,body:'Опрос: Choose',kind:'text',created_at:now,...(legacy?{}:{poll_id:pid})}]}
 state.polls.set(pid,{id:pid,question:'Choose',multiple:false,closes_at:null,closed:false,can_close:true,total:0,options:[{id:'66666666-6666-4666-8666-666666666666',label:'First',votes:0,mine:false},{id:'77777777-7777-4777-8777-777777777777',label:'Second',votes:0,mine:false}]})
 await context.route('**/*',async route=>{
  const req=route.request(),url=new URL(req.url());if(!['127.0.0.1','localhost'].includes(url.hostname)){await route.abort();return}
  if(!url.pathname.startsWith('/api/')){await route.continue();return}
  const p=url.pathname.slice(4),method=req.method();state.calls.push(method+' '+p)
  let body={};if(method!=='GET'){try{body=req.postDataJSON()??{}}catch{}}
  const reply=(data,status=200)=>route.fulfill({status,contentType:'application/json',body:status===204?'':JSON.stringify(data)})
  if(p==='/auth/me')return reply(state.authenticated?{user}:{error:'unauthorized'},state.authenticated?200:401)
  if(p==='/auth/login')return reply({mfa_required:true,challenge:'a'.repeat(64)},202)
  if(p==='/auth/login/mfa'){state.authenticated=true;return reply({user})}
  if(p==='/health')return reply({status:'ok',version:'fixture',capabilities:legacy?{birth_dates:true}:{birth_dates:true,session_devices:true,totp:true,scheduled_messages:true,group_polls:true}})
  if(p==='/realtime/config')return reply({enabled:false,media:{groups:true,contact_profiles:true,chat_controls:true,chat_removal:true,chat_images:true,profile_photos:true,voice:true}})
  if(p==='/conversations')return reply({conversations:[{id:cid,participant:peer,last_message:'Hello',last_activity:now}],next_cursor:''})
  if(p==='/groups')return reply({groups:[group]})
  if(p==='/notes')return reply({group:{...group,id:'88888888-8888-4888-8888-888888888888',name:'Мои заметки',member_count:1}})
  if(p==='/unread')return reply({unread_chats:0})
  if(p==='/delivery')return reply({deliveries:[],conversations:[]})
  if(p==='/presence')return reply({},204)
  if(p==='/profile/privacy')return reply({messages:'everyone',calls:'everyone',presence:'everyone',search:'everyone',birthday:'nobody'})
  if(p==='/profile/contact')return reply({phone:'',phone_visibility:'nobody',birthday_visibility:'nobody'})
  if(p==='/profile')return reply({profile:{...user,age:20,bio:'Local fixture',city:''}})
  if(p==='/push/config')return reply({enabled:false,public_key:''})
  if(p==='/calls')return reply({calls:[],call:null})
  if(p==='/security/sessions')return reply({sessions:state.revoked?[{id:'current',user_agent:'Windows Chrome/123',created_at:now,last_seen_at:now,expires_at:now,current:true}]:[{id:'current',user_agent:'Windows Chrome/123',created_at:now,last_seen_at:now,expires_at:now,current:true},{id:'other',user_agent:'Linux Firefox/123',created_at:now,last_seen_at:now,expires_at:now,current:false}]})
  if(p==='/security/sessions/revoke-others'){assert.equal(body.confirm,true);state.revoked=true;return reply({},204)}
  if(p==='/security/events')return reply({events:[{id:'1',kind:'login',created_at:now}]})
  if(p==='/security/mfa')return reply({enabled:state.enabled,configured:true})
  if(p==='/security/mfa/setup')return reply({secret:'LOCALFIXTUREKEY',uri:'otpauth://totp/Zently',qr:'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jW/8AAAAASUVORK5CYII='})
  if(p==='/security/mfa/verify')return reply({backup_codes:Array.from({length:10},(_,i)=>'fixture-backup-'+i)})
  if(p==='/security/mfa/activate'){assert.equal(body.saved,true);state.enabled=true;return reply({},204)}
  if(p==='/scheduled'&&method==='GET')return reply({scheduled:state.scheduled})
  if(p==='/scheduled'&&method==='POST'){state.scheduled.push({id:'job-1',...body,status:'pending',failure:''});return reply({scheduled:state.scheduled.at(-1)},201)}
  if(p==='/scheduled/job-1'&&method==='PUT'){Object.assign(state.scheduled[0],body);return reply({},204)}
  if(p==='/scheduled/job-1'&&method==='DELETE'){state.scheduled[0].status='cancelled';return reply({},204)}
  if(p===`/groups/${gid}`)return reply({group})
  if(p===`/groups/${gid}/members`)return reply({members:[{...user,role:'owner'},{...peer,role:'member'}]})
  if(p==='/groups/contacts')return reply({contacts:[]})
  if(p===`/groups/${gid}/messages`&&method==='POST'){
   if(legacy)assert.deepEqual(Object.keys(body).sort(),['body','client_message_id','reply_to_id'].filter(key=>body[key]!==undefined).sort())
   const message={...body,id:String(state.messages.length+1),group_id:gid,sender_id:user.id,sender_name:user.name,kind:'text',created_at:now}
   state.messages.unshift(message);return reply({message},201)
  }
  if(p===`/groups/${gid}/messages`)return reply({messages:state.messages,next_cursor:''})
  if(p===`/groups/${gid}/polls`&&method==='POST'){const id='99999999-9999-4999-8999-999999999999';state.polls.set(id,{id,question:body.question,multiple:body.multiple,closes_at:body.closes_at,closed:false,can_close:true,total:0,options:body.options.map((label,i)=>({id:'option-'+i,label,mine:false,votes:0}))});state.messages.unshift({id:'2',group_id:gid,sender_id:user.id,sender_name:user.name,body:'Опрос: '+body.question,kind:'text',created_at:now,poll_id:id});return reply({poll:state.polls.get(id)},201)}
  const pollMatch=p.match(/^\/groups\/[^/]+\/polls\/([^/]+)(\/vote|\/close)?$/)
  if(pollMatch){const poll=state.polls.get(pollMatch[1]);if(!poll)return reply({error:'not_found'},404);if(pollMatch[2]==='/vote'){poll.total=body.option_ids.length?1:0;poll.options.forEach(option=>{option.mine=body.option_ids.includes(option.id);option.votes=option.mine?1:0})}if(pollMatch[2]==='/close'){poll.closed=true;return reply({},204)}return reply({poll})}
  if(p===`/conversations/${cid}/profile`)return reply({...peer,display_name:peer.name,online:false,sent_count:0,received_count:0})
  if(p===`/conversations/${cid}/controls`)return reply({hidden:false,blocked_by_me:false,blocked_me:false,cleared_through:'0'})
  if(p===`/conversations/${cid}/messages`)return reply({messages:[],next_cursor:''})
  if(p===`/conversations/${cid}/read`)return reply({unread_count:0,last_message_id:'',peer_last_read_id:''})
  if(p.endsWith('/pins'))return reply({messages:[]})
  if(p.endsWith('/search'))return reply({messages:url.searchParams.get('type')==='links'?[{id:'9',created_at:now,body:'https://trusted.example@evil.example/path'}]:[],next_cursor:''})
  return reply({})
 })
 await page.goto(base);return {context,page,state}
}
try{
 const {context,page,state}=await harness()
 debugPage=page
 await page.getByLabel('Почта',{exact:true}).fill('fixture@example.test')
 await page.getByLabel('Пароль',{exact:true}).fill('Fixture1&password')
 await page.locator('form').getByRole('button',{name:'Войти',exact:true}).click()
 await page.getByRole('heading',{name:'Подтверди вход'}).waitFor()
 assert.equal(await page.locator('.z-shell').count(),0)
 await page.getByLabel('Код подтверждения',{exact:true}).fill('123456')
 await page.locator('form').getByRole('button',{name:'Войти',exact:true}).click()
 await page.getByRole('button',{name:'Настройки',exact:true}).click()
 await page.getByRole('heading',{name:'Активные устройства'}).waitFor()
 await page.getByRole('button',{name:'Завершить все остальные сессии',exact:true}).click()
 assert.equal(state.revoked,undefined)
 await page.locator('.z-device-settings .notice').getByRole('button',{name:'Завершить',exact:true}).click()
 await page.getByText('Linux · Firefox',{exact:true}).waitFor({state:'detached'})
 await page.getByRole('button',{name:'Включить 2FA',exact:true}).click()
 await page.locator('.z-mfa-settings').getByLabel('Текущий пароль').fill('Fixture1&password')
 await page.locator('.z-mfa-settings').getByRole('button',{name:'Продолжить',exact:true}).click()
 await page.getByAltText('QR-код подключения Zently к аутентификатору').waitFor()
 await page.locator('.z-mfa-settings').getByLabel('Код из приложения',{exact:true}).fill('123456')
 await page.getByRole('button',{name:'Проверить код',exact:true}).click()
 await page.getByLabel('Одноразовые резервные коды').waitFor()
 assert.equal(state.enabled,false)
 await page.getByLabel('Я сохранил резервные коды').check()
 await page.getByRole('button',{name:'Активировать 2FA',exact:true}).click()
 await page.getByRole('button',{name:'Отключить 2FA',exact:true}).waitFor()
 await page.locator('.z-conversation').filter({hasText:'Friend'}).click()
 await page.locator('.z-composer-field textarea').fill('Planned from composer')
 await page.getByRole('button',{name:'Добавить вложение',exact:true}).click()
 await page.getByRole('button',{name:'Отложенные сообщения',exact:true}).click()
 const schedule=page.getByRole('dialog',{name:'Отложенные сообщения',exact:true})
 await schedule.getByRole('button',{name:'Запланировать',exact:true}).click()
 await schedule.locator('.z-scheduled-item').waitFor()
 assert.equal(await page.locator('.z-composer-field textarea').inputValue(),'')
 await schedule.getByRole('button',{name:'Изменить',exact:true}).click()
 await schedule.getByLabel('Сообщение',{exact:true}).fill('Edited scheduled text')
 await schedule.getByRole('button',{name:'Сохранить изменения',exact:true}).click()
 await schedule.locator('.z-scheduled-item').getByText('Edited scheduled text',{exact:true}).waitFor()
 await schedule.getByRole('button',{name:'Отменить отправку',exact:true}).click()
 await schedule.locator('.z-scheduled-item').waitFor({state:'detached'})
 await schedule.getByRole('button',{name:'Закрыть',exact:true}).click()
 await page.locator('.z-conversation').filter({hasText:'Test Group'}).click()
 await page.locator('.z-group-poll').getByText('Choose',{exact:true}).waitFor()
 await page.locator('.z-group-poll input').first().check()
 await page.locator('.z-group-poll').getByRole('button',{name:'Голосовать',exact:true}).click()
 await page.getByText('Участников проголосовало: 1',{exact:true}).waitFor()
 await page.getByRole('button',{name:'Добавить вложение',exact:true}).click()
 await page.getByRole('button',{name:'Создать опрос',exact:true}).click()
 const create=page.getByRole('dialog',{name:'Опрос в группе',exact:true})
 await create.getByLabel('Вопрос',{exact:true}).fill('New local poll')
 await create.getByLabel('Вариант 1',{exact:true}).fill('Yes')
 await create.getByLabel('Вариант 2',{exact:true}).fill('No')
 await create.getByRole('button',{name:'Создать опрос',exact:true}).click()
 await page.locator('.z-group-poll').getByText('New local poll',{exact:true}).waitFor()
 await page.locator('.z-conversation').filter({hasText:'Friend'}).click()
 await page.getByRole('button',{name:'Открыть профиль: Friend',exact:true}).click()
 await page.getByRole('tab',{name:'Медиа',exact:true}).click()
 await page.getByRole('button',{name:'Ссылки',exact:true}).click()
 await page.locator('.z-risky-link').waitFor()
 assert.equal(await page.locator('.z-link-warning').isVisible(),false)
 await page.locator('.z-risky-link').click()
 await page.getByRole('dialog',{name:'Проверь ссылку',exact:true}).getByText('Домен: evil.example',{exact:true}).waitFor()
 await page.keyboard.press('Escape')
 await page.locator('.z-link-warning').waitFor({state:'hidden'})
 await page.setViewportSize({width:375,height:812})
 assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth+1),true)
 await context.close()
 const old=await harness(true,true)
 await old.page.getByRole('button',{name:'Настройки',exact:true}).click()
 await old.page.getByText('Управление устройствами пока недоступно на сервере.',{exact:true}).waitFor()
 await old.page.getByLabel('Звуки сообщений',{exact:true}).waitFor()
 assert.equal(old.state.calls.some(value=>value.includes('/security/')),false)
 await old.page.locator('.z-conversation').filter({hasText:'Test Group'}).click()
 await old.page.locator('.z-composer-field textarea').fill('Legacy server message')
 await old.page.getByRole('button',{name:'Отправить сообщение группе',exact:true}).click()
 await old.page.locator('.z-bubble').getByText('Legacy server message',{exact:true}).waitFor()
 assert.equal(old.state.calls.some(value=>value.includes('/notifications')||value.endsWith('/typing')),false)
 await old.context.close()
 assert.deepEqual(failures,[])
 console.log('PASS: MFA login/enrollment, session confirmation, scheduling/edit/cancel, polls/vote/create, safe links, mobile width, legacy API gating')
}catch(error){if(debugPage&&!debugPage.isClosed())console.error('Local fixture diagnostics:',JSON.stringify({errors:failures,dialogs:await debugPage.locator('dialog').evaluateAll(elements=>elements.map(element=>({open:element.open,text:element.textContent.slice(0,1200),labels:[...element.querySelectorAll('label')].map(label=>label.textContent)})))}));throw error}finally{await browser.close()}
