export type SquareAvatarCrop={x:number;y:number;size:number}
const clamp=(value:number,min:number,max:number)=>Math.min(max,Math.max(min,Number.isFinite(value)?value:min))
export function squareAvatarCrop(width:number,height:number,zoom=1,x=.5,y=.5):SquareAvatarCrop {
 if(!Number.isFinite(width)||!Number.isFinite(height)||width<1||height<1)throw new Error('Не удалось прочитать размер фото.')
 const size=Math.min(width,height)/clamp(zoom,1,3)
 return {x:(width-size)*clamp(x,0,1),y:(height-size)*clamp(y,0,1),size}
}
export function moveAvatarCrop(width:number,height:number,zoom:number,x:number,y:number,dx:number,dy:number,viewportSize:number):{x:number;y:number} {
 const crop=squareAvatarCrop(width,height,zoom,x,y)
 if(!Number.isFinite(viewportSize)||viewportSize<=0)return {x:clamp(x,0,1),y:clamp(y,0,1)}
 return {x:width>crop.size?clamp((crop.x-dx*crop.size/viewportSize)/(width-crop.size),0,1):clamp(x,0,1),y:height>crop.size?clamp((crop.y-dy*crop.size/viewportSize)/(height-crop.size),0,1):clamp(y,0,1)}
}
export async function renderAvatarCrop(image:HTMLImageElement,crop:SquareAvatarCrop):Promise<File> {
 const canvas=document.createElement('canvas');canvas.width=512;canvas.height=512
 const context=canvas.getContext('2d');if(!context)throw new Error('Браузер не смог подготовить фото.')
 context.fillStyle='#ffffff';context.fillRect(0,0,512,512)
 context.drawImage(image,crop.x,crop.y,crop.size,crop.size,0,0,512,512)
 const blob=await new Promise<Blob>((resolve,reject)=>canvas.toBlob(value=>value?resolve(value):reject(new Error('Не удалось подготовить фото.')),'image/jpeg',.85))
 if(blob.size>2*1024*1024)throw new Error('Не удалось уменьшить фото. Выберите другое изображение.')
 return new File([blob],'avatar.jpg',{type:'image/jpeg'})
}
