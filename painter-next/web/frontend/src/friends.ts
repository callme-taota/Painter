// Initials are the default, including no-JS reading. Reveal only decoded images.
export function initFriendAvatars(){
 for(const image of document.querySelectorAll<HTMLImageElement>('[data-friend-avatar]')){
  const failed=()=>{image.hidden=true}
  const loaded=()=>{image.setAttribute('data-avatar-loaded','');image.hidden=false}
  image.addEventListener('error',failed,{once:true})
  image.addEventListener('load',loaded,{once:true})
  if(image.complete){if(image.naturalWidth>0)loaded();else failed()}
 }
}
