# Calculator

A calculator with a React + TypeScript frontend and a Go backend. The backend
parses and evaluates expressions such as `2 * (3 + 4) ^ 2` with a hand-written
parser and exact decimal arithmetic. One Go binary serves both the API and the
frontend.

This README covers the essentials. [PLAN.md](PLAN.md) has the full design, the
reasoning behind each decision and the step-by-step implementation plan.

## Repository layout

| Path | Contents |
|---|---|
| `backend/` | Go module: `cmd/server` and `internal/{arith,parser,eval,api,web}` |
| `frontend/` | Vite + React + TypeScript app |
| `coverage/` | Committed coverage reports, see [Coverage](#coverage) |
| `Dockerfile` | Multi-stage build of the single image |
| `PLAN.md` | Design decisions and implementation plan |
| `Calculator UI.html` | The UI design, made while planning |
| `prompts.md` | Prompts given to the AI assistant during development |
| `frontend_prompts.md` | Prompts from the separate session that built the frontend |

## Setup

### Prerequisites

- Go 1.27+. `backend/go.mod` names the 1.27.2 toolchain, which an older `go`
  downloads once, so local coverage matches the Docker build.
- Node.js 22.12+ and npm.

With Docker, neither is needed.

### Docker

```sh
docker build -t calculator .
docker run --rm -p 8080:8080 calculator
```

The calculator is then at `http://localhost:8080/`. The image holds only the
static server binary, with the frontend embedded, and runs on distroless as a
non-root user.

### Without Docker

Build the frontend, copy the build into the backend, which embeds it, and start
the server. From the repository root:

```sh
npm --prefix frontend ci
npm --prefix frontend run build
cp -r frontend/dist/. backend/internal/web/dist/
cd backend
go run ./cmd/server
```

In Windows PowerShell, copy with
`Copy-Item -Recurse -Force frontend/dist/* backend/internal/web/dist/`.

The calculator is then at `http://localhost:8080/`. Without the copied build,
which is gitignored, the server serves only the API.

## Running the backend and frontend

For development, run both side by side. The Vite dev server forwards `/api` to
the backend on `localhost:8080`.

From `backend/`:

```sh
go run ./cmd/server     # serve on :8080 (-addr or env ADDR to change it)
go vet ./...
go test ./...           # also runs the fuzz seeds

# search for new failing inputs
go test ./internal/parser -fuzz=FuzzParse -fuzztime=1m
go test ./internal/eval -fuzz=FuzzEvaluate -fuzztime=1m
```

The server logs JSON lines to stderr. Ctrl+C or `SIGTERM` stops it
gracefully: requests in flight get up to 5 seconds to finish.

From `frontend/`:

```sh
npm ci              # install dependencies
npm run dev         # dev server on http://localhost:5173
npm test            # tests with coverage (fails below 85%)
npm run typecheck   # type-check with tsc
npm run lint        # lint with oxlint
npm run build       # production build into dist/
```

## Coverage

| Area | Coverage |
|---|---|
| `internal/arith`, `parser`, `eval`, `api` | 100% |
| `internal/web` | 97.8%: an unreachable `panic` |
| `cmd/server` | 83.6%: `main` and three error returns in `run` |
| **Backend total** | **98.0%** of statements |
| **Frontend** | **100%** of statements, branches, functions and lines |

The reports are committed under `coverage/`: a `summary.txt` and an HTML report
for each side. GitHub shows HTML as source, so open those from a local clone.
To regenerate them:

```sh
docker build --target coverage --output type=local,dest=coverage .
```

The build fails when the Go total drops below 90% or the frontend below 85%.
PLAN.md lists the commands without Docker, under [Coverage](PLAN.md#coverage).

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
  right-associative: `-2^2` is `-4` (spreadsheets give 4), and `2^3^2` is 512.
  `0^0` is 1.
- **Percent** works like a phone calculator: `200 + 10%` is 220, not 200.1 as
  in a spreadsheet.
- **Precision.** 34 significant digits are enough. The screen shows at most 12.
- **Undefined math is an error**, never Infinity or NaN.
- **Stateless server.** The client keeps `Ans` and sends it with every
  request. There are no sessions, accounts or history.
- **Public endpoint** without authentication, so every input is bounded.
- **Input** is ASCII, with `.` as the decimal point. `sqrt` is the only
  function.

### Backend

- **Exact decimal arithmetic.** `cockroachdb/apd` with the limits of IEEE 754
  decimal128: 34 significant digits, ties rounded to even. `0.1 + 0.2` is
  exactly `0.3`. Inputs keep every digit, and only operation results are
  rounded. Numbers cross the API as strings, so JSON never turns them into
  floats.
- **Hand-written parser, one precedence table.** Precedence climbing, with the
  binding power of every operator in one table in
  `internal/parser/parser.go`: `+ -` < `* /` < unary `-` < `^`
  (right-associative) < postfix `%`.
- **Layers point one way.** `parser` depends on nothing and `arith` only on
  apd, the only third-party Go dependency. `eval` combines the two, `api`
  knows only `eval`, and `cmd/server` wires `api` and `web` together.
- **The evaluator owns percent.** Its meaning depends on context: `a ± b%` is
  a ± a·b/100, and everywhere else `b%` is b/100. Parentheses stay in the tree
  as a `Group` node and block the rule: `100 + (10%)` is 100.1.
- **`Ans` is a variable, not text.** It travels in its own `ans` field.
  Substituting it into the expression would move error positions away from
  what the user sees, and the output format (`1.5E+40`) would have to be valid
  input.
- **Functions are a registry.** The parser accepts a call to any name, and the
  evaluator looks it up in `arith`. Adding `log10` takes one entry and a test.
- **Every error has a code.** Error types expose `Code()`, and positional ones
  `Pos()`. The API maps codes to statuses in one table, and a test walks every
  code, so a new one cannot fall through to a 500. Names are checked before
  any math, so `1/0 + x` is a 400.
- **Strict, bounded input.** The body is decoded with `encoding/json/v2`,
  which rejects unknown, differently cased and repeated fields. A misspelled
  `ans` would otherwise be ignored, and the result would silently be wrong.
  Every input has a limit (see the error codes), and the exponent range makes
  `10^10^10` fail fast with `overflow`.
- **One binary.** The frontend build is embedded with `go:embed`, so the app
  and the API share an origin and need no CORS. Hashed assets are cached as
  immutable, and `index.html` is revalidated with an ETag.

### Frontend

- **One pure reducer.** The keypad and the keyboard dispatch the same actions.
  Every request carries an id, any edit cancels it, and a response whose id
  does not match is dropped, so a slow response never overwrites newer input.
- **The screen shows exactly what is sent.** Only one-character swaps are
  made (`*` → `×`, `/` → `÷`, `-` → `−`), so error positions from the backend
  highlight the right character. The backend is the only parser.
- **Rounded on screen, exact underneath.** The display shows at most 12
  significant digits, rounded on the decimal string, never through
  `Number()`. A rounded result is marked `≈` and shows its exact value with a
  copy button. `Ans` keeps the exact value.
- **`Ans` as on a scientific calculator.** It starts at 0. After a result, an
  operator continues from `Ans`, and `=` runs an expression that uses `Ans`
  again: `Ans*2+1`, `=`, `=` gives 1, 3, 7.
- **Keyboard and screen readers.** Every key can be typed: `s` for `sqrt(`,
  `a` for `Ans`, `y` for `^`, Enter, Backspace and Escape. The dead `^` of
  Turkish layouts works too. The result is a live region, errors are alerts,
  and every key has an accessible name.
- **Responses are validated at runtime.** Anything outside the API contract
  is shown as an unexpected response and never reaches the formatter.

### Testing

- Table-driven tests in every package, and `httptest` for the API and the
  static files. A test shuts the server down with a request in flight.
- Two Go fuzz targets check the parser and evaluator invariants. Fuzzing found
  a real bug: a non-integer power of a very long base failed without an error
  code, and was slow well before that.
- Frontend tests use Vitest and React Testing Library. The API call is
  injected, so tests decide when a response arrives.
- Most rules were checked by breaking them on purpose and confirming that a
  test fails. Headless Chrome also ran 19 end-to-end scenarios against the
  real server.

### Trade-offs and limitations

- Precision is finite: `1/3*3` is `0.999…9` with 34 nines. The screen shows
  `≈ 1`, and `Ans` keeps the exact value, as on phone calculators.
- Non-integer powers are computed through ln/exp and are approximate.
- The percent rule applies only to the right operand of `+` and `-`:
  `200 + 10% * 2` is 200.2.
- Very long or deeply nested input is rejected, and there is no rate limiting.
- Only the Turkish dead `^` is recognized. On German and French layouts, type
  `y` or use the `xʸ` key.
- The copy button needs the clipboard API, which browsers offer only on https
  and localhost.
- Out of scope: functions other than `sqrt`, constants, implicit
  multiplication, scientific notation in input, modulo and history.
