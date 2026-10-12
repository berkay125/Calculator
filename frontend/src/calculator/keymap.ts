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
  // A `^` that works on every layout, after the xʸ key.
  ['y', input('^')],
  ['Y', input('^')],
  ['Enter', { type: 'evaluate' }],
  ['=', { type: 'evaluate' }],
  ['Backspace', { type: 'backspace' }],
  ['Escape', { type: 'clear' }],
])

// `^` is a dead key on the 3 key of Turkish layouts (Shift+3). The browser
// reports it as `Dead`, and on macOS no later event carries the `^`: the Space
// that completes it arrives as a plain ' '. So it is inserted right away.
export function isDeadCaret(event: KeyInput): boolean {
  return event.key === 'Dead' && event.code === 'Digit3'
}

// The action for a key press, or undefined when the key is not ours.
export function keyToAction(event: KeyInput): Action | undefined {
  // Shortcuts such as Ctrl+C stay with the browser. Windows reports AltGr as
  // Ctrl+Alt, and what it types is a character, not a shortcut.
  if (event.metaKey || (event.ctrlKey && !event.altKey)) {
    return undefined
  }
  if (isDeadCaret(event)) {
    return input('^')
  }
  // The numpad decimal key types `,` on Turkish and German layouts.
  const key =
    event.code === 'NumpadDecimal' && event.key === ',' ? '.' : event.key
  return KEYS.get(key)
}
