import { describe, expect, it } from 'vitest'
import { formatForDisplay } from './format.ts'

describe('formatForDisplay', () => {
  // The table from PLAN.md, step 11.
  it.each([
    ['123456789012', '123456789012', false],
    ['0.3333333333333333333333333333333333', '0.333333333333', true],
    ['0.03333333333333333333333333333333333', '0.033333333333', true],
    ['0.1234567890125', '0.123456789012', true],
    ['0.1234567890135', '0.123456789014', true],
    ['9.9999999999999', '10', true],
    ['999999999999.9', '1E+12', true],
    ['1099511627776', '1.09951163E+12', true],
    ['0.00123456789012345', '0.00123456789', true],
    ['0.001', '0.001', false],
    ['0.00012345678901', '1.23456789E-4', true],
    ['0.00005', '5E-5', false],
    ['0.0009999999999999', '0.001', true],
    ['-0.3333333333333333333333333333333333', '-0.333333333333', true],
    ['1.5E-20', '1.5E-20', false],
    ['1E+6144', '1E+6144', false],
  ])('%s → %s (planned)', (exact, text, rounded) => {
    expect(formatForDisplay(exact)).toEqual({ text, rounded })
  })

  it.each([
    // zero and short values
    ['0', '0', false],
    ['-0', '0', false],
    ['42', '42', false],
    ['-42', '-42', false],
    ['1.50', '1.5', false],
    // plain range bounds
    ['999999999999', '999999999999', false],
    ['1E+12', '1E+12', false],
    ['-999999999999.9', '-1E+12', true],
    ['0.0001', '1E-4', false],
    ['0.0000001', '1E-7', false],
    ['-1.5E-10', '-1.5E-10', false],
    // rounding
    ['123456.7890123456', '123456.789012', true],
    ['0.12345678901251', '0.123456789013', true],
    ['1.234567885E+20', '1.23456788E+20', true],
    ['1.234567895E+20', '1.2345679E+20', true],
    // the exponent's length limits the mantissa
    ['1.23456789012345E+6144', '1.234568E+6144', true],
    ['9.9999999999E+99', '1E+100', true],
  ])('%s → %s', (exact, text, rounded) => {
    expect(formatForDisplay(exact)).toEqual({ text, rounded })
  })

  it('keeps generated values within the width and notation rules', () => {
    for (const exact of randomDecimals(2000)) {
      const unsigned = formatForDisplay(exact).text.replace(/^-/, '')
      const [mantissa, exp] = unsigned.split('E')

      expect(unsigned.length, exact).toBeLessThanOrEqual(14)
      if (exp === undefined) {
        // plain: 0.001 ≤ abs(x) < 10^12, so at most 12 integer digits
        expect(unsigned, exact).toMatch(/^(0\.0{0,2}[1-9]\d*|[1-9]\d{0,11}(\.\d*[1-9])?)$/)
      } else {
        const e = Number.parseInt(exp, 10)
        expect(e < -3 || e > 11, exact).toBe(true)
        expect(mantissa, exact).toMatch(/^[1-9](\.\d{0,10}[1-9])?$/)
      }
    }
  })
})

// A deterministic stream of decimals with up to 34 digits, in scientific
// notation. Half of the exponents are near the plain range.
function randomDecimals(count: number): string[] {
  let state = 2463534242
  const random = (n: number) => {
    state ^= state << 13
    state ^= state >>> 17
    state ^= state << 5
    return (state >>> 0) % n
  }
  const values = []
  for (let i = 0; i < count; i++) {
    let digits = String(1 + random(9))
    const length = 1 + random(34)
    while (digits.length < length) {
      digits += random(10)
    }
    const exp = random(2) === 0 ? random(25) - 8 : random(12321) - 6176
    const sign = random(2) === 0 ? '-' : ''
    const mantissa =
      digits.length === 1 ? digits : digits[0] + '.' + digits.slice(1)
    values.push(`${sign}${mantissa}E${exp < 0 ? '-' : '+'}${Math.abs(exp)}`)
  }
  return values
}
