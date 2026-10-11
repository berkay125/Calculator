// Mirrors the error catalog in PLAN.md.
export const ERROR_CODES = [
  'invalid_request',
  'invalid_ans',
  'expression_too_long',
  'nesting_too_deep',
  'syntax_error',
  'unknown_function',
  'unknown_variable',
  'wrong_argument_count',
  'division_by_zero',
  'domain_error',
  'overflow',
  'underflow',
] as const

export type ErrorCode = (typeof ERROR_CODES)[number]

const codes: ReadonlySet<string> = new Set(ERROR_CODES)

export function isErrorCode(code: string): code is ErrorCode {
  return codes.has(code)
}

// Text shown to the user. Syntax errors prefer the backend's text, see
// errorMessage.
export const ERROR_MESSAGES: Record<ErrorCode, string> = {
  invalid_request: 'Invalid request',
  invalid_ans: 'Invalid Ans value',
  expression_too_long: 'Expression is too long',
  nesting_too_deep: 'Too many nested parentheses',
  syntax_error: 'Syntax error',
  unknown_function: 'Unknown function',
  unknown_variable: 'Unknown variable',
  wrong_argument_count: 'Wrong number of arguments',
  division_by_zero: 'Cannot divide by zero',
  domain_error: 'Result is not a real number',
  overflow: 'Result is too large',
  underflow: 'Result is too close to zero',
}

// The message for an API error. For syntax errors the backend's text names
// the problem ("unexpected ')'"), so it is used, capitalized like ours.
export function errorMessage(code: ErrorCode, backendMessage: string): string {
  if (code === 'syntax_error' && backendMessage !== '') {
    return backendMessage.charAt(0).toUpperCase() + backendMessage.slice(1)
  }
  return ERROR_MESSAGES[code]
}
