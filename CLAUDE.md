# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

gitforge is a shared Go library (no `main.go`, no CLI) providing unified abstractions for GitHub, GitLab, and Azure DevOps. Consumed by [autobump](https://github.com/rios0rios0/autobump) and [autoupdate](https://github.com/rios0rios0/autoupdate). Breaking changes to exported types affect both consumers.

## Commands

```bash
go build ./...          # Compile check (~2s)
make lint               # golangci-lint via pipeline scripts (~3-5s)
make test               # Full test suite (~3s clean, <1s cached)
make sast               # CodeQL, Semgrep, Trivy, Hadolint, Gitleaks (~1-3min)
go test ./...           # Quick test during development (acceptable shortcut)
```

Never run `golangci-lint`, `semgrep`, `gitleaks`, etc. directly — always use `make` targets. There is no `make build` or `make run`.

## Architecture

Clean Architecture with DDD bounded contexts under `pkg/`. Each context owns `domain/` (contracts) and `infrastructure/` (implementations). Dependencies point inward.

### Bounded Contexts

| Context | Purpose |
|---------|---------|
| `pkg/changelog/` | Version calculation, entry deduplication, section management |
| `pkg/config/` | YAML config loading, token resolution, validation |
| `pkg/git/` | Local git operations (go-git wrapper), URL parsing |
| `pkg/global/` | Shared interfaces and value objects (the core contracts) |
| `pkg/providers/` | GitHub, GitLab, Azure DevOps implementations |
| `pkg/registry/` | Provider factory, adapter lookup, service discovery |
| `pkg/signing/` | GPG and SSH commit signing |

### Provider Interface Hierarchy

`ForgeProvider` (base) is extended by `FileAccessProvider`, `ReviewProvider`, and `LocalGitAuthProvider`. Each concrete provider (GitHub, GitLab, ADO) implements all four interfaces. Consumers type-assert to the level they need.

### Key Patterns

- **Factory + Registry**: `ProviderRegistry` creates providers by name/token and resolves adapters by URL or `ServiceType`
- **Adapter**: `GitOperations` receives `AdapterFinder` (implemented by `ProviderRegistry`) to decouple auth resolution
- **Constructor injection**: No DI framework; dependencies passed via constructors

## Testing

All tests use `//go:build unit` build tags, BDD structure (`// given` / `// when` / `// then`), `t.Parallel()`, and `testify` assertions. Test doubles live in `test/doubles/` (stubs) and `test/builders/` (builder pattern).

**Provider test patterns:**
- GitHub/GitLab: override SDK `BaseURL` → `httptest.Server`
- Azure DevOps: `redirectTransport` rewrites `dev.azure.com` URLs → `httptest.Server`
- Provider internal tests use the internal package (not `_test` suffix) to access unexported fields

**Parallelism exceptions:** tests using `t.Setenv`, `t.Chdir`, or mutating global state must NOT call `t.Parallel()`.

## Validation After Changes

1. `go build ./...` — zero errors
2. `make lint` — zero issues
3. `make test` — all pass
4. When changing exported types: verify `autobump` and `autoupdate` still compile
