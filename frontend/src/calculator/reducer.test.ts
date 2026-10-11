import { describe, expect, it } from 'vitest'
import type { CalculateResult } from '../api/client.ts'
import {
  type Action,
  type State,
  type Token,
  initialState,
  reducer,
} from './reducer.ts'

function reduce(state: State, ...actions: Action[]): State {
  return actions.reduce(reducer, state)
}

function press(...tokens: Token[]): Action[] {
  return tokens.map((token) => ({ type: 'input', token }))
}

const evaluate: Action = { type: 'evaluate' }
const backspace: Action = { type: 'backspace' }
const clear: Action = { type: 'clear' }

function respond(id: number, result: CalculateResult): Action {
  return { type: 'response', id, result }
}

const syntaxError: CalculateResult = {
  kind: 'error',
  code: 'syntax_error',
  message: 'unexpected end of expression',
  position: 2,
}

// Types the tokens, presses `=` and returns the state with the request sent.
function sent(...tokens: Token[]): State {
  return reduce(initialState, ...press(...tokens), evaluate)
}

// The state after the tokens were evaluated to `result`.
function calculated(tokens: Token[], result: string): State {
  const state = sent(...tokens)
  return reduce(state, respond(state.lastRequestId, { kind: 'ok', result }))
}

describe('reducer', () => {
  it('starts empty, with ans at 0', () => {
    expect(initialState).toEqual({
      expression: '',
      ans: '0',
      result: null,
      error: null,
      pending: null,
      lastRequestId: 0,
    })
  })

  it('appends tokens to the expression', () => {
    const state = reduce(initialState, ...press('2', '*', '(', '3', '+', 'sqrt(', '4', ')', ')'))
    expect(state.expression).toBe('2*(3+sqrt(4))')
  })

  it('evaluates: sends the request, then shows the result and keeps it as Ans', () => {
    const state = sent('2', '+', '3')
    expect(state.pending).toEqual({ id: 1, expression: '2+3', ans: '0' })

    const done = reduce(state, respond(1, { kind: 'ok', result: '5' }))
    expect(done).toMatchObject({
      expression: '2+3',
      result: '5',
      ans: '5',
      pending: null,
    })
  })

  it('sends the current ans with the request', () => {
    const state = reduce(calculated(['7'], '7'), ...press('*', '2'), evaluate)
    expect(state.pending).toEqual({ id: 2, expression: 'Ans*2', ans: '7' })
  })

  it('shows a failed request as an error and keeps ans', () => {
    const before = calculated(['4'], '4')
    const state = reduce(before, ...press('*'), evaluate)
    const failed = reduce(state, respond(state.lastRequestId, syntaxError))
    expect(failed).toMatchObject({
      expression: 'Ans*',
      result: null,
      error: syntaxError,
      ans: '4',
      pending: null,
    })
  })

  it.each([
    ['an empty expression', initialState],
    ['a pending request', sent('1')],
    ['a result of an expression without Ans', calculated(['1', '+', '2'], '3')],
  ])('ignores = with %s', (_, state) => {
    expect(reducer(state, evaluate)).toBe(state)
  })

  describe('repeated = with Ans', () => {
    // Ans starts at 0, so Ans*2+1 gives 1, 3, 7, ...
    const first = calculated(['Ans', '*', '2', '+', '1'], '1')

    it('runs the expression again with the new Ans', () => {
      const state = reducer(first, evaluate)
      expect(state).toMatchObject({
        result: '1',
        pending: { id: 2, expression: 'Ans*2+1', ans: '1' },
      })

      const second = reduce(state, respond(2, { kind: 'ok', result: '3' }), evaluate)
      expect(second).toMatchObject({
        result: '3',
        ans: '3',
        pending: { id: 3, expression: 'Ans*2+1', ans: '3' },
      })
    })

    it('ignores = while the repeat is pending', () => {
      const state = reducer(first, evaluate)
      expect(reducer(state, evaluate)).toBe(state)
    })

    it('backspace cancels the repeat and returns to editing', () => {
      const state = reduce(first, evaluate, backspace)
      expect(state).toMatchObject({ expression: 'Ans*2+1', result: null, pending: null })
    })

    it('a failed repeat replaces the result with the error', () => {
      const failed: CalculateResult = { kind: 'error', code: 'overflow', message: 'overflow', position: 3 }
      const state = reduce(first, evaluate, respond(2, failed))
      expect(state).toMatchObject({ result: null, error: failed, ans: '1' })
    })
  })

  it('sends the expression again when = follows an error', () => {
    const failed = reduce(sent('1', '+'), respond(1, { kind: 'network' }))
    const state = reducer(failed, evaluate)
    expect(state.error).toBeNull()
    expect(state.pending).toEqual({ id: 2, expression: '1+', ans: '0' })
  })

  describe('backspace', () => {
    it.each([
      ['12', '1'],
      ['2*sqrt(', '2*'],
      ['2*Ans', '2*'],
      ['sqrt(', ''],
      ['', ''],
    ])('turns %j into %j', (expression, expected) => {
      const state = reducer({ ...initialState, expression }, backspace)
      expect(state.expression).toBe(expected)
    })

    it('returns from a result to editing the same expression', () => {
      const state = reducer(calculated(['2', '+', '3'], '5'), backspace)
      expect(state).toMatchObject({ expression: '2+3', result: null, ans: '5' })
    })
  })

  describe('after a result', () => {
    const done = calculated(['2', '+', '3'], '5')

    it.each(['+', '-', '*', '/', '^', '%'] as const)(
      '%s continues from Ans',
      (token) => {
        const state = reducer(done, { type: 'input', token })
        expect(state).toMatchObject({ expression: `Ans${token}`, result: null })
      },
    )

    it.each(['7', '.', '(', ')', 'sqrt(', 'Ans'] as const)(
      '%s starts a new expression',
      (token) => {
        const state = reducer(done, { type: 'input', token })
        expect(state).toMatchObject({ expression: token, result: null, ans: '5' })
      },
    )
  })

  describe('right after Ans', () => {
    const state = { ...initialState, expression: '2*Ans' }

    it.each(['0', '9', '(', 'Ans', 'sqrt('] as const)('%s is ignored', (token) => {
      expect(reducer(state, { type: 'input', token })).toBe(state)
    })

    it.each(['.', '+', '-', '*', '/', '^', '%', ')'] as const)(
      '%s is appended',
      (token) => {
        const next = reducer(state, { type: 'input', token })
        expect(next.expression).toBe(`2*Ans${token}`)
      },
    )
  })

  it('clear keeps ans', () => {
    const state = reducer(calculated(['6'], '6'), clear)
    expect(state).toEqual({ ...initialState, ans: '6', lastRequestId: 1 })
  })

  describe('edits clear the error and cancel the pending request', () => {
    const failed = reduce(sent('2', '+'), respond(1, syntaxError))
    const waiting = sent('2', '+', '2')

    it.each([
      ['input', press('3')[0]],
      ['backspace', backspace],
      ['clear', clear],
    ])('%s', (_, action) => {
      expect(reducer(failed, action).error).toBeNull()
      expect(reducer(waiting, action).pending).toBeNull()
    })
  })

  describe('stale responses', () => {
    it('drops a response to a request cancelled by an edit', () => {
      const edited = reduce(sent('1', '+', '1'), ...press('0'))
      expect(reducer(edited, respond(1, { kind: 'ok', result: '2' }))).toBe(edited)
    })

    it('drops a response to an older request', () => {
      const state = reduce(sent('1'), backspace, ...press('2'), evaluate)
      expect(state.pending?.id).toBe(2)

      const after = reducer(state, respond(1, { kind: 'ok', result: '1' }))
      expect(after).toBe(state)
      expect(reducer(after, respond(2, { kind: 'ok', result: '2' })).result).toBe('2')
    })

    it('drops a second response to the same request', () => {
      const done = calculated(['1'], '1')
      expect(reducer(done, respond(1, syntaxError))).toBe(done)
    })
  })
})
