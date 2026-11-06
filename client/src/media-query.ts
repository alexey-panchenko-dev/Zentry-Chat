// The same media query controls layout and capture visibility. React owns cleanup.
export function createMediaQueryStore(media: MediaQueryList) {
 return {
  getSnapshot: () => media.matches,
  subscribe: (listener: () => void) => {
   media.addEventListener('change', listener)
   return () => media.removeEventListener('change', listener)
  },
 }
}
