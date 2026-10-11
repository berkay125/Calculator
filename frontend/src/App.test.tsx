import { act, fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import App from './App.tsx'
import type { Calculate, CalculateResult } from './api/client.ts'

function setup(calculate: Calculate) {
  render(<App calculate={calculate} />)
  return userEvent.setup()
}

function respondWith(...results: CalculateResult[]) {
  const calculate = vi.fn<Calculate>()
  for (const result of results) {
    calculate.mockResolvedValueOnce(result)
  }
  return calculate
}

// Clicks keypad buttons by their accessible names.
async function click(user: ReturnType<typeof setup>, ...names: string[]) {
  for (const name of names) {
    await user.click(screen.getByRole('button', { name }))
  }
}

const expression = () => screen.getByTestId('expression')
const result = () => screen.getByRole('status', { name: 'result' })

describe('App', () => {
  it('renders the calculator heading and an empty display', () => {
    setup(vi.fn<Calculate>())
    expect(
      screen.getByRole('heading', { name: 'Calculator' }),
    ).toBeInTheDocument()
    expect(expression()).toHaveTextContent('0')
    expect(result()).toHaveTextContent('')
  })

  it('clicking 2 + 3 = shows 5', async () => {
    const calculate = respondWith({ kind: 'ok', result: '5' })
    const user = setup(calculate)

    await click(user, '2', 'plus', '3', 'equals')

    expect(await screen.findByText('= 5')).toBeInTheDocument()
    expect(calculate).toHaveBeenCalledWith('2+3', '0', expect.any(AbortSignal))
  })

  it('shows operators as symbols but sends ASCII', async () => {
    const calculate = respondWith({ kind: 'ok', result: '-2' })
    const user = setup(calculate)

    await click(user, '6', 'divide', '3', 'multiply', 'minus', '1', 'equals')

    expect(expression()).toHaveTextContent('6÷3×−1')
    expect(calculate).toHaveBeenCalledWith('6/3*-1', '0', expect.any(AbortSignal))
  })

  it('marks rounded results with ≈', async () => {
    const user = setup(
      respondWith({ kind: 'ok', result: '0.3333333333333333333333333333333333' }),
    )
    await click(user, '1', 'divide', '3', 'equals')
    expect(await screen.findByText('≈ 0.333333333333')).toBeInTheDocument()
  })

  it('highlights the character an error points at', async () => {
    const user = setup(
      respondWith({
        kind: 'error',
        code: 'syntax_error',
        message: "unexpected ')'",
        position: 2,
      }),
    )
    await click(user, '2', 'plus', 'close parenthesis', 'equals')

    expect(await screen.findByText("Unexpected ')'")).toBeInTheDocument()
    const mark = expression().querySelector('mark')
    expect(mark).toHaveTextContent(')')
    expect(mark).not.toHaveClass('end')
  })

  it('highlights a slot after the expression for an error at the end', async () => {
    const user = setup(
      respondWith({
        kind: 'error',
        code: 'syntax_error',
        message: 'unexpected end of expression',
        position: 2,
      }),
    )
    await click(user, '2', 'plus', 'equals')

    await screen.findByText('Unexpected end of expression')
    expect(expression().querySelector('mark')).toHaveClass('end')
    expect(expression()).toHaveTextContent('2+')
  })

  it.each([
    [
      { kind: 'error', code: 'division_by_zero', message: 'division by zero', position: 1 },
      'Cannot divide by zero',
    ],
    [{ kind: 'unexpected', status: 500, message: 'internal error' }, 'internal error'],
    [{ kind: 'unexpected', status: 502 }, 'Unexpected response from the server'],
    [{ kind: 'network' }, 'Cannot reach the server'],
  ] as const)('shows %j as %j', async (failure, text) => {
    const user = setup(respondWith(failure))
    await click(user, '1', 'divide', '0', 'equals')
    expect(await screen.findByText(text)).toBeInTheDocument()
    expect(result()).toHaveClass('error')
  })

  it('starts Ans at 0 and repeats Ans expressions on =', async () => {
    const calculate = respondWith(
      { kind: 'ok', result: '1' },
      { kind: 'ok', result: '3' },
    )
    const user = setup(calculate)

    await click(user, 'answer', 'multiply', '2', 'plus', '1', 'equals')
    await screen.findByText('= 1')
    await click(user, 'equals')
    await screen.findByText('= 3')

    expect(calculate.mock.calls.map(([e, ans]) => [e, ans])).toEqual([
      ['Ans*2+1', '0'],
      ['Ans*2+1', '1'],
    ])
  })

  it('edits with backspace and clear', async () => {
    const user = setup(vi.fn<Calculate>())
    await click(user, 'square root', '9', 'backspace')
    expect(expression()).toHaveTextContent('sqrt(')
    await click(user, 'backspace')
    expect(expression()).toHaveTextContent('0')
    await click(user, '4', 'clear')
    expect(expression()).toHaveTextContent('0')
  })

  it('aborts and ignores a response that arrives after an edit', async () => {
    let resolve: (result: CalculateResult) => void = () => {}
    const calculate = vi.fn<Calculate>(
      () =>
        new Promise((r) => {
          resolve = r
        }),
    )
    const user = setup(calculate)

    await click(user, '1', 'equals')
    await click(user, '2')
    const signal = calculate.mock.calls[0][2]
    expect(signal?.aborted).toBe(true)

    await act(async () => resolve({ kind: 'ok', result: '1' }))
    expect(result()).toHaveTextContent('')
    expect(expression()).toHaveTextContent('12')
  })

  describe('keyboard', () => {
    it('typing 2*3 and Enter shows the result', async () => {
      const calculate = respondWith({ kind: 'ok', result: '6' })
      const user = setup(calculate)

      await user.keyboard('2*3{Enter}')

      expect(await screen.findByText('= 6')).toBeInTheDocument()
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

      await click(user, '2', 'plus', '3')
      await user.keyboard('{Enter}')

      expect(await screen.findByText('= 5')).toBeInTheDocument()
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

  it('handles the rejection of an aborted request quietly', async () => {
    // Like the real client: an aborted request rejects.
    const calculate = vi.fn<Calculate>(
      (_expression, _ans, signal) =>
        new Promise((_, reject) => {
          signal?.addEventListener('abort', () => reject(signal.reason))
        }),
    )
    const user = setup(calculate)

    await click(user, '1', 'equals', 'clear')

    expect(calculate.mock.calls[0][2]?.aborted).toBe(true)
    expect(result()).toHaveTextContent('')
    expect(expression()).toHaveTextContent('0')
  })
})
