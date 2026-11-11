import { Emoji } from './Emoji'
import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { ActionMenu } from './ActionMenu'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowDownLeft, Pin, Reply, X } from 'lucide-react'
import { useFavoriteChange } from './ChatWorkspace'
import { api, errorText } from './api'
import type { Conversation, Message, MessageReaction } from './api'

const reactions=['❤️','👍','😂','😮','😢','🔥']

export function MessageActions({message,userID,conversationID,onReply,onChanged}:{message:Message;userID:string;conversationID:string;onReply:(message:Message)=>void;onChanged:()=>void}){
 const cache=useQueryClient();const [editing,setEditing]=useState(false);const [body,setBody]=useState(message.body);const [error,setError]=useState('');const [forwardOpen,setForwardOpen]=useState(false);const [forwardQuery,setForwardQuery]=useState('');const [forwardTargets,setForwardTargets]=useState<Conversation[]>([]);const [forwardBusy,setForwardBusy]=useState(false)
 const query=useQuery({queryKey:['message-reactions',message.id],enabled:message.reactions===undefined,queryFn:async()=> (await api.get<{reactions:MessageReaction[]}>(`/messages/${message.id}/reactions`)).data.reactions,retry:false})
 async function react(emoji:string){try{const result=(await api.post<{reactions:MessageReaction[]}>(`/messages/${message.id}/reactions`,{emoji})).data;cache.setQueryData(['message-reactions',message.id],result.reactions);onChanged()}catch(e){setError(errorText(e))}}
 async function edit(){try{await api.put(`/messages/${message.id}`,{body});setEditing(false);setError('');onChanged()}catch(e){setError(errorText(e))}}
 async function pin(){try{await api.post(`/conversations/${conversationID}/pins`,{message_id:message.id,pinned:true});onChanged()}catch(e){setError(errorText(e))}}
 async function openForward(){setError('');setForwardQuery('');try{const response=await api.get<{conversations:Conversation[]}>('/conversations',{params:{limit:100}});setForwardTargets(response.data.conversations.filter(item=>item.id!==conversationID));setForwardOpen(true)}catch(e){setError(errorText(e))}}
 async function forward(target:Conversation){setForwardBusy(true);try{await api.post(`/conversations/${target.id}/forward`,{message_id:message.id,client_message_id:crypto.randomUUID()});setForwardOpen(false);setError('Переслано.');onChanged()}catch(e){setError(errorText(e))}finally{setForwardBusy(false)}}
 const forwardDialog=useRef<HTMLDialogElement>(null)
 useEffect(()=>{const element=forwardDialog.current;if(forwardOpen)element?.showModal();return()=>element?.close()},[forwardOpen])
 const favorite=useFavoriteChange()
 const visibleReactions=message.reactions??query.data??[]
 const canEdit=message.sender_id===userID&&(!message.kind||message.kind==='text')&&Date.now()-Date.parse(message.created_at)<900_000
 return <div className="z-message-tools" onDoubleClick={event=>event.stopPropagation()}>
  {visibleReactions.length? <div className="z-reaction-list">{visibleReactions.map(item=><button type="button" key={item.emoji} className={item.mine?'is-mine':''} aria-label={`Реакция ${item.emoji}: ${item.count}`} onClick={()=>void react(item.mine?'':item.emoji)}><Emoji emoji={item.emoji}/><span>{item.count}</span></button>)}</div>:null}
  <ActionMenu className="z-message-menu" label="Действия с сообщением"><button type="button" onClick={()=>onReply(message)}><Reply size={15}/>Ответить</button>{canEdit&&<button type="button" onClick={()=>{setBody(message.body);setEditing(true)}}>Изменить</button>}<button type="button" onClick={()=>void pin()}><Pin size={15}/>Закрепить</button><button type="button" onClick={()=>void openForward()}><ArrowDownLeft size={15}/>Переслать</button><button type="button" disabled={favorite.isPending} onClick={()=>favorite.mutate({scope:"personal",id:message.id,save:true},{onSuccess:()=>setError("Добавлено в избранное."),onError:e=>setError(errorText(e))})}>В избранное</button>{message.body&&<button type="button" onClick={()=>void navigator.clipboard.writeText(message.body).then(()=>setError("Скопировано."),()=>setError("Не удалось скопировать."))}>Копировать</button>}<div className="z-reaction-picker" aria-label="Выбрать реакцию">{reactions.map(emoji=><button type="button" key={emoji} aria-label={`Поставить ${emoji}`} onClick={()=>void react(emoji)}><Emoji emoji={emoji}/></button>)}</div></ActionMenu>
  {editing&&<div className="z-edit-message"><textarea autoFocus aria-label="Редактировать сообщение" maxLength={1000} value={body} onChange={event=>setBody(event.target.value)}/><button type="button" disabled={!body.trim()} onClick={()=>void edit()}>Сохранить</button><button type="button" onClick={()=>setEditing(false)}>Отмена</button></div>}
  {forwardOpen&&createPortal(<dialog ref={forwardDialog} className="z-forward-dialog" aria-label="Переслать сообщение" onCancel={event=>{event.preventDefault();if(!forwardBusy)setForwardOpen(false)}}><header><strong>Переслать в чат</strong><button type="button" className="z-icon" aria-label="Закрыть" onClick={()=>setForwardOpen(false)}><X size={15}/></button></header><input autoFocus value={forwardQuery} onChange={event=>setForwardQuery(event.target.value)} placeholder="Найти чат…"/><div>{forwardTargets.filter(target=>(target.display_name||target.participant.name).toLocaleLowerCase().includes(forwardQuery.toLocaleLowerCase())).map(target=><button type="button" key={target.id} disabled={forwardBusy} onClick={()=>void forward(target)}>{target.display_name||target.participant.name}</button>)}</div>{forwardTargets.length===0&&<small>Нет других чатов для пересылки.</small>}</dialog>,document.body)}
  {error&&<span role="status" className="z-feature-error">{error}</span>}
 </div>
}
