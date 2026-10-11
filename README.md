# Calculator

A calculator with a React + TypeScript frontend and a Go backend. The backend
parses and evaluates expressions such as `2 * (3 + 4) ^ 2` with a hand-written
parser and exact decimal arithmetic.

> Work in progress. [PLAN.md](PLAN.md) describes the design and the
> implementation steps.

## Prerequisites

- Go 1.27+
- Node.js 22.12+ and npm

## Repository layout

| Path | Contents |
|---|---|
| `backend/` | Go module: `internal/{arith,parser,eval,api,web}` |
| `frontend/` | Vite + React + TypeScript app |
| `PLAN.md` | Design decisions and implementation plan |
| `prompts.md` | Prompts given to the AI assistant during development |

## Backend

Run these from `backend/`:

```sh
go vet ./...
go test ./...
```

## Frontend

Run these from `frontend/`:

```sh
npm ci              # install dependencies
npm run dev         # start the dev server
npm test            # run tests with coverage (fails below the thresholds)
npm run test:watch  # run tests in watch mode
npm run typecheck   # type-check with tsc
npm run lint        # lint with oxlint
npm run build       # production build into dist/
```

## Coverage

### Backend

Run these from `backend/`:

```sh
go test ./... -coverpkg=./... -coverprofile=coverage.out
go tool cover -func=coverage.out                    # summary
go tool cover -html=coverage.out -o coverage.html   # HTML report
```

### Frontend

`npm test` writes a text summary to the terminal and an HTML report to
`frontend/coverage/index.html`.

### Committed snapshot

The development reports above are gitignored. The final snapshot is committed
under `coverage/` (see PLAN.md, step 18).
