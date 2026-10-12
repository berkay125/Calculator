# syntax=docker/dockerfile:1

# One image serves the API and the frontend: the Go binary embeds the frontend
# build. From the repository root:
#
#   docker build -t calculator .
#   docker run --rm -p 8080:8080 calculator
#
# The coverage target runs both test suites and exports their reports:
#
#   docker build --target coverage --output type=local,dest=coverage .

# Node: the active LTS line. Go: the toolchain line in backend/go.mod, which
# local builds use too. Keep the two equal: Go versions can count coverage
# statements differently, and the committed snapshot must match this build.
ARG NODE_VERSION=24.21.0
ARG GO_VERSION=1.27.2

# ---- Frontend ----

FROM node:${NODE_VERSION}-trixie-slim AS frontend-source
WORKDIR /src/frontend
# Dependencies first, so a source change does not reinstall them.
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY frontend/ ./

FROM frontend-source AS frontend-build
RUN npm run build

# ---- Backend ----

FROM golang:${GO_VERSION}-trixie AS backend-source
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# go:embed cannot reach ../frontend, so the build is copied into the package.
COPY --from=frontend-build /src/frontend/dist internal/web/dist

FROM backend-source AS backend-build
# Without cgo the binary is static, so it runs on distroless/static.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ---- Coverage: docker build --target coverage ----

FROM frontend-source AS frontend-coverage
# Fails when coverage drops below the thresholds in vite.config.ts.
RUN npm test

FROM backend-source AS backend-coverage
# The tests run against the real frontend build, so they also check that the
# server can load it.
RUN go test ./... -coverpkg=./... -coverprofile=coverage.out \
 && go tool cover -func=coverage.out > summary.txt \
 && go tool cover -html=coverage.out -o coverage.html
# Fails when the total drops below the 90% target in PLAN.md.
RUN total=$(awk '/^total:/ { sub(/%/, "", $3); print $3 }' summary.txt) \
 && echo "Go coverage: ${total}%" \
 && awk -v total="$total" 'BEGIN { exit !(total >= 90) }' \
 || { echo "Go coverage is below 90%" >&2; exit 1; }

# Only the reports, so --output exports nothing else.
FROM scratch AS coverage
COPY --from=backend-coverage /src/backend/summary.txt /src/backend/coverage.html /backend/
COPY --from=frontend-coverage /src/frontend/coverage/ /frontend/

# ---- Runtime (the default target) ----

FROM gcr.io/distroless/static-debian13:nonroot AS runtime
COPY --from=backend-build /out/server /server
USER nonroot:nonroot
# The server listens on :8080 unless -addr or ADDR says otherwise.
EXPOSE 8080
# Exec form: the server runs as PID 1 and gets the SIGTERM from docker stop
# itself, so it shuts down gracefully.
ENTRYPOINT ["/server"]
