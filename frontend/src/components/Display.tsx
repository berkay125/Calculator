import { errorMessage } from '../api/errors.ts'
import { formatForDisplay } from '../calculator/format.ts'
import type { Failure } from '../calculator/reducer.ts'

// One-character swaps keep every index in place, so error positions from the
// backend still point at the right character.
const SYMBOLS: Record<string, string> = { '*': '×', '/': '÷', '-': '−' }

type DisplayProps = {
  expression: string
  result: string | null
  error: Failure | null
}

export function Display({ expression, result, error }: DisplayProps) {
  const shown = expression.replace(/[*/-]/g, (c) => SYMBOLS[c])
  const position = error?.kind === 'error' ? error.position : undefined
  let line = ''
  if (error !== null) {
    line = failureText(error)
  } else if (result !== null) {
    const { text, rounded } = formatForDisplay(result)
    line = `${rounded ? '≈' : '='} ${text}`
  }

  return (
    <div className="display">
      <div className="expression" data-testid="expression">
        {expression === '' ? (
          <span className="placeholder">0</span>
        ) : (
          highlight(shown, position)
        )}
      </div>
      <output className={error ? 'result error' : 'result'} aria-label="result">
        {line}
      </output>
    </div>
  )
}

// Marks the character at `position`. An error at the end of the expression
// marks an empty slot after it.
function highlight(text: string, position: number | undefined) {
  if (position === undefined) {
    return text
  }
  const atEnd = position === text.length
  return (
    <>
      {text.slice(0, position)}
      <mark className={atEnd ? 'end' : undefined}>
        {atEnd ? ' ' : text[position]}
      </mark>
      {text.slice(position + 1)}
    </>
  )
}

function failureText(failure: Failure): string {
  switch (failure.kind) {
    case 'error':
      return errorMessage(failure.code, failure.message)
    case 'unexpected':
      return failure.message ?? 'Unexpected response from the server'
    case 'network':
      return 'Cannot reach the server'
  }
}
