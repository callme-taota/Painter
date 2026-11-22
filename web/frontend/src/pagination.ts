// Keep existing filters intact when refreshing or moving to the next page.
export function withCursor(path:string,cursor=''):string {
  const [base,query=''] = path.split('?',2)
  const params = new URLSearchParams(query)
  params.delete('cursor')
  if (cursor) params.set('cursor',cursor)
  const search = params.toString()
  return base + (search ? '?' + search : '')
}
