import {computed,onScopeDispose,ref,type Ref} from 'vue'
import {appendUnique,list,type Row} from './api'

type CursorListOptions<T extends Row> = {
  request:(cursor:string,signal:AbortSignal)=>Promise<Row>
  disabled?:()=>boolean
  parse?:(row:Row)=>T
  received?:(page:Row)=>void
  failed?:(cause:unknown)=>void
  error?:Ref<string>
}

// One owner per list: replacement reads cancel the previous generation, while
// append errors retain the current rows and cursor for an explicit retry.
export function useCursorList<T extends Row=Row>(options:CursorListOptions<T>) {
  const items = ref<T[]>([]) as Ref<T[]>
  const loading = ref(true), error = options.error || ref(''), cursor = ref('')
  const hasMore = computed(()=>!!cursor.value)
  let generation = 0, disposed = false, controller:AbortController|undefined

  async function load(append=false):Promise<boolean> {
    if (disposed || (append && (loading.value || options.disabled?.() || !cursor.value))) return false
    const at = ++generation, requestedCursor = append ? cursor.value : ''
    controller?.abort()
    controller = new AbortController()
    if (!append) cursor.value = ''
    loading.value = true
    error.value = ''
    try {
      const page = await options.request(requestedCursor,controller.signal)
      if (disposed || at !== generation) return false
      const incoming = list(page).map(row=>options.parse ? options.parse(row) : row as T)
      options.received?.(page)
      items.value = append ? appendUnique(items.value,incoming) : incoming
      const next = page.has_more === false ? '' : String(page.next_cursor || '')
      // A broken cursor must not trigger an infinite auto-load loop.
      cursor.value = next === requestedCursor ? '' : next
      return true
    } catch (cause) {
      if (!disposed && at === generation && (cause as Error).name !== 'AbortError') {
        error.value = (cause as Error).message
        options.failed?.(cause)
      }
      return false
    } finally {
      if (!disposed && at === generation) loading.value = false
    }
  }

  onScopeDispose(()=>{disposed=true;generation++;controller?.abort()})
  return {items,loading,error,cursor,hasMore,load}
}
