import { useRef } from 'react'
import { FileUp, ImagePlus, Plus } from 'lucide-react'
import { ActionMenu } from './ActionMenu'
import { ImageComposer } from './ImageComposer'
import { FileComposer } from './FileComposer'
import { MediaAvailability } from './MediaAvailability'
import { useMediaAvailability } from './useMediaAvailability'
import type { Message } from './api'

export function Attachments({ conversationID, groupID, disabled, photoDisabled, fileDisabled, onPhotoActive, onPhotoSent, onFileSent }: {
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
  const file = useRef<HTMLInputElement>(null)
  const media = useMediaAvailability('chat_images')
  const supportsPhoto = !!groupID || media.supported
  const photoComposer = <ImageComposer conversationID={conversationID} groupID={groupID}
    pickerRef={photo} hideTrigger disabled={disabled || photoDisabled === true}
    onActiveChange={onPhotoActive} onSent={onPhotoSent}/>
  return <>
    <ActionMenu className="z-attachments-menu" label="Добавить вложение" disabled={disabled} icon={<Plus size={20}/>}>
      <button type="button" disabled={disabled || photoDisabled || !supportsPhoto} onClick={() => photo.current?.click()}><ImagePlus size={18}/>Фото</button>
      <button type="button" disabled={disabled || fileDisabled} onClick={() => file.current?.click()}><FileUp size={18}/>Файл или видео</button>
    </ActionMenu>
    {groupID ? photoComposer : <MediaAvailability feature="chat_images">{photoComposer}</MediaAvailability>}
    <FileComposer conversationID={conversationID} groupID={groupID} pickerRef={file} hideTrigger disabled={disabled || fileDisabled === true} onSent={onFileSent}/>
  </>
}
