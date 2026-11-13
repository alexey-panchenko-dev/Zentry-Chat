import { useRef,useState } from 'react'
import {useQuery} from '@tanstack/react-query'
import {api} from './api'
import {ScheduleDialog} from './ScheduleDialog'
import {CreatePollDialog} from './GroupPoll'
import { FileUp, ImagePlus, Plus } from 'lucide-react'
import { ActionMenu } from './ActionMenu'
import { ImageComposer } from './ImageComposer'
import { FileComposer } from './FileComposer'
import { MediaAvailability } from './MediaAvailability'
import { useMediaAvailability } from './useMediaAvailability'
import type { Message } from './api'

export function Attachments({ conversationID, groupID, disabled, photoDisabled, fileDisabled, onPhotoActive, onPhotoSent, onFileSent,scheduleText='',onScheduled,allowPolls=false }: {
  allowPolls?:boolean
  scheduleText?:string
  onScheduled?:(body:string)=>void
  conversationID?: string
  groupID?: string
  disabled: boolean
  photoDisabled?: boolean
  fileDisabled?: boolean
  onPhotoActive: (active: boolean) => void
  onPhotoSent: (result: { message: Message; realtime: string }) => void
  onFileSent: (message: Message) => void
}) {
  const photo = useRef<HTMLInputElement>(null)
  const [scheduleOpen,setScheduleOpen]=useState(false)
  const [pollOpen,setPollOpen]=useState(false)
  const health=useQuery({queryKey:['api-health'],queryFn:async()=>(await api.get<{capabilities?:Record<string,boolean>}>('/health')).data,retry:false,staleTime:60_000})
  const scheduleSupported=health.data?.capabilities?.scheduled_messages===true
  const file = useRef<HTMLInputElement>(null)
  const media = useMediaAvailability('chat_images')
  const supportsPhoto = !!groupID || media.supported
  const photoComposer = <ImageComposer conversationID={conversationID} groupID={groupID}
    pickerRef={photo} hideTrigger disabled={disabled || photoDisabled === true}
    onActiveChange={onPhotoActive} onSent={onPhotoSent}/>
  return <>
    <ActionMenu className="z-attachments-menu" label="Добавить вложение" disabled={disabled&&!scheduleSupported} icon={<Plus size={20}/>}>
      <button type="button" disabled={disabled || photoDisabled || !supportsPhoto} onClick={() => photo.current?.click()}><ImagePlus size={18}/>Фото</button>
      <button type="button" disabled={disabled || fileDisabled} onClick={() => file.current?.click()}><FileUp size={18}/>Файл или видео</button>
      {scheduleSupported&&<button type="button" onClick={()=>setScheduleOpen(true)}>Отложенные сообщения</button>}
      {groupID&&allowPolls&&health.data?.capabilities?.group_polls===true&&<button type="button" disabled={disabled} onClick={()=>setPollOpen(true)}>Создать опрос</button>}
    </ActionMenu>
    {scheduleOpen&&<ScheduleDialog scope={groupID?'group':'personal'} chatID={groupID??conversationID!} text={scheduleText} onClose={()=>setScheduleOpen(false)} onScheduled={onScheduled}/>}
    {pollOpen&&groupID&&<CreatePollDialog groupID={groupID} onClose={()=>setPollOpen(false)}/>}
    {groupID ? photoComposer : <MediaAvailability feature="chat_images">{photoComposer}</MediaAvailability>}
    <FileComposer conversationID={conversationID} groupID={groupID} pickerRef={file} hideTrigger disabled={disabled || fileDisabled === true} onSent={onFileSent}/>
  </>
}
