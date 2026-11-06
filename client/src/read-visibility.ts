// Loading data does not acknowledge it. Require viewport intersection and focus.
export function observeReadReceipt(latest: string, marker: Element, acknowledged: {value:string}, send: (latest:string)=>Promise<unknown>, changed: ()=>void) {
    let intersecting = false
    let disposed = false
    let pending = false
    let timer: ReturnType<typeof setTimeout> | undefined
    const acknowledge = () => {
      clearTimeout(timer)
      if (!intersecting || document.visibilityState !== 'visible' || !document.hasFocus() || BigInt(latest)<=BigInt(acknowledged.value)) return
      timer = setTimeout(() => {
        if(disposed || pending || !intersecting || document.visibilityState !== 'visible' || !document.hasFocus()) return
        pending = true
        void send(latest).then(() => {
          if(disposed) return
          acknowledged.value = latest
          changed()
        }).catch(() => { if (!disposed) timer=setTimeout(acknowledge, 5000) }).finally(() => {pending=false})
      }, 250)
    }
    const observer = new IntersectionObserver(entries => {intersecting=entries.some(entry=>entry.isIntersecting);acknowledge()})
    observer.observe(marker)
    document.addEventListener('visibilitychange', acknowledge)
    window.addEventListener('focus', acknowledge)
    return () => {disposed=true;clearTimeout(timer);observer.disconnect();document.removeEventListener('visibilitychange',acknowledge);window.removeEventListener('focus',acknowledge)}
}
