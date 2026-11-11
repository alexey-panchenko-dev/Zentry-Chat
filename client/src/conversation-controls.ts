export type ConversationControls={blocked_by_me:boolean;blocked_me:boolean;cleared_through:string;hidden?:boolean}
export type ControlAction='clear'|'block'|'unblock'|'delete_chat'|'delete_everyone'|'remove_contact'
export function messageVisible(id:string,clearedThrough='0'):boolean {
 try{return BigInt(id)>BigInt(clearedThrough||'0')}catch{return false}
}
export function blockNotice(state:Pick<ConversationControls,'blocked_by_me'|'blocked_me'>):string {
 if(state.blocked_by_me&&state.blocked_me)return 'Вы заблокировали контакт. Вас заблокировали :('
 if(state.blocked_me)return 'Вас заблокировали :('
 return state.blocked_by_me?'Вы заблокировали контакт.':''
}

export type RemovalAction='delete_chat'|'delete_everyone'|'remove_contact'
export function isRemovalAction(action:ControlAction):action is RemovalAction{return action==='delete_chat'||action==='delete_everyone'||action==='remove_contact'}
export function removalCommitted(action:ControlAction,state:{hidden?:boolean}):boolean{return isRemovalAction(action)&&state.hidden===true}
export function withoutConversation<Page extends{conversations:Array<{id:string}>}>(pages:Page[],id:string):Page[]{return pages.map(page=>({...page,conversations:page.conversations.filter(conversation=>conversation.id!==id)}))}
