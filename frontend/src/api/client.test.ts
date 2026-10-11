import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { CALCULATE_URL, calculate } from './client.ts'

const fetchMock = vi.fn<typeof fetch>()

beforeEach(() => {
  vi.stubGlobal('fetch', fetchMock)
})

afterEach(() => {
  vi.unstubAllGlobals()
  fetchMock.mockReset()
})

// Strings are sent as the raw body, anything else as JSON.
function respond(status: number, body: unknown) {
  const text = typeof body === 'string' ? body : JSON.stringify(body)
  fetchMock.mockResolvedValue(new Response(text, { status }))
}

function apiError(code: string, message?: string, position?: number) {
  return { error: { code, message, position } }
}

describe('calculate', () => {
  it('posts the expression and ans as JSON', async () => {
    respond(200, { result: '42' })
    const controller = new AbortController()

    await calculate('2*Ans', '21', controller.signal)

    expect(fetchMock).toHaveBeenCalledOnce()
    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe(CALCULATE_URL)
    expect(init?.method).toBe('POST')
    expect(init?.headers).toEqual({ 'Content-Type': 'application/json' })
    expect(JSON.parse(init?.body as string)).toEqual({
      expression: '2*Ans',
      ans: '21',
    })
    expect(init?.signal).toBe(controller.signal)
  })

  it.each(['42', '-0.5', '0.3333333333333333333333333333333333', '1.5E+40', '-1.5E-10'])(
    'returns the result %s',
    async (result) => {
      respond(200, { result })
      expect(await calculate('x', '0')).toEqual({ kind: 'ok', result })
    },
  )

  it('returns a positional 400 error', async () => {
    respond(400, apiError('syntax_error', "unexpected ')'", 7))
    expect(await calculate('2 * (3 +) 4', '0')).toEqual({
      kind: 'error',
      code: 'syntax_error',
      message: "unexpected ')'",
      position: 7,
    })
  })

  it('returns a 422 error', async () => {
    respond(422, apiError('division_by_zero', 'division by zero', 1))
    expect(await calculate('1/0', '0')).toEqual({
      kind: 'error',
      code: 'division_by_zero',
      message: 'division by zero',
      position: 1,
    })
  })

  it('returns an error without a position', async () => {
    respond(400, apiError('expression_too_long', 'expression too long'))
    const result = await calculate('1+1', '0')
    expect(result).toEqual({
      kind: 'error',
      code: 'expression_too_long',
      message: 'expression too long',
    })
    expect(result).not.toHaveProperty('position')
  })

  it('accepts a position at the end of the expression', async () => {
    respond(400, apiError('syntax_error', 'unexpected end of expression', 2))
    expect(await calculate('2+', '0')).toMatchObject({
      kind: 'error',
      position: 2,
    })
  })

  // Every response here is outside the contract. The expression is `2+`, so
  // valid positions are 0 to 2.
  it.each([
    ['a 500 with a message', 500, apiError('internal', 'boom'), 'boom'],
    ['a body that is not JSON', 502, '<html>Bad Gateway</html>', undefined],
    ['an unexpected status', 418, apiError('syntax_error', 'teapot', 0), 'teapot'],
    ['an unknown code', 400, apiError('new_code', 'something new'), 'something new'],
    ['an inherited property name as code', 400, apiError('toString', 'x'), 'x'],
    ['a missing message', 400, apiError('syntax_error', undefined, 0), undefined],
    ['a position past the end', 400, apiError('syntax_error', 'x', 3), 'x'],
    ['a negative position', 400, apiError('syntax_error', 'x', -1), 'x'],
    ['a fractional position', 400, apiError('syntax_error', 'x', 0.5), 'x'],
    ['a position that is not a number', 400, { error: { code: 'syntax_error', message: 'x', position: '1' } }, 'x'],
    ['an error that is not an object', 400, { error: 'syntax_error' }, undefined],
    ['a 200 that is not JSON', 200, 'ok', undefined],
    ['a 200 with an array body', 200, ['42'], undefined],
    ['a 200 without a result', 200, {}, undefined],
    ['a numeric result', 200, { result: 42 }, undefined],
    ['a result that is not a decimal', 200, { result: 'NaN' }, undefined],
    ['a result in lowercase scientific notation', 200, { result: '1e5' }, undefined],
    ['a result with a trailing point', 200, { result: '5.' }, undefined],
  ])('returns unexpected for %s', async (_, status, body, message) => {
    respond(status, body)
    expect(await calculate('2+', '0')).toEqual({
      kind: 'unexpected',
      status,
      message,
    })
  })

  it('returns network when fetch fails', async () => {
    fetchMock.mockRejectedValue(new TypeError('Failed to fetch'))
    expect(await calculate('1+1', '0')).toEqual({ kind: 'network' })
  })

  it('returns network when reading the body fails', async () => {
    fetchMock.mockResolvedValue({
      status: 200,
      text: () => Promise.reject(new TypeError('connection reset')),
    } as Response)
    expect(await calculate('1+1', '0')).toEqual({ kind: 'network' })
  })

  it('rejects when the caller aborts', async () => {
    fetchMock.mockImplementation((_, init) =>
      Promise.reject(init?.signal?.reason),
    )
    const controller = new AbortController()
    controller.abort()

    await expect(
      calculate('1+1', '0', controller.signal),
    ).rejects.toMatchObject({ name: 'AbortError' })
  })
})
