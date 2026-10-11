import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import type { Action, Token } from '../calculator/reducer.ts'
import { Keypad } from './Keypad.tsx'

const input = (token: Token): Action => ({ type: 'input', token })
const digits = '0123456789'.split('') as Token[]

it.each<[string, Action]>([
  ['clear', { type: 'clear' }],
  ['backspace', { type: 'backspace' }],
  ['open parenthesis', input('(')],
  ['close parenthesis', input(')')],
  ['square root', input('sqrt(')],
  ['power', input('^')],
  ['percent', input('%')],
  ['divide', input('/')],
  ['multiply', input('*')],
  ['minus', input('-')],
  ['plus', input('+')],
  ['decimal point', input('.')],
  ['answer', input('Ans')],
  ['equals', { type: 'evaluate' }],
  ...digits.map((d): [string, Action] => [d, input(d)]),
])('the %s key dispatches %j', async (name, action) => {
  const onAction = vi.fn<(action: Action) => void>()
  render(<Keypad onAction={onAction} />)

  await userEvent.click(screen.getByRole('button', { name }))

  expect(onAction).toHaveBeenCalledExactlyOnceWith(action)
})

it('keys do not take focus when clicked', async () => {
  render(<Keypad onAction={() => {}} />)
  const key = screen.getByRole('button', { name: '7' })

  await userEvent.click(key)

  expect(key).not.toHaveFocus()
})

it('has 24 keys', () => {
  render(<Keypad onAction={() => {}} />)
  expect(screen.getAllByRole('button')).toHaveLength(24)
})
