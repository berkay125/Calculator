import { useEffect, useReducer } from 'react'
import type { Calculate } from '../api/client.ts'
import { initialState, reducer } from './reducer.ts'

// Runs the reducer and sends its pending request. `calculate` must be stable
// across renders, or every render would restart the request.
export function useCalculator(calculate: Calculate) {
  const [state, dispatch] = useReducer(reducer, initialState)
  const { pending } = state

  // `pending` is a new object for every request, so this runs once per
  // request id. An edit clears `pending`, and the cleanup aborts the request.
  useEffect(() => {
    if (pending === null) {
      return
    }
    const controller = new AbortController()
    const { id, expression, ans } = pending
    calculate(expression, ans, controller.signal).then(
      (result) => dispatch({ type: 'response', id, result }),
      () => {
        // calculate rejects only when aborted, and then nobody needs the
        // response.
      },
    )
    return () => controller.abort()
  }, [pending, calculate])

  return { state, dispatch }
}
