import type { CalculateResult } from '../api/client.ts'

// The text a key inserts into the expression.
export type Token =
  | '0' | '1' | '2' | '3' | '4' | '5' | '6' | '7' | '8' | '9'
  | '.' | '+' | '-' | '*' | '/' | '^' | '%' | '(' | ')'
  | 'sqrt(' | 'Ans'

export type Failure = Exclude<CalculateResult, { kind: 'ok' }>

export type Request = { id: number; expression: string; ans: string }

export type State = {
  expression: string
  // The exact value of the last result, sent with every request.
  ans: string
  // The exact result of the expression on screen, until the next edit.
  result: string | null
  error: Failure | null
  // The request in flight. Editing cancels it.
  pending: Request | null
  lastRequestId: number
}

export type Action =
  | { type: 'input'; token: Token }
  | { type: 'backspace' }
  | { type: 'clear' }
  | { type: 'evaluate' }
  | { type: 'response'; id: number; result: CalculateResult }

export const initialState: State = {
  expression: '',
  ans: '0',
  result: null,
  error: null,
  pending: null,
  lastRequestId: 0,
}

// After a result these continue from Ans. Every other token starts over.
const OPERATORS: ReadonlySet<Token> = new Set(['+', '-', '*', '/', '^', '%'])

// Backspace removes these in one step.
const WHOLE_TOKENS = ['sqrt(', 'Ans']

export function reducer(state: State, action: Action): State {
  switch (action.type) {
    case 'input':
      return input(state, action.token)
    case 'backspace':
      return backspace(state)
    case 'clear':
      return { ...initialState, ans: state.ans, lastRequestId: state.lastRequestId }
    case 'evaluate':
      return evaluate(state)
    case 'response':
      return respond(state, action.id, action.result)
  }
}

function input(state: State, token: Token): State {
  const edited = { ...state, result: null, error: null, pending: null }
  if (state.result !== null) {
    const expression = OPERATORS.has(token) ? 'Ans' + token : token
    return { ...edited, expression }
  }
  // A digit, `(`, `Ans` or `sqrt(` would merge with Ans into one identifier
  // (`Ans5`, `Ans(`), which the backend reports as an unknown name.
  if (state.expression.endsWith('Ans') && /^[\w(]/.test(token)) {
    return state
  }
  return { ...edited, expression: state.expression + token }
}

function backspace(state: State): State {
  if (state.result !== null) {
    return { ...state, result: null, pending: null }
  }
  const { expression } = state
  const token = WHOLE_TOKENS.find((t) => expression.endsWith(t))
  return {
    ...state,
    expression: expression.slice(0, expression.length - (token?.length ?? 1)),
    error: null,
    pending: null,
  }
}

function evaluate(state: State): State {
  // After a result `=` runs the expression again only if it uses Ans, which
  // has changed. Repeated presses step a recurrence like `Ans*2+1`, as on a
  // Casio. The old result stays on screen until the new one arrives.
  const repeatable = state.result === null || state.expression.includes('Ans')
  if (state.expression === '' || state.pending !== null || !repeatable) {
    return state
  }
  const id = state.lastRequestId + 1
  return {
    ...state,
    error: null,
    pending: { id, expression: state.expression, ans: state.ans },
    lastRequestId: id,
  }
}

function respond(state: State, id: number, result: CalculateResult): State {
  // The request was cancelled by an edit or replaced by a newer one.
  if (state.pending?.id !== id) {
    return state
  }
  if (result.kind === 'ok') {
    return { ...state, result: result.result, ans: result.result, pending: null }
  }
  return { ...state, result: null, error: result, pending: null }
}
