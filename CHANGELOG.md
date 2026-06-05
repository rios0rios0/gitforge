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

## [3.0.0] - 2026-06-05

### Added

- added `ReviewProvider.ReplyToThread(ctx, repo, prID, threadID, body)` so callers can append a comment to an EXISTING pull request thread (a nested reply) instead of opening a new same-line thread that fragments the discussion. The Azure DevOps provider POSTs to the thread's `/comments` collection with no `threadContext`, so the comment nests in the existing conversation rather than anchoring a brand-new thread; the GitHub provider replies to the thread's root review comment via `CreateCommentInReplyTo` (on GitHub `PullRequestComment.ThreadID` carries the root comment's id). Callers must source `threadID` from `PullRequestComment.ThreadID` (e.g. via `ListPullRequestComments`), not from the review id returned by `PostPullRequestThreadComment` on GitHub, which the reply API rejects with HTTP 422/404. Lets `code-guru`'s `@code-guru` re-review post its per-thread verdict nested under the author's reply — behaving like a human reviewer — instead of as a confusing parallel comment on the same line. Pinned by provider-level `httptest` tests asserting the ADO reply carries no `threadContext` and the GitHub reply carries `in_reply_to=<root comment id>`

### Changed

- **BREAKING CHANGE:** `ReviewProvider` gained a `ReplyToThread` method. Downstream types that implement `ReviewProvider` (custom providers, wrappers, mocks) MUST add this method to compile against this version

## [2.0.5] - 2026-06-03

### Changed

- changed the Go version to `1.26.4` and updated all module dependencies

## [2.0.4] - 2026-05-25

### Changed

- refreshed `.github/copilot-instructions.md` to add missing `FileAccessProvider` to Codeberg's interface list and align `go test` guidance with `CLAUDE.md`

## [2.0.3] - 2026-05-22

### Changed

- changed the Go module dependencies to their latest versions

## [2.0.2] - 2026-05-20

### Changed

- changed the Go module dependencies to their latest versions

## [2.0.1] - 2026-05-19

### Changed

- changed the Go module dependencies to their latest versions
- refreshed `CLAUDE.md` and `.github/copilot-instructions.md` to document Codeberg's `FileAccessProvider` implementation, new `ReviewProvider` methods (`SubmitPullRequestReview`, `ListPullRequestComments`, `UpdatePullRequestThreadStatus`, `GetPullRequestStatus`), new entity types (`PullRequestComment`, `ReviewVerdict`, `ReviewSubmission`, `CommentOption`, `MergeOption`), `PullRequestDetail.IsDraft`, config helpers (`ResolveTokenFromEnv`, `TokenEnvHint`), and Go 1.26.3

## [2.0.0] - 2026-05-08

### Added

- added `"rebaseMerge"` to the Azure DevOps `MergePullRequest` strategy map (previously only `"squash"`, `"merge"`, `"rebase"` were honoured). Surfaced live by an internal repo whose `Require a merge strategy` branch policy on `main` had `allowRebaseMerge: true` as the only allowed strategy — every `MergePullRequest` call against that repo was rejected with `GitPullRequestUpdateRejectedByPolicyException` because no string in the existing map produced the policy-compliant integer. The new entry maps to `4` (`adoMergeStrategyRebaseMerge`), the `GitPullRequestMergeStrategy` value documented in the [ADO REST update endpoint](https://learn.microsoft.com/en-us/rest/api/azure/devops/git/pull-requests/update). Pinned by a new `TestMapADOMergeStrategy` table covering every supported string plus the empty / unknown fallback to `squash`
- added `entities.MergeOption` functional-options type plus `entities.WithBypassPolicy(reason)` so callers can ask `MergePullRequest` to bypass branch policies on completion. The Azure DevOps provider sets `completionOptions.bypassPolicy=true` and forwards `bypassReason` so the action is recorded in the ADO audit trail; an empty reason falls back to the literal `"bypass"` because ADO rejects an empty audit string. The GitHub provider silently ignores the option (branch-protection bypass on GitHub is governed by the authenticated user's permission model, not a per-call flag, so callers wanting bypass on GitHub must mint a PAT with the right permissions). Surfaced live on `code-guru` where `Required reviewers` policies were rejecting trivial-detector auto-merges with `GitPullRequestUpdateRejectedByPolicyException` — the new option lets the bot self-merge per operator opt-in. Pinned by `TestResolveMergeOptions` covering the disabled default, the enabled-with-reason path, the empty-reason fallback, and a defensive nil-option entry, plus provider-level `httptest` rows asserting the Azure DevOps PATCH body carries `completionOptions.bypassPolicy`/`bypassReason` and the GitHub merge request still issues the normal `PUT /pulls/:n/merge` call when `WithBypassPolicy` is supplied

### Changed

- **BREAKING CHANGE:** `ReviewProvider.MergePullRequest` now accepts a variadic `...entities.MergeOption` parameter at the end of its signature. Call sites that pass no options keep the previous "respect policies" behaviour, but downstream types that implement `ReviewProvider` (custom providers, wrappers, mocks) MUST update their method set to compile against this version
- changed the Go version to `1.26.3` and bumped `github.com/go-git/go-git/v5` to `v5.19.0` (with the transitive `github.com/go-git/go-billy/v5` to `v5.9.0` and `golang.org/x/exp` to the `v0.0.0-20260410095643-746e56fc9e2f` snapshot)

### Fixed

- fixed `golangci-lint` failures (`goconst`, `nolintlint`) by extracting repeated string literals into package-level constants across the `changelog`, `azuredevops`, and `github` packages and removing an unused `//nolint:gosec` directive in `pkg/signing/infrastructure/helpers/gpg.go`

## [1.0.0] - 2026-05-03

### Added

- added `entities.CommentOption` functional-options type plus the `entities.WithThreadStatus(status)` helper so callers can post pull request comments and inline thread comments as `"fixed"`/`"closed"` (or any provider-specific status) instead of the default `"active"` — useful for informational annotations (start/success/failure markers) that should not show up as "needs attention" in Azure DevOps; backed by a new `entities.ResolveCommentOptions` helper used by the providers and the `entities.DefaultCommentStatus` constant documenting the historical default
- added `GetPullRequestStatus` to the `ReviewProvider` interface so callers can re-check whether a PR is still active before posting comments (Azure DevOps returns the raw `status` field; GitHub maps `state` plus `merged` into `open`/`closed`/`merged`)
- added `IsDraft` field to `PullRequestDetail` so consumers can apply their own draft-handling policy (e.g. skip drafts unless an opt-in flag is set). Both providers populate the field on every `ListOpenPullRequests` entry; the Azure DevOps provider no longer drops draft PRs client-side — the policy now lives in the consumer
- added `ListPullRequestComments` to the `ReviewProvider` interface so consumers can iterate every comment on a PR (both PR-wide "issue" comments and inline review comments) through a single unified `PullRequestComment` shape. GitHub walks both `GET /repos/.../issues/:n/comments` and `GET /repos/.../pulls/:n/comments`, paginates each, and concatenates; Azure DevOps walks the threads API and flattens each thread's `comments[]`, dropping `commentType: system` entries (vote / status notifications) so callers see only human + bot text. Designed to back two new code-guru gates: (1) the "has the bot already reviewed this PR?" check that prevents flooding on every push, (2) the comment-dedup pass that drops a proposed inline comment when the bot has already posted a near-identical one on the same file + line. Pinned by table-driven test rows in both providers covering issue+inline merging, `ThreadID` / `InReplyToID` propagation, and the ADO system-comment drop
- added `SubmitPullRequestReview` to the `ReviewProvider` interface so consumers can record native pull request reviews (Approved / Changes Requested / Waiting for Author / Comment) instead of only posting verdicts as free-form comments. GitHub maps the verdict to the `event` field on `POST /pulls/:n/reviews` (`APPROVE`/`REQUEST_CHANGES`/`COMMENT`), swallowing HTTP 422 self-review errors (matched against the GitHub error message) as a soft failure while surfacing other 422 validation failures as wrapped errors. Azure DevOps maps the verdict to the integer reviewer vote (`10`/`-10`/`-5`/`0`) on `PUT /pullrequests/:id/reviewers/:reviewerId`, auto-discovering the reviewer ID via `/_apis/connectionData` and caching it per organization via `sync.Once` so a single `Provider` reused across orgs resolves the correct identity for each. The accompanying `entities.ReviewVerdict` type, `entities.ReviewSubmission` struct, and verdict constants (`ReviewVerdictApprove`, `ReviewVerdictRequestChanges`, `ReviewVerdictWaitingForAuthor`, `ReviewVerdictComment`) describe the cross-provider contract.
- added `UpdatePullRequestThreadStatus` to the `ReviewProvider` interface for marking pull request threads as `fixed`/`closed`/etc. (Azure DevOps PATCHes the thread; GitHub returns `ErrThreadStatusUpdateUnsupported` until GraphQL `resolveReviewThread` is wired up)

### Changed

- **BREAKING CHANGE:** `ReviewProvider.ListOpenPullRequests` on the Azure DevOps provider now returns draft PRs alongside ready ones; consumers that previously relied on the client-side filter must inspect `PullRequestDetail.IsDraft` and skip them themselves.
- **BREAKING CHANGE:** `ReviewProvider.PostPullRequestComment` and `ReviewProvider.PostPullRequestThreadComment` now accept a variadic `...entities.CommentOption` parameter at the end of their signatures. Callers that pass no options keep the previous default (`"active"` thread status); callers wanting a closed informational annotation pass `entities.WithThreadStatus("closed")`. Implementers of `ReviewProvider` must update their method signatures even if they do not honour the new option (the GitHub provider, for example, silently ignores `WithThreadStatus` because the REST review API has no per-comment status field).
- **BREAKING CHANGE:** `ReviewProvider.PostPullRequestThreadComment` now returns `(int, error)` instead of `error`; the new integer is the thread ID (Azure DevOps) or review ID (GitHub) and can be passed to `UpdatePullRequestThreadStatus` to update the thread later. All callers must be updated to capture the thread ID from the return tuple.
- **BREAKING CHANGE:** `ReviewProvider` gained `ListPullRequestComments(ctx, repo, prID)`. All implementers of `ReviewProvider` must add the new method; the GitHub and Azure DevOps providers ship with implementations that unify PR-wide and inline comments into the `PullRequestComment` shape.
- **BREAKING CHANGE:** `ReviewProvider` gained `SubmitPullRequestReview(ctx, repo, prID, sub)`. All implementers of `ReviewProvider` must add the new method; the GitHub and Azure DevOps providers ship with native implementations.
- changed the Go module dependencies to their latest versions

### Fixed

- changed `ReviewVerdictWaitingForAuthor` on the GitHub provider to map to event `COMMENT` (was `REQUEST_CHANGES`) so the verdict surfaces as a soft "I have a comment, no formal vote" signal that does not block the PR — the same neutral-signal semantic Azure DevOps gives via vote `-5`. `REQUEST_CHANGES` would block the PR, which is too strong for a verdict that on ADO is explicitly a reviewer signal rather than a hard block. Existing test row updated and `TestSubmitPullRequestReviewSkipsEmptyComment` now covers both `Comment` and `WaitingForAuthor` empty-body short-circuits
- fixed `ListPullRequestComments` on Azure DevOps to follow the `X-Ms-Continuationtoken` header so PRs with enough discussion to spill onto a second threads page no longer return an incomplete comment set (was breaking both the "already reviewed" gate and the comment dedup), and corrected the GitHub provider's `ThreadID` to the thread root's comment ID (via the `in_reply_to_id` chain) instead of `pull_request_review_id` so unrelated inline threads from the same review submission are no longer merged into one bucket
- fixed `SubmitPullRequestReview` on the Azure DevOps provider so the reviewer-ID lookup against `/_apis/connectionData` uses `api-version=7.0-preview.1` (the endpoint is preview-only on ADO and rejected the package-wide `7.0` constant with `VssInvalidPreviewVersionException`). Without the suffix every native review submission failed with `failed to resolve reviewer ID: API error (status 400)` and the bot silently fell back to the text-only annotation. Captured live in dev pod logs at `2026-05-01T20:52Z`. Pinned by a new test row asserting the request URL carries the preview suffix
- fixed `SubmitPullRequestReview` on the GitHub provider so `ReviewVerdictRequestChanges` and `ReviewVerdictWaitingForAuthor` with an empty `Body` short-circuit with the new exported `ErrReviewBodyRequired` sentinel instead of triggering a GitHub HTTP 422; matches the existing `ReviewVerdictComment` empty-body skip and gives callers a deterministic error to match against
- fixed `SubmitPullRequestReview` so the Azure DevOps reviewer ID is cached per organization (a `Provider` reused across orgs no longer silently returns the first org's cached identity) and the GitHub 422 swallow only fires for the documented self-review message instead of masking every validation error, plus tightened the GitHub `captureSubmitReviewEvent` test helper to assert read/unmarshal errors and clarified the `entities.ReviewSubmission` docstring to acknowledge that providers may post the body as a regular pull request comment when the native review API does not support an attached body
- fixed Azure DevOps inline review threads being rendered with the warning "This file no longer exists in the latest pull request changes" by adding the `pullRequestThreadContext.iterationContext` and `pullRequestThreadContext.changeTrackingId` fields to the `POST .../threads` payload in `PostPullRequestThreadComment` (looked up via the `iterations` and `iterations/{id}/changes` ADO endpoints, with defensive fall-back that still posts the thread when either lookup fails); covered by new `httptest`-based test rows for the happy path, iteration-lookup failure, no-matching-change-entry, and leading-slash path normalisation

## [0.9.6] - 2026-04-29

### Changed

- changed the Go module dependencies to their latest versions

## [0.9.5] - 2026-04-28

### Changed

- refreshed `CLAUDE.md` and `.github/copilot-instructions.md` to document the Codeberg provider, `MirrorProvider` interface, and `ReviewProvider` additions (`GetPullRequestCheckStatus`, `MergePullRequest`)

## [0.9.4] - 2026-04-17

### Changed

- changed the Go module dependencies to their latest versions

## [0.9.3] - 2026-04-16

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
