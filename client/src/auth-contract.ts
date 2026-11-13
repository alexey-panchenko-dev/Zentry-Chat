import type {User} from './api'
export type LoginResponse={kind:'user';user:User}|{kind:'mfa';challenge:string}
export function parseLoginResponse(value:unknown):LoginResponse{
 if(value&&typeof value==='object'){
  const data=value as Record<string,unknown>
  if(data.mfa_required===true){if(typeof data.challenge==='string'&&/^[a-f0-9]{64}$/.test(data.challenge))return {kind:'mfa',challenge:data.challenge}}
  else if(data.user&&typeof data.user==='object'){const user=data.user as User;if(typeof user.id==='string'&&/^[a-f0-9-]{36}$/i.test(user.id)&&typeof user.name==='string'&&user.name.length>0)return {kind:'user',user}}
 }
 throw Error('Сервер вернул несовместимый ответ входа. Обнови страницу и попробуй снова.')
}
export function requireLoginUser(value:unknown):User{const result=parseLoginResponse(value);if(result.kind!=='user')throw Error('Для входа требуется второй фактор. Начни вход заново.');return result.user}
