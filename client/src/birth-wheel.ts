export type BirthParts={day:string;month:string;year:string}
export type BirthPart='day'|'month'|'year'
export function splitBirthDate(value:string):BirthParts{
 const [day='',month='',year='']=value.split('.')
 return {day:day.slice(0,2),month:month.slice(0,2),year:year.slice(0,4)}
}
export function joinBirthDate(parts:BirthParts):string{return [parts.day,parts.month,parts.year].join('.')}
export function stepBirthPart(value:string,part:BirthPart,delta:number,yearNow=new Date().getFullYear()):string{
 const min=part==='year'?1900:1,max=part==='day'?31:part==='month'?12:yearNow
 const current=Number(value),start=Number.isFinite(current)&&current>=min&&current<=max?current:(part==='year'?yearNow-18-Math.sign(delta):delta>0?min-1:max+1)
 const next=start+Math.sign(delta)
 const stepped=part==='year'?Math.min(max,Math.max(min,next)):((next-min+max-min+1)%(max-min+1))+min
 return String(stepped).padStart(part==='year'?4:2,'0')
}
export function limitBirthInput(value:string,part:BirthPart,yearNow=new Date().getFullYear()):string{
 const digits=value.replace(/\D/g,'').slice(0,part==='year'?4:2)
 if(!digits)return ''
 const max=part==='day'?31:part==='month'?12:yearNow
 const number=Number(digits)
 if(number===0)return '0'
 if(number>max)return String(max).padStart(part==='year'?4:2,'0')
 return digits
}
export function normalizeBirthInput(value:string,part:BirthPart):string{
 if(!value)return ''
 return value.replace(/\D/g,'').slice(0,part==='year'?4:2).padStart(part==='year'?4:2,'0')
}
