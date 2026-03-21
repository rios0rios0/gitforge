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

The project follows **Clean Architecture** organized as DDD-style bounded contexts under `pkg/`. Each bounded context owns its own `domain/` and `infrastructure/` sub-packages. Dependencies always point inward toward the domain layer.

### Repository Structure

```
gitforge/
├── pkg/
│   ├── changelog/
│   │   └── domain/entities/
│   │       ├── changelog.go           # Changelog struct: NewChangelog, Lines, IsUnreleasedEmpty, FindLatestVersion
│   │       ├── changelog_dedup.go     # DeduplicateEntries: token-overlap semantic deduplication
│   │       ├── changelog_insert.go    # InsertChangelogEntry: inserts bullets under Unreleased/Changed
│   │       ├── changelog_processor.go # Changelog.Process, Changelog.ProcessNew
│   │       ├── changelog_section.go   # UpdateSection, MakeNewSections, ParseUnreleasedIntoSections, FixSectionHeadings
│   │       └── changelog_test.go      # BDD tests for changelog processing
│   ├── config/
│   │   ├── domain/
│   │   │   ├── entities/
│   │   │   │   ├── config.go          # Config struct: NewConfig, Validate, ErrConfigKeyMissing
│   │   │   │   ├── config_test.go     # BDD tests for Config.Validate and ProviderConfig.ResolveToken
│   │   │   │   └── provider_config.go # ProviderConfig struct: ResolveToken (env var / file path expansion)
│   │   │   └── helpers/
│   │   │       └── finder.go          # FindConfigFile: searches standard locations for app config files
│   │   └── infrastructure/
│   │       ├── config_loader.go       # LoadConfig: reads and validates YAML config from file or URL
│   │       └── helpers/
│   │           ├── download.go        # HTTP download helper
│   │           └── loader.go          # ReadData: reads content from file path or HTTP/HTTPS URL
│   ├── git/
│   │   ├── domain/entities/
│   │   │   └── adapter_finder.go      # AdapterFinder interface: GetAdapterByServiceType, GetAdapterByURL
│   │   └── infrastructure/
│   │       ├── operations.go          # GitOperations struct: NewGitOperations, OpenRepo; error sentinels
│   │       ├── operations_auth.go     # Authentication method resolution
│   │       ├── operations_branch.go   # Branch creation
│   │       ├── operations_clone.go    # Repository cloning
│   │       ├── operations_commit.go   # Commit creation (GPG/SSH signing)
│   │       ├── operations_push.go     # Push (SSH/HTTPS)
│   │       ├── operations_repo.go     # Repository-level helpers
│   │       ├── operations_test.go     # BDD tests for git operations
│   │       ├── operations_worktree.go # Worktree management
│   │       ├── url_parser.go          # ParseRemoteURL: detects ServiceType from remote URL
│   │       ├── url_parser_test.go     # BDD tests for URL parsing
│   │       ├── user_config.go         # Git user config lookup
│   │       └── helpers/
│   │           ├── gitconfig.go       # GetGlobalGitConfig, GetOptionFromConfig
│   │           └── ssh.go             # SSH key helpers
│   ├── global/
│   │   └── domain/
│   │       ├── entities/
│   │       │   ├── branch_input.go          # BranchInput struct
│   │       │   ├── branch_status.go         # BranchStatus enum: BranchCreated, BranchExistsWithPR, BranchExistsNoPR
│   │       │   ├── commit_signer.go         # CommitSigner interface: Sign(ctx, content) (string, error)
│   │       │   ├── controller.go            # Controller interface: GetBind(), Execute() error
│   │       │   ├── controller_bind.go       # ControllerBind struct (Cobra bridge)
│   │       │   ├── file.go                  # File struct: Path, ObjectID, IsDir
│   │       │   ├── file_access_provider.go  # FileAccessProvider interface (extends ForgeProvider)
│   │       │   ├── file_change.go           # FileChange struct: Path, Content, ChangeType
│   │       │   ├── forge_provider.go        # ForgeProvider interface (base)
│   │       │   ├── latest_tag.go            # LatestTag struct: Tag (*semver.Version), Date
│   │       │   ├── local_git_auth_provider.go # LocalGitAuthProvider interface (extends ForgeProvider)
│   │       │   ├── pull_request.go          # PullRequest struct: ID, Title, URL, Status
│   │       │   ├── pull_request_detail.go   # PullRequestDetail struct (embeds PullRequest + SourceBranch, TargetBranch, Author)
│   │       │   ├── pull_request_file.go     # PullRequestFile struct: Path, OldPath, Status, Additions, Deletions, Patch
│   │       │   ├── pull_request_input.go    # PullRequestInput struct
│   │       │   ├── repository.go            # Repository struct
│   │       │   ├── repository_discoverer.go # RepositoryDiscoverer interface: Name(), DiscoverRepositories()
│   │       │   ├── review_provider.go       # ReviewProvider interface (extends ForgeProvider)
│   │       │   └── service_type.go          # ServiceType enum: UNKNOWN, GITHUB, GITLAB, AZUREDEVOPS, BITBUCKET, CODECOMMIT
│   │       └── helpers/
│   │           └── versions.go              # SortVersionsDescending, NormalizeVersion
│   ├── providers/
│   │   └── infrastructure/
│   │       ├── github/
│   │       │   ├── provider.go              # Provider struct: NewProvider, Name, MatchesURL, AuthToken, CloneURL, GetServiceType, ...
│   │       │   ├── provider_discovery.go    # DiscoverRepositories
│   │       │   ├── provider_file_access.go  # GetFileContent, ListFiles, GetTags, HasFile, CreateBranchWithChanges
│   │       │   ├── provider_pull_request.go # CreatePullRequest, PullRequestExists
│   │       │   ├── provider_review.go       # ListOpenPullRequests, GetPullRequestDiff, GetPullRequestFiles, PostPullRequestComment, PostPullRequestThreadComment
│   │       │   ├── github_internal_test.go  # Internal BDD tests (httptest server)
│   │       │   └── github_test.go           # External BDD tests
│   │       ├── gitlab/
│   │       │   ├── provider.go              # Provider struct for GitLab
│   │       │   ├── provider_discovery.go    # DiscoverRepositories
│   │       │   ├── provider_file_access.go  # File access operations
│   │       │   ├── provider_pull_request.go # MR creation / existence check
│   │       │   ├── gitlab_internal_test.go  # Internal BDD tests (httptest server)
│   │       │   └── gitlab_test.go           # External BDD tests
│   │       └── azuredevops/
│   │           ├── provider.go              # Provider struct for Azure DevOps
│   │           ├── provider_discovery.go    # DiscoverRepositories
│   │           ├── provider_file_access.go  # File access operations
│   │           ├── provider_http.go         # HTTP transport helpers
│   │           ├── provider_pull_request.go # PR creation / existence check
│   │           ├── provider_review.go       # PR review operations
│   │           ├── provider_url.go          # URL construction helpers
│   │           ├── azuredevops_internal_test.go # Internal BDD tests (redirectTransport)
│   │           └── azuredevops_test.go      # External BDD tests
│   ├── registry/
│   │   └── infrastructure/
│   │       ├── discoverer_factory.go  # DiscovererFactory type (func(token) RepositoryDiscoverer)
│   │       ├── provider_factory.go    # ProviderFactory type (func(token) ForgeProvider)
│   │       ├── provider_registry.go   # ProviderRegistry: RegisterFactory/Adapter/Discoverer, Get, GetDiscoverer, GetAdapterByURL, GetAdapterByServiceType, GetReviewProvider, Names
│   │       └── registry_test.go       # BDD tests for registry
│   └── signing/
│       └── infrastructure/
│           ├── gpg_signer.go   # GPGSigner struct: NewGPGSigner, Key, Sign
│           ├── ssh_signer.go   # SSHSigner struct: NewSSHSigner, Sign
│           └── helpers/
│               ├── gpg.go      # GPG key export, loading, passphrase decryption
│               └── ssh.go      # SSH signing via ssh-keygen
├── test/
│   ├── doubles/
│   │   ├── adapter_finder_stub.go          # AdapterFinderStub (mock AdapterFinder)
│   │   ├── auth_stub.go                    # Authentication mock
│   │   ├── commit_signer_stub.go           # CommitSignerStub (mock CommitSigner)
│   │   ├── forge_provider_stub.go          # ForgeProviderStub (mock ForgeProvider + LocalGitAuthProvider)
│   │   └── repository_discoverer_stub.go   # RepositoryDiscovererStub (mock RepositoryDiscoverer)
│   └── builders/
│       ├── adapter_finder_stub_builder.go          # Builder for AdapterFinderStub
│       ├── forge_provider_stub_builder.go          # Builder for ForgeProviderStub
│       └── repository_discoverer_stub_builder.go   # Builder for RepositoryDiscovererStub
├── Makefile                      # Imports pipeline scripts (lint, test, sast)
├── go.mod                        # Module: github.com/rios0rios0/gitforge (Go 1.26)
└── .github/
    └── workflows/default.yaml    # CI/CD pipeline (delegates to rios0rios0/pipelines go-library workflow)
```

### Layer Responsibilities

| Layer                              | Directory                                    | Responsibility                                                                                                                        |
|------------------------------------|----------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------|
| **Changelog / Domain**             | `pkg/changelog/domain/entities/`             | `Changelog` struct with processing, insertion, deduplication, and section management. No infrastructure dependencies.                |
| **Config / Domain**                | `pkg/config/domain/entities/`                | `Config` and `ProviderConfig` structs; `FindConfigFile` helper. No infrastructure dependencies.                                       |
| **Config / Infrastructure**        | `pkg/config/infrastructure/`                 | `LoadConfig`: reads YAML from file or URL and validates it.                                                                           |
| **Git / Domain**                   | `pkg/git/domain/entities/`                   | `AdapterFinder` interface. No infrastructure dependencies.                                                                            |
| **Git / Infrastructure**           | `pkg/git/infrastructure/`                    | `GitOperations` struct (go-git): branch, commit, push, tag, remote detection, URL parsing. Injected with `AdapterFinder`.             |
| **Global / Domain**                | `pkg/global/domain/entities/`                | All shared interfaces (`ForgeProvider`, `FileAccessProvider`, `ReviewProvider`, `LocalGitAuthProvider`, `CommitSigner`, etc.) and value objects. |
| **Global / Helpers**               | `pkg/global/domain/helpers/`                 | `SortVersionsDescending`, `NormalizeVersion`.                                                                                         |
| **Providers / Infrastructure**     | `pkg/providers/infrastructure/{github,gitlab,azuredevops}/` | Concrete provider implementations satisfying `ForgeProvider`, `FileAccessProvider`, `ReviewProvider`, and `LocalGitAuthProvider`.  |
| **Registry / Infrastructure**      | `pkg/registry/infrastructure/`               | `ProviderRegistry`: factory + adapter patterns, `DiscovererFactory` support, `GetReviewProvider`.                                     |
| **Signing / Infrastructure**       | `pkg/signing/infrastructure/`                | `GPGSigner` and `SSHSigner` — both implement `CommitSigner`.                                                                          |
| **Test Doubles**                   | `test/doubles/` and `test/builders/`         | Stubs and builder helpers for isolated unit testing without real Git hosting connections.                                             |

### Key Design Patterns

- **DDD bounded contexts**: Each sub-domain (`changelog`, `config`, `git`, `global`, `providers`, `registry`, `signing`) owns its own `domain/` and `infrastructure/` sub-packages under `pkg/`.
- **Interface composition**: `ForgeProvider` (base) -> `FileAccessProvider` (adds API file ops) / `ReviewProvider` (adds PR review ops) / `LocalGitAuthProvider` (adds go-git auth). Each concrete provider implements all four.
- **Adapter pattern**: Consumers type-assert to the interface level they need (`ForgeProvider`, `FileAccessProvider`, `ReviewProvider`, or `LocalGitAuthProvider`).
- **Factory pattern**: `ProviderRegistry` creates providers by name + token via registered factory functions.
- **Registry pattern**: `ProviderRegistry` supports factory-based creation, direct adapter lookup by URL or service type, and `GetReviewProvider`.
- **Dependency injection**: `GitOperations` receives an `AdapterFinder` (implemented by `ProviderRegistry`) to resolve auth methods without circular imports.
- **Test doubles**: `test/doubles/` and `test/builders/` provide stubs and builder helpers consumed by all package-level tests.

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
├── ReviewProvider (extends ForgeProvider)
│   ├── ListOpenPullRequests(), GetPullRequestDiff(), GetPullRequestFiles()
│   └── PostPullRequestComment(), PostPullRequestThreadComment()
│
└── LocalGitAuthProvider (extends ForgeProvider)
    ├── GetServiceType(), PrepareCloneURL(), ConfigureTransport()
    └── GetAuthMethods()
```

### Key Domain Types

| Type                    | Package path                              | Purpose                                                                                                          |
|-------------------------|-------------------------------------------|------------------------------------------------------------------------------------------------------------------|
| `Repository`            | `pkg/global/domain/entities`              | Git repository: ID, Name, Organization, Project, DefaultBranch, RemoteURL, SSHURL, ProviderName                 |
| `ServiceType`           | `pkg/global/domain/entities`              | Enum: UNKNOWN, GITHUB, GITLAB, AZUREDEVOPS, BITBUCKET, CODECOMMIT                                               |
| `PullRequest`           | `pkg/global/domain/entities`              | PR entity: ID, Title, URL, Status                                                                                |
| `PullRequestDetail`     | `pkg/global/domain/entities`              | Extends `PullRequest` with SourceBranch, TargetBranch, Author (used by `ReviewProvider`)                        |
| `PullRequestFile`       | `pkg/global/domain/entities`              | Changed file in a PR: Path, OldPath, Status, Additions, Deletions, Patch                                        |
| `PullRequestInput`      | `pkg/global/domain/entities`              | PR creation input: SourceBranch, TargetBranch, Title, Description, AutoComplete                                  |
| `BranchInput`           | `pkg/global/domain/entities`              | Branch creation input: BranchName, BaseBranch, Changes, CommitMessage                                           |
| `File` / `FileChange`   | `pkg/global/domain/entities`              | File entry and file modification structs                                                                         |
| `LatestTag`             | `pkg/global/domain/entities`              | Latest git tag: Tag (*semver.Version), Date                                                                      |
| `BranchStatus`          | `pkg/global/domain/entities`              | Enum: BranchCreated, BranchExistsWithPR, BranchExistsNoPR                                                       |
| `Controller`            | `pkg/global/domain/entities`              | CLI controller interface (Cobra bridge): GetBind(), Execute() error                                              |
| `CommitSigner`          | `pkg/global/domain/entities`              | Interface: Sign(ctx, commitContent) (string, error) — implemented by GPGSigner and SSHSigner                    |
| `AdapterFinder`         | `pkg/git/domain/entities`                 | Interface: GetAdapterByServiceType(), GetAdapterByURL() — implemented by ProviderRegistry                        |
| `Config`                | `pkg/config/domain/entities`              | Full app config: Providers []ProviderConfig; methods: NewConfig(), Validate()                                    |
| `ProviderConfig`        | `pkg/config/domain/entities`              | Provider config: Type, Token, Organizations; method: ResolveToken()                                              |
| `Changelog`             | `pkg/changelog/domain/entities`           | Changelog document: NewChangelog(lines), Lines(), IsUnreleasedEmpty(), FindLatestVersion(), Process(), ProcessNew() |
| `GPGSigner`             | `pkg/signing/infrastructure`              | GPG commit signer: NewGPGSigner(key), Key(), Sign()                                                              |
| `SSHSigner`             | `pkg/signing/infrastructure`              | SSH commit signer: NewSSHSigner(keyPath), Sign()                                                                 |
| `GitOperations`         | `pkg/git/infrastructure`                  | Local git operations: NewGitOperations(finder), plus methods for branch/commit/push/clone/tag                    |
| `ProviderRegistry`      | `pkg/registry/infrastructure`             | Provider registry: RegisterFactory/Adapter/Discoverer, Get, GetDiscoverer, GetAdapterByURL, GetAdapterByServiceType, GetReviewProvider, Names |

### Key Domain Functions and Methods

**Changelog** (`pkg/changelog/domain/entities`):
- `NewChangelog(lines []string) *Changelog` -- constructs a changelog from lines
- `(c *Changelog) Process() (*semver.Version, []string, error)` -- calculates next version and new content from existing changelog
- `(c *Changelog) ProcessNew() (*semver.Version, []string, error)` -- handles changelogs with no previous release version (releases as 0.1.0)
- `(c *Changelog) FindLatestVersion() (*semver.Version, error)` -- finds the highest released version
- `(c *Changelog) IsUnreleasedEmpty() (bool, error)` -- checks if the [Unreleased] section has any entries
- `InsertChangelogEntry(content string, entries []string) string` -- inserts bullet entries under Unreleased/Changed
- `DeduplicateEntries(entries []string) []string` -- removes exact duplicates and semantically overlapping entries
- `UpdateSection(unreleased []string, version semver.Version) ([]string, *semver.Version, error)` -- deduplicates, sorts, and calculates version bump

**Config** (`pkg/config/domain/entities` and `pkg/config/domain/helpers`):
- `NewConfig(providers []ProviderConfig) *Config` -- constructs a Config
- `(c *Config) Validate() error` -- validates all provider entries (type, token, organizations)
- `(p *ProviderConfig) ResolveToken() string` -- expands `${ENV_VAR}` references and reads from file if path exists
- `FindConfigFile(appName string) (string, error)` -- searches standard locations for `.{appName}.yaml` config files

**Config infrastructure** (`pkg/config/infrastructure`):
- `LoadConfig(path string) (*Config, error)` -- reads, parses, and validates a YAML config from file or URL

**Git** (`pkg/git/infrastructure`):
- `NewGitOperations(finder AdapterFinder) *GitOperations` -- creates a GitOperations instance
- `OpenRepo(projectPath string) (*git.Repository, error)` -- opens a local git repository
- `PushChangesSSH(repo, refSpec, authMethods) error` -- pushes over SSH; tries explicit auth methods first, falls back to default SSH agent
- `PushWithTransportDetection(repo, refSpec, authMethods) error` -- auto-detects SSH/HTTPS from remote URL and forwards auth methods to both transports

**Signing** (`pkg/signing/infrastructure`):
- `NewGPGSigner(key *openpgp.Entity) *GPGSigner` -- creates a GPG commit signer
- `NewSSHSigner(keyPath string) *SSHSigner` -- creates an SSH commit signer
- `ResolveSignerFromGitConfig(gpgSign, signingFormat, signingKey, gpgKeyPath, gpgPassphrase, appName) (CommitSigner, error)` -- resolves GPG/SSH signer from git config values

**Version helpers** (`pkg/global/domain/helpers`):
- `SortVersionsDescending(versions []string)` -- sorts version strings descending by semver
- `NormalizeVersion(version string) string` -- ensures a "v" prefix for semver compatibility

## Consumer Projects

This library is imported by two projects:

| Project        | What it uses                                                                                                                          |
|----------------|---------------------------------------------------------------------------------------------------------------------------------------|
| **autobump**   | Changelog processing, git operations, GPG/SSH signing, push with transport detection, provider adapters (via `LocalGitAuthProvider`), config loading |
| **autoupdate** | Provider implementations (via `FileAccessProvider`), config loading, changelog insertion, registry, push with transport detection, signing, `ReviewProvider` (via autoreview) |

### Adding New Shared Functionality

When adding features to gitforge:

1. Consider whether the feature is truly shared (needed by both consumers) or project-specific.
2. Place domain concepts in the appropriate `pkg/<context>/domain/entities/` package.
3. Place implementations in the corresponding `pkg/<context>/infrastructure/` package.
4. Ensure backward compatibility — exported type changes break consumers.
5. After changes, verify both consumer projects still compile: run `go build ./...` in each.

## Testing

### Standards

- All tests follow **BDD** structure with `// given`, `// when`, `// then` comment blocks.
- Test descriptions use `"should ... when ..."` format via `t.Run()` subtests.
- Unit tests use `t.Parallel()` at both parent and subtest level.
- Tests that use `t.Setenv` or `t.Chdir` must NOT call `t.Parallel()` on that subtest (Go restriction).
- Tests that mutate global state (e.g., `SetAdapterFinder`) must NOT call `t.Parallel()`.
- All tests use `testify` (`assert`/`require`) — never bare `t.Error`/`t.Fatal`.

### Test Infrastructure

`test/doubles/` contains stub implementations of all key interfaces:

| Stub                        | Implements                              |
|-----------------------------|-----------------------------------------|
| `ForgeProviderStub`         | `ForgeProvider`, `LocalGitAuthProvider` |
| `RepositoryDiscovererStub`  | `RepositoryDiscoverer`                  |
| `AdapterFinderStub`         | `AdapterFinder`                         |
| `CommitSignerStub`          | `CommitSigner`                          |
| `AuthStub`                  | go-git `transport.AuthMethod`           |

`test/builders/` provides builder-pattern helpers for constructing stubs in tests.

### Test Files

| File                                                                | Tests                                                                              |
|---------------------------------------------------------------------|------------------------------------------------------------------------------------|
| `pkg/changelog/domain/entities/changelog_test.go`                  | FindLatestVersion, IsUnreleasedEmpty, DeduplicateEntries, InsertChangelogEntry     |
| `pkg/config/domain/entities/config_test.go`                        | Config.Validate, ProviderConfig.ResolveToken (env var, file, inline)               |
| `pkg/git/infrastructure/operations_test.go`                        | NewGitOperations, branch/commit/push/clone/tag operations                          |
| `pkg/git/infrastructure/url_parser_test.go`                        | ParseRemoteURL (GitHub, GitLab, Azure DevOps, SSH, HTTPS)                          |
| `pkg/providers/infrastructure/github/github_test.go`               | NewProvider, Name, MatchesURL, GetServiceType                                      |
| `pkg/providers/infrastructure/github/github_internal_test.go`      | DiscoverRepositories, CreatePullRequest, file access (httptest server)             |
| `pkg/providers/infrastructure/gitlab/gitlab_test.go`               | NewProvider, Name, MatchesURL, GetServiceType                                      |
| `pkg/providers/infrastructure/gitlab/gitlab_internal_test.go`      | DiscoverRepositories, CreatePullRequest, file access (httptest server)             |
| `pkg/providers/infrastructure/azuredevops/azuredevops_test.go`     | NewProvider, Name, MatchesURL, GetServiceType                                      |
| `pkg/providers/infrastructure/azuredevops/azuredevops_internal_test.go` | DiscoverRepositories, file access (redirectTransport to httptest server)      |
| `pkg/registry/infrastructure/registry_test.go`                     | NewProviderRegistry, Get, GetDiscoverer, GetAdapterByURL, GetReviewProvider        |

### Provider Test Patterns

- **GitHub / GitLab**: Override the SDK `BaseURL` to point to an `httptest.Server`.
- **Azure DevOps**: Use a `redirectTransport` that rewrites hardcoded `dev.azure.com` URLs to an `httptest.Server`.
- All provider tests use the **internal** package (`package github`, not `github_test`) so they can access unexported fields.

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
