import { useState } from 'react'
import type { Ref } from 'react'
import { countryForCode, countryForPhone, formatPhoneInput, phoneCountries } from './phone-number'

export function PhoneNumberField({value,onChange,disabled=false,inputRef,invalid=false,describedBy}:{value:string;onChange:(value:string)=>void;disabled?:boolean;inputRef?:Ref<HTMLInputElement>;invalid?:boolean;describedBy?:string}){
 const [countryID,setCountryID]=useState(()=>countryForPhone(value).id)
 const country=countryForCode(countryID)
 const prefix=country.code.replace(/\D/g,'')
 const digits=value.replace(/\D/g,'')
 const national=value.trim().startsWith('+')&&digits.startsWith(prefix)?digits.slice(prefix.length):digits
 const displayNational=national?formatPhoneInput(national,country.code).slice(country.code.length).trim():''
 const nationalLimit=country.code==='+7'||country.code==='+1'?10:country.code==='+380'||country.code==='+420'?9:15-prefix.length
 function chooseCountry(id:string){
  const next=countryForCode(id)
  const digits=value.replace(/\D/g,'')
  const oldPrefix=country.code.replace(/\D/g,'')
  const national=value.trim().startsWith('+')&&digits.startsWith(oldPrefix)?digits.slice(oldPrefix.length):digits
  setCountryID(id)
  onChange(national?formatPhoneInput('+'+next.code.replace(/\D/g,'')+national,next.code):next.code+' ')
 }
 function edit(raw:string){
  if(raw.trim().startsWith('+')){
   const detected=countryForPhone(raw)
   setCountryID(detected.id)
   onChange(formatPhoneInput(raw,detected.code))
  }else onChange(formatPhoneInput(raw.replace(/\D/g,'').slice(0,nationalLimit),country.code))
 }
 return <div className="z-phone-control"><select aria-label="Страна и телефонный код" value={countryID} disabled={disabled} onChange={event=>chooseCountry(event.target.value)}>{phoneCountries.map(item=><option key={item.id} value={item.id}>{item.label} ({item.code})</option>)}</select><input ref={inputRef} type="tel" inputMode="numeric" autoComplete="tel-national" placeholder="___ ___ ____" maxLength={nationalLimit+4} value={displayNational} disabled={disabled} aria-invalid={invalid} aria-describedby={describedBy} onChange={event=>edit(event.target.value)}/></div>
}
