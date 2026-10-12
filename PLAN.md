# Calculator: Implementation Plan

A calculator with a React + TypeScript frontend and a Go backend. The backend
exposes a single endpoint that parses and evaluates an expression string using a
hand-written parser and decimal arithmetic.

## Goals

- `POST /api/v1/calculate` evaluates expressions such as `2 * (3 + 4) ^ 2`.
- Supported: `+ - * /`, exponentiation `^`, `sqrt`, percent `%`, parentheses,
  negative numbers.
- Hand-written parser. Binary operators use precedence climbing, and their
  precedence lives in one table.
- Decimal arithmetic (`cockroachdb/apd`), so `0.1 + 0.2` is exactly `0.3`.
  Numbers cross the API as strings.
- Percent works like a phone calculator: `200 + 10% = 220`.
- New functions can be added without touching the parser.
- Clear errors that separate invalid input (400) from undefined math (422).
  Syntax errors report where the problem is.
- Frontend: a keypad that also works from the keyboard, an `Ans` key, and long
  results rounded on screen while the exact value is kept.
- Standard library wherever possible. Math, parsing, evaluation and HTTP are
  separate packages. Every part has tests, and coverage reports are delivered.
- Shipped as a single image built by a multi-stage Dockerfile.

## Resolved decisions

| Topic | Decision |
|---|---|
| `-2^2` | `-4`: unary minus binds looser than `^` |
| `2^3^2` | `512`: `^` is right-associative |
| `0^0` | `1`, special-cased if apd returns something else |
| Precision | 34 significant digits and exponent range ±6144 (decimal128 limits); ties round to even (IEEE 754 default). Inputs are parsed exactly; only operation results are rounded |
| Implicit multiplication (`2(3)`, `2sqrt(4)`) | Not supported; returns `syntax_error` |
| Number literals | `.5` is accepted and `5.` is rejected. No scientific notation in input |
| Output format | Plain notation when the adjusted exponent is between −7 and 33, scientific (`1.5E+40`) outside that range. Trailing zeros are stripped and `-0` becomes `0` |
| Percent | See [Percent semantics](#percent-semantics). Parentheses block the rule; unary minus does not |
| `Ans` | Sent as a separate `ans` field and referenced as a variable. It is never substituted into the expression text. At most 34 digits, like every result |
| HTTP status | 400 for invalid input, 422 for undefined math |
| Error codes | Specific codes (`syntax_error`, `unknown_function`, `division_by_zero`, ...). See [Error catalog](#error-catalog) |
| Coverage | A snapshot is committed under `coverage/`, and one command regenerates it |
| Deployment | One image: the Go binary serves both the API and the embedded frontend build |
| Language | Everything in the repository is written in English |

## Project structure

```
calculator/
├── backend/
│   ├── go.mod
│   ├── cmd/server/main.go     # thin main; real work in a testable run()
│   └── internal/
│       ├── arith/             # math: apd context, operations, function registry, MathError
│       ├── parser/            # lexer, AST, precedence climbing, SyntaxError
│       ├── eval/              # AST -> arith: percent rule, name resolution, variables (Ans)
│       ├── api/               # handler, request/response types, error -> HTTP mapping
│       └── web/               # embedded frontend build (go:embed) and static handler
├── frontend/                  # Vite + React + TypeScript
│   └── src/
│       ├── api/               # fetch client and types (incl. ErrorCode union)
│       ├── calculator/        # reducer, keymap, display formatter (all pure functions)
│       ├── components/        # Display, Keypad
│       └── App.tsx
├── coverage/                  # committed coverage report snapshot (deliverable)
├── Dockerfile
├── .dockerignore
├── .gitattributes
├── PLAN.md
└── README.md
```

### Dependency direction

Dependencies point one way only:

- **`parser` depends on nothing.** Number literals stay as strings in the AST,
  so the parser does not even import apd.
- **`arith` depends only on apd.** apd is the project's only third-party Go
  dependency.
- **`eval`** combines `parser` and `arith`. The percent rule and function lookup
  belong to neither, so they live here.
- **`api`** depends only on `eval`.
- **`cmd/server`** wires `api` and `web` together.

## API contract

```
POST /api/v1/calculate
{ "expression": "2 * (3 + 4) ^ 2", "ans": "98" }        // "ans" is optional

200 { "result": "98" }
400 { "error": { "code": "syntax_error", "message": "unexpected ')'", "position": 7 } }
422 { "error": { "code": "division_by_zero", "message": "division by zero", "position": 1 } }
```

- `position` is a 0-based character index into `expression`. It is left out
  when it does not apply.
- The lexer accepts ASCII only, so byte index, rune index and JavaScript string
  index are the same. The frontend can highlight the offending character
  directly.

### Error catalog

| Status | Code | When | `position` |
|---|---|---|---|
| 400 | `invalid_request` | Malformed JSON, missing `expression`, body too large | – |
| 400 | `invalid_ans` | `ans` is not a valid decimal, is outside the exponent range (`1E+7000`), or has more than 34 digits | – |
| 400 | `expression_too_long` | Expression length limit exceeded | – |
| 400 | `nesting_too_deep` | Parenthesis depth limit exceeded | ✓ |
| 400 | `syntax_error` | Unexpected token, unclosed parenthesis, empty expression | ✓ |
| 400 | `unknown_function` | `foo(1)` | ✓ |
| 400 | `unknown_variable` | `x`, or `Ans` used without an `ans` value | ✓ |
| 400 | `wrong_argument_count` | `sqrt(1, 2)` | ✓ |
| 422 | `division_by_zero` | `1/0`, `0/0`, `0^-1` | ✓ (operator) |
| 422 | `domain_error` | `sqrt(-1)`, `(-8)^(1/3)` | ✓ (function / operator) |
| 422 | `overflow` | Result exceeds the exponent range | ✓ (operator) |
| 422 | `underflow` | Result is too close to zero for the exponent range | ✓ (operator) |

**Go side:**

- Every error type has a `Code() string` method. Positional errors also have
  `Pos() int`.
- `api` knows only these interfaces, not the concrete types, and keeps one
  code → status table.
- A test walks every code and checks that it maps to 400 or 422. A new code
  that is missing from the table breaks the test instead of falling through to
  500.

**Frontend side:**

- An `ErrorCode` union type mirrors the catalog.
- User messages come from a `Record<ErrorCode, string>`, so a missing message is
  a compile error.
- The UI shows its own text, except for `syntax_error`. There the backend
  `message` names the problem (`unexpected ')'`), so it is shown, with its
  first letter capitalized. For other codes the backend `message` is a
  debugging aid, and for `unexpected` responses a fallback.

## Parser and evaluator design

### Precedence table

Unary minus lives in the same table, so all binding powers are in one place.

| Operator | Power | Associativity |
|---|---|---|
| `+ -` | 1 | left |
| `* /` | 2 | left |
| unary `-` | 3 | prefix |
| `^` | 4 | right |
| `%` | 5 | postfix |

Unary minus parses its operand at power 3. The operand picks up `^` and `%` but
stops at `*` and `+`. Results:

- `-2^2 = -4`
- `2^-1 = 0.5`
- `2^3^2 = 512`
- `2 * (3 + 4) ^ 2 = 98`

### Grammar of primaries

```
primary := number
         | identifier                        // variable: Ans (later pi, e)
         | identifier "(" [ args ] ")"       // function call, any name
         | "(" expression ")"                // kept as a Group node
args    := expression { "," expression }
number  := digit+ [ "." digit+ ] | "." digit+
```

### AST

| Node | Fields |
|---|---|
| `Number` | `Text`, `Pos` |
| `Variable` | `Name`, `Pos` |
| `Call` | `Name`, `Args`, `Pos` |
| `Unary` | `Op`, `X`, `Pos` |
| `Binary` | `Op`, `L`, `R`, `Pos` |
| `Percent` | `X`, `Pos` |
| `Group` | `Inner`, `Pos` |

- Every node carries the position of the token that created it.
- `String()` prints a fully parenthesized S-expression, which keeps parser
  tests readable. Example: `2 * (3 + 4) ^ 2` → `(* 2 (^ (group (+ 3 4)) 2))`.

### Percent semantics

Percent depends on context: the value of `10%` in `200 + 10%` depends on the
left operand. The precedence table cannot express that.

- **Parser:** `%` is the tightest-binding postfix operator. The parser only
  produces a `Percent(x)` node.
- **Evaluator:** the rule lives here.

| Expression | Meaning | Example |
|---|---|---|
| `a + b%`, `a - b%` | a ± a·b/100 | `200 + 10%` = 220 |
| `a * b%`, `a / b%` | b/100 | `200 * 10%` = 20 |
| `b%` on its own | b/100 | `50%` = 0.5 |

The rule fires when the right operand of `+`/`-` is a `Percent` node. Any
number of unary minus signs in front of it still count. A `Group` node is a
different node type, so parentheses block the rule without any special-casing.

| Expression | Result | Why |
|---|---|---|
| `100 + 10%` | 110 | rule fires |
| `100 + (10%)` | 100.1 | parentheses block the rule |
| `100 + -10%` | 90 | unary minus does not block |
| `100 - -10%` | 110 | unary minus does not block |
| `100 + (-10%)` | 99.9 | parentheses block the rule |
| `100 + (5+5)%` | 110 | the parentheses are inside the percent; rule fires |
| `(100 + 10)%` | 1.1 | standalone percent |
| `200 + 10% * 2` | 200.2 | right operand is `*`, not a percent |
| `200 + 10% + 10%` | 242 | compounds, like a phone calculator |

### Functions and variables

- **Calls:** the parser turns any `identifier(args...)` into `Call{Name, Args, Pos}`
  without checking the name.
- **Lookup:** `eval` resolves names during evaluation against a registry in
  `arith`, keyed by name and holding `{arity, fn}`.
- **Adding a function** means one registry entry plus a test. The parser does
  not change.
- **Variables:** a bare identifier is a `Variable`. `eval` resolves it from a
  variable table that holds `Ans` from the request. Constants such as `pi` can
  be added later the same way.
- Because nodes carry positions, `unknown_function` and `unknown_variable` are
  still positional errors.

### Why `Ans` is not substituted textually

If `2*Ans` were sent as `2*(0.333…)`:

- Error positions would point into the expanded text instead of what the user
  sees.
- Whatever format the backend outputs (for example `1E+40`) would also have to
  be valid input to the grammar.

Sending `ans` as its own field avoids both problems. The backend parses it with
apd directly, so no grammar is involved.

### Exactness and its limits

- **Exact:** apd computes terminating decimals such as `0.1 + 0.2` exactly.
- **Inputs are never rounded:** literals and `ans` keep every digit. Each
  operation rounds only its result (ties to even), and the evaluator rounds the
  final value. Rounding inputs first would make results depend on digits that
  were already discarded. An `ans` with more than 34 digits is rejected, not
  rounded (step 8).
- **Negation is exact:** unary minus does not round, so
  `-12345678901234567890123456789012345 + 12345678901234567890123456789012344`
  is exactly `-1`.
- **Rounded to 34 digits:** `1/3` and `sqrt(2)`.
- **Approximate:** non-integer powers, which apd computes through ln/exp. A
  base with more than 44 digits is split into integer and fractional powers
  first (see step 7).
- **Example:** `1/3*3` is `0.999…9`. The display rounds it to `1`, while `Ans`
  keeps the exact value. Phone calculators work the same way, and the README
  documents this trade-off.
- **Normalization:** `1.50 + 1.50` returns `"3"`, not `"3.00"` (`Reduce`), and
  `-0` returns `"0"`.
- **apd conditions:** `DivisionByZero`, `DivisionUndefined`, `InvalidOperation`,
  `Overflow` and `Underflow` are translated to `MathError` kinds in a single
  function.

### Limits

The endpoint is public, so every input is bounded. These are starting values,
pinned by tests:

| Limit | Value |
|---|---|
| Request body | `http.MaxBytesReader`, 4 KiB |
| Expression length | 1000 characters |
| `ans` digits | 34, the most a result has |
| Parenthesis nesting depth | 100 |
| Number range | bounded exponent range in the apd context, so `10^10^10` fails fast with `overflow` |
| Server timeouts | `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout` on `http.Server` |

## Frontend design

- **Single reducer.** The keypad and the keyboard dispatch the same actions, so
  all logic sits in one pure reducer. State:
  `{ expression, ans, result, error, pending, lastRequestId }`.
  - `ans` starts at `"0"`, as on scientific calculators, so `Ans` works from
    the first calculation (for example, recurrences such as `Ans*2+1`). It is
    sent with every request.
  - `error` is the failed client result (`error`, `unexpected` or `network`).
    The display turns it into text.
  - `pending` is the request in flight: `{ id, expression, ans }`. Ids come
    from the `lastRequestId` counter, so the reducer stays pure.
- **Permissive input.** The reducer does not duplicate the grammar. The backend
  checks the syntax, and the error highlight shows where the problem is.
  - One exception: right after `Ans`, a digit, `(`, `Ans` or `sqrt(` is
    ignored. The lexer would merge them into one identifier (`Ans5`, `Ans(`,
    `Anssqrt(`), and the error would be a misleading `unknown_variable` or
    `unknown_function`.
- **Editing:**
  - Backspace deletes `sqrt(` and `Ans` as whole tokens.
  - Any edit clears the error and cancels a pending request.
  - `=` sends the request. It does nothing when the expression is empty or a
    request is already pending. After an error it sends the same expression
    again, which retries a network failure.
  - Clear resets the expression, result, error and pending request, and keeps
    `Ans`.
- **After a result:**
  - `+ - * / ^ %` continue from `Ans`, like a phone calculator: `+` gives
    `Ans+`.
  - Every other input key (digit, `.`, `(`, `)`, `sqrt(`, `Ans`) starts a new
    expression.
  - Backspace hides the result and returns to editing the same expression. It
    does not delete a character, and it cancels a repeat that is pending.
  - `=` runs the expression again if it uses `Ans`, as on a Casio: with
    `Ans*2+1`, each press gives the next value (1, 3, 7, …). The old result
    stays on screen until the new one arrives, and a failed repeat replaces it
    with the error. Without `Ans`, `=` does nothing. The last operation is
    never repeated phone-style (`+5`, `=`, `=`).
- **Keypad.** Four columns, six rows, as in the design:

  ```
  C   (   )   ⌫
  √   xʸ  %   ÷
  7   8   9   ×
  4   5   6   −
  1   2   3   +
  0   .   Ans =
  ```

  - `√` inserts `sqrt(`, `xʸ` inserts `^`, and `Ans` inserts `Ans`, the
    variable name the backend expects.
  - Accessible names come from the design: `Clear`, `Delete` (⌫), `Power`,
    `Insert last answer`, `Calculate` (=), ...
- **Visual design.** It follows `Calculator UI.html` at the repository root,
  made while planning:
  - A dark display card over a light page, keys colored by kind (digits,
    operators, functions, Clear and Delete, `=`).
  - IBM Plex Sans and IBM Plex Mono, self-hosted through `@fontsource`, so
    the app makes no third-party requests.
  - Phones: the display grows to fill the screen above the keypad. From
    600px wide: a centered card with the subtitle and a keyboard hint.
  - The display's top label reads "Decimal · 34 digits".
  - Not taken from the design: the history panel and button (history is out
    of scope), and spaces in the expression (the display must match the
    request).
- **Display matches the request.** The expression on screen must equal the
  string sent to the backend character for character, or error positions drift.
  - One-character visual swaps are fine: `*` → `×`, `/` → `÷`, `-` → `−`
    (U+2212). Each is a single UTF-16 code unit, so positions still line up.
  - Length-changing swaps are not (`√` ↔ `sqrt(`).
  - No digit grouping.
- **Error highlighting.** Positional errors highlight the offending character.
  The position can equal the expression length (an error at the end, as in
  `2+`). Then a placeholder after the last character is highlighted.
- **Errors on screen.** The display gets a red outline. Under the expression,
  an element with `role="alert"` shows:
  - A badge with the code (`SYNTAX_ERROR`, or `HTTP 502` for an unexpected
    response). It is hidden from screen readers.
  - The message. Syntax errors add the column (`Unexpected ')' at column 4`),
    which also tells screen-reader users where the problem is.

  The expression is marked `aria-invalid` and points to the alert with
  `aria-describedby`.
- **Results on screen.** The result is an `<output>` with `aria-live`.
  - Results over 7 characters switch from 56px to 30px, so 14 characters fit
    a 390px phone.
  - Screen readers hear "Equals" or "Approximately" instead of the `=` or `≈`
    marker.
  - A rounded result also shows its exact value under a divider, with a copy
    button. The clipboard API works only in secure contexts (https,
    localhost).
- **Display formatter.** `formatForDisplay(exact)` returns `{ text, rounded }`,
  following [Display formatting](#display-formatting). It rounds on the
  decimal string itself and never calls `Number()`. `Number()` would lose
  precision and overflow above 1e308.
- **Requests and stale responses:**
  - The `useCalculator` hook sends the request from an effect keyed on the
    request id, and aborts it with an `AbortController` in the cleanup.
  - Each response carries its request id. The reducer drops a response that
    does not match the pending request, so dropping stale responses is a pure
    reducer rule with its own tests.
  - The keyboard listener only calls `dispatch`, so it is registered once and
    never reads stale state.
- **API client.** `calculate(expression, ans)` returns one of:
  - `ok`, with the result.
  - `error`, with a known `ErrorCode` and an optional position.
  - `unexpected`, for a response outside the contract: a 5xx, a body that is
    not JSON, or an unknown code. The backend `message` is shown when present.
  - `network`, when `fetch` itself fails.

  The response body is checked at runtime. TypeScript types alone do not
  validate it.
- **Keyboard mapping.** A pure `keyToAction(event)` function and one `window`
  listener.
  - Digits and `. + - * / ^ % ( )` insert themselves. `s` or `S` inserts
    `sqrt(`, and `a` or `A` inserts `Ans`.
  - The numpad decimal key inserts `.`. It is matched by
    `code === 'NumpadDecimal'`, because it types `,` on Turkish and German
    layouts. With NumLock off it is Delete and is ignored.
  - Enter and `=` evaluate, Backspace deletes, Escape clears.
  - Events with Ctrl or Meta held are ignored, so Ctrl+C still works. Ctrl+Alt
    is not ignored, because that is how Windows reports AltGr.
  - The listener calls `preventDefault` for every key it handles. Among other
    things, this keeps `/` from opening Firefox's quick find. Other keys keep
    their default action.
  - `^` is a dead key on some layouts (Turkish, German, French), where
    `keydown` reports `key: 'Dead'`. Checked on a Turkish Q keyboard:
    Shift+3 then Space types `^`, but Shift+3 then a digit types only the
    digit, because the browser never reports the `^`. The keypad's `xʸ` key
    always works.
  - Keypad buttons do not take focus on mouse click (`preventDefault` on
    mouse down; on pointer down it would not stop the focus). Otherwise Enter
    after a click would press the focused button again instead of
    evaluating. When a button has focus from Tab navigation, Enter and Space
    are left to the button.
- **Testability.** `App` receives `calculate` as a prop that defaults to the
  real client, so component tests decide when responses arrive. Only the
  client tests mock `fetch`.
- **API URL.** The client uses the relative URL `/api/v1/calculate`. In
  development, the Vite proxy (`server.proxy` and `preview.proxy`) forwards
  `/api` to `localhost:8080`. In production the request goes to the same
  origin. No CORS and no environment configuration needed.

### Display formatting

The display shows the result rounded, while the exact value is kept as `Ans`.

| Rule | Value |
|---|---|
| Significant digits | At most 12, rounded once from the exact value, ties to even |
| Width | 14 characters. A leading minus sign is not counted |
| Trailing zeros | Stripped. `-0` becomes `0` |
| Plain notation | `0.001 ≤ abs(x) < 10^12`, with as many digits as fit the width |
| Scientific notation | Everything else, in the backend's format (`1.5E+40`), with the mantissa rounded to fit the width. `0.00005` is shown as `5E-5` |
| Result marker | `=` before an exact result, `≈` before a rounded one |

Why these thresholds:

- Below 10^12 every integer digit is significant, so plain notation never
  shows zeros that rounding produced. `1099511627776` (2^40) becomes
  `1.09951163E+12`, not `1099511627780`.
- At 0.001 both notations show 10 significant digits. Below it, scientific
  notation shows more.
- A width of 14 fits 12 significant digits after `0.`, so `1/3` shows all 12.
- The range is checked on the rounded value, so a carry can move a value
  across a boundary: `999999999999.9` becomes `1E+12`. The number of digits
  comes from the exact value. After a carry only the digit 1 is left, which
  fits either notation.

## Coverage

Coverage is a required deliverable. It is set up in step 0 and checked as part
of every step's definition of done.

### Backend

```
go test ./... -coverpkg=./... -coverprofile=coverage.out
go tool cover -func=coverage.out                       # summary
go tool cover -html=coverage.out -o coverage.html      # report
```

- With `-coverpkg=./...`, lines in one package that are exercised by another
  package's tests also count. For example, `eval` tests cover `arith` code.
- `main` stays thin and delegates to `run(ctx, args)`, so `cmd/server` is
  testable too.

### Frontend

- `@vitest/coverage-v8` with `text` and `html` reporters. A second `text`
  reporter writes every file to `coverage/summary.txt`, because the terminal
  lists only the files with missing coverage.
- Coverage includes `src/**` and excludes `main.tsx` and config files.
- `coverage.thresholds` in the Vitest config fails `npm test` when coverage
  drops below the target.

### Targets

| Area | Target |
|---|---|
| Backend `internal/*` | ≥ 90% (95%+ is realistic for `arith`, `parser`, `eval`) |
| Frontend | ≥ 85% |

### Deliverable

- During development, reports go to gitignored locations:
  `backend/coverage.out`, `backend/coverage.html` and `frontend/coverage/`.
- The final snapshot is exported to the root `coverage/` directory and
  committed:
  - `coverage/backend/coverage.html`
  - `coverage/backend/summary.txt`
  - `coverage/frontend/`: the HTML report and `summary.txt`
- The README contains a summary table.
- The snapshot is reproducible on any OS with one command:
  `docker build --target coverage --output type=local,dest=coverage .`

## Docker

One multi-stage Dockerfile produces one image that serves both the API and the
frontend.

| Stage | Base | Work |
|---|---|---|
| `frontend-build` | pinned Node LTS | `npm ci` → `npm run build` |
| `backend-build` | Go version from `go.mod` | `go mod download` → copy `dist` into `internal/web/dist` → `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w"` |
| `coverage` (optional target) | previous stages | Runs both test suites with coverage, then copies only the reports into a `scratch` stage for `--output`. Optionally fails if Go coverage is below target |
| `runtime` (default) | distroless static, `nonroot` | Binary only, runs as a non-root user, about 10–15 MB |

**Things to watch:**

- **`.dockerignore`** must list `node_modules`, `dist`, `coverage` and `.git`.
  This matters on Windows: the host's `node_modules` contains Windows-native
  esbuild/rollup binaries that break inside a Linux container.
- **`//go:embed` cannot reach `../frontend/dist`.** The build copies `dist`
  into the package directory. The package uses `all:dist` plus a committed
  `.gitkeep`, so it still compiles when the frontend has not been built.
- **Listen address.** The server listens on `:8080`, not `localhost:8080`.
  Otherwise it is unreachable from outside the container.
- **Routing.** The API lives under `/api/` in its own mux, and static files are
  served at `/`. This avoids pattern conflicts and keeps
  `GET /api/v1/calculate` returning 405.
- **Line endings.** `.gitattributes` enforces LF, so files authored on Windows
  do not break in Linux containers.

## Implementation steps

**Definition of done for every step:**

- `go test ./...` and `npm test` pass.
- Coverage of the new code has been checked.
- `gofmt`/`go vet` and `tsc`/oxlint are clean.

### Phase 0: Skeleton

**0. Repository skeleton**

- `git init`, `.gitignore`, and `.gitattributes` (LF line endings).
- `backend/`: `go mod init`, then add `github.com/cockroachdb/apd/v3`. Use Go
  1.27.
- `frontend/`: Vite `react-ts` template, plus Vitest, jsdom, React Testing
  Library, user-event and `@vitest/coverage-v8`.
- Coverage commands for both sides.
- README with run, test and coverage commands.

*Tests:* both test commands run and produce a coverage report, even with no
tests yet.

### Phase 1: Math (`internal/arith`)

**1. Context, arithmetic, formatting, MathError**

- A shared apd context: 34-digit precision, a bounded exponent range and
  round-half-even.
- `Parse`, which keeps every digit of its input.
- `Add`, `Sub`, `Mul`, `Div`.
- One function that translates apd conditions into `MathError{Kind}`. Every
  kind carries its error code: `division_by_zero`, `domain_error`, `overflow`,
  `underflow`.
- `Format`: normalization plus the plain/scientific threshold.

*Tests:*

- `0.1+0.2` → `"0.3"`
- `1.50+1.50` → `"3"`
- `-0` → `"0"`
- `1/3` has 34 significant digits
- `1/0` and `0/0` → `division_by_zero`
- Plain/scientific boundary cases

**2. Power, square root, percent helper, function registry**

- Registry with `sqrt` only, for now.

*Tests:*

- `2^10`, and `2^-1 = 0.5`
- `0^0 = 1`
- `0^-1` → `division_by_zero`
- `sqrt(-1)` and `(-8)^(1/3)` → `domain_error`
- `10^1000000` → `overflow`, quickly

### Phase 2: Parsing (`internal/parser`)

**3. Lexer**

- Tokens: number, `+ - * / ^ % ( ) ,`, identifier, EOF. Each token carries its
  position. Number text is kept exactly as written.
- Identifiers start with an ASCII letter or `_`, followed by letters, digits or
  `_`, so a later `log10` needs no lexer change.
- Whitespace (space, tab, CR, LF) is skipped.
- Only ASCII is accepted. The first non-ASCII character is a `syntax_error` at
  its position.
- The lexer produces tokens on demand. The parser pulls them one at a time and
  stops at the first error, whether lexical or syntactic.
- `1e5` lexes as the number `1` followed by the identifier `e5`, and the parser
  rejects it.

*Tests:*

- Positions are correct and whitespace is skipped.
- `1.2.3` and `5.` are rejected; `.5` is accepted. The error points at the
  offending `.`: position 3 in `1.2.3`, position 1 in `5.`.
- `2 $ 3` → `syntax_error` at position 2.
- A non-ASCII character is rejected.

**4. AST and precedence climbing**

- Binary operators, unary minus, and parentheses as `Group` nodes. `Op` holds
  the operator as written (`"+"`, `"-"`, ...).
- Unary plus is not supported: `+2` is a `syntax_error`.
- Depth limit (`nesting_too_deep`): at most `MaxDepth` = 100 parentheses can
  be open at once. The error points at the first `(` beyond the limit.
  Parentheses that are already closed do not count, so sibling groups do not
  add up.
- S-expression `String()`.

*Tests:*

- The precedence and associativity table.
- `-2^2` → `(- (^ 2 2))`.
- `2^-1`.
- `2 * (3 + 4) ^ 2` → `(* 2 (^ (group (+ 3 4)) 2))`.
- Errors with their positions: `""`, `"2 +"`, `"(2"`, `"2)"`, `"2 3"`, `"* 2"`,
  `"2(3)"`.
  - Empty input → `empty expression`.
  - A missing operand → `unexpected end of expression` at the end of the input.
  - An unclosed parenthesis → `unclosed parenthesis` at its `(`.
  - Any other token in the wrong place → `unexpected 'x'` at that token.

**5. Postfix percent, function calls, variables**

- `%` is a postfix operator with power 5, the tightest: `2^10%` is `2^(10%)`
  and `-10%` is `-(10%)`. Repeated percent (`50%%`) is allowed.
- `name(args)` is a `Call` whatever the name. Its `Pos` is the position of the
  name. An empty argument list (`f()`) parses, and `eval` checks the arity.
  Whitespace between the name and `(` is allowed.
- A name without `(` is a `Variable`.
- Function call parentheses count toward `MaxDepth`.
- `String()` prints `(% x)` and `(call name args...)`. The `call` tag keeps a
  function named `group` apart from a `Group` node.

*Tests:*

- AST shapes for `200+10%`, `100+(10%)`, `100+-10%` and `(5+5)%`.
- `sqrt(9)` parses, and so does `foo(1, 2)`: the parser does not know function
  names.
- `sqrt(` → `syntax_error` (`unexpected end of expression` at position 5).
- `Ans` parses as a `Variable` node.

### Phase 3: Evaluation (`internal/eval`)

**6. Evaluator**

- Walks the AST and calls `arith`.
- Applies the percent rule, including unary-minus unwrapping and `Group`
  blocking.
- Looks up functions in the registry, with an arity check.
- Resolves variables from the variable table. Names are case-sensitive: `ans`
  is not `Ans`.
- Rounds the final value to 34 digits, so an expression with no operation
  (a long literal, `-x`, `(x)`) also returns at most 34 digits.
- Math errors carry the position of the operator or function that caused them.
- API: `Evaluate(expression, vars)` returns a `Value`. `ParseValue` parses an
  `ans` string, and `Value.String()` formats a result. The `api` package uses
  only these, so it depends on `eval` alone.

Settled while implementing:

- **Names before math.** A first pass checks every name and argument count
  before anything is computed. An invalid expression always gets a 400, even
  when evaluating it would also divide by zero: `1/0 + x` → `unknown_variable`.
  Otherwise operands are evaluated left to right, and the first error wins.
- **Percent errors.** In `a ± b%`, a failure while computing b percent of a
  points at the `%`, and a failure of the `±` points at the operator.
- **Final rounding.** Rounding up can overflow (a literal of 6145 nines). The
  error points at the root of the expression.
- **Literals out of range.** `arith.Parse` now reports `overflow` or
  `underflow` instead of `invalid number`, so a literal with thousands of
  digits fails at its own position. apd drops the condition flags when a
  trapped parse fails, so `Parse` runs without traps and checks the flags
  itself.
- **Exact negation.** Unary minus uses `arith.Neg`, which does not round.
- **Node positions.** `parser.Node` gains `Position()`, which gives the root's
  position for the final-rounding error.

*Tests:*

- An end-to-end table: `2 * (3 + 4) ^ 2 = 98`, plus the full percent table from
  this plan.
- A 40-digit literal on its own comes back rounded to 34 digits.
- `foo(1)` → `unknown_function` at position 0.
- `x` → `unknown_variable`.
- `sqrt(1,2)` → `wrong_argument_count`.
- `1/(2-2)` → `division_by_zero` at the `/`.
- `Ans*2` with `ans=21` → `42`.

**7. Fuzz test** (built-in Go fuzzing)

Two fuzz targets. Their seeds run as part of `go test`, and `-fuzz` searches
for new failing inputs (commands in the README).

`FuzzParse` invariants:

- No input panics.
- Errors are `syntax_error` or `nesting_too_deep`, with a position in
  `[0, len(input)]`.
- Every node's position points at the token that created it, which is the
  character the frontend highlights.
- The tree follows the [precedence table](#precedence-table): wherever the
  table needs parentheses, the tree holds a `Group` node. The test keeps its
  own copy of the table, so it checks the parser against the spec instead of
  against itself.
- For valid input, `Parse(infix(ast))` returns the same AST, compared through
  `String()`.
  - `infix` is a test-only printer. It writes `Group` nodes as parentheses and
    adds no others, so the tree shape must come back from the tokens alone.
    This catches dropped parentheses.
  - `String()` cannot be used here: it prints an S-expression, which the
    parser does not accept.
  - The round trip cannot catch a precedence rule that is wrong but
    consistent: a left-associative `^` prints and parses back the same tree.
    The precedence check catches that.

`FuzzEvaluate` invariants, with a fuzzed `ans`:

- No input panics.
- Every error has a code from the error catalog and a position in
  `[0, len(expression)]`. An error without a code would be a 500 in the API.
- A result has at most 34 significant digits and parses back through
  `ParseValue` to the same value, so it can be sent back as `ans`.

Settled while implementing:

- **Mutation check.** Six parser bugs were injected one at a time: `^`
  left-associative, `-` right-associative, dropped groups, operator positions
  at the left operand, and unary minus binding loosest or tightest. The seeds
  catch each of them.
- **Long bases in `Pow`.** Probing expensive inputs while fuzzing found a bug.
  For a non-integer exponent, apd computes ln/exp at a precision above the
  number of digits in the base. A base with about 2700 digits or more (a
  crafted `ans`) failed with apd's "too many iterations", an error without a
  code. A 2500-digit base took 0.5 s per power, so `Ans^0.5+Ans^0.5+…` could
  take a minute. Step 8 later capped `ans` at 34 digits, so only a literal
  can be that long, and the API caps it at 1000 characters. `arith.Pow` now
  splits such a power: `x^y = x^int(y) · x^frac(y)`.
  - The integer part uses every digit of `x`.
  - The fractional part uses `x` rounded to 44 digits (34 plus 10 guard
    digits). Because `|frac(y)| < 1`, its relative error stays below that of
    the rounded `x`.
  - Bases of up to 44 digits, including every operation result, still go
    straight to apd. A test checks that the split path matches apd wherever
    apd still works.
  - The slowest input found at the API limits now takes about 0.4 s:
    `1^Ans` repeated with the largest `ans`, because apd squares once per bit
    of an integer exponent.

### Phase 4: HTTP (`internal/api`, `cmd/server`)

**8. Handler**

- Decodes the request and applies the limits. This is where `invalid_request`,
  `invalid_ans` and `expression_too_long` are produced.
- `ans` goes through `eval.ParseValue`. Any error from it is `invalid_ans`
  (400), including overflow and underflow: a bad `ans` is invalid input, not
  undefined math. The handler maps this error itself and does not pass it to
  the code → status table. `ParseValue("1E+7000")` returns an error whose
  `Code()` is `overflow`, which the table would turn into a 422.
- `ans` has at most 34 digits. No result has more, so a longer `ans` can only
  be crafted, and every operation on it would work with all its digits.
  `ParseValue` checks the digit count of the parsed number and returns
  `ErrTooManyDigits`, which is `invalid_ans` like its other errors.
  - Leading zeros do not count: `0.0001234…` with 34 significant digits is
    accepted.
  - Trailing zeros count, because they are digits of the parsed number:
    `1.50` has three. Results never end in zeros, so an `ans` from the
    frontend is never affected.
- Calls `eval`.
- Maps errors to status codes through the `Code()`/`Pos()` interfaces and one
  code → status table.

*Tests (`httptest`):*

- 200 success.
- Each 400 code, with `position` where applicable.
- Each 422 code.
- Malformed JSON, invalid `ans`, oversized body.
- `ans` = `"1E+7000"` → 400 `invalid_ans`, not 422 `overflow`. The same for
  `"1E-7000"` and `underflow`.
- `ans` with 35 digits → 400 `invalid_ans`; with 34 digits it is accepted.
- `GET` → 405.
- A test that walks every code and checks it maps to 400 or 422.

Settled while implementing:

- **Handler.** `api.NewHandler(log)` returns a mux that serves only
  `POST /api/v1/calculate`, and step 9 mounts it at `/api/`. The method
  pattern gives other methods a plain-text 405 with `Allow: POST`.
- **Decoding.**
  - The body must be exactly one JSON object.
  - Member names must match exactly. An unknown name (`answer`), a name with
    different case (`ANS`) or a repeated name is `invalid_request`. Otherwise
    a misspelled `ans` would be ignored, and the result would silently be
    wrong.
  - This uses `encoding/json/v2` with `RejectUnknownMembers`. v2 matches names
    case-sensitively and rejects repeated names and invalid UTF-8 by default.
    v1 matches names case-insensitively and lets the last repeated name win.
  - A missing or `null` `expression` is `invalid_request`. An empty one is a
    `syntax_error` at position 0.
  - A `null` `ans` is the same as no `ans`.
  - Messages do not show Go type names: `unknown field "answer"`,
    `expression must be a string`, `request body is not valid JSON`,
    `request body is larger than 4096 bytes`, and so on.
- **Order of checks.** Request, expression length, `ans`, then evaluation. An
  expression that is too long is reported even when `ans` is invalid too.
- **Expression length** counts bytes. Only ASCII is valid, so for valid input
  that is the number of characters.
- **`invalid_ans` messages** give the reason: `invalid ans: not a valid
  number`, `invalid ans: outside the exponent range`,
  `invalid ans: number has more than 34 digits`.
- **Every code has a status.** `eval.Codes()` lists every code `Evaluate` can
  return. The walk test checks each of them, plus the handler's own codes, and
  checks that the table holds no other codes. `FuzzEvaluate` checks returned
  codes against the same list.
- **Unexpected errors.** An error without a code, or with a code missing from
  the table, is a bug. It is logged with `slog`, and the client gets 500 with
  the code `internal_error`. That code is outside the catalog, and the
  frontend treats every 5xx as `unexpected` anyway.

**9. Server entry point**

- `main` stays thin and calls `run(ctx, args)`.
- Address from a flag or env var, defaulting to `:8080`.
- `http.Server` timeouts, `slog` logging, graceful shutdown.
- The API is mounted under `/api/` in its own mux.

*Tests:* `run()` starts on a random port and serves one request. Plus a manual
smoke test with `curl.exe`.

Settled while implementing:

- **Signature.** `run(ctx, args, getenv, stderr)`. `main` cancels `ctx` on
  `os.Interrupt` (Ctrl+C) and on `SIGTERM`, which `docker stop` sends.
- **Shutdown.** When `ctx` is cancelled, `run` calls `http.Server.Shutdown`.
  Requests in flight get 5 seconds to finish, which leaves margin before
  `docker stop` kills the process after 10 seconds. Then `run` returns nil.
- **Address.** The `-addr` flag, then the `ADDR` environment variable, then
  `:8080`.
  - Strict, as everywhere else: an unknown flag, an extra argument or an
    empty `-addr` (which would listen on a random port) is an error.
  - `-h` prints the usage and exits with 0.
- **Timeouts.** `ReadHeaderTimeout` 5 s, `ReadTimeout` and `WriteTimeout`
  10 s, `IdleTimeout` 60 s. The slowest request found takes about 0.4 s
  (step 7).
- **Logging.** JSON lines from `slog` on stderr: `listening` with the actual
  address, `shutting down`, `stopped`, and the handler's unexpected errors.
  `http.Server` errors go through `slog` too. Requests are not logged.
- **Tests.**
  - `run` starts on `127.0.0.1:0`, and the test reads the address from the
    `listening` log line. It serves a calculation, 405 and 404.
  - When `ctx` is cancelled, `run` returns nil within 5 seconds, logs
    `shutting down` and `stopped`, and the port no longer accepts connections.
  - A request in flight during shutdown still gets its response. The test
    sends `Expect: 100-continue`: the server answers `100 Continue` only once
    the handler reads the body, so the request is in flight for certain. The
    test sends the body only after new connections are refused, which shows
    that `Shutdown` has started. Replacing `Shutdown` with `Close` fails it.
  - Windows cannot send `SIGTERM` to a process, so a test only checks the
    signal list. Step 17 checks the signal itself with `docker stop`.
  - Error cases get a context that is already cancelled, so a bug that made
    `run` serve would return at once instead of hanging the test.
  - Not covered: `main`, `Serve` failing on its own, and `Shutdown` running
    out of time.

### Phase 5: Frontend

The frontend depends only on the [API contract](#api-contract), so this phase
runs in parallel with phases 2–4 against a mocked API. An end-to-end check
against the real backend follows step 9.

*End-to-end check (after step 9):* headless Chrome drives the real frontend
against the real server, typing on the keyboard. 19 checks pass:

- Results: precedence, exact decimals, percent, a negative result, rounded
  results (`1/3`, `2^100`) with their exact values, and a small result shown
  as `1E-6`.
- `Ans`: it starts at `0`, repeated `=` steps `Ans*2+1` through 1, 3, 7, and an
  operator after a result continues from `Ans`.
- Errors: a syntax error in the middle and at the end, division by zero, a
  domain error, overflow, underflow, nesting too deep, and an expression that
  is too long. These are all the errors the keypad can produce. Each shows the
  right message, badge and highlight.

**10. API client**

- `calculate(expression, ans, signal?)` returns a discriminated union: `ok`,
  `error` (with an `ErrorCode` and an optional position), `unexpected` and
  `network`. It resolves for every outcome and rejects only when the caller
  aborts it through `signal`.
- The response body is validated at runtime. Anything else is `unexpected`:
  - A 200 needs a `result` in the backend's number format (`-0.5`,
    `1.5E+40`), so the display formatter can trust its input.
  - An error needs status 400 or 422, a known code, a message, and a position
    (when present) that is an integer from 0 to `expression.length`.
- `ErrorCode` union type and a `Record<ErrorCode, string>` message table.
  `errorMessage(code, message)` picks the text: the backend's for
  `syntax_error`, the table's for every other code.
- Vite proxy for `/api` (`server.proxy` and `preview.proxy`).

*Tests:* mocked `fetch` for success, a positional 400, a 422, a 500, a body
that is not JSON, an unknown code and a network failure.

**11. Display formatter**

- `formatForDisplay(exact)` returns `{ text, rounded }`, following
  [Display formatting](#display-formatting). It rounds on the string, without
  `Number()`.

*Tests:*

| Exact | Shown | Rounded | Why |
|---|---|---|---|
| `123456789012` | `123456789012` | no | 12 digits fit |
| `0.3333333333333333333333333333333333` | `0.333333333333` | yes | 12 digits after `0.` |
| `0.03333333333333333333333333333333333` | `0.033333333333` | yes | only 11 digits fit |
| `0.1234567890125` | `0.123456789012` | yes | tie, rounds to even |
| `0.1234567890135` | `0.123456789014` | yes | tie, rounds to even |
| `9.9999999999999` | `10` | yes | carry, trailing zeros stripped |
| `999999999999.9` | `1E+12` | yes | carry crosses 10^12 |
| `1099511627776` | `1.09951163E+12` | yes | no zeros produced by rounding |
| `0.00123456789012345` | `0.00123456789` | yes | plain down to 0.001 |
| `0.001` | `0.001` | no | lower bound is inclusive |
| `0.00012345678901` | `1.23456789E-4` | yes | below 0.001 |
| `0.00005` | `5E-5` | no | below 0.001, even though it would fit |
| `0.0009999999999999` | `0.001` | yes | carry back into plain range |
| `-0.3333333333333333333333333333333333` | `-0.333333333333` | yes | minus sign not counted |
| `1.5E-20` | `1.5E-20` | no | backend scientific input |
| `1E+6144` | `1E+6144` | no | largest exponent |

Plus cases for zero, the range bounds, ties in scientific notation, and long
exponents that leave fewer digits for the mantissa (`1.234568E+6144`). 2000
generated values are checked against the width and notation rules.

**12. Calculator reducer**

*Tests:*

- Action sequences produce the expected state.
- Backspace on `sqrt(` and `Ans` removes the whole token.
- After a result: operators continue from `Ans`, other keys start a new
  expression, and Backspace returns to editing.
- Repeated `=`: an expression with `Ans` runs again with the new `Ans`, one
  without `Ans` does nothing, Backspace cancels a pending repeat, and a failed
  repeat replaces the result with the error.
- A digit, `(`, `Ans` or `sqrt(` right after `Ans` is ignored.
- `ans` starts at `"0"`, and Clear keeps it.
- `=` does nothing on an empty expression or while a request is pending.
- Input clears any error and cancels the pending request.
- A response whose id does not match the pending request is dropped.

**13. Display, Keypad and the `useCalculator` hook**

- The hook sends the request from an effect keyed on the request id and aborts
  it on cleanup.
  - `calculate` rejects only when aborted, so the hook ignores rejections.
  - `calculate` must be stable across renders, because the effect depends on
    it. `App` defaults to the module-level client.
- The display has an expression line and a result line (an `<output>`). An
  empty expression shows a dimmed `0`, which is not part of the expression.
- Positional errors highlight the offending character, or a placeholder at
  the end of the expression.
- Rounded results are shown with `≈` instead of `=`.
- Failure text: `errorMessage` for API errors. For `unexpected`, the backend
  message, or "Unexpected response from the server". For `network`, "Cannot
  reach the server".
- oxlint's `react/exhaustive-deps` rule is enabled.

*Tests (React Testing Library, with an injected `calculate`):*

- Clicking `2 + 3 =` shows `5`.
- Operators are shown as symbols (`×`, `÷`, `−`) and sent as ASCII.
- The error highlight appears at the right character, including at the end of
  the expression.
- Each failure kind shows its text.
- The `Ans` key works, starting from `0`, and repeated `=` steps an `Ans`
  expression.
- A response that arrives after the expression has changed is not shown, and
  the rejection of an aborted request is handled quietly.
- Every keypad key dispatches its action.

**14. Keyboard support**

- A pure `keyToAction(event)` function in `calculator/keymap.ts`, and a
  `useKeyboard(dispatch)` hook that registers the `window` listener.
- Keypad buttons do not take focus on mouse click.

*Tests:*

- A `keyToAction` table: Ctrl/Meta passthrough, AltGr (Ctrl+Alt) still types,
  the numpad decimal key (with NumLock on and off), `s` and `a`, and keys that
  are not ours (a dead key, Space, `x`).
- Typing `2*3{Enter}` with user-event shows the result. Backspace and Escape
  edit.
- `preventDefault` is called for `/`, but not for `x` or Ctrl+C.
- Enter after clicking a keypad button evaluates once and does not press the
  button again.
- Enter and Space press a key that was reached with Tab.
- Manual check of `^` on a Turkish keyboard (dead key): Shift+3 then Space
  types `^`, Shift+3 then a digit loses the `^`.

**15. Polish**

- The visual design from `Calculator UI.html`, mobile first.
- Accessibility: `aria-live` on the result, `role="alert"` on errors,
  `aria-invalid` and `aria-describedby` on the expression, and labels on all
  buttons.
- Long expressions scroll horizontally and keep the end in view while
  typing. When there is an error, the highlight is scrolled into view
  instead.
- Loading state while a request is pending: `aria-busy` on the result, and
  after 200 ms dots appear (or an old result dims), so fast requests do not
  flash.
- The exact value of a rounded result, with a copy button.

*Tests:* the loading state, the old result kept during a repeated `=`, both
kinds of scrolling, the error badge, message and description, the rounded
result with its exact value, and copying it (including a clipboard failure).

*Layout check:* headless Chrome at 390 × 844 with a mocked API: no
horizontal page scroll, 80 × 64px keys, the whole calculator on one screen,
a 34-character exact value wraps, and the scroll positions behave as above.
Compared side by side with the design's screenshots.

### Phase 6: Packaging and delivery

**16. Static file serving (`internal/web`)**

- `//go:embed all:dist`, with a handler mounted at `/`.

*Tests (`httptest`):*

- `GET /` returns `index.html`.
- `GET /api/v1/calculate` still returns 405.

Settled while implementing:

- **Only the build's files.** `web.NewHandler(fsys)` reads the build once,
  at startup, and serves exactly its files: `/` serves `index.html`, and
  every other path serves the file of that name.
  - No directory listings.
  - No fallback to `index.html` for unknown paths, because the app has no
    client-side routes. They get 404.
  - Names that start with a dot (`.gitkeep`, `.vite/`) are not served.
- **Methods.** GET and HEAD. Other methods get 405 with `Allow: GET, HEAD`.
- **Content types** come from a fixed table of the extensions in the build:
  `.html`, `.js`, `.css`, `.svg`, `.woff`, `.woff2`. Go's own table has no
  font types, and on Windows it also reads the registry. A file with any
  other extension stops the server at startup, so no file is ever served
  with a guessed type.
- **Caching.**
  - Vite puts its content-hashed files under `assets/`. They get
    `Cache-Control: public, max-age=31536000, immutable`.
  - Every other file, `index.html` first of all, gets `no-cache` and an
    ETag (the first 128 bits of its SHA-256). The browser revalidates it
    and gets 304 while it is unchanged, and a new build shows up at once.
    Embedded files have no modification time, so without the ETag there
    would be nothing to revalidate against.
- **Routing.** `cmd/server` mounts the API at `/api/` and the frontend at
  `/`. The API answers every `/api/` path itself, so `GET /api/v1/calculate`
  gets 405, and a file under `api/` in the build would never be served.
- **Without a build.** `dist/` holds only `.gitkeep`, and the copied build
  is gitignored. The server then serves only the API and logs a warning.
- **Not done: compression.** The JavaScript is 230 KB, 72 KB gzipped. The
  fonts are already compressed.
- **Tests.**
  - `web` tests use an `fstest.MapFS` laid out like a Vite build.
  - `cmd/server` tests the routes through `newMux`, with a fake build.
  - Nine bugs were injected into the handler and three into the routing, one
    at a time. The tests catch each.
  - Checked by hand with the real build copied in: `curl.exe` for the
    headers, 304, 404 and 405, and headless Chrome for `2*(3+4)^2` = `98`,
    typed on the keyboard, with the fonts loaded and no failed requests.

**17. Multi-stage Dockerfile and `.dockerignore`**

*Tests:*

- `docker build` succeeds.
- `docker run -p 8080:8080` serves a working calculator end to end in the
  browser.
- `docker stop` sends `SIGTERM`. The log shows `shutting down` and `stopped`,
  and the container exits right away instead of being killed after 10
  seconds.
- The `coverage` target exports the reports.

**18. Delivery**

- Generate the coverage snapshot into `coverage/` and commit it.
- Finish the README: setup, Docker usage, design decisions, error catalog,
  coverage summary and known trade-offs.
- Final check from a clean clone.

Settled while implementing:

- **Order.** Step 18 was done before step 17. The Docker parts (Docker usage
  in the README, regenerating the snapshot with `docker build`) are left for
  step 17, and the README marks their places with comments.
- **Snapshot.** Generated with the commands in the README, which need no
  Docker. Backend 98.1% of statements: every `internal` package is at 100%
  except an unreachable `panic` in `web`, and `cmd/server` misses `main` and
  three error returns in `run`. Frontend 100%.
  - Both sides have a `summary.txt`. GitHub shows HTML reports only as source,
    so these are the reports a reviewer can read there.
  - The reports name files by module or project path. Nothing in them comes
    from the machine that generated them.
- **README.** Written against the code, not only this plan. Every API example
  was sent to the running server.
  - Its top-level sections follow the assignment's list: setup, running the
    backend and frontend, API examples, and design decisions and assumptions.
    Coverage has a section of its own.
  - New: a quick start, the coverage summary, the assumptions, the backend
    design decisions, and trade-offs and limitations.
- **Final check from a clean clone**, without Docker:
  - `npm ci`, type check, lint, tests and build.
  - The build copied into `internal/web/dist`, then `gofmt`, `go vet`, the
    tests and `go build`.
  - The binary served the page and all 60 files of the build.
  - Headless Chrome typed expressions on the keyboard: results, a rounded
    result with its exact value, a syntax error and a division by zero. The
    fonts loaded, and no request failed.

## Known trade-offs (to document in the README)

- **Inexact results.** Precision is finite: `1/3*3` evaluates to `0.999…9`, and
  only display rounding shows `1`. Non-integer powers are approximate.
- **Percent rule.** The rule applies only when the percent is the right operand
  of `+`/`-` (possibly negated). For example, `200 + 10% * 2` is `200.2`.
- **Limits.** Body size, expression length and nesting depth are capped, so
  very long or very deeply nested inputs are rejected.

## Out of scope

- Implicit multiplication.
- Modulo.
- Scientific notation in input.
- Calculation history.
- Functions beyond `sqrt`. The registry makes them a small addition.
