# gitforge

gitforge is a shared Go library providing common abstractions for Git hosting platforms (GitHub, GitLab, Azure DevOps). It is consumed by [autobump](https://github.com/rios0rios0/autobump) and [autoupdate](https://github.com/rios0rios0/autoupdate) via Go module imports. This is a **library**, not a standalone binary — there is no `main.go` or CLI.

Always reference these instructions first and fall back to search or bash commands only when you encounter unexpected information that does not match the info here.

## Working Effectively

### Bootstrap and Test

- Install dependencies: `go mod download`
- Build (compile check): `go build ./...`
- Run tests: `make test` -- NEVER run `go test` directly.
- Run linting: `make lint` -- NEVER run `golangci-lint` directly.
- Run security analysis: `make sast` -- NEVER run `gitleaks`, `semgrep`, `trivy`, `hadolint`, or `codeql` directly.
- Tidy dependencies: `go mod tidy`

### Linting, Testing, and SAST with Makefile

This project uses the [rios0rios0/pipelines](https://github.com/rios0rios0/pipelines) repository for shared CI/CD scripts. The `Makefile` imports these scripts via `SCRIPTS_DIR`. Always use `make` targets:

```bash
make lint    # golangci-lint via pipeline scripts
make test    # unit + integration tests via pipeline scripts
make sast    # CodeQL, Semgrep, Trivy, Hadolint, Gitleaks
```

### Important: This Is a Library

- There is **no `main` package**, no CLI, and no `make build` or `make run` targets.
- Changes must be validated by compiling (`go build ./...`) and running tests (`make test`).
- Any breaking change to exported types or interfaces affects both consumer projects (`autobump` and `autoupdate`).

## Architecture

The project follows **Clean Architecture** with dependencies always pointing inward toward the domain layer. As a library, it exposes packages that consumers import.

### Repository Structure

```
gitforge/
├── domain/
│   ├── entities/
│   │   ├── repository.go         # Repository, ServiceType, BranchStatus, LatestTag,
│   │   │                         #   RepositoryDiscoverer interface
│   │   ├── pull_request.go       # PullRequest, PullRequestInput, BranchInput
│   │   ├── file.go               # File, FileChange
│   │   ├── config.go             # ProviderConfig, ResolveToken, FindConfigFile,
│   │   │                         #   ValidateProviders
│   │   ├── changelog.go          # Full changelog toolkit: ProcessChangelog,
│   │   │                         #   InsertChangelogEntry, FindLatestVersion,
│   │   │                         #   DeduplicateEntries, UpdateSection, etc.
│   │   ├── changelog_test.go     # BDD tests for changelog processing
│   │   ├── config_test.go        # BDD tests for config utilities
│   │   └── controller.go         # Controller interface, ControllerBind (Cobra bridge)
│   └── repositories/
│       └── provider.go           # ForgeProvider, FileAccessProvider,
│                                 #   LocalGitAuthProvider interfaces
├── infrastructure/
│   ├── providers/
│   │   ├── github/
│   │   │   └── github.go         # GitHub: discovery, file ops, PR, local auth
│   │   ├── gitlab/
│   │   │   └── gitlab.go         # GitLab: discovery, file ops, MR, local auth
│   │   └── azuredevops/
│   │       └── azuredevops.go    # Azure DevOps: discovery, file ops, PR, local auth
│   ├── git/
│   │   ├── operations.go         # Local git: open, branch, commit (GPG), push
│   │   │                         #   (SSH/HTTPS), tag, remote detection (go-git)
│   │   └── config.go             # GetGlobalGitConfig, GetOptionFromConfig
│   ├── signing/
│   │   ├── gpg.go                # GPG key export, loading, passphrase decryption
│   │   └── ssh.go                # SSH signing placeholder (future)
│   ├── config/
│   │   └── loader.go             # ReadData (file path or URL)
│   └── registry/
│       └── registry.go           # ProviderRegistry: factory + adapter patterns,
│                                 #   DiscovererFactory support
├── support/
│   ├── utils.go                  # ReadLines, WriteLines, DownloadFile, StripUsernameFromURL
│   ├── utils_test.go             # BDD tests for file I/O and URL utils
│   ├── version.go                # SortVersionsDescending, NormalizeVersion
│   └── version_test.go           # BDD tests for version sorting
├── Makefile                      # Imports pipeline scripts (lint, test, sast)
├── go.mod                        # Module: github.com/rios0rios0/gitforge (Go 1.26)
└── .github/
    └── workflows/default.yaml    # CI/CD pipeline
```

### Layer Responsibilities

| Layer                          | Directory                   | Responsibility                                                                                                                             |
|--------------------------------|-----------------------------|--------------------------------------------------------------------------------------------------------------------------------------------|
| **Domain / Entities**          | `domain/entities/`          | Core business objects (`Repository`, `PullRequest`, `File`, etc.), changelog processing, config utilities. No infrastructure dependencies. |
| **Domain / Repositories**      | `domain/repositories/`      | Provider interfaces (`ForgeProvider`, `FileAccessProvider`, `LocalGitAuthProvider`). Contracts only — no implementations.                  |
| **Infrastructure / Providers** | `infrastructure/providers/` | GitHub, GitLab, Azure DevOps implementations satisfying all three provider interfaces.                                                     |
| **Infrastructure / Git**       | `infrastructure/git/`       | Local git operations via go-git: branch, commit, push, tag, remote detection.                                                              |
| **Infrastructure / Signing**   | `infrastructure/signing/`   | GPG key management and commit signing. SSH signing placeholder.                                                                            |
| **Infrastructure / Config**    | `infrastructure/config/`    | Configuration data reading (file or URL).                                                                                                  |
| **Infrastructure / Registry**  | `infrastructure/registry/`  | Factory-based provider and discoverer registries.                                                                                          |
| **Support**                    | `support/`                  | Shared utilities: file I/O, HTTP downloads, URL manipulation, semantic version sorting.                                                    |

### Key Design Patterns

- **Interface composition**: `ForgeProvider` (base) -> `FileAccessProvider` (adds API file ops) -> `LocalGitAuthProvider` (adds go-git auth). Each concrete provider implements all three.
- **Adapter pattern**: Consumers type-assert to the interface level they need (`ForgeProvider`, `FileAccessProvider`, or `LocalGitAuthProvider`).
- **Factory pattern**: `ProviderRegistry` creates providers by name + token via registered factory functions.
- **Registry pattern**: `ProviderRegistry` supports both factory-based creation and direct adapter lookup by URL or service type.

### Provider Interface Hierarchy

```
ForgeProvider (base)
├── Name(), MatchesURL(), AuthToken(), CloneURL()
├── DiscoverRepositories(), CreatePullRequest(), PullRequestExists()
│
├── FileAccessProvider (extends ForgeProvider)
│   ├── GetFileContent(), ListFiles(), GetTags(), HasFile()
│   └── CreateBranchWithChanges()
│
└── LocalGitAuthProvider (extends ForgeProvider)
    ├── GetServiceType(), PrepareCloneURL(), ConfigureTransport()
    └── GetAuthMethods()
```

### Key Domain Types

| Type                  | File              | Purpose                                                                                                     |
|-----------------------|-------------------|-------------------------------------------------------------------------------------------------------------|
| `Repository`          | `repository.go`   | Git repository with fields: ID, Name, Organization, Project, DefaultBranch, RemoteURL, SSHURL, ProviderName |
| `ServiceType`         | `repository.go`   | Enum: UNKNOWN, GITHUB, GITLAB, AZUREDEVOPS, BITBUCKET, CODECOMMIT                                           |
| `PullRequest`         | `pull_request.go` | PR entity: ID, Title, URL, Status                                                                           |
| `PullRequestInput`    | `pull_request.go` | PR creation input: SourceBranch, TargetBranch, Title, Description, AutoComplete                             |
| `BranchInput`         | `pull_request.go` | Branch creation input: BranchName, BaseBranch, Changes, CommitMessage                                       |
| `File` / `FileChange` | `file.go`         | File entry and file modification structs                                                                    |
| `ProviderConfig`      | `config.go`       | Provider config: Type, Token, Organizations                                                                 |
| `LatestTag`           | `repository.go`   | Latest git tag: Tag (*semver.Version), Date                                                                 |
| `BranchStatus`        | `repository.go`   | Enum: BranchCreated, BranchExistsWithPR, BranchExistsNoPR                                                   |
| `Controller`          | `controller.go`   | CLI controller interface (Cobra bridge): GetBind(), Execute() error                                         |

### Key Domain Functions

- `ProcessChangelog(lines) (*semver.Version, []string, error)` -- processes changelog, calculates next version based on changes (major/minor/patch)
- `InsertChangelogEntry(content, entries) string` -- inserts bullet entries under Unreleased/Changed
- `FindLatestVersion(lines) (*semver.Version, error)` -- finds highest version in changelog
- `DeduplicateEntries(entries) []string` -- removes exact duplicates and merges semantically overlapping entries using token overlap
- `UpdateSection(unreleased, version) ([]string, *semver.Version, error)` -- updates unreleased section, deduplicates, sorts, calculates version bump
- `ResolveToken(raw) string` -- expands `${ENV_VAR}` references and reads from file if path exists
- `FindConfigFile(appName) (string, error)` -- searches standard locations for `.{appName}.yaml` config files
- `ValidateProviders(providers) error` -- validates provider config entries (type, token, organizations)

## Consumer Projects

This library is imported by two projects:

| Project        | What it uses                                                                                                                   |
|----------------|--------------------------------------------------------------------------------------------------------------------------------|
| **autobump**   | Entities, changelog processing, git operations, GPG signing, provider adapters (via `LocalGitAuthProvider`), support utilities |
| **autoupdate** | Entities, provider implementations (via `FileAccessProvider`), config utilities, changelog insertion, registry                 |

### Adding New Shared Functionality

When adding features to gitforge:

1. Consider whether the feature is truly shared (needed by both consumers) or project-specific.
2. Place domain concepts in `domain/entities/` or `domain/repositories/`.
3. Place implementations in the appropriate `infrastructure/` sub-package.
4. Ensure backward compatibility — exported type changes break consumers.
5. After changes, verify both consumer projects still compile: run `go build ./...` in each.

## Testing

### Standards

- All tests follow **BDD** structure with `// given`, `// when`, `// then` comment blocks.
- Test descriptions use `"should ... when ..."` format via `t.Run()` subtests.
- Unit tests use `t.Parallel()` at both parent and subtest level.
- Tests that use `t.Setenv` or `t.Chdir` must NOT call `t.Parallel()` on that subtest (Go restriction).

### Test Files

| File                                | Tests                                                                                   |
|-------------------------------------|-----------------------------------------------------------------------------------------|
| `domain/entities/changelog_test.go` | FindLatestVersion, IsChangelogUnreleasedEmpty, DeduplicateEntries, InsertChangelogEntry |
| `domain/entities/config_test.go`    | ResolveToken (env var, file, inline), FindConfigFile, ValidateProviders                 |
| `support/utils_test.go`             | ReadLines, WriteLines, StripUsernameFromURL                                             |
| `support/version_test.go`           | SortVersionsDescending, NormalizeVersion                                                |

### Running Tests

```bash
make test             # Full test suite via pipeline scripts (ALWAYS use this)
go test ./...         # Quick compile + test check during development (acceptable)
```

## Validation

### After Making Changes

1. `go build ./...` -- must compile with zero errors
2. `make lint` -- must report 0 issues
3. `make test` -- all tests must pass
4. `make sast` -- should report no new findings
5. Verify consumer projects still compile:
   - `cd ../autobump && go build ./...`
   - `cd ../autoupdate && go build ./...`

### Pre-commit

- Always run `make lint` before committing (CI will fail otherwise).
- Always run `make test` to ensure no regressions.
- Always run `make sast` to catch security issues.
- Always verify consumer compatibility when changing exported types/interfaces.

## Build and Test Timing Expectations

- **Compile check** (`go build ./...`): <2 seconds.
- **Tests**: <1 second cached, ~3 seconds clean.
- **Lint**: ~3-5 seconds.
- **SAST**: ~1-3 minutes. Set timeout to 60+ minutes.
- **Go mod operations**: <1 second after first download.

## Common Development Commands

```bash
# Full validation cycle
go build ./... && make lint && make test

# Quick test cycle during development
go test ./...

# Full security + quality gate
make lint && make test && make sast

# Verify consumer compatibility after interface changes
cd ../autobump && go build ./... && cd ../autoupdate && go build ./...
```

Always validate that changes do not break the consumer projects (`autobump` and `autoupdate`).
