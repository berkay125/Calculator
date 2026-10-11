# Frontend prompts

This file lists the prompts given to the AI assistant (Claude Code) while
building the frontend, in chronological order. Each prompt is followed by a
short summary of the changes it led to.

The frontend was built in a separate Claude Code session that ran in parallel
with the backend session. Prompts from the backend session, and from the
planning before the split, are in [prompts.md](prompts.md).

- Prompts that shaped the design or content are quoted verbatim.
- Short procedural follow-ups are summarized.

## 1. Parallel session and frontend plan review

```text
create a new file named frontend_prompts.md and use it for saving my prompts. investigate plan for frontend part. an other session is working on backend directory so dont touch that directory to avoid conflict.
```

**Changes:**

- Created this file.
- Reviewed the frontend part of PLAN.md against the API contract and the
  backend's actual output format. Gaps and open decisions were reported in
  chat. No code or plan changes yet.

## 2. Frontend decisions

```text
Agree with most of your decisions. E- you can use 0 for first Ans. scientific calculators use it like that for recursive operations.small note backend uses Ans so keypad should insert Ans. for format for display: max 12 significant digits,strip trailing zeros, not = when rounded. switch to scientific when it does not fit you can select thresholds
```

"E" refers to the proposed behavior of `Ans` before the first result.

**Changes:** PLAN.md "Frontend design" and Phase 5 updated:

- `Ans` starts at `0`. The keypad inserts `Ans`. `s` and `a` are keyboard
  shortcuts for `sqrt(` and `Ans`.
- Input stays permissive, except that keys which would merge with `Ans` into
  a single identifier (`Ans5`) are ignored.
- Behavior after a result, Backspace, `=` and Clear is spelled out.
- Display formatting: at most 12 significant digits in a 14-character width.
  Plain notation from 0.001 up to 10^12, scientific outside that range.
  Rounded results are marked with `≈` instead of `=`. Step 11 has a table of
  test cases.
- Requests carry an id, so the reducer drops stale responses. The API client
  gets an `unexpected` result for responses outside the contract.
- Keyboard handling covers AltGr, the numpad decimal key, the dead `^` key and
  focused keypad buttons.

## 3. One notation rule and step 10

```text
only one rule is enough( 0.001 ≤ |x| < 10^12) 0.00005 can look like 5E-5. you can start step 10
```

**Changes:**

- Display formatting: plain notation only for `0.001 ≤ abs(x) < 10^12`. Small
  values that would fit, such as `0.00005`, now use scientific notation
  (`5E-5`).
- `src/api/errors.ts`: the `ErrorCode` union, built from one list of codes,
  and a `Record<ErrorCode, string>` table of user messages.
- `src/api/client.ts`: `calculate(expression, ans, signal?)` returns `ok`,
  `error`, `unexpected` or `network`. It validates the response body at
  runtime. A result must be in the backend's number format, and an error
  position must fall inside the expression.
- A Vite proxy forwards `/api` to the backend for `npm run dev` and
  `npm run preview`.
- Table-driven tests with mocked `fetch`, at 100% coverage. Three checks were
  removed one at a time to confirm a test fails without each: the abort
  rejection, the position bound, and the lookup that ignores inherited names
  like `toString`.

## 4. Backend text for syntax errors

```text
for syntax errors backend show the backend response since it is more specific other than that use your messages.
```

**Changes:**

- `errorMessage(code, message)` in `src/api/errors.ts` shows the backend's
  message for `syntax_error` (`unexpected ')'` becomes `Unexpected ')'`) and
  the frontend's own message for every other code. An empty backend message
  falls back to `Syntax error`.
- Tests for both paths. PLAN.md updated to match.

## 5. Step 11 (summarized)

- Approved the error messages and asked to start step 11.

**Changes:**

- `src/calculator/format.ts`: `formatForDisplay(exact)` returns
  `{ text, rounded }`. It works on the decimal string only, never `Number()`.
  It rounds once to the digits that fit, ties to even, and picks plain or
  scientific notation from the rounded value.
- Tests: the PLAN.md table plus edge cases, and 2000 generated values checked
  against the width and notation rules. 100% coverage. Three rules were broken
  one at a time to confirm a test fails for each: ties to even, notation chosen
  after a carry, and the width limit.
- One row in the PLAN.md table had a miscounted expected value
  (`0.03333333333` instead of `0.033333333333`). The test caught it, and the
  table was corrected.

## 6. Step 12 (summarized)

- Approved step 11 and asked to start step 12.

**Changes:**

- `src/calculator/reducer.ts`: one pure reducer for the keypad and the
  keyboard, with `input`, `backspace`, `clear`, `evaluate` and `response`
  actions. Request ids come from a counter in the state. A response whose id
  does not match the pending request is dropped.
- Behavior from PLAN.md: whole-token Backspace for `sqrt(` and `Ans`; keys
  that would merge with `Ans` are ignored; after a result, operators continue
  from `Ans`; `ans` starts at `0` and survives Clear.
- `=` after an error sends the expression again, so a network failure can be
  retried. PLAN.md updated.
- Table-driven tests, 100% coverage. Four rules were broken one at a time to
  confirm a test fails for each: the stale-response check, the `Ans` merge
  rule, whole-token Backspace, and cancelling the pending request on input.

## 7. Casio-style repeated `=`

```text
aggree with 3 decisions. yes do it like Casio style (repeated =)
```

The three decisions: request ids from a counter in the state, the failed
client result stored as the error, and `=` after an error as a retry. The
question was whether `=` after a result should run an expression with `Ans`
again, so `Ans*2+1` steps through a recurrence.

**Changes:**

- After a result, `=` runs the expression again with the new `Ans` if the
  expression uses `Ans`. Without `Ans` it still does nothing.
- The old result stays on screen while the repeat is pending. Backspace
  cancels the repeat, and a failed repeat replaces the result with the error.
- Tests for each rule, each confirmed to fail without it. PLAN.md updated.

## 8. Step 13 (summarized)

- Asked to continue with step 13.

**Changes:**

- `useCalculator` hook: runs the reducer and sends the pending request from
  an effect. The effect's cleanup aborts the request when an edit cancels it.
- `Display`: the expression with `×`, `÷`, `−` symbols and the error
  highlight (a slot after the last character for errors at the end), and the
  result line with `=` or `≈`, or the error text.
- `Keypad`: 24 keys in a 4 × 6 grid, with accessible names for symbol keys.
- `App` takes `calculate` as a prop, so tests control when responses arrive.
  Basic styling. Responsive layout and accessibility polish are left for step 15.
- oxlint's `react/exhaustive-deps` rule is enabled, and it was checked to
  flag a missing dependency.
- React Testing Library tests, 100% coverage. Removing the abort in the
  effect's cleanup, or the end-of-expression slot, makes tests fail.
