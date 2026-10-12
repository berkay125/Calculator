import { useEffect } from 'react'
import { isDeadCaret, keyToAction } from './keymap.ts'
import type { Action } from './reducer.ts'

// One window listener for the whole app. `onAction` must be stable across
// renders; `dispatch` is.
export function useKeyboard(onAction: (action: Action) => void) {
  useEffect(() => {
    // A dead `^` is inserted when it is pressed. Windows still delivers the
    // finished `^` with the next key, which must not insert a second one.
    let afterDeadCaret = false

    function onKeyDown(event: KeyboardEvent) {
      // A keypad key reached with Tab presses itself on Enter.
      if (event.key === 'Enter' && event.target instanceof HTMLButtonElement) {
        return
      }
      const duplicate = afterDeadCaret && event.key === '^'
      afterDeadCaret = isDeadCaret(event)
      if (duplicate) {
        event.preventDefault()
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
