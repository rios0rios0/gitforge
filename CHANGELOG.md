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
- changed the Go module dependencies to their latest versions
- changed the Go module dependencies to their latest versions

### Added

- added comprehensive tests across all packages achieving 83%+ coverage using testify, BDD structure, and parallel execution
- added GPG signing utilities and SSH signing placeholder
- added changelog processing: version calculation, entry deduplication, section management, entry insertion
- added composed provider interfaces: `ForgeProvider`, `FileAccessProvider`, `LocalGitAuthProvider`
- added local git operations: open, branch, commit, push (SSH/HTTPS), tag, remote detection
- added provider and discoverer registries with factory pattern support
- added shared `Controller` interface and `ControllerBind` struct for CLI controllers
- added shared `ProviderConfig`, `ResolveToken`, `FindConfigFile`, and `ValidateProviders` for configuration handling
- added shared `Repository`, `ServiceType`, `BranchStatus`, `LatestTag`, `PullRequest`, `PullRequestInput`, `BranchInput`, `File`, `FileChange` entities
- added unified GitHub, GitLab, and Azure DevOps provider implementations with discovery, file access, PR creation, and local git auth
- added utility functions: `ReadLines`, `WriteLines`, `DownloadFile`, `StripUsernameFromURL`, version sorting
