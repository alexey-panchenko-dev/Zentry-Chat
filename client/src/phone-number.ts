export type PhoneCountry={id:string;label:string;code:string}

export const phoneCountries:PhoneCountry[]=[
 {id:'KZ',label:'Казахстан',code:'+7'},{id:'RU',label:'Россия',code:'+7'},{id:'UA',label:'Украина',code:'+380'},{id:'CZ',label:'Чехия',code:'+420'},
 {id:'US',label:'США',code:'+1'},{id:'CA',label:'Канада',code:'+1'},{id:'GB',label:'Великобритания',code:'+44'},{id:'PL',label:'Польша',code:'+48'},
 {id:'DE',label:'Германия',code:'+49'},{id:'FR',label:'Франция',code:'+33'},{id:'IT',label:'Италия',code:'+39'},{id:'ES',label:'Испания',code:'+34'},
 {id:'TR',label:'Турция',code:'+90'},{id:'AM',label:'Армения',code:'+374'},{id:'GE',label:'Грузия',code:'+995'},{id:'UZ',label:'Узбекистан',code:'+998'},
 {id:'KG',label:'Кыргызстан',code:'+996'},{id:'BY',label:'Беларусь',code:'+375'},{id:'AZ',label:'Азербайджан',code:'+994'},{id:'MD',label:'Молдова',code:'+373'},
 {id:'NL',label:'Нидерланды',code:'+31'},{id:'BE',label:'Бельгия',code:'+32'},{id:'AT',label:'Австрия',code:'+43'},{id:'CH',label:'Швейцария',code:'+41'},
 {id:'SE',label:'Швеция',code:'+46'},{id:'NO',label:'Норвегия',code:'+47'},{id:'DK',label:'Дания',code:'+45'},{id:'FI',label:'Финляндия',code:'+358'},
 {id:'PT',label:'Португалия',code:'+351'},{id:'GR',label:'Греция',code:'+30'},{id:'IL',label:'Израиль',code:'+972'},{id:'AE',label:'ОАЭ',code:'+971'},
 {id:'IN',label:'Индия',code:'+91'},{id:'CN',label:'Китай',code:'+86'},{id:'JP',label:'Япония',code:'+81'},{id:'KR',label:'Южная Корея',code:'+82'},
 {id:'AU',label:'Австралия',code:'+61'},{id:'BR',label:'Бразилия',code:'+55'},{id:'MX',label:'Мексика',code:'+52'},{id:'AR',label:'Аргентина',code:'+54'},
 {id:'ZA',label:'ЮАР',code:'+27'},{id:'EG',label:'Египет',code:'+20'},{id:'VN',label:'Вьетнам',code:'+84'},{id:'TH',label:'Таиланд',code:'+66'},
 {id:'ID',label:'Индонезия',code:'+62'},{id:'PH',label:'Филиппины',code:'+63'},{id:'SG',label:'Сингапур',code:'+65'},{id:'HK',label:'Гонконг',code:'+852'}
]

export function countryForPhone(value:string){
 const digits=value.replace(/\D/g,'')
 return [...phoneCountries].sort((a,b)=>b.code.length-a.code.length).find(country=>digits.startsWith(country.code.slice(1)))??phoneCountries[0]
}

function groupsForNational(code:string,digits:string){
 if(code==='+7')return [digits.slice(0,3),digits.slice(3,6),digits.slice(6,10)].filter(Boolean)
 if(code==='+1')return [digits.slice(0,3),digits.slice(3,6),digits.slice(6,10)].filter(Boolean)
 if(code==='+380')return [digits.slice(0,2),digits.slice(2,5),digits.slice(5,7),digits.slice(7,9)].filter(Boolean)
 if(code==='+420')return [digits.slice(0,3),digits.slice(3,6),digits.slice(6,9)].filter(Boolean)
 return [digits.slice(0,3),digits.slice(3,6),digits.slice(6,9),digits.slice(9,12)].filter(Boolean)
}

export function formatPhoneInput(value:string,code:string){
 const raw=value.trim()
 if(!raw)return ''
 const digits=raw.replace(/\D/g,'')
 const prefix=code.replace(/\D/g,'')
 const national=raw.startsWith('+')&&digits.startsWith(prefix)?digits.slice(prefix.length):digits
 return `${code} ${groupsForNational(code,national).join(' ')}`.trimEnd()
}

export function countryForCode(id:string){return phoneCountries.find(country=>country.id===id)??phoneCountries[0]}

export function formatManualPhone(value:string){
 const raw=value.trim()
 if(!raw.startsWith('+'))return formatPhoneInput(raw,phoneCountries[0].code)
 const country=countryForPhone(raw)
 const digits=raw.replace(/\D/g,'')
 if(digits.startsWith(country.code.slice(1)))return formatPhoneInput(raw,country.code)
 return '+'+digits
}
