import { describe, expect, it } from 'vitest'
import {
  ERROR_CODES,
  ERROR_MESSAGES,
  errorMessage,
  isErrorCode,
} from './errors.ts'

describe('error codes', () => {
  it.each(ERROR_CODES)('%s is a known code with a message', (code) => {
    expect(isErrorCode(code)).toBe(true)
    expect(ERROR_MESSAGES[code]).not.toBe('')
  })

  it.each(['', 'unknown', 'SYNTAX_ERROR', 'toString', 'constructor'])(
    'rejects %j',
    (code) => {
      expect(isErrorCode(code)).toBe(false)
    },
  )
})

describe('errorMessage', () => {
  it.each([
    ['syntax_error', "unexpected ')'", "Unexpected ')'"],
    ['syntax_error', 'unclosed parenthesis', 'Unclosed parenthesis'],
    ['syntax_error', '', 'Syntax error'],
    ['division_by_zero', 'division by zero', 'Cannot divide by zero'],
    ['unknown_function', 'unknown function "foo"', 'Unknown function'],
    ['nesting_too_deep', 'parentheses are nested too deep', 'Too many nested parentheses'],
  ] as const)('%s with %j shows %j', (code, backendMessage, shown) => {
    expect(errorMessage(code, backendMessage)).toBe(shown)
  })
})
