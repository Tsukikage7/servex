# Contributing

servex accepts changes that keep the public API small, explicit, and production-oriented.

## Project Boundary

servex is a Go toolkit for production AI applications and microservice infrastructure. The stable core is:

- application lifecycle
- transport servers and clients
- auth and authorization
- config and discovery
- storage and messaging abstractions
- observability
- testing helpers
- LLM facade, providers, gateway, MCP, and selected adapters

Generic helpers, business components, examples, and adapters are not automatically part of the stable core. New public APIs should be added only when they solve a current, concrete use case.

## Development Rules

- Keep packages cohesive and small.
- Prefer explicit constructors returning errors. Use `MustXxx` only for startup wiring and tests.
- Do not add compatibility wrappers for removed APIs.
- Do not add optional parameters that are not used.
- Public APIs need Go doc comments.
- Root packages must not pull heavy provider SDKs unless the package is explicitly an adapter.

## Verification

Run focused tests first, then the release checks:

```bash
just check
just check-workspace
just vuln
```

For changes in submodules, run their package tests directly as well:

```bash
(cd cmd/servex && go test ./... && go vet ./...)
(cd llm/adapter/eino && go test ./... && go vet ./...)
(cd llm/adapter/adk && go test ./... && go vet ./...)
(cd testx/container && go test ./... && go vet ./...)
```

## Pull Requests

PR descriptions should include:

- affected packages
- API changes
- behavior changes
- migration notes for breaking changes
- verification commands

Breaking changes must update `BREAKING.md`.
