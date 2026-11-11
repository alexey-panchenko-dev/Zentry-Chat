export function maskBirthDate(value:string):string {
 const digits=value.replace(/\D/g,'').slice(0,8)
 return [digits.slice(0,2),digits.slice(2,4),digits.slice(4,8)].filter(Boolean).join('.')
}
export function parseBirthDate(value:string,today=new Date()):string {
 const match=/^(\d{2})\.(\d{2})\.(\d{4})$/.exec(value)
 const invalid=()=>new Error('Для регистрации нужно указать настоящую дату рождения и быть не младше 14 лет.')
 if(!match)throw invalid()
 const [,day,month,year]=match;const date=new Date(Number(year),Number(month)-1,Number(day))
 const end=new Date(today.getFullYear(),today.getMonth(),today.getDate())
 const age=end.getFullYear()-date.getFullYear()-((end.getMonth()<date.getMonth()||(end.getMonth()===date.getMonth()&&end.getDate()<date.getDate()))?1:0)
 if(Number(year)<1900||date.getFullYear()!==Number(year)||date.getMonth()!==Number(month)-1||date.getDate()!==Number(day)||date>=end||age<14)throw invalid()
 return `${year}-${month}-${day}`
}
export function formatBirthDate(value?:string):string {
 const match=/^(\d{4})-(\d{2})-(\d{2})$/.exec(value??'')
 return match?`${match[3]}.${match[2]}.${match[1]}`:''
}
export function birthdayLabel(value?:string,today=new Date()):{label:string;today:boolean}|null {
 const match=/^(\d{2})-(\d{2})$/.exec(value??'');if(!match)return null
 const month=Number(match[1]),day=Number(match[2]),date=new Date(2000,month-1,day)
 if(date.getMonth()!==month-1||date.getDate()!==day)return null
 return {label:date.toLocaleDateString('ru-RU',{day:'numeric',month:'long'}),today:today.getMonth()===month-1&&today.getDate()===day}
}
export function ageFromBirthDate(iso:string,today=new Date()):number {
 const [year,month,day]=iso.split('-').map(Number)
 return today.getFullYear()-year-(today.getMonth()+1<month||(today.getMonth()+1===month&&today.getDate()<day)?1:0)
}
export function birthDatesSupported(config:unknown):boolean {
 return !!config&&typeof config==='object'&&'capabilities' in config&&!!config.capabilities&&typeof config.capabilities==='object'&&'birth_dates' in config.capabilities&&config.capabilities.birth_dates===true
}
