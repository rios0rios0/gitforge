# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

When a new release is proposed:

1. Create a new branch `bump/x.x.x` (this isn't a long-lived branch!!!);
2. The Unreleased section on `CHANGELOG.md` gets a version number and date;
3. Open a Pull Request with the bump version changes targeting the `main` branch;
4. When the Pull Request is merged, a new Git tag must be created using [GitHub environment](https://github.com/rios0rios0/gitforge/tags).

Releases to productive environments should run from a tagged version.
Exceptions are acceptable depending on the circumstances (critical bug fixes that can be cherry-picked, etc.).

## [Unreleased]

### Changed

- changed the Go module dependencies to their latest versions

## [0.9.2] - 2026-04-15

### Changed

- changed the Go version to `1.26.2` and updated all module dependencies

## [0.9.1] - 2026-04-01

### Changed

- changed the Go module dependencies to their latest versions

## [0.9.0] - 2026-03-30

### Added

- added `CODEBERG` service type with token resolution via `CODEBERG_TOKEN` environment variable
- added `NewProviderWithClient` constructor to Codeberg provider for test-friendly HTTP client injection
- added Codeberg (Forgejo) provider with mirror support, repository discovery, pull requests, file access, and local git auth

### Changed

- changed the Go module dependencies to their latest versions

### Fixed

- fixed `CreateBranchWithChanges` in Codeberg provider to handle `delete` change type and reject unsupported change types
- fixed `DiscoverRepositories` in Codeberg provider to only fall back to user repos on HTTP 404 instead of any error
- fixed `MirrorProviderStub` to use pointer embedding so `ForgeProviderStub` methods are properly promoted
- fixed `PullRequestExists` in Codeberg provider to paginate through all open PRs instead of checking only the first page
- fixed `SSHCloneURL` across all providers to use the default SSH hostname when `sshAlias` is empty, and the alias convention (`{host}-{alias}`) when provided
- fixed Azure DevOps `SSHCloneURL` to use `dev.azure.com` alias convention instead of `ssh.dev.azure.com`, matching the standard SSH config `Host` entry pattern

## [0.8.0] - 2026-03-23

### Added

- added `SSHCloneURL(repo, sshAlias)` to `ForgeProvider` interface for SSH alias-based clone URLs

## [0.7.3] - 2026-03-22

### Changed

- changed `PushChangesSSH` to accept optional SSH auth methods, enabling explicit SSH key and custom agent socket authentication
- changed `PushWithTransportDetection` to forward auth methods to SSH push instead of discarding them

## [0.7.2] - 2026-03-20

### Fixed

- fixed Azure DevOps `GetPullRequestDiff` returning empty diffs by fetching file content at both source and target branches via the items API and computing unified diffs locally with `sergi/go-diff`
- fixed Azure DevOps `ListOpenPullRequests` returning draft PRs by filtering on the `isDraft` field

## [0.7.1] - 2026-03-19

### Changed

- changed the Go module dependencies to their latest versions

### Fixed

- fixed `SSH_AUTH_SOCK` check being incorrectly skipped when `gpg.ssh.program` is explicitly set to `ssh-keygen` (or an absolute path to it)
- fixed SSH signing ignoring `gpg.ssh.program` git config, causing failures on WSL2 with 1Password (`op-ssh-sign-wsl`); now reads the config and delegates to the configured signing binary instead of hardcoding `ssh-keygen`
- fixed version heading detection in `Process()` and `IsUnreleasedEmpty()` to apply `TrimSpace` consistently with `FindLatestVersion()`, preventing the unreleased section from swallowing the rest of the file when headings have leading whitespace

## [0.7.0] - 2026-03-17

### Added

- added `IsFork` and `IsArchived` fields to `Repository` entity, populated from the GitHub API response
- added URL credential sanitization in clone log messages to prevent token leakage in CI logs

### Changed

- changed GitHub user repo discovery from `Type: "owner"` to `Type: "all"` to include member and collaborator repos in discovery results
- changed org-fallback log message from `Warn` to `Debug` level to reduce noise for personal GitHub accounts

### Fixed

- fixed clone log and error messages using inconsistent URLs by always referencing the adapter-rewritten `cloneURL`
- fixed org-to-user discovery fallback silently swallowing non-404 API errors (auth failures, rate limits, 5xx) by only falling back on HTTP 404

## [0.6.2] - 2026-03-17

### Fixed

- fixed GitHub SSH URL parsing to support SSH config aliases (e.g. `git@github.com-mine:owner/repo.git`) by using flexible host matching instead of exact prefix

## [0.6.1] - 2026-03-17

### Fixed

- fixed cross-compilation failure on Windows in GPG passphrase prompt where `syscall.Stdin` type mismatch prevented building
- fixed SSH signing failing when `user.signingkey` is an inline public key string (e.g. `ssh-ed25519 AAAAC3...`) used by ssh-agent workflows (1Password, YubiKey, WSL interop); now detects inline keys and signs via the SSH agent with `ssh-keygen -U`

## [0.6.0] - 2026-03-16

### Added

- added `GetPullRequestCheckStatus()` to `ReviewProvider` for querying CI check/status results on a PR (GitHub and Azure DevOps)
- added `MergePullRequest()` to `ReviewProvider` for merging/completing a pull request (GitHub and Azure DevOps)

### Fixed

- fixed branch checkout failing in CI after native `git clone` by using forced checkout for newly created branches
- fixed GPG passphrase prompt breaking CI logs by detecting non-TTY environments before printing

## [0.5.0] - 2026-03-13

### Added

- added verb-based entry reclassification in changelog processing, automatically moving entries to the correct section based on their leading verb (e.g., `- removed X` under Changed moves to Removed)

### Changed

- changed backtick content handling in changelog deduplication to preserve text inside backticks instead of stripping it entirely, preventing false-positive duplicate detection for entries differing only in backtick content
- changed deduplication overlap threshold from 0.6 to 0.9 to prevent aggressive false-positive merging of distinct changelog entries
- changed the Go module dependencies to their latest versions

### Removed

- removed alphabetical sorting of changelog entries within sections, preserving the original order written by users

## [0.4.0] - 2026-03-12

### Added

- added `ResolveTokenFromEnv` and `TokenEnvHint` helpers in `pkg/config/domain/helpers/` for provider-specific token resolution from environment variables, eliminating duplication across autobump and autoupdate

### Changed

- changed the Go version to `1.26.1` and updated all module dependencies

## [0.3.0] - 2026-03-11

### Added

- added `PushWithTransportDetection()` in `pkg/git/infrastructure/` to auto-detect SSH/HTTPS transport from remote URL and push with auth method retry, eliminating duplication across autobump and autoupdate
- added `ResolveSignerFromGitConfig()` in `pkg/signing/infrastructure/` to centralize commit signer resolution (GPG/SSH) from git config values, eliminating duplication across autobump and autoupdate
- added `ServiceTypeToProviderName()` in `pkg/registry/infrastructure/provider_registry.go` to map `ServiceType` values to registry provider names shared by consumer projects

## [0.2.0] - 2026-03-09

### Added

- added `ensureRefsPrefix()` helper to normalize branch names with `refs/heads/` prefix in Azure DevOps provider
- added `resolveRepoIdentifier()` helper to fall back to `repo.Name` when `repo.ID` is empty in Azure DevOps provider
- added base64-encoded GPG key auto-detection and decoding in `GetGpgKeyReader()`
- added explicit `passphrase` parameter to `GetGpgKey()` for non-interactive environments (CI/CD)

### Changed

- changed `GetGpgKey()` signature to accept a `passphrase` parameter

### Fixed

- fixed Azure DevOps PR creation and existence check failing with 404 when `repo.ID` is empty
- fixed Azure DevOps PR creation not prepending `refs/heads/` to branch names, causing API errors
- fixed Azure DevOps URL construction to URL-encode repository names and query parameters with special characters
- fixed GPG key error message leaking truncated private key material in logs
- fixed GPG key reader failing on Base64-encoded keys with line wrapping (76-char wrapped output from `base64` CLI)
- fixed GPG key reader providing unhelpful error messages when key file is empty or in unexpected format

## [0.1.1] - 2026-03-06

### Changed

- upgraded Go dependencies to their latest versions

## [0.1.0] - 2026-03-06

### Added

- added "Exported for use by `autobump`/`autoupdate`" clarifying comments to all exported functions that have no callers within `gitforge` itself
- added `CloneRepo` to `GitOperations` for cloning remote repositories with multi-auth retry and adapter-based URL preparation
- added `CommitSigner` interface in `pkg/global/domain/entities/` for abstracting commit signing
- added `CommitSignerStub` test double
- added `GPGSigner` and `SSHSigner` structs in `pkg/signing/infrastructure/` implementing `CommitSigner`
- added `LoadConfig` to `pkg/config/infrastructure/` as the parent caller for the orphaned `DownloadFile`/`ReadData` infrastructure helpers
- added `ParseRemoteURL` and `ParsePullRequestURL` in `pkg/git/infrastructure/` to provide unified Git remote and PR URL parsing for all consumers (`autobump`, `autoupdate`, `code-guru`)
- added `ReadUserConfig` to `pkg/git/infrastructure/` as the parent caller for the orphaned `GetGlobalGitConfig`/`GetOptionFromConfig` git config helpers
- added `StageAll` helper to stage all changes in the worktree (go-git equivalent of `git add -A`)
- added `WorktreeIsClean` helper to check whether a worktree has uncommitted changes (go-git equivalent of `git status --porcelain`)
- added changelog processing: version calculation, entry deduplication, section management, entry insertion
- added composed provider interfaces: `ForgeProvider`, `FileAccessProvider`, `LocalGitAuthProvider`
- added comprehensive tests across all packages achieving 80%+ coverage using testify, BDD structure, and parallel execution
- added GPG signing utilities and SSH signing placeholder
- added local git operations: open, branch, commit, push (SSH/HTTPS), tag, remote detection
- added provider and discoverer registries with factory pattern support
- added shared `Controller` interface and `ControllerBind` struct for CLI controllers
- added shared `ProviderConfig`, `ResolveToken`, `FindConfigFile`, and `ValidateProviders` for configuration handling
- added shared `Repository`, `ServiceType`, `BranchStatus`, `LatestTag`, `PullRequest`, `PullRequestInput`, `BranchInput`, `File`, `FileChange` entities
- added SSH commit signing support using `ssh-keygen -Y sign` in `pkg/signing/infrastructure/ssh.go`
- added standalone `ResolveToken` package-level function in `pkg/config/domain/entities/` to allow consumers to resolve tokens without requiring a `ProviderConfig` instance
- added unified GitHub, GitLab, and Azure DevOps provider implementations with discovery, file access, PR creation, and local git auth
- added utility functions: `DownloadFile`, `StripUsernameFromURL`, version sorting

### Changed

- changed `CommitChanges` signature to accept `CommitSigner` interface instead of `*SigningOptions`
- changed the Go module dependencies to their latest versions
- distributed `support/` utilities into their consuming domains
- extracted `AdapterFinder` interface to its own file `adapter_finder.go`
- extracted test builders (`AdapterFinderStubBuilder`, `ForgeProviderStubBuilder`, `RepositoryDiscovererStubBuilder`) to `test/builders/`
- extracted test doubles to `test/doubles/` using builder pattern with `testkit` library
- moved `AdapterFinder` interface from `pkg/git/infrastructure/` to `pkg/git/domain/entities/`
- moved `FindConfigFile` to `pkg/config/domain/helpers/`
- moved `ProviderConfig` struct to `pkg/config/domain/entities/`
- moved `SortVersionsDescending` and `NormalizeVersion` to `pkg/global/domain/helpers/`
- moved changelog code from `pkg/changelog/domain/` into `pkg/changelog/domain/entities/`
- moved changelog domain logic to `pkg/changelog/domain/`
- moved config domain and infrastructure to `pkg/config/`
- moved config infrastructure helpers (`DownloadFile`, `ReadData`) to `pkg/config/infrastructure/helpers/`
- moved git config helpers (`GetGlobalGitConfig`, `GetOptionFromConfig`) to `pkg/git/infrastructure/helpers/gitconfig.go` (renamed from `config.go` for clarity)
- moved git operations to `pkg/git/infrastructure/`
- moved providers to `pkg/providers/infrastructure/`
- moved registry to `pkg/registry/infrastructure/`
- moved shared entities and interfaces to `pkg/forge/domain/`
- moved signing helpers (`SignSSHCommit`, `ReadSSHSigningKey`, GPG helpers) to `pkg/signing/infrastructure/helpers/`
- moved signing to `pkg/signing/infrastructure/`
- refactored `Changelog` from free functions to struct with methods, split into `changelog.go`, `changelog_processor.go`, `changelog_section.go`, `changelog_dedup.go`, `changelog_insert.go`
- refactored `ResolveToken` from free function to `ProviderConfig` method, added `Config` entity with `Validate()`
- refactored git operations from free functions to `GitOperations` struct with constructor injection, split into `operations.go`, `operations_branch.go`, `operations_commit.go`, `operations_push.go`, `operations_auth.go`, `operations_repo.go`
- refactored signing from free functions to `GPGSigner` and `SSHSigner` structs implementing `CommitSigner` interface
- removed global `adapterFinder` variable in favor of constructor injection via `GitOperations` struct
- renamed shared kernel from `pkg/forge/` to `pkg/global/` and split entities into `pkg/global/domain/entities/` with one entity per file
- reorganized project from flat `domain/` and `infrastructure/` to DDD bounded contexts under `pkg/`
- replaced raw struct literals in tests with `testkit` builders for consistent test data construction
- restored `pkg/config/domain/entities/provider_config.go` with `ResolveToken()` as a method on `ProviderConfig` (replacing the old free function)
- split `registry.go` into `provider_factory.go`, `discoverer_factory.go`, and `provider_registry.go`
- split each provider (`github.go`, `gitlab.go`, `azuredevops.go`) into `provider.go`, `provider_discovery.go`, `provider_pull_request.go`, `provider_file_access.go` (plus `provider_http.go` and `provider_url.go` for Azure DevOps)
- split multi-entity files into individual files following one-entity-per-file principle

### Fixed

- changed default initial release version from `1.0.0` to `0.1.0` when the changelog contains no released versions
- filled in the empty `pkg/config/domain/entities/config.go` placeholder with `Config` struct, `NewConfig()`, and `Validate()` method
- filled in the empty `pkg/config/domain/helpers/finder.go` placeholder with `FindConfigFile()` function
- fixed `CommitChanges` to set the `Author` field in `CommitOptions` using the already-passed `name`/`email` parameters, preventing "author field is required" errors in CI environments without global git config
- fixed `config_test.go` directly testing the `FindConfigFile` helper function; removed helper tests to respect the rule that helpers are tested through their callers
- fixed `gochecknoglobals` findings by converting global variables to functions in URL parser
- fixed `testifylint` findings by using `require.Error` instead of `assert.Error` for fatal error checks in URL parser tests
- fixed `tparallel` findings by adding `t.Parallel()` to all subtests in URL parser tests
- fixed GitLab provider compilation errors caused by invalid `new(value)` usage; replaced with `&variable` address-of expressions

### Removed

- removed broken `.gitleaks.toml` allowlist that caused gitleaks to reject the config on newer versions
- removed direct utility tests (`fileutils_test.go`, `versions_test.go`) in favor of indirect testing through callers
- removed unused `ReadLines` and `WriteLines` utilities from `pkg/global/domain/fileutils.go`
