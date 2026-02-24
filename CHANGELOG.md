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
- changed `CommitChanges` signature to accept `CommitSigner` interface instead of `*SigningOptions`
- reorganized project from flat `domain/` and `infrastructure/` to DDD bounded contexts under `pkg/`
- moved shared entities and interfaces to `pkg/forge/domain/`
- moved changelog domain logic to `pkg/changelog/domain/`
- moved config domain and infrastructure to `pkg/config/`
- moved git operations to `pkg/git/infrastructure/`
- moved providers to `pkg/providers/infrastructure/`
- moved registry to `pkg/registry/infrastructure/`
- moved signing to `pkg/signing/infrastructure/`
- distributed `support/` utilities into their consuming domains
- renamed shared kernel from `pkg/forge/` to `pkg/global/` and split entities into `pkg/global/domain/entities/` with one entity per file
- extracted test doubles to `test/doubles/` using builder pattern with `testkit` library
- split multi-entity files into individual files following one-entity-per-file principle
- split `registry.go` into `provider_factory.go`, `discoverer_factory.go`, and `provider_registry.go`
- extracted `AdapterFinder` interface to its own file `adapter_finder.go`
- moved `ProviderConfig` struct to `pkg/config/domain/entities/`
- refactored `Changelog` from free functions to struct with methods, split into `changelog.go`, `changelog_processor.go`, `changelog_section.go`, `changelog_dedup.go`, `changelog_insert.go`
- refactored git operations from free functions to `GitOperations` struct with constructor injection, split into `operations.go`, `operations_branch.go`, `operations_commit.go`, `operations_push.go`, `operations_auth.go`, `operations_repo.go`
- refactored signing from free functions to `GPGSigner` and `SSHSigner` structs implementing `CommitSigner` interface
- split each provider (`github.go`, `gitlab.go`, `azuredevops.go`) into `provider.go`, `provider_discovery.go`, `provider_pull_request.go`, `provider_file_access.go` (plus `provider_http.go` and `provider_url.go` for Azure DevOps)
- removed global `adapterFinder` variable in favor of constructor injection via `GitOperations` struct

### Added

- added SSH commit signing support using `ssh-keygen -Y sign` in `pkg/signing/infrastructure/ssh.go`
- added comprehensive tests across all packages achieving 80%+ coverage using testify, BDD structure, and parallel execution
- added GPG signing utilities and SSH signing placeholder
- added changelog processing: version calculation, entry deduplication, section management, entry insertion
- added composed provider interfaces: `ForgeProvider`, `FileAccessProvider`, `LocalGitAuthProvider`
- added local git operations: open, branch, commit, push (SSH/HTTPS), tag, remote detection
- added provider and discoverer registries with factory pattern support
- added shared `Controller` interface and `ControllerBind` struct for CLI controllers
- added shared `ProviderConfig`, `ResolveToken`, `FindConfigFile`, and `ValidateProviders` for configuration handling
- added shared `Repository`, `ServiceType`, `BranchStatus`, `LatestTag`, `PullRequest`, `PullRequestInput`, `BranchInput`, `File`, `FileChange` entities
- added unified GitHub, GitLab, and Azure DevOps provider implementations with discovery, file access, PR creation, and local git auth
- added utility functions: `DownloadFile`, `StripUsernameFromURL`, version sorting
- added `CommitSigner` interface in `pkg/global/domain/entities/` for abstracting commit signing
- added `GPGSigner` and `SSHSigner` structs in `pkg/signing/infrastructure/` implementing `CommitSigner`
- added `CommitSignerStub` test double

### Removed

- removed unused `ReadLines` and `WriteLines` utilities from `pkg/global/domain/fileutils.go`
- removed direct utility tests (`fileutils_test.go`, `versions_test.go`) in favor of indirect testing through callers
