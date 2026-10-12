# Calculator

A calculator with a React + TypeScript frontend and a Go backend. The backend
parses and evaluates expressions such as `2 * (3 + 4) ^ 2` with a hand-written
parser and exact decimal arithmetic. One Go binary serves both the API and the
frontend.

[PLAN.md](PLAN.md) has the full design and the step-by-step implementation
plan. The design decisions below summarize it.

## Repository layout

| Path | Contents |
|---|---|
| `backend/` | Go module: `cmd/server` and `internal/{arith,parser,eval,api,web}` |
| `frontend/` | Vite + React + TypeScript app |
| `coverage/` | Committed coverage reports, see [Coverage](#coverage) |
| `Dockerfile` | Multi-stage build of the single image, see [Docker](#docker) |
| `PLAN.md` | Design decisions and implementation plan |
| `Calculator UI.html` | The UI design, made while planning |
| `prompts.md` | Prompts given to the AI assistant during development |
| `frontend_prompts.md` | Prompts from the separate session that built the frontend |

## Setup

### Prerequisites

- Go 1.27+
- Node.js 22.12+ and npm

With Docker, neither is needed.

### Docker

The Dockerfile builds one image: the server with the frontend embedded in
it. From the repository root:

```sh
docker build -t calculator .
docker run --rm -p 8080:8080 calculator
```

The calculator is then at `http://localhost:8080/`.

- Build stages install the dependencies, build the frontend and compile the
  server. The image holds only the static binary, on distroless, and runs
  as a non-root user.
- `docker stop` sends `SIGTERM`, and the server shuts down gracefully:
  requests in flight get up to 5 seconds to finish.

### Quick start

Build the frontend, copy the build into the backend, which embeds it into the
binary, and start the server. From the repository root:

```sh
npm --prefix frontend ci
npm --prefix frontend run build
cp -r frontend/dist/. backend/internal/web/dist/
cd backend
go run ./cmd/server
```

In Windows PowerShell, copy with
`Copy-Item -Recurse -Force frontend/dist/* backend/internal/web/dist/`.

The calculator is then at `http://localhost:8080/`. The copied build is
gitignored. Without it, the server serves only the API and logs a warning.

## Running the backend and frontend

For development, run the backend and the Vite dev server side by side. The
dev server forwards `/api` to the backend.

### Backend

Run these from `backend/`:

```sh
go run ./cmd/server                 # serve on :8080
go run ./cmd/server -addr :8090     # or another address (env ADDR works too)
go vet ./...
go test ./...
```

The server logs JSON lines to stderr. Ctrl+C or `SIGTERM` stops it
gracefully: requests in flight get up to 5 seconds to finish.

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

## Coverage

### Backend

Run these from `backend/`:

```sh
go test ./... -coverpkg=./... -coverprofile=coverage.out
go tool cover -func=coverage.out                    # summary
go tool cover -html=coverage.out -o coverage.html   # HTML report
```

With `-coverpkg=./...`, code counts as covered when any package's tests run
it. For example, the `eval` tests also cover `arith`.

### Frontend

`npm test` writes a summary to the terminal, a table of every file to
`frontend/coverage/summary.txt` and an HTML report to
`frontend/coverage/index.html`. It fails when any of statements, branches,
functions or lines drops below 85%.

### Committed snapshot

The reports above are gitignored. A snapshot is committed under `coverage/`:

| File | Contents |
|---|---|
| `coverage/backend/summary.txt` | Coverage of every Go function |
| `coverage/backend/coverage.html` | Go source annotated with coverage |
| `coverage/frontend/summary.txt` | Coverage of every frontend file |
| `coverage/frontend/index.html` | HTML report of the frontend |

GitHub shows the HTML reports as source, so open them from a local clone.

| Backend package | Statements |
|---|---|
| `internal/arith` | 100% |
| `internal/parser` | 100% |
| `internal/eval` | 100% |
| `internal/api` | 100% |
| `internal/web` | 98.0% |
| `cmd/server` | 85.7% |
| **Total** | **98.1%** |

Not covered: `main` itself, three error returns in `run` (the embedded build
fails to load, `Serve` fails on its own, `Shutdown` runs out of time) and an
unreachable `panic` in `web.Dist`.

The frontend is at 100% for statements, branches, functions and lines.

To regenerate the snapshot, from the repository root:

```sh
cd backend
go test ./... -coverpkg=./... -coverprofile=coverage.out
go tool cover -func=coverage.out -o ../coverage/backend/summary.txt
go tool cover -html=coverage.out -o ../coverage/backend/coverage.html
cd ../frontend
npm test
cp -r coverage/. ../coverage/frontend/
```

In Windows PowerShell, copy with
`Copy-Item -Recurse -Force coverage/* ../coverage/frontend/`.

With Docker, one command runs both test suites on Linux and writes the
snapshot. It fails when the Go total drops below 90% or the frontend below
its thresholds. `--output` needs BuildKit, the default builder since Docker
23.

```sh
docker build --target coverage --output type=local,dest=coverage .
```

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
| `{"expression": "-2^2"}` | 200 | `{"result":"-4"}` |
| `{"expression": "200 + 10%"}` | 200 | `{"result":"220"}` |
| `{"expression": "100 + (10%)"}` | 200 | `{"result":"100.1"}` |
| `{"expression": "Ans * 2", "ans": "21"}` | 200 | `{"result":"42"}` |
| `{"expression": "1 / 3"}` | 200 | `{"result":"0.3333333333333333333333333333333333"}` |
| `{"expression": "2 ^ 200"}` | 200 | `{"result":"1.606938044258990275541962092341163E+60"}` |
| `{"expression": "2 * (3 + 4))"}` | 400 | `{"error":{"code":"syntax_error","message":"unexpected ')'","position":11}}` |
| `{"expression": "1/0 + x"}` | 400 | `{"error":{"code":"unknown_variable","message":"unknown variable 'x'","position":6}}` |
| `{"expression": "1", "answer": "5"}` | 400 | `{"error":{"code":"invalid_request","message":"unknown field \"answer\""}}` |
| `{"expression": "1 / 0"}` | 422 | `{"error":{"code":"division_by_zero","message":"division by zero","position":2}}` |
| `{"expression": "sqrt(-1)"}` | 422 | `{"error":{"code":"domain_error","message":"result is not a real number","position":0}}` |

- `position` is a 0-based index into `expression`. It is left out when it does
  not apply.
- Any method other than `POST` gets `405 Method Not Allowed`. Other paths
  under `/api/` get 404.

### Error codes

400 means the input is invalid, and 422 means the math is undefined.

| Status | Code | When | `position` |
|---|---|---|---|
| 400 | `invalid_request` | Malformed JSON, missing `expression`, unknown or repeated field, body over 4 KiB | – |
| 400 | `invalid_ans` | `ans` is not a valid decimal, is outside the exponent range, or has more than 34 digits | – |
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

An error without a code from this table is a bug. The server logs it and
answers 500 with the code `internal_error`.

## Design decisions and assumptions

### Assumptions

Where the assignment left room for interpretation:

- **Math conventions.** `^` binds tighter than unary minus and is
  right-associative, as in mathematics: `-2^2` is `-4` (spreadsheets give 4),
  and `2^3^2` is 512. `0^0` is 1.
- **Percent** works like a phone calculator: `200 + 10%` is 220, not 200.1 as
  in a spreadsheet.
- **Precision.** 34 significant digits are enough. Longer results are rounded,
  ties to even, and the screen shows at most 12 digits.
- **Undefined math is an error.** `1/0` and `sqrt(-1)` return an error, never
  Infinity or NaN.
- **Stateless server.** The client keeps `Ans` and sends it with every
  request. There are no sessions, accounts or history.
- **Public endpoint.** There is no authentication, so every input is bounded
  (see [Limits](#limits)).
- **Input.** Expressions are ASCII, with `.` as the decimal point. `sqrt` is
  the only function.

### Backend

The full rules are in PLAN.md, under
[Parser and evaluator design](PLAN.md#parser-and-evaluator-design).

#### Packages

Each layer is its own package, and dependencies point one way:

| Package | Job | Depends on |
|---|---|---|
| `internal/parser` | Lexer, AST, precedence climbing | nothing |
| `internal/arith` | Decimal operations, function registry, math errors | apd |
| `internal/eval` | Walks the AST: percent rule, names, `Ans` | `parser`, `arith` |
| `internal/api` | HTTP handler, limits, error code → status | `eval` |
| `internal/web` | Serves the embedded frontend build | nothing |
| `cmd/server` | Wires `api` and `web`, timeouts, graceful shutdown | `api`, `web` |

apd is the only third-party Go dependency. The parser keeps number literals as
text, so it does not import apd.

#### Hand-written parser, one precedence table

Binary operators are parsed by precedence climbing. Their binding powers sit
in one table in `internal/parser/parser.go`, next to the powers of unary minus
and postfix percent:

| Operator | Power | Associativity |
|---|---|---|
| `+ -` | 1 | left |
| `* /` | 2 | left |
| unary `-` | 3 | prefix |
| `^` | 4 | right |
| `%` | 5 | postfix |

- So `-2^2` is `-4`, `2^-1` is `0.5`, `2^3^2` is `512` and
  `2 * (3 + 4) ^ 2` is `98`.
- Parentheses stay in the tree as a `Group` node, because they change what
  percent means (see below).
- The lexer accepts ASCII only, so a position is a byte, rune and JavaScript
  string index at once. The frontend highlights it directly.
- Syntax errors, at the offending character: implicit multiplication
  (`2(3)`), unary plus (`+2`), scientific notation (`1e5`) and `5.`. `.5` is
  accepted.

#### Exact decimal arithmetic

- `cockroachdb/apd` with the limits of IEEE 754 decimal128: 34 significant
  digits, exponents up to ±6144, and ties rounded to even.
- Literals and `ans` keep every digit. Only operation results are rounded, so
  a result never depends on digits that were dropped before the operation.
  Unary minus is exact.
- The final value is rounded to 34 digits too, so a long literal on its own
  comes back with 34 digits.
- Results are normalized: `1.50 + 1.50` is `3`, and `-0` is `0`. They use
  plain notation from `1E-7` up to 34 integer digits, and scientific notation
  (`1E-8`, `1.5E+40`) outside that. Every result parses back as `ans`.
- `0^0` is `1`, and `0^-1` is `division_by_zero`. apd treats the first as
  undefined and returns Infinity for the second without an error.
- Results outside the exponent range are `overflow` or `underflow`, never
  infinity or zero. For powers the direction is worked out from the operands,
  because apd reports some powers that overflow as underflow.

#### Percent like a phone calculator

The parser treats `%` as a postfix operator. What it means depends on where it
appears, so the evaluator applies the rule:

| Expression | Meaning | Example |
|---|---|---|
| `a + b%`, `a - b%` | a ± a·b/100 | `200 + 10%` = 220 |
| `a * b%`, `a / b%` | b/100 | `200 * 10%` = 20 |
| `b%` on its own | b/100 | `50%` = 0.5 |

- The rule applies when the right operand of `+` or `-` is a percent, also
  behind minus signs: `100 + -10%` is 90.
- Parentheses block it, because a `Group` node is not a percent:
  `100 + (10%)` is 100.1.
- It compounds, like a phone calculator: `200 + 10% + 10%` is 242.

#### `Ans` is a variable, not text

- The request sends `ans` as its own field, and `Ans` in the expression is a
  variable. Names are case-sensitive, so `ans` in an expression is an unknown
  variable.
- Substituting the value into the text would move error positions away from
  what the user sees. The backend's output format (`1.5E+40`) would also have
  to be valid input.
- `ans` has at most 34 digits, like every result, and a longer one is
  `invalid_ans`. Every problem with `ans` is a 400, overflow included: it is
  invalid input, not undefined math.

#### Functions without touching the parser

- The parser accepts a call to any name. The evaluator looks the name up in a
  registry in `arith`, which holds each function's arity and implementation.
- Adding a function such as `log10` takes one registry entry and a test. Only
  `sqrt` exists today.
- Variables are resolved from a table as well, so a constant such as `pi`
  would be one more entry.

#### Errors

- Every error type has a `Code()` method, and positional errors also have
  `Pos()`. The `api` package knows only these interfaces, plus one
  code → status table.
- `eval.Codes()` lists every code evaluation can return, and a test checks
  that each one has a status. A new code without one fails the test instead
  of becoming a 500.
- Names and argument counts are checked before anything is computed, so
  `1/0 + x` is a 400 `unknown_variable`, not a 422.
- Math errors point at the operator or function that failed: `1 / 0` at the
  `/`.
- The request body is decoded strictly with `encoding/json/v2`. An unknown
  field, a field in different case (`ANS`) or a repeated field is
  `invalid_request`. Otherwise a misspelled `ans` would be ignored, and the
  result would silently be wrong.

#### Limits

The endpoint is public, so every input is bounded:

| Limit | Value |
|---|---|
| Request body | 4 KiB |
| Expression | 1000 characters |
| `ans` | 34 digits |
| Parenthesis nesting | 100 levels |
| Numbers | decimal128 exponent range, so `10^10^10` fails fast with `overflow` |
| Server timeouts | Read header 5 s, read and write 10 s, idle 60 s |

The slowest request found takes about 0.4 s.

#### Serving the frontend

- The frontend build is embedded with `go:embed`, so one binary serves the API
  under `/api/` and the frontend at `/`. They share an origin, so there is no
  CORS to configure.
- Only the build's files are served, with GET and HEAD. There are no directory
  listings, no dotfiles, and no fallback to `index.html`, because the app has
  no client-side routes.
- Content types come from a fixed table. A file with an unknown extension stops
  the server at startup, so nothing is served with a guessed type.
- Vite's content-hashed files under `assets/` are cached as immutable.
  `index.html` is revalidated with an ETag, so a new build shows up at once.

#### Tests

- Table-driven tests in every package. The `api` and `web` packages are tested
  with `httptest`. `run` is tested on a random port, including a graceful
  shutdown with a request in flight.
- Two fuzz targets:
  - `FuzzParse`: no input panics, error and node positions are in range, the
    tree follows the precedence table, and printing a tree and parsing it
    again gives the same tree.
  - `FuzzEvaluate`, with a fuzzed `ans`: every error has a code from the
    catalog and a position in range, and every result has at most 34 digits
    and parses back as `ans`.
- Fuzzing found a bug: a non-integer power of a base with thousands of digits
  failed with an error that had no code, and was slow well before that.
  `arith.Pow` now splits such a power into integer and fractional parts.
- Most rules were checked by breaking them on purpose, for example a
  left-associative `^` or `Close` instead of `Shutdown`, and confirming that a
  test fails.

#### Trade-offs and limitations

- Precision is finite. `1/3*3` is `0.9999999999999999999999999999999999`.
  The display rounds it to `≈ 1`, and `Ans` keeps the exact value, as on phone
  calculators.
- Non-integer powers are computed through ln/exp and are approximate.
- The percent rule applies only to the right operand of `+` and `-`:
  `200 + 10% * 2` is 200.2, because the right operand is `10% * 2`.
- Very long or very deeply nested input is rejected by the limits.
- There is no rate limiting, and requests are not logged.
- Out of scope: functions other than `sqrt`, constants, implicit
  multiplication, scientific notation in input, modulo and history.
- Static files are not compressed. The JavaScript is 230 KB, 72 KB gzipped.

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
- An end-to-end check in headless Chrome typed 19 scenarios against the real
  server: results, rounding, `Ans` and repeated `=`, and every error the
  keypad can produce.

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
