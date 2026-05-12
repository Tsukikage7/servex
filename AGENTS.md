# Repository Guidelines

## Project Structure & Module Organization

This repository is a Go module for `github.com/Tsukikage7/servex/v2`. Core framework packages live at the root by domain: `app`, `auth`, `transport`, `middleware`, `observability`, `storage`, `errors`, `config`, `discovery`, `domain`, `llm`, `messaging`, `notify`, and `tenant`. The CLI is under `cmd/servex`. Tests are colocated as `*_test.go`; integration assets and Docker Compose dependencies live under `tests/`. Examples are under `examples/`, especially `examples/ecommerce`. Protobuf auth extensions are in `auth/proto`.

## Build, Test, and Development Commands

Use `just` when available:

- `just check`: run lint, unit tests, and build.
- `just build`: compile all packages with `go build ./...`.
- `just test-unit`: run short race-enabled unit tests and coverage.
- `just test pkg`: run tests for one package, for example `just test auth/jwt`.
- `just test-run pattern`: run tests matching a name pattern.
- `just fmt`: run `gofmt -s` and `goimports`.
- `just vet`: run `go vet ./...`.
- `just services-up` / `just services-down`: start or stop integration dependencies.

If `just` is unavailable, run `go test ./...`, `go vet ./...`, and `go build ./...` directly.

## Coding Style & Naming Conventions

Format Go code with `gofmt -s` and `goimports -local github.com/Tsukikage7/servex/v2`. Keep packages small and cohesive. Prefer explicit, domain-oriented names. Public APIs need clear Go doc comments. Use structured logging through `observability/logger`; messages should follow `[Module] Chinese action description`, with dynamic values in `logger.String`, `logger.Int`, or `logger.Err` fields.

## Testing Guidelines

Use Go’s standard `testing` package with helpers from `testx` where useful. Place tests next to the package they cover and name them `TestXxx`. Add focused regression tests for bug fixes and public API behavior. Run targeted package tests first, then finish with `go test ./...` and `go vet ./...`.

## Commit & Pull Request Guidelines

Recent history follows Conventional Commits, for example `feat(transport/gateway): 统一成功响应格式`. Keep commits scoped and semantic: `feat(scope): ...`, `fix(scope): ...`, `refactor(scope): ...`, or `docs(scope): ...`. Pull requests should include a summary, affected packages, behavior changes, and verification commands. Link related issues when available. Do not mix unrelated refactors with feature or bug-fix work.

## Agent-Specific Instructions

Inspect before editing. Preserve user changes in the working tree. Do not create commits, tags, or branches unless explicitly requested. Prefer small, low-risk changes aligned with existing package patterns, and verify with tests before reporting completion.
