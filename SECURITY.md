# Security Policy

## Supported Versions

The active development line is the current v2 development line described in `BREAKING.md`.

Security fixes are expected to target the current development branch first. Backports are handled case by case when a maintained release branch exists.

## Reporting a Vulnerability

Please report security issues privately before public disclosure. Include:

- affected package and version or commit
- a minimal reproduction or exploit sketch
- expected impact
- whether the issue is remotely triggerable

Do not include production secrets, tokens, private keys, or customer data in reports.

## Security Baseline

Release candidates must pass:

```bash
just check
just check-workspace
just vuln
```

The CI workflow enforces `go test`, `go vet`, dependency boundary checks, and `govulncheck` across all Go modules in `go.work`.

## Current Toolchain Note

At the time of this hardening pass, both `govulncheck ./...` and `gopls go_vulncheck` report no reachable vulnerabilities in servex code after these dependency updates:

- OpenTelemetry OTLP trace exporters upgraded to `v1.44.0`.
- `golang.org/x/crypto` upgraded to `v0.52.0` where it is reachable from `testx/container`.

When a patched Go toolchain is required by future advisories, update `go.work`, submodule `go.mod` toolchain directives, and CI `GO_VERSION` together.
