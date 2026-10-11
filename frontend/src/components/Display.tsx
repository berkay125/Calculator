import {
  type ReactNode,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from 'react'
import { errorMessage } from '../api/errors.ts'
import { formatForDisplay } from '../calculator/format.ts'
import type { Failure } from '../calculator/reducer.ts'

// One-character swaps keep every index in place, so error positions from the
// backend still point at the right character.
const SYMBOLS: Record<string, string> = { '*': '×', '/': '÷', '-': '−' }

// Results longer than this switch to the smaller font, so they fit a phone.
const LARGE_MAX_CHARS = 7

type DisplayProps = {
  expression: string
  result: string | null
  error: Failure | null
  pending: boolean
}

export function Display({ expression, result, error, pending }: DisplayProps) {
  const lineRef = useRef<HTMLDivElement>(null)
  const shown = expression.replace(/[*/-]/g, (c) => SYMBOLS[c])
  const position = error?.kind === 'error' ? error.position : undefined

  // A long expression scrolls: keep its end in view while typing, or the
  // error highlight when there is one.
  useLayoutEffect(() => {
    const line = lineRef.current as HTMLDivElement
    const mark = line.querySelector('mark')
    if (mark === null) {
      line.scrollLeft = line.scrollWidth
    } else {
      mark.scrollIntoView({ block: 'nearest', inline: 'center' })
    }
  }, [expression, position])

  let value: ReactNode = null
  let exact: string | null = null
  let long = false
  if (result !== null) {
    const { text, rounded } = formatForDisplay(result)
    value = (
      <>
        <span className="marker" aria-hidden="true">
          {rounded ? '≈' : '='}
        </span>
        <span className="visually-hidden">
          {rounded ? 'Approximately ' : 'Equals '}
        </span>
        {text}
      </>
    )
    exact = rounded ? result : null
    long = text.length > LARGE_MAX_CHARS
  } else if (pending) {
    value = <span className="dots">…</span>
  }

  return (
    <section
      aria-label="Display"
      className={error === null ? 'display' : 'display invalid'}
    >
      <span className="precision">Decimal · 34 digits</span>
      <div className="readout">
        <div
          ref={lineRef}
          className="expression"
          role="textbox"
          aria-readonly="true"
          aria-label="Expression"
          aria-invalid={error !== null}
          aria-describedby={error === null ? undefined : 'expression-error'}
        >
          {expression === '' ? (
            <span className="placeholder">0</span>
          ) : (
            highlight(shown, position)
          )}
        </div>
        <output
          className={['result', long && 'long', pending && 'pending']
            .filter(Boolean)
            .join(' ')}
          aria-live="polite"
          aria-busy={pending}
        >
          {value}
        </output>
        {error !== null && <ErrorMessage id="expression-error" failure={error} />}
        {exact !== null && <Exact key={exact} value={exact} />}
      </div>
    </section>
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

function ErrorMessage({ id, failure }: { id: string; failure: Failure }) {
  let badge: string | null = null
  if (failure.kind === 'error') {
    badge = failure.code.toUpperCase()
  } else if (failure.kind === 'unexpected') {
    badge = `HTTP ${failure.status}`
  }
  return (
    <div id={id} className="error" role="alert">
      {badge !== null && (
        <span className="code" aria-hidden="true">
          {badge}
        </span>
      )}
      <span className="message">{failureText(failure)}</span>
    </div>
  )
}

function failureText(failure: Failure): string {
  switch (failure.kind) {
    case 'error': {
      const message = errorMessage(failure.code, failure.message)
      // Columns count from 1. The highlight shows the same place.
      return failure.code === 'syntax_error' && failure.position !== undefined
        ? `${message} at column ${failure.position + 1}`
        : message
    }
    case 'unexpected':
      return failure.message ?? 'Unexpected response from the server'
    case 'network':
      return 'Cannot reach the server'
  }
}

// The exact value behind a rounded result, with a copy button.
function Exact({ value }: { value: string }) {
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (!copied) {
      return
    }
    const timer = setTimeout(() => setCopied(false), 1500)
    return () => clearTimeout(timer)
  }, [copied])

  return (
    <div className="exact">
      <div className="exact-text">
        <span className="exact-label">Exact</span>
        <span className="exact-value">{value}</span>
      </div>
      <button
        type="button"
        className="copy"
        aria-label={copied ? 'Copied' : 'Copy exact value'}
        // Like the keypad: keep Enter for evaluating.
        onMouseDown={(e) => e.preventDefault()}
        onClick={() => {
          // The clipboard API exists only in secure contexts (https, localhost).
          navigator.clipboard?.writeText(value).then(
            () => setCopied(true),
            () => {},
          )
        }}
      >
        {copied ? checkIcon : copyIcon}
      </button>
    </div>
  )
}

const copyIcon = (
  <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <rect x="9" y="9" width="11" height="11" rx="2" />
    <path d="M5 15V6a2 2 0 0 1 2-2h9" />
  </svg>
)

const checkIcon = (
  <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <path d="M5 12.5l4.5 4.5L19 7.5" />
  </svg>
)
