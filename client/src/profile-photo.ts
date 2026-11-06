export function photoInputError(file:{type:string;size:number}) {
 return !['image/jpeg','image/png'].includes(file.type)||file.size<1||file.size>15*1024*1024
  ? 'Выберите JPG или PNG размером до 15 МБ.' : ''
}
export function photoDimensions(width:number,height:number):[number,number] {
 if(!Number.isFinite(width)||!Number.isFinite(height)||width<1||height<1)throw new Error('Не удалось прочитать размер фото.')
 const scale=Math.min(1,1024/Math.max(width,height))
 return [Math.max(1,Math.round(width*scale)),Math.max(1,Math.round(height*scale))]
}
export async function prepareProfilePhoto(file:File):Promise<File> {
 const error=photoInputError(file)
 if(error)throw new Error(error)
 const url=URL.createObjectURL(file)
 try {
  const image=new Image()
  await new Promise<void>((resolve,reject)=>{image.onload=()=>resolve();image.onerror=()=>reject(new Error('Не удалось открыть фото. Выберите другой JPG или PNG.'));image.src=url})
  const [width,height]=photoDimensions(image.naturalWidth,image.naturalHeight)
  const canvas=document.createElement('canvas')
  canvas.width=width;canvas.height=height
  const context=canvas.getContext('2d')
  if(!context)throw new Error('Браузер не смог подготовить фото.')
  context.fillStyle='#ffffff';context.fillRect(0,0,width,height)
  context.drawImage(image,0,0,width,height)
  const blob=await new Promise<Blob>((resolve,reject)=>canvas.toBlob(value=>value?resolve(value):reject(new Error('Не удалось подготовить фото.')),'image/jpeg',0.85))
  if(blob.size>2*1024*1024)throw new Error('Не удалось уменьшить фото. Выберите другое изображение.')
  return new File([blob],'profile.jpg',{type:'image/jpeg'})
 }finally{URL.revokeObjectURL(url)}
}
