# Repository Guidelines

## Project Structure & Module Organization
- Current layout is minimal: `main.go` contains the entry point and `go.mod` defines the module (`aks-azureFiles-controller`) and Go version (1.25).
- Target structure (per the contract): `cmd/manager/main.go` for the binary and `internal/` for packages like `controller/`, `azure/`, `k8s/`, `naming/`, `config/`, `logging/`.
- Keep Kubernetes- and Azure-specific logic in those focused packages; avoid cross-cutting globals.

## Build, Test, and Development Commands
- `go run .` : run the current `main` package.
- `go build .` : compile the binary in the repo root.
- `go test ./...` : run all tests.
- `gofmt -w main.go` : format Go source (tabs for indentation).

## Coding Style & Naming Conventions
- Follow `gofmt` and standard Go style; use short, lowercase package names (e.g., `controller`, `azure`).
- Use dependency injection for clients/config; no global state.
- Use `context.Context` in all public functions; avoid `context.Background()` in business logic.
- Wrap errors with `%w`, and separate retryable vs terminal errors.
- Structured logging should use zap keys: `namespace`, `pvc`, `pv`, `share`.

## Testing Guidelines
- Use Go’s `testing` package; tests live next to code as `*_test.go` and use `TestXxx` naming.
- Prefer table-driven tests and fakes/interfaces for Azure; avoid real Azure calls in unit tests.

## Commit & Pull Request Guidelines
- No git history is available here, so no established commit convention.
- Use concise, imperative commit summaries (e.g., "add pvc reconciler skeleton").
- PRs should explain intent, list verification steps (e.g., `go test ./...`), and note API or behavior changes.

## Controller Best Practices (Contract)
- Reconciliation must be idempotent and safe to retry; support leader election and concurrency safety.
- Validate inputs early (storage class params, PVC requests, naming rules).
- Emit Kubernetes Events for user-facing lifecycle steps.
- Keep RBAC minimal to required permissions.
