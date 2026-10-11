// Rounds results for the display. The rules are in PLAN.md, "Display
// formatting". Everything works on the decimal string: Number() would lose
// precision and overflow above 1e308.

const MAX_DIGITS = 12
const WIDTH = 14 // a leading minus sign is not counted
const PLAIN_MIN_EXP = -3 // 0.001
const PLAIN_MAX_EXP = 11 // below 10^12

export type DisplayValue = { text: string; rounded: boolean }

// A nonzero value d1.d2d3… × 10^exp. `digits` has no leading or trailing
// zeros.
type Decimal = { negative: boolean; digits: string; exp: number }

// `exact` is a result in the backend's number format (`-0.5`, `1.5E+40`).
export function formatForDisplay(exact: string): DisplayValue {
  const value = parse(exact)
  if (value === undefined) {
    return { text: '0', rounded: false }
  }
  const n = digitsFor(value.exp)
  const r = round(value, n)
  // A carry can move the value across a range boundary, so the notation is
  // chosen on the rounded value. After a carry only the digit 1 is left, which
  // fits either notation.
  const text = isPlain(r.exp) ? plain(r) : scientific(r)
  return {
    text: (r.negative ? '-' : '') + text,
    rounded: value.digits.length > n,
  }
}

function parse(exact: string): Decimal | undefined {
  const negative = exact.startsWith('-')
  const [mantissa, expText = '0'] = (negative ? exact.slice(1) : exact).split(
    'E',
  )
  const [intPart, fracPart = ''] = mantissa.split('.')
  const all = intPart + fracPart
  const lead = all.search(/[1-9]/)
  if (lead === -1) {
    return undefined
  }
  return {
    negative,
    digits: all.slice(lead).replace(/0+$/, ''),
    exp: Number.parseInt(expText, 10) + intPart.length - 1 - lead,
  }
}

function isPlain(exp: number): boolean {
  return exp >= PLAIN_MIN_EXP && exp <= PLAIN_MAX_EXP
}

// How many significant digits fit the width at this exponent.
function digitsFor(exp: number): number {
  if (isPlain(exp)) {
    // `0.` and the zeros after the point come before the digits.
    return exp >= 0 ? MAX_DIGITS : Math.min(MAX_DIGITS, WIDTH - 1 + exp)
  }
  // The mantissa's point and the exponent come after them.
  return Math.min(MAX_DIGITS, WIDTH - 1 - exponent(exp).length)
}

// Rounds to n significant digits, ties to even.
function round(value: Decimal, n: number): Decimal {
  const { digits } = value
  if (digits.length <= n) {
    return value
  }
  const kept = digits.slice(0, n)
  const next = digits[n]
  const up =
    next > '5' ||
    (next === '5' &&
      (/[1-9]/.test(digits.slice(n + 1)) || '13579'.includes(kept[n - 1])))
  if (!up) {
    return { ...value, digits: kept.replace(/0+$/, '') }
  }
  // Adding one turns the trailing nines into zeros, which are dropped.
  const last = kept.search(/[0-8]9*$/)
  if (last === -1) {
    return { ...value, digits: '1', exp: value.exp + 1 }
  }
  return {
    ...value,
    digits: kept.slice(0, last) + String.fromCharCode(kept.charCodeAt(last) + 1),
  }
}

function plain({ digits, exp }: Decimal): string {
  if (exp < 0) {
    return '0.' + '0'.repeat(-exp - 1) + digits
  }
  const intLength = exp + 1
  if (digits.length <= intLength) {
    return digits.padEnd(intLength, '0')
  }
  return digits.slice(0, intLength) + '.' + digits.slice(intLength)
}

function scientific({ digits, exp }: Decimal): string {
  const mantissa =
    digits.length === 1 ? digits : digits[0] + '.' + digits.slice(1)
  return mantissa + exponent(exp)
}

function exponent(exp: number): string {
  return 'E' + (exp < 0 ? '-' : '+') + Math.abs(exp)
}
