import type { Action, Token } from './reducer.ts'

export type KeyInput = Pick<
  KeyboardEvent,
  'key' | 'code' | 'ctrlKey' | 'metaKey' | 'altKey'
>

function input(token: Token): Action {
  return { type: 'input', token }
}

const KEYS = new Map<string, Action>([
  ...[...'0123456789.+-*/^%()'].map((c): [string, Action] => [c, input(c as Token)]),
  ['s', input('sqrt(')],
  ['S', input('sqrt(')],
  ['a', input('Ans')],
  ['A', input('Ans')],
  ['Enter', { type: 'evaluate' }],
  ['=', { type: 'evaluate' }],
  ['Backspace', { type: 'backspace' }],
  ['Escape', { type: 'clear' }],
])

// The action for a key press, or undefined when the key is not ours.
export function keyToAction(event: KeyInput): Action | undefined {
  // Shortcuts such as Ctrl+C stay with the browser. Windows reports AltGr as
  // Ctrl+Alt, and what it types is a character, not a shortcut.
  if (event.metaKey || (event.ctrlKey && !event.altKey)) {
    return undefined
  }
  // The numpad decimal key types `,` on Turkish and German layouts.
  const key =
    event.code === 'NumpadDecimal' && event.key === ',' ? '.' : event.key
  return KEYS.get(key)
}
