// A missing custom image retains the saved palette without a broken-image icon.
export function initAuthorBackgrounds(){
 document.querySelectorAll<HTMLImageElement>('.author-backdrop img').forEach(image=>{
  const fallback=()=>{image.hidden=true;image.style.display='none'}
  image.addEventListener('error',fallback,{once:true})
  if(image.complete&&image.naturalWidth===0)fallback()
 })
}
