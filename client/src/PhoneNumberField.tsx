import { useState } from 'react'
import type { Ref } from 'react'
import { countryForCode, countryForPhone, formatPhoneInput, phoneCountries } from './phone-number'

export function PhoneNumberField({value,onChange,disabled=false,inputRef,invalid=false,describedBy}:{value:string;onChange:(value:string)=>void;disabled?:boolean;inputRef?:Ref<HTMLInputElement>;invalid?:boolean;describedBy?:string}){
 const [countryID,setCountryID]=useState(()=>countryForPhone(value).id)
 const country=countryForCode(countryID)
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
  }else onChange(formatPhoneInput(raw,country.code))
 }
 return <div className="z-phone-control"><select aria-label="Страна и телефонный код" value={countryID} disabled={disabled} onChange={event=>chooseCountry(event.target.value)}>{phoneCountries.map(item=><option key={item.id} value={item.id}>{item.label} ({item.code})</option>)}</select><input ref={inputRef} type="tel" inputMode="tel" autoComplete="tel" placeholder={`${country.code} 777 777 7777`} maxLength={24} value={value} disabled={disabled} aria-invalid={invalid} aria-describedby={describedBy} onChange={event=>edit(event.target.value)}/></div>
}
