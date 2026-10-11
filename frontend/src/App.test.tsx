import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from './App.tsx'
import type { Calculate, CalculateResult } from './api/client.ts'

afterEach(() => {
  vi.restoreAllMocks()
})

const THIRD = '0.3333333333333333333333333333333333'

function setup(calculate: Calculate) {
  const user = userEvent.setup()
  render(<App calculate={calculate} />)
  return user
}

function respondWith(...results: CalculateResult[]) {
  const calculate = vi.fn<Calculate>()
  for (const result of results) {
    calculate.mockResolvedValueOnce(result)
  }
  return calculate
}

// A calculate whose response arrives only when the test resolves it.
function deferred() {
  let resolve: (result: CalculateResult) => void = () => {}
  const calculate = vi.fn<Calculate>(
    () =>
      new Promise((r) => {
        resolve = r
      }),
  )
  return { calculate, resolve: (r: CalculateResult) => resolve(r) }
}

// Clicks keypad buttons by their accessible names.
async function click(user: ReturnType<typeof setup>, ...names: string[]) {
  for (const name of names) {
    await user.click(screen.getByRole('button', { name }))
  }
}

const expression = () => screen.getByRole('textbox', { name: 'Expression' })
const result = () => screen.getByRole('status')
const marker = () => result().querySelector('.marker')

// The result as a screen reader reads it, e.g. "Equals 5".
async function expectResult(text: string) {
  await waitFor(() => expect(result()).toHaveTextContent(text))
}

const syntaxError = (message: string, position: number): CalculateResult => ({
  kind: 'error',
  code: 'syntax_error',
  message,
  position,
})

describe('App', () => {
  it('renders the calculator heading and an empty display', () => {
    setup(vi.fn<Calculate>())
    expect(
      screen.getByRole('heading', { name: 'Calculator' }),
    ).toBeInTheDocument()
    expect(expression()).toHaveTextContent('0')
    expect(expression()).toHaveAttribute('aria-invalid', 'false')
    expect(result()).toHaveTextContent('')
  })

  it('clicking 2 + 3 = shows 5', async () => {
    const calculate = respondWith({ kind: 'ok', result: '5' })
    const user = setup(calculate)

    await click(user, '2', 'Plus', '3', 'Calculate')

    await expectResult('Equals 5')
    expect(marker()).toHaveTextContent('=')
    expect(result()).not.toHaveClass('long')
    expect(screen.queryByText('Exact')).toBeNull()
    expect(calculate).toHaveBeenCalledWith('2+3', '0', expect.any(AbortSignal))
  })

  it('shows operators as symbols but sends ASCII', async () => {
    const calculate = respondWith({ kind: 'ok', result: '-2' })
    const user = setup(calculate)

    await click(user, '6', 'Divide', '3', 'Multiply', 'Minus', '1', 'Calculate')

    expect(expression()).toHaveTextContent('6÷3×−1')
    expect(calculate).toHaveBeenCalledWith('6/3*-1', '0', expect.any(AbortSignal))
  })

  it('shows a rounded result with ≈, a smaller font and the exact value', async () => {
    const user = setup(respondWith({ kind: 'ok', result: THIRD }))
    await click(user, '1', 'Divide', '3', 'Calculate')

    await expectResult('Approximately 0.333333333333')
    expect(marker()).toHaveTextContent('≈')
    expect(result()).toHaveClass('long')
    expect(screen.getByText(THIRD)).toBeInTheDocument()
  })

  it('copies the exact value of a rounded result', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    try {
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
      render(<App calculate={respondWith({ kind: 'ok', result: THIRD })} />)
      await click(user, '1', 'Divide', '3', 'Calculate')

      await user.click(
        await screen.findByRole('button', { name: 'Copy exact value' }),
      )

      expect(await navigator.clipboard.readText()).toBe(THIRD)
      expect(screen.getByRole('button', { name: 'Copied' })).toBeInTheDocument()
      act(() => {
        vi.advanceTimersByTime(1500)
      })
      expect(
        screen.getByRole('button', { name: 'Copy exact value' }),
      ).toBeInTheDocument()
    } finally {
      vi.useRealTimers()
    }
  })

  it('leaves the copy button as it is when copying fails', async () => {
    const user = setup(respondWith({ kind: 'ok', result: THIRD }))
    const writeText = vi
      .spyOn(navigator.clipboard, 'writeText')
      .mockRejectedValue(new Error('denied'))
    await click(user, '1', 'Divide', '3', 'Calculate')

    await user.click(
      await screen.findByRole('button', { name: 'Copy exact value' }),
    )

    expect(writeText).toHaveBeenCalledWith(THIRD)
    expect(
      screen.getByRole('button', { name: 'Copy exact value' }),
    ).toBeInTheDocument()
  })

  it('highlights the character an error points at and describes the error', async () => {
    const user = setup(respondWith(syntaxError("unexpected ')'", 2)))
    await click(user, '2', 'Plus', 'Close parenthesis', 'Calculate')

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('SYNTAX_ERROR')
    expect(alert).toHaveTextContent("Unexpected ')' at column 3")
    expect(expression()).toHaveAttribute('aria-invalid', 'true')
    expect(expression()).toHaveAccessibleDescription("Unexpected ')' at column 3")
    expect(screen.getByRole('region', { name: 'Display' })).toHaveClass('invalid')
    const mark = expression().querySelector('mark')
    expect(mark).toHaveTextContent(')')
    expect(mark).not.toHaveClass('end')
  })

  it('highlights a slot after the expression for an error at the end', async () => {
    const user = setup(respondWith(syntaxError('unexpected end of expression', 2)))
    await click(user, '2', 'Plus', 'Calculate')

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Unexpected end of expression at column 3',
    )
    expect(expression().querySelector('mark')).toHaveClass('end')
    expect(expression()).toHaveTextContent('2+')
  })

  it.each([
    [
      { kind: 'error', code: 'division_by_zero', message: 'division by zero', position: 1 },
      'DIVISION_BY_ZERO',
      'Cannot divide by zero',
    ],
    [{ kind: 'unexpected', status: 500, message: 'internal error' }, 'HTTP 500', 'internal error'],
    [{ kind: 'unexpected', status: 502 }, 'HTTP 502', 'Unexpected response from the server'],
    [{ kind: 'network' }, null, 'Cannot reach the server'],
  ] as const)('shows %j with badge %j and text %j', async (failure, badge, text) => {
    const user = setup(respondWith(failure))
    await click(user, '1', 'Divide', '0', 'Calculate')

    const alert = await screen.findByRole('alert')
    expect(alert.querySelector('.message')).toHaveTextContent(text)
    expect(alert.querySelector('.code')?.textContent ?? null).toBe(badge)
    expect(result()).toHaveTextContent('')
  })

  it('shows a loading state while a request is pending', async () => {
    const { calculate, resolve } = deferred()
    const user = setup(calculate)

    await click(user, '1', 'Calculate')
    expect(result()).toHaveAttribute('aria-busy', 'true')
    expect(result()).toHaveClass('pending')
    expect(result()).toHaveTextContent('…')

    await act(async () => resolve({ kind: 'ok', result: '1' }))
    expect(result()).toHaveAttribute('aria-busy', 'false')
    expect(result()).not.toHaveClass('pending')
    expect(result()).toHaveTextContent('Equals 1')
  })

  it('keeps the old result while a repeated = is pending', async () => {
    const calculate = respondWith({ kind: 'ok', result: '1' })
    calculate.mockReturnValueOnce(new Promise(() => {}))
    const user = setup(calculate)

    await click(user, 'Insert last answer', 'Plus', '1', 'Calculate')
    await expectResult('Equals 1')
    await click(user, 'Calculate')

    expect(result()).toHaveTextContent('Equals 1')
    expect(result()).toHaveClass('pending')
  })

  it('keeps the end of a long expression in view', async () => {
    const user = setup(vi.fn<Calculate>())
    const line = expression()
    let left = 0
    Object.defineProperty(line, 'scrollWidth', { value: 640 })
    Object.defineProperty(line, 'scrollLeft', {
      get: () => left,
      set: (value: number) => {
        left = value
      },
    })

    await user.keyboard('123')

    expect(line.scrollLeft).toBe(640)
  })

  it('scrolls the error highlight into view', async () => {
    const scrollIntoView = vi.spyOn(Element.prototype, 'scrollIntoView')
    const user = setup(respondWith(syntaxError("unexpected ')'", 2)))

    await click(user, '2', 'Plus', 'Close parenthesis', 'Calculate')
    await screen.findByRole('alert')

    expect(scrollIntoView).toHaveBeenCalledExactlyOnceWith({
      block: 'nearest',
      inline: 'center',
    })
    expect(scrollIntoView.mock.contexts[0]).toBe(
      expression().querySelector('mark'),
    )
  })

  it('starts Ans at 0 and repeats Ans expressions on =', async () => {
    const calculate = respondWith(
      { kind: 'ok', result: '1' },
      { kind: 'ok', result: '3' },
    )
    const user = setup(calculate)

    await click(user, 'Insert last answer', 'Multiply', '2', 'Plus', '1', 'Calculate')
    await expectResult('Equals 1')
    await click(user, 'Calculate')
    await expectResult('Equals 3')

    expect(calculate.mock.calls.map(([e, ans]) => [e, ans])).toEqual([
      ['Ans*2+1', '0'],
      ['Ans*2+1', '1'],
    ])
  })

  it('edits with Delete and Clear', async () => {
    const user = setup(vi.fn<Calculate>())
    await click(user, 'Square root', '9', 'Delete')
    expect(expression()).toHaveTextContent('sqrt(')
    await click(user, 'Delete')
    expect(expression()).toHaveTextContent('0')
    await click(user, '4', 'Clear')
    expect(expression()).toHaveTextContent('0')
  })

  it('aborts and ignores a response that arrives after an edit', async () => {
    const { calculate, resolve } = deferred()
    const user = setup(calculate)

    await click(user, '1', 'Calculate')
    await click(user, '2')
    const signal = calculate.mock.calls[0][2]
    expect(signal?.aborted).toBe(true)

    await act(async () => resolve({ kind: 'ok', result: '1' }))
    expect(result()).toHaveTextContent('')
    expect(expression()).toHaveTextContent('12')
  })

  it('handles the rejection of an aborted request quietly', async () => {
    // Like the real client: an aborted request rejects.
    const calculate = vi.fn<Calculate>(
      (_expression, _ans, signal) =>
        new Promise((_, reject) => {
          signal?.addEventListener('abort', () => reject(signal.reason))
        }),
    )
    const user = setup(calculate)

    await click(user, '1', 'Calculate', 'Clear')

    expect(calculate.mock.calls[0][2]?.aborted).toBe(true)
    expect(result()).toHaveTextContent('')
    expect(expression()).toHaveTextContent('0')
  })

  describe('keyboard', () => {
    it('typing 2*3 and Enter shows the result', async () => {
      const calculate = respondWith({ kind: 'ok', result: '6' })
      const user = setup(calculate)

      await user.keyboard('2*3{Enter}')

      await expectResult('Equals 6')
      expect(expression()).toHaveTextContent('2×3')
      expect(calculate).toHaveBeenCalledWith('2*3', '0', expect.any(AbortSignal))
    })

    it('edits with Backspace and Escape', async () => {
      const user = setup(vi.fn<Calculate>())
      await user.keyboard('s9{Backspace}')
      expect(expression()).toHaveTextContent('sqrt(')
      await user.keyboard('{Backspace}a')
      expect(expression()).toHaveTextContent('Ans')
      await user.keyboard('{Escape}')
      expect(expression()).toHaveTextContent('0')
    })

    it('prevents the default action only for its own keys', () => {
      setup(vi.fn<Calculate>())
      // fireEvent returns false when the default action was prevented.
      expect(fireEvent.keyDown(document.body, { key: '/' })).toBe(false)
      expect(fireEvent.keyDown(document.body, { key: 'x' })).toBe(true)
      expect(fireEvent.keyDown(document.body, { key: 'c', ctrlKey: true })).toBe(true)
      expect(expression()).toHaveTextContent('÷')
    })

    it('Enter after clicking a key evaluates once and does not press the key again', async () => {
      const calculate = respondWith({ kind: 'ok', result: '5' })
      const user = setup(calculate)

      await click(user, '2', 'Plus', '3')
      await user.keyboard('{Enter}')

      await expectResult('Equals 5')
      expect(calculate).toHaveBeenCalledOnce()
      expect(calculate).toHaveBeenCalledWith('2+3', '0', expect.any(AbortSignal))
    })

    it('Enter and Space press a key reached with Tab', async () => {
      const calculate = vi.fn<Calculate>()
      const user = setup(calculate)

      screen.getByRole('button', { name: '7' }).focus()
      await user.keyboard('{Enter}[Space]')

      expect(expression()).toHaveTextContent('77')
      expect(calculate).not.toHaveBeenCalled()
    })
  })
})
