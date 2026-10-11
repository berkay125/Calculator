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
