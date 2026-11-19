# AGENTS.md

This file provides guidance to AI coding agents (Claude Code, Codex, Cursor, GitHub Copilot) when working with code in this repository.

## What This Is

gitforge is a shared Go library (no `main.go`, no CLI) providing unified abstractions for GitHub, GitLab, Azure DevOps, and Codeberg (Forgejo). Consumed by [autobump](https://github.com/rios0rios0/autobump), [autoupdate](https://github.com/rios0rios0/autoupdate), [code-guru](https://github.com/rios0rios0/code-guru), and dev-toolkit. Breaking changes to exported types affect all consumers.

## Commands

```bash
go build ./...          # Compile check (~2s)
make lint               # golangci-lint via pipeline scripts (~3-5s)
make test               # Full test suite (~3s clean, <1s cached)
make sast               # CodeQL, Semgrep, Hadolint, ShellCheck, Gitleaks (~1-3min)
go test ./...           # Quick check during development; skips //go:build unit files
```

Never run `golangci-lint`, `semgrep`, `gitleaks`, etc. directly — always use `make` targets. There is no `make build` or `make run`.

## Architecture

Clean Architecture with DDD bounded contexts under `pkg/`. Each context owns `domain/` (contracts) and `infrastructure/` (implementations). Dependencies point inward.

### Bounded Contexts

| Context | Purpose |
|---------|---------|
| `pkg/changelog/` | Version calculation, entry deduplication, section management |
| `pkg/config/` | YAML config loading, token resolution, validation |
| `pkg/credentials/` | Forge tokens from environment variables, then the forge's own CLI (`gh`, `az`) |
| `pkg/git/` | Local git operations (go-git wrapper), URL parsing, SSH/HTTPS push with auth retry |
| `pkg/global/` | Shared interfaces and value objects (the core contracts) |
| `pkg/providers/` | GitHub, GitLab, Azure DevOps, Codeberg (Forgejo) implementations |
| `pkg/registry/` | Provider factory, adapter lookup, service discovery |
| `pkg/signing/` | GPG and SSH commit signing |

### Provider Interface Hierarchy

`ForgeProvider` (base) is extended by `FileAccessProvider`, `ReviewProvider`, `LocalGitAuthProvider`, and `MirrorProvider`. GitHub and Azure DevOps implement `ForgeProvider`, `FileAccessProvider`, `ReviewProvider`, and `LocalGitAuthProvider`. GitLab implements `ForgeProvider`, `FileAccessProvider`, and `LocalGitAuthProvider` — it does **not** implement `ReviewProvider` (no `pkg/providers/infrastructure/gitlab/provider_review.go`). Codeberg implements `ForgeProvider`, `FileAccessProvider`, `LocalGitAuthProvider`, and `MirrorProvider`. Consumers type-assert to the interface level they need.

Two optional single-method interfaces sit beside the hierarchy: `OpenPullRequestFinder` (GitHub, Azure DevOps) and `OrganizationRepositoryLister` (Azure DevOps). New capabilities are added this way, never as a method on `ForgeProvider`, because consumers ship their own implementations of it (autoupdate's provider doubles and its `ProviderRepository` alias) and a new method breaks their build at the next bump. `DiscoverRepositories` on Azure DevOps stays best-effort — it logs and skips a project it cannot list — because autoupdate abandons an organization on any discovery error and autobump skips it; a consumer that must not mistake a partial listing for a complete one uses `ListOrganizationRepositories`.

### Key Patterns

- **Factory + Registry**: `ProviderRegistry` creates providers by name/token and resolves adapters by URL or `ServiceType`
- **Adapter**: `GitOperations` receives `AdapterFinder` (implemented by `ProviderRegistry`) to decouple auth resolution
- **Constructor injection**: No DI framework; dependencies passed via constructors
- **Never force a go-git checkout**: `Force: true` resets the working tree hard, which discards local changes and deletes every untracked *and ignored* file (`.env`, build output, editor settings), even when the target commit is HEAD's and `Status` reports the tree clean. The consumers run on operators' own checkouts (`autobump .`, `autoupdate .`), so `CreateAndSwitchBranch` switches to a branch at HEAD's commit by pointing HEAD at it, as `git switch -c` does, and anything else goes through the plain `CheckoutBranch`, which refuses rather than discards. Two traps around that: compare HEAD *before* writing the branch (when the branch is the current one, writing it first makes HEAD resolve to the new commit, so the move passes for "nothing to change"), and know that go-git points HEAD at the target branch *before* it refuses over local changes, which is why `CreateAndSwitchBranch` puts HEAD and the branch back after a refusal
- **Latest tag means highest version**: `GetLatestTag` returns the highest tag that parses as a semantic version, ignoring the rest, and peels annotated tags for the commit date. Tag listing order is not version order (`1.10.0` sorts before `1.9.0`), and a non-version tag used to come back with a nil version that callers printed as `<nil>`
- **Default SSH host key hook**: `SetDefaultSSHHostKeyCallback` (`pkg/git/infrastructure/operations_ssh.go`) hands a known_hosts callback to the default SSH agent fallback behind `PushChangesSSH` and remote reference listing, which otherwise verifies hosts with go-git's own `SSH_KNOWN_HOSTS`/`~/.ssh/known_hosts` lookup alone. `InstallSSHTransport` installs gitforge's own discovery there
- **Credential chain**: `NewDefaultCredentialResolver` (`pkg/credentials/infrastructure`) is the environment variables of `configHelpers.TokenEnvVars` (the same table and order as `ResolveTokenFromEnv`), then the forge's CLI from `providerCLIs`: `gh auth token --hostname github.com` (never a bare `gh auth token`, which answers for `GH_HOST`), and `az account get-access-token` for the Azure DevOps resource, with `--tenant` from the organization's anonymous `connectionData` response when the request names an `Organization`. GitLab's entry is empty because `glab` cannot print a token, and `EnvCredentialResolver` declines a GitLab token while `GITLAB_HOST`/`GL_HOST` names a server other than `gitlab.com`, the only host the GitLab provider calls. `ExecCLIRunner` runs the absolute path from the home directory (the filesystem root when `HOME` is unset or missing, never `""`, which would be the consumer's checkout and puts it on a pip-installed `az`'s module path) with no stdin, kills the whole process group on timeout (`exec_cli_runner_unix.go`), accepts output whose only fault is `exec.ErrWaitDelay` after a clean exit, and its errors carry stderr only, because stdout is the token; the resolver also refuses output holding whitespace and a tenant that is not a GUID, since the tenant becomes an `az` argument. Every failure matches `ErrNoCredential`, and the chain's error lists each source's reason. Consumers call it only when their own configured token is empty, and send the result only to provider APIs, never to git-over-HTTPS auth for a remote's host (forge detection is a URL substring match)
- **SSH client configuration**: `InstallSSHTransport` (`ssh_transport.go`) installs `sshTransport` for the `ssh` scheme with `client.InstallProtocol`, sets `gitssh.DefaultSSHConfig = nil` so an alias is resolved once, and installs `SSHHostKeyCallback`. It is `InstallSSHTransportWith(SSHTransportOptions{})`: `SSHTransportOptions` carries what only a consumer knows, so far `MissingKeyRemedy`, which `warnNoKeyOffered` appends after `, or ` to the warning it logs once per alias when `IdentitiesOnly` leaves the alias no key to offer (`configure it as ssh_key_path` would name AutoBump's key file setting); when it is empty the warning ends at `add that key to the SSH agent`. Add the next consumer setting as a field there, never as a parameter of `InstallSSHTransport` or `NewSSHTransport`, whose signatures consumers compile against. The transport sits under every go-git SSH connection, the fallback included, and per connection resolves the host alias through ssh_config (`ssh_config.go`, read as one OpenSSH configuration; a Windows profile's file only under WSL when git runs `ssh.exe`, `ssh_wsl.go`), narrows an agent auth method to the keys `IdentityFile` names in OpenSSH's `pubkey_prepare` order (`ssh_identities.go`), and sets `HostKeyCallbackHelper.HostKeyAlgorithms` to the types `knownHostsDB` records (`ssh_known_hosts.go`), because x/crypto asks for ECDSA before Ed25519. It pins types only on a method whose callback was given to it, since go-git fills an empty one from its own known_hosts; consumers give their methods `KnownHostsCallback` so the pinned types are ones it can verify. Agent signers (`ssh_agent.go`) dial per listing and per signature, because x/crypto signs after the `PublicKeysCallback` returns; a listing whose connection drops before the agent answers is retried, a signature never is. The loopback server in `ssh_fixtures_test.go` exercises the whole chain without a network

## Testing

All tests use BDD structure (`// given` / `// when` / `// then`), `t.Parallel()`, and `testify` assertions. Some test files carry a `//go:build unit` tag (newer tests adopted it; not all have been migrated), so a bare `go test ./...` silently skips them — `make test` is the canonical command that runs the full suite. Test doubles live in `test/doubles/` (stubs) and `test/builders/` (builder pattern).

**Provider test patterns:**
- GitHub/GitLab: override SDK `BaseURL` → `httptest.Server`
- Azure DevOps: `redirectTransport` rewrites `dev.azure.com` URLs → `httptest.Server`
- Provider internal tests use the internal package (not `_test` suffix) to access unexported fields

**Parallelism exceptions:** tests using `t.Setenv`, `t.Chdir`, or mutating process-wide state (e.g., `SetDefaultSSHHostKeyCallback`) must NOT call `t.Parallel()`.

## Validation After Changes

1. `go build ./...` — zero errors
2. `make lint` — zero issues
3. `make test` — all pass
4. When changing exported types: verify the consumer projects still compile

<!-- chlog:start -->
## Changelog (chlog) — MANDATORY

If the repository you are working in uses chlog (a `.chlog.yaml` or `.chlog.yml`
config file, or a `.changes/` directory, exists at the project root), the
following is binding and ALWAYS applies: whenever you make ANY change, you MUST
create a changelog fragment as part of the same change — automatically, without
being asked, before committing.

- Do NOT edit CHANGELOG.md directly; it is generated from fragments.
- Create the fragment with:
  `chlog new --kind <Kind> --body '<past-tense description>'`
- Write an apostrophe inside the single-quoted body as `'\''`.
- Valid kinds: Added, Changed, Deprecated, Removed, Fixed, Security
- Choose the kind that best matches the change (e.g., new feature → Added,
  bug fix → Fixed, behavior change → Changed, removal → Removed, security fix → Security).
- If the change is backward-INCOMPATIBLE with the public API (a breaking
  change), you MUST add the `--breaking` flag:
  `chlog new --kind <Kind> --breaking --body '<past-tense description>'`.
  This is the ONLY thing that triggers a major version bump — the kind alone
  never does (per SemVer, major = incompatible change). When unsure whether a
  change breaks compatibility, ask the user instead of guessing.
- Fragments are YAML files in `.changes/unreleased/`; stage them with your commit.
- `chlog check` fails the build when a fragment is missing — never skip it.
<!-- chlog:end -->
