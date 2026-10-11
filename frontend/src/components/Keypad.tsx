import type { Action, Token } from '../calculator/reducer.ts'

type Key = {
  label: string
  // The accessible name, when the label alone is not clear.
  name?: string
  action: Action
  kind?: 'operator' | 'control' | 'equals'
}

function insert(token: Token, label: string = token, name?: string): Key {
  return { label, name, action: { type: 'input', token } }
}

function operator(token: Token, label: string, name: string): Key {
  return { ...insert(token, label, name), kind: 'operator' }
}

// Four columns, row by row.
const KEYS: Key[] = [
  { label: 'C', name: 'clear', action: { type: 'clear' }, kind: 'control' },
  { label: '⌫', name: 'backspace', action: { type: 'backspace' }, kind: 'control' },
  insert('(', '(', 'open parenthesis'),
  insert(')', ')', 'close parenthesis'),
  insert('sqrt(', '√', 'square root'),
  operator('^', '^', 'power'),
  operator('%', '%', 'percent'),
  operator('/', '÷', 'divide'),
  insert('7'),
  insert('8'),
  insert('9'),
  operator('*', '×', 'multiply'),
  insert('4'),
  insert('5'),
  insert('6'),
  operator('-', '−', 'minus'),
  insert('1'),
  insert('2'),
  insert('3'),
  operator('+', '+', 'plus'),
  insert('0'),
  insert('.', '.', 'decimal point'),
  insert('Ans', 'Ans', 'answer'),
  { label: '=', name: 'equals', action: { type: 'evaluate' }, kind: 'equals' },
]

export function Keypad({ onAction }: { onAction: (action: Action) => void }) {
  return (
    <div className="keypad">
      {KEYS.map((key) => (
        <button
          key={key.label}
          type="button"
          className={key.kind}
          aria-label={key.name}
          onClick={() => onAction(key.action)}
        >
          {key.label}
        </button>
      ))}
    </div>
  )
}
