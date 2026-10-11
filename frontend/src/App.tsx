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
      <h1>Calculator</h1>
      <Display
        expression={state.expression}
        result={state.result}
        error={state.error}
      />
      <Keypad onAction={dispatch} />
    </main>
  )
}

export default App
