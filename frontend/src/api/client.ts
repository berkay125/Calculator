import { type ErrorCode, isErrorCode } from './errors.ts'

export const CALCULATE_URL = '/api/v1/calculate'

export type CalculateResult =
  | { kind: 'ok'; result: string }
  | { kind: 'error'; code: ErrorCode; message: string; position?: number }
  // A response outside the API contract: wrong status, a body that is not
  // JSON, an unknown code. `message` is the backend's text, if there is one.
  | { kind: 'unexpected'; status: number; message?: string }
  | { kind: 'network' }

export type Calculate = typeof calculate

// The backend's number format: plain (`-0.5`) or scientific (`1.5E+40`).
const DECIMAL = /^-?\d+(\.\d+)?(E[+-]\d+)?$/

// Sends one calculation. It resolves for every outcome, and rejects only when
// the caller aborts it through `signal`.
export async function calculate(
  expression: string,
  ans: string,
  signal?: AbortSignal,
): Promise<CalculateResult> {
  let status: number
  let text: string
  try {
    const response = await fetch(CALCULATE_URL, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ expression, ans }),
      signal,
    })
    status = response.status
    text = await response.text()
  } catch (err) {
    if (signal?.aborted) {
      throw err
    }
    return { kind: 'network' }
  }
  return parseResponse(status, text, expression)
}

function parseResponse(
  status: number,
  text: string,
  expression: string,
): CalculateResult {
  const body = parseJSON(text)

  if (status === 200) {
    if (
      isObject(body) &&
      typeof body.result === 'string' &&
      DECIMAL.test(body.result)
    ) {
      return { kind: 'ok', result: body.result }
    }
    return { kind: 'unexpected', status }
  }

  const error: Record<string, unknown> =
    isObject(body) && isObject(body.error) ? body.error : {}
  const { code, position } = error
  const message =
    typeof error.message === 'string' ? error.message : undefined
  if (
    (status === 400 || status === 422) &&
    message !== undefined &&
    typeof code === 'string' &&
    isErrorCode(code)
  ) {
    if (position === undefined) {
      return { kind: 'error', code, message }
    }
    // Positions point into the expression, one past its end at most.
    if (
      typeof position === 'number' &&
      Number.isInteger(position) &&
      position >= 0 &&
      position <= expression.length
    ) {
      return { kind: 'error', code, message, position }
    }
  }
  return { kind: 'unexpected', status, message }
}

function parseJSON(text: string): unknown {
  try {
    return JSON.parse(text)
  } catch {
    return undefined
  }
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
