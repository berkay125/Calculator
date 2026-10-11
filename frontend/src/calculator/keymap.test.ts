import { describe, expect, it } from 'vitest'
import { type KeyInput, keyToAction } from './keymap.ts'
import type { Action } from './reducer.ts'

function key(key: string, extra: Partial<KeyInput> = {}): KeyInput {
  return { key, code: '', ctrlKey: false, metaKey: false, altKey: false, ...extra }
}

const input = (token: string) => ({ type: 'input', token }) as Action

describe('keyToAction', () => {
  it.each([...'0123456789.+-*/^%()'])('%s inserts itself', (c) => {
    expect(keyToAction(key(c))).toEqual(input(c))
  })

  it.each<[string, KeyInput, Action]>([
    ['s', key('s'), input('sqrt(')],
    ['S', key('S'), input('sqrt(')],
    ['a', key('a'), input('Ans')],
    ['A', key('A'), input('Ans')],
    ['Enter', key('Enter'), { type: 'evaluate' }],
    ['=', key('='), { type: 'evaluate' }],
    ['Backspace', key('Backspace'), { type: 'backspace' }],
    ['Escape', key('Escape'), { type: 'clear' }],
    ['numpad , (Turkish layout)', key(',', { code: 'NumpadDecimal' }), input('.')],
    ['numpad .', key('.', { code: 'NumpadDecimal' }), input('.')],
    ['AltGr (Ctrl+Alt)', key('^', { ctrlKey: true, altKey: true }), input('^')],
    ['Alt', key('(', { altKey: true }), input('(')],
  ])('%s', (_, event, action) => {
    expect(keyToAction(event)).toEqual(action)
  })

  it.each<[string, KeyInput]>([
    ['Ctrl+C', key('c', { ctrlKey: true })],
    ['Ctrl+1', key('1', { ctrlKey: true })],
    ['Ctrl+Backspace', key('Backspace', { ctrlKey: true })],
    ['Meta+V', key('v', { metaKey: true })],
    ['Meta+Alt+1', key('1', { metaKey: true, altKey: true })],
    ['x', key('x')],
    ['Space', key(' ')],
    ['Tab', key('Tab')],
    ['a dead key', key('Dead', { code: 'Digit3' })],
    ['comma', key(',', { code: 'Comma' })],
    ['numpad Delete (NumLock off)', key('Delete', { code: 'NumpadDecimal' })],
    ['constructor', key('constructor')],
  ])('ignores %s', (_, event) => {
    expect(keyToAction(event)).toBeUndefined()
  })
})
