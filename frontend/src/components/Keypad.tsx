import type { ReactNode } from 'react'
import type { Action, Token } from '../calculator/reducer.ts'

// Each kind has its own colors in the design.
type Kind = 'digit' | 'op' | 'fn' | 'ans' | 'clear' | 'equals'

type Key = {
  label: ReactNode
  name: string // the accessible name
  action: Action
  kind: Kind
}

function insert(token: Token, kind: Kind, name: string = token, label: string = token): Key {
  return { label, name, action: { type: 'input', token }, kind }
}

const backspaceIcon = (
  <svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <path d="M9 5h10a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H9l-6-7z" />
    <path d="M12 9.5l5 5" />
    <path d="M17 9.5l-5 5" />
  </svg>
)

// Four columns, row by row.
const KEYS: Key[] = [
  { label: 'C', name: 'Clear', action: { type: 'clear' }, kind: 'clear' },
  insert('(', 'fn', 'Open parenthesis'),
  insert(')', 'fn', 'Close parenthesis'),
  { label: backspaceIcon, name: 'Delete', action: { type: 'backspace' }, kind: 'clear' },
  insert('sqrt(', 'fn', 'Square root', '√'),
  insert('^', 'fn', 'Power', 'xʸ'),
  insert('%', 'fn', 'Percent'),
  insert('/', 'op', 'Divide', '÷'),
  insert('7', 'digit'),
  insert('8', 'digit'),
  insert('9', 'digit'),
  insert('*', 'op', 'Multiply', '×'),
  insert('4', 'digit'),
  insert('5', 'digit'),
  insert('6', 'digit'),
  insert('-', 'op', 'Minus', '−'),
  insert('1', 'digit'),
  insert('2', 'digit'),
  insert('3', 'digit'),
  insert('+', 'op', 'Plus'),
  insert('0', 'digit'),
  insert('.', 'digit', 'Decimal point'),
  insert('Ans', 'ans', 'Insert last answer'),
  { label: '=', name: 'Calculate', action: { type: 'evaluate' }, kind: 'equals' },
]

export function Keypad({ onAction }: { onAction: (action: Action) => void }) {
  return (
    <div className="keypad">
      {KEYS.map((key) => (
        <button
          key={key.name}
          type="button"
          className={key.kind}
          aria-label={key.name}
          // A clicked key must not keep focus: Enter on the keyboard would
          // press it again instead of evaluating.
          onMouseDown={(e) => e.preventDefault()}
          onClick={() => onAction(key.action)}
        >
          {key.label}
        </button>
      ))}
    </div>
  )
}
