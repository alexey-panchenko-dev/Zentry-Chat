export function photoInputError(file:{type:string;size:number}) {
 return !['image/jpeg','image/png'].includes(file.type)||file.size<1||file.size>15*1024*1024
  ? 'Выберите JPG или PNG размером до 15 МБ.' : ''
}
export function photoDimensions(width:number,height:number,maxEdge=512):[number,number] {
 if(!Number.isFinite(width)||!Number.isFinite(height)||width<1||height<1)throw new Error('Не удалось прочитать размер фото.')
 // The API normalizes to PNG. 512px also bounds normalized PNG data
 // below its 2 MiB storage limit, even for detailed/noisy photographs.
 const scale=Math.min(1,maxEdge/Math.max(width,height))
 return [Math.max(1,Math.round(width*scale)),Math.max(1,Math.round(height*scale))]
}
type PhotoPreparation={maxEdge:number;maxBytes:number;name:string}
async function preparePhoto(file:File,options:PhotoPreparation,signal?:AbortSignal):Promise<File> {
 const error=photoInputError(file)
 if(error)throw new Error(error)
 const url=URL.createObjectURL(file)
 try {
  const image=new Image()
  signal?.throwIfAborted()
  await new Promise<void>((resolve,reject)=>{
   const cleanup=()=>{signal?.removeEventListener('abort',abort);image.onload=null;image.onerror=null}
   const abort=()=>{cleanup();image.src='';reject(new DOMException('Подготовка фото отменена.','AbortError'))}
   image.onload=()=>{cleanup();resolve()}
   image.onerror=()=>{cleanup();reject(new Error('Не удалось открыть фото. Выберите другой JPG или PNG.'))}
   signal?.addEventListener('abort',abort,{once:true});image.src=url
  })
  signal?.throwIfAborted()
  const [width,height]=photoDimensions(image.naturalWidth,image.naturalHeight,options.maxEdge)
  const canvas=document.createElement('canvas')
  canvas.width=width;canvas.height=height
  const context=canvas.getContext('2d')
  if(!context)throw new Error('Браузер не смог подготовить фото.')
  context.fillStyle='#ffffff';context.fillRect(0,0,width,height)
  context.drawImage(image,0,0,width,height)
  const blob=await new Promise<Blob>((resolve,reject)=>canvas.toBlob(value=>value?resolve(value):reject(new Error('Не удалось подготовить фото.')),'image/jpeg',0.85))
  signal?.throwIfAborted()
  if(blob.size>options.maxBytes)throw new Error('Не удалось уменьшить фото. Выберите другое изображение.')
  return new File([blob],options.name,{type:'image/jpeg'})
 }finally{URL.revokeObjectURL(url)}
}

export function prepareProfilePhoto(file:File):Promise<File>{return preparePhoto(file,{maxEdge:512,maxBytes:2*1024*1024,name:'profile.jpg'})}
export function prepareChatPhoto(file:File,signal?:AbortSignal):Promise<File>{return preparePhoto(file,{maxEdge:1600,maxBytes:4*1024*1024,name:'photo.jpg'},signal)}
