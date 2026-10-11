import { useEffect } from 'react'
import { keyToAction } from './keymap.ts'
import type { Action } from './reducer.ts'

// One window listener for the whole app. `onAction` must be stable across
// renders; `dispatch` is.
export function useKeyboard(onAction: (action: Action) => void) {
  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      // A keypad key reached with Tab presses itself on Enter.
      if (event.key === 'Enter' && event.target instanceof HTMLButtonElement) {
        return
      }
      const action = keyToAction(event)
      if (action === undefined) {
        return
      }
      // This also keeps `/` from opening Firefox's quick find.
      event.preventDefault()
      onAction(action)
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [onAction])
}
