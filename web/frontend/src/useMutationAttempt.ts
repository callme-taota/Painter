import {readonly,shallowRef} from 'vue'
import {APIError} from './api'
import {randomID} from './random'

type Command = {path:string;method:'POST'|'PUT'|'PATCH'|'DELETE';version?:number|string;payload:unknown}
export type MutationAttempt = Readonly<{path:string;method:Command['method'];version?:number|string;body:string;key:string}>

// Only use for endpoints backed by the server's idempotency ledger. An
// unconfirmed command keeps its exact wire body, version and key until the
// caller validates an acknowledgement or explicitly abandons the editor.
export function useMutationAttempt() {
  const pending=shallowRef<MutationAttempt|null>(null)
  function begin(command:()=>Command):MutationAttempt {
    if(pending.value)return pending.value
    const {payload,...request}=command()
    const body=JSON.stringify(payload)
    if(body===undefined)throw new TypeError('Mutation payload must be JSON')
    return pending.value=Object.freeze({...request,body,key:randomID()})
  }
  function failed(cause:unknown) {
    // A network error, a malformed success, 408, 429 or 5xx does not establish
    // whether a prior attempt committed. Never manufacture a fresh retry key.
    if(cause instanceof APIError&&cause.status>=400&&cause.status<500&&![408,429].includes(cause.status))clear()
  }
  function clear(){pending.value=null}
  return {pending:readonly(pending),begin,failed,clear}
}
