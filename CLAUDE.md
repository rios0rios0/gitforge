# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

gitforge is a shared Go library (no `main.go`, no CLI) providing unified abstractions for GitHub, GitLab, Azure DevOps, and Codeberg (Forgejo). Consumed by [autobump](https://github.com/rios0rios0/autobump) and [autoupdate](https://github.com/rios0rios0/autoupdate). Breaking changes to exported types affect both consumers.

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
| `pkg/git/` | Local git operations (go-git wrapper), URL parsing, SSH/HTTPS push with auth retry |
| `pkg/global/` | Shared interfaces and value objects (the core contracts) |
| `pkg/providers/` | GitHub, GitLab, Azure DevOps, Codeberg (Forgejo) implementations |
| `pkg/registry/` | Provider factory, adapter lookup, service discovery |
| `pkg/signing/` | GPG and SSH commit signing |

### Provider Interface Hierarchy

`ForgeProvider` (base) is extended by `FileAccessProvider`, `ReviewProvider`, `LocalGitAuthProvider`, and `MirrorProvider`. GitHub and Azure DevOps implement `ForgeProvider`, `FileAccessProvider`, `ReviewProvider`, and `LocalGitAuthProvider`. GitLab implements `ForgeProvider`, `FileAccessProvider`, and `LocalGitAuthProvider` — it does **not** implement `ReviewProvider` (no `pkg/providers/infrastructure/gitlab/provider_review.go`). Codeberg implements `ForgeProvider`, `FileAccessProvider`, `LocalGitAuthProvider`, and `MirrorProvider`. Consumers type-assert to the interface level they need.

### Key Patterns

- **Factory + Registry**: `ProviderRegistry` creates providers by name/token and resolves adapters by URL or `ServiceType`
- **Adapter**: `GitOperations` receives `AdapterFinder` (implemented by `ProviderRegistry`) to decouple auth resolution
- **Constructor injection**: No DI framework; dependencies passed via constructors

## Testing

All tests use BDD structure (`// given` / `// when` / `// then`), `t.Parallel()`, and `testify` assertions. Some test files carry a `//go:build unit` tag (newer tests adopted it; not all have been migrated), so a bare `go test ./...` silently skips them — `make test` is the canonical command that runs the full suite. Test doubles live in `test/doubles/` (stubs) and `test/builders/` (builder pattern).

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

<!-- chlog:start -->
## Changelog (chlog) — MANDATORY

If the repository you are working in uses chlog (a `.chlog.yaml` or `.chlog.yml`
config file, or a `.changes/` directory, exists at the project root), the
following is binding and ALWAYS applies: whenever you make ANY change, you MUST
create a changelog fragment as part of the same change — automatically, without
being asked, before committing.

- Do NOT edit CHANGELOG.md directly; it is generated from fragments.
- Create the fragment with:
  `chlog new --kind <Kind> --body "<imperative description>"`
- Valid kinds: Added, Changed, Deprecated, Removed, Fixed, Security
- Choose the kind that best matches the change (e.g., new feature → Added,
  bug fix → Fixed, behavior change → Changed, removal → Removed, security fix → Security).
- If the change is backward-INCOMPATIBLE with the public API (a breaking
  change), you MUST add the `--breaking` flag:
  `chlog new --kind <Kind> --breaking --body "<description>"`.
  This is the ONLY thing that triggers a major version bump — the kind alone
  never does (per SemVer, major = incompatible change). When unsure whether a
  change breaks compatibility, ask the user instead of guessing.
- Fragments are YAML files in `.changes/unreleased/`; stage them with your commit.
- `chlog check` fails the build when a fragment is missing — never skip it.
<!-- chlog:end -->
