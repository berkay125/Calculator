# Calculator

A calculator with a React + TypeScript frontend and a Go backend. The backend
parses and evaluates expressions such as `2 * (3 + 4) ^ 2` with a hand-written
parser and exact decimal arithmetic.

> Work in progress. [PLAN.md](PLAN.md) describes the design and the
> implementation steps.

## Repository layout

| Path | Contents |
|---|---|
| `backend/` | Go module: `internal/{arith,parser,eval,api,web}` |
| `frontend/` | Vite + React + TypeScript app |
| `PLAN.md` | Design decisions and implementation plan |
| `prompts.md` | Prompts given to the AI assistant during development |
| `frontend_prompts.md` | Prompts from the separate session that built the frontend |

## Setup

### Prerequisites

- Go 1.27+
- Node.js 22.12+ and npm

### Backend

<!-- Backend session: the Docker usage (step 17) goes here. -->

Run these from `backend/`:

```sh
go run ./cmd/server                 # serve the API on :8080
go run ./cmd/server -addr :8090     # or another address (env ADDR works too)
go vet ./...
go test ./...
```

The server logs JSON lines to stderr. Ctrl+C or `SIGTERM` stops it
gracefully: requests in flight get up to 5 seconds to finish.

```sh
curl -X POST localhost:8080/api/v1/calculate -d '{"expression": "2 * (3 + 4) ^ 2"}'
# {"result":"98"}
```

`go test` also runs the fuzz seeds. To search for new failing inputs:

```sh
go test ./internal/parser -fuzz=FuzzParse -fuzztime=1m
go test ./internal/eval -fuzz=FuzzEvaluate -fuzztime=1m
```

### Frontend

Run these from `frontend/`:

```sh
npm ci              # install dependencies
npm run dev         # dev server on http://localhost:5173
npm test            # run tests with coverage (fails below the thresholds)
npm run test:watch  # run tests in watch mode
npm run typecheck   # type-check with tsc
npm run lint        # lint with oxlint
npm run build       # production build into dist/
npm run preview     # serve the production build
```

`npm run dev` and `npm run preview` forward `/api` to the backend on
`localhost:8080`, so start the backend first. The frontend calls the relative
URL `/api/v1/calculate`. In production the Go server serves the frontend and
the API from the same origin, so there is no CORS and nothing to configure.

### Coverage

#### Backend

Run these from `backend/`:

```sh
go test ./... -coverpkg=./... -coverprofile=coverage.out
go tool cover -func=coverage.out                    # summary
go tool cover -html=coverage.out -o coverage.html   # HTML report
```

#### Frontend

`npm test` writes a text summary to the terminal and an HTML report to
`frontend/coverage/index.html`. It fails when any of statements, branches,
functions or lines drops below 85%.

#### Committed snapshot

The development reports above are gitignored. The final snapshot is committed
under `coverage/` (see PLAN.md, step 18).

## API examples

There is one endpoint, `POST /api/v1/calculate`. It takes an expression and an
optional `ans`, the value that `Ans` stands for. Numbers travel as strings, so
no precision is lost in JSON.

```sh
curl -s http://localhost:8080/api/v1/calculate \
  -H 'Content-Type: application/json' \
  -d '{"expression": "2 * (3 + 4) ^ 2"}'
```

```json
{"result":"98"}
```

In Windows PowerShell:

```powershell
Invoke-RestMethod -Method Post http://localhost:8080/api/v1/calculate `
  -ContentType application/json -Body '{"expression": "2 * (3 + 4) ^ 2"}'
```

More requests and their responses:

| Request body | Status | Response body |
|---|---|---|
| `{"expression": "0.1 + 0.2"}` | 200 | `{"result":"0.3"}` |
| `{"expression": "200 + 10%"}` | 200 | `{"result":"220"}` |
| `{"expression": "Ans * 2", "ans": "21"}` | 200 | `{"result":"42"}` |
| `{"expression": "1 / 3"}` | 200 | `{"result":"0.3333333333333333333333333333333333"}` |
| `{"expression": "2 * (3 + 4))"}` | 400 | `{"error":{"code":"syntax_error","message":"unexpected ')'","position":11}}` |
| `{"expression": "1 / 0"}` | 422 | `{"error":{"code":"division_by_zero","message":"division by zero","position":2}}` |

- `position` is a 0-based index into `expression`. It is left out when it does
  not apply.
- Any method other than `POST` gets `405 Method Not Allowed`.

### Error codes

400 means the input is invalid, and 422 means the math is undefined.

| Status | Code | When | `position` |
|---|---|---|---|
| 400 | `invalid_request` | Malformed JSON, missing `expression`, unknown field, body over 4 KiB | – |
| 400 | `invalid_ans` | `ans` is not a valid decimal, or has more than 34 digits | – |
| 400 | `expression_too_long` | Expression longer than 1000 characters | – |
| 400 | `nesting_too_deep` | Parentheses nested more than 100 levels deep | ✓ |
| 400 | `syntax_error` | Unexpected token, unclosed parenthesis, empty expression | ✓ |
| 400 | `unknown_function` | `foo(1)` | ✓ |
| 400 | `unknown_variable` | `x`, or `Ans` without an `ans` value | ✓ |
| 400 | `wrong_argument_count` | `sqrt(1, 2)` | ✓ |
| 422 | `division_by_zero` | `1/0`, `0/0`, `0^-1` | ✓ |
| 422 | `domain_error` | `sqrt(-1)`, `(-8)^(1/3)` | ✓ |
| 422 | `overflow` | Result exceeds the exponent range | ✓ |
| 422 | `underflow` | Result is too close to zero for the exponent range | ✓ |

## Design decisions

### Backend

<!-- Backend session: parser, decimal arithmetic, percent rule, Ans, errors and limits (PLAN.md step 18). -->

### Frontend

The full rules are in PLAN.md, under
[Frontend design](PLAN.md#frontend-design).

#### One reducer, and stale responses are dropped

- The keypad and the keyboard dispatch the same actions to one pure reducer
  (`src/calculator/reducer.ts`). Every behavior has a single implementation
  with table-driven tests.
- Pressing `=` puts a request with a new id into the state. The
  `useCalculator` hook sends it from an effect and aborts it in the effect's
  cleanup.
- Any edit clears the pending request. The reducer drops a response whose id
  does not match, so a slow response can never overwrite newer input.

#### The display shows exactly what is sent

Error positions from the backend are indexes into the expression, so the
screen shows the same string, character for character.

- Only one-character swaps are allowed: `*` → `×`, `/` → `÷`, `-` → `−`.
- `√` therefore inserts the text `sqrt(`, and the expression is not spaced
  out.

#### The backend is the only parser

The reducer does not know the grammar. It appends keys, and the backend
reports syntax errors with a position, which the display highlights.

There is one exception. Right after `Ans`, a digit, `(`, `Ans` or `√` is
ignored. The lexer would merge it into one name (`Ans5`), and the error would
be a misleading "unknown variable".

#### Ans

- `Ans` is sent as the `ans` field and is never substituted into the text, so
  error positions match the screen.
- It starts at `0`, as on scientific calculators.
- After a result, `=` runs an expression that uses `Ans` again with the new
  value, as on a Casio: `Ans*2+1`, `=`, `=`, … gives 1, 3, 7, …
- After a result, an operator continues from `Ans` (`+` gives `Ans+`). Any
  other key starts a new expression.

#### Rounding for the screen

The backend returns up to 34 significant digits. The display shows at most
12, rounded once with ties to even. The formatter works on the decimal string
and never calls `Number()`, which would lose digits and overflow above 1e308.

| Rule | Value |
|---|---|
| Width | 14 characters, not counting a minus sign |
| Plain notation | `0.001 ≤ abs(x) < 10^12` |
| Scientific notation | Everything else, in the backend's format (`1.5E+40`) |
| Marker | `=` for an exact result, `≈` for a rounded one |

A rounded result also shows its exact value, with a copy button. `Ans` always
keeps the exact value. [Display formatting](PLAN.md#display-formatting)
explains the thresholds.

#### Errors

- The UI shows its own message for each error code. Syntax errors are the
  exception: the backend's message is more specific, and the column is added
  (`Unexpected ')' at column 4`).
- The API client returns `ok`, `error` (a known code), `unexpected` or
  `network`. `unexpected` covers anything outside the contract: a 5xx, a body
  that is not JSON, an unknown code, or a position outside the expression.
  Responses are validated at runtime, so the display can trust every value.

#### Keyboard

| Key | Action |
|---|---|
| `0`–`9` `.` `+` `-` `*` `/` `^` `%` `(` `)` | Insert the character |
| `s` | Insert `sqrt(` |
| `a` | Insert `Ans` |
| Enter, `=` | Calculate |
| Backspace | Delete; `sqrt(` and `Ans` go in one step |
| Escape | Clear |

- Ctrl and Meta shortcuts such as Ctrl+C stay with the browser. AltGr, which
  Windows reports as Ctrl+Alt, still types.
- The numpad decimal key types `,` on Turkish and German layouts. It inserts
  `.` here.
- Clicked keys do not keep focus, so Enter calculates instead of pressing the
  last clicked key again. Keys reached with Tab still respond to Enter and
  Space.

#### Accessibility

- Every key has an accessible name: `Divide`, `Square root`,
  `Insert last answer`, …
- The result is a live region. Screen readers hear "Equals 5" or
  "Approximately 0.333333333333".
- Errors appear in a `role="alert"` element. The expression is marked
  `aria-invalid` and points to the alert with `aria-describedby`.
- While a request is pending, the result has `aria-busy`. The visual loading
  state appears only after 200 ms, so fast requests do not flash.

#### Visual design

The layout follows `Calculator UI.html`, a design made while planning.

- The fonts are IBM Plex Sans and IBM Plex Mono, self-hosted through
  `@fontsource`, so the app makes no third-party requests.
- On phones the calculator fills the screen. From 600px wide it becomes a
  centered card.
- The design's history panel is not built, because history is out of scope.

#### Tests

- Vitest, React Testing Library and user-event, in jsdom.
- The reducer, formatter, keymap and API client have table-driven unit tests.
  The formatter is also checked against 2000 generated values for its width
  and notation rules.
- Component tests render the app with an injected `calculate`, so they control
  when a response arrives, and whether it arrives at all. Only the API client
  tests mock `fetch`.
- The key rules were checked by breaking each one on purpose and confirming
  that a test fails.
- The layout was checked in headless Chrome at 390 × 844 and at desktop width.

#### Known limitations

- On layouts where `^` is a dead key (Turkish, German, French), type `^`
  followed by Space. A `^` followed directly by a digit is lost. The keypad's
  `xʸ` key always works.
- The copy button needs the clipboard API, which browsers offer only on https
  and localhost.
- When the backend is stopped, the dev server's proxy answers with an empty
  500. The app then shows "Unexpected response from the server" rather than
  "Cannot reach the server".
- The result switches to the smaller font based on the number of characters,
  not on the measured width of the text.
- There is only a light theme, as in the design.
