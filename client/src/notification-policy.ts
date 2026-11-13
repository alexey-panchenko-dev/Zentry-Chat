export type NotificationPolicy={sound_messages:boolean;push_enabled:boolean;sound_calls:boolean;mentions_enabled:boolean;dnd_until:string|null;dnd_mute_message_sound:boolean;dnd_suppress_push:boolean;dnd_mute_call_sound:boolean;dnd_suppress_mentions:boolean;quiet_enabled:boolean;quiet_days:number;quiet_start:string;quiet_end:string;timezone:string}
export type ChatMode='all'|'silent'|'mentions'|'off'
export function notificationDecision(global:NotificationPolicy,mode:ChatMode,mutedUntil:string|null,kind:'message'|'mention'|'call'|'important',scope:'personal'|'group',now:Date,exception?:{allow_dnd_push:boolean;allow_call_sound:boolean;important:boolean}){
 let push=global.push_enabled,visual=true,messageSound=kind==='message'&&global.sound_messages,callSound=kind==='call'&&global.sound_calls,mention=kind==='mention'&&global.mentions_enabled
 if(mutedUntil){const until=Date.parse(mutedUntil);if(Number.isFinite(until)){if(now.getTime()<until)mode='off';else if(mode==='off')mode='all'}}
 if(mode==='off'){push=false;visual=false;messageSound=false;callSound=false;mention=false}else if(mode==='silent'){messageSound=false;callSound=false}else if(mode==='mentions'&&kind!=='mention'){push=false;visual=false;messageSound=false;callSound=false;mention=false}
 const dndUntil=global.dnd_until?Date.parse(global.dnd_until):NaN
 const dnd=Number.isFinite(dndUntil)&&now.getTime()<dndUntil||inQuietSchedule(global,now)
 const allowed=!!exception&&(exception.important&&(kind==='mention'||kind==='important')||kind==='call'&&exception.allow_call_sound)
 if(dnd&&!allowed){if(global.dnd_suppress_push)push=false;if(global.dnd_mute_message_sound)messageSound=false;if(global.dnd_mute_call_sound)callSound=false;if(kind==='mention'&&global.dnd_suppress_mentions){mention=false;visual=false}}
 if(dnd&&allowed&&exception?.allow_dnd_push&&mode!=='off')push=global.push_enabled
 if(scope==='group'&&mode==='mentions'&&kind!=='mention'){push=false;visual=false}
 return {push,visual,messageSound,callSound,mention}
}
function inQuietSchedule(p:NotificationPolicy,now:Date){if(!p.quiet_enabled)return false;try{
 const parts=new Intl.DateTimeFormat('en-US',{timeZone:p.timezone,weekday:'short',hour:'2-digit',minute:'2-digit',hourCycle:'h23'}).formatToParts(now)
 const values=Object.fromEntries(parts.map(x=>[x.type,x.value]));const days=['Mon','Tue','Wed','Thu','Fri','Sat','Sun'];const day=days.indexOf(values.weekday);if(day<0)return false
 const minute=Number(values.hour)*60+Number(values.minute),[sh,sm]=p.quiet_start.split(':').map(Number),[eh,em]=p.quiet_end.split(':').map(Number),start=sh*60+sm,end=eh*60+em,today=(p.quiet_days&(1<<day))!==0
 if(start===end)return p.quiet_days!==0;if(start<end)return today&&minute>=start&&minute<end;if(minute>=start)return today;if(minute<end)return(p.quiet_days&(1<<((day+6)%7)))!==0;return false
 }catch{return false}}
