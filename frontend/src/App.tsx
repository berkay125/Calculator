import './App.css'
import { type Calculate, calculate as callApi } from './api/client.ts'
import { useCalculator } from './calculator/useCalculator.ts'
import { useKeyboard } from './calculator/useKeyboard.ts'
import { Display } from './components/Display.tsx'
import { Keypad } from './components/Keypad.tsx'

// Tests pass their own `calculate` to control when responses arrive.
function App({ calculate = callApi }: { calculate?: Calculate }) {
  const { state, dispatch } = useCalculator(calculate)
  useKeyboard(dispatch)
  return (
    <main className="calculator">
      <header className="header">
        <h1>Calculator</h1>
        <p>Exact decimal arithmetic, computed on the server.</p>
      </header>
      <div className="card">
        <Display
          expression={state.expression}
          result={state.result}
          error={state.error}
          pending={state.pending !== null}
        />
        <Keypad onAction={dispatch} />
        <p className="hint">
          Type freely · <kbd>Enter</kbd> to calculate · <kbd>Esc</kbd> to clear
        </p>
      </div>
    </main>
  )
}

export default App
