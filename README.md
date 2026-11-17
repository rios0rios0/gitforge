<h1 align="center">gitforge</h1>
<p align="center">
    <a href="https://github.com/rios0rios0/gitforge/releases/latest">
        <img src="https://img.shields.io/github/release/rios0rios0/gitforge.svg?style=for-the-badge&logo=github" alt="Latest Release"/></a>
    <a href="https://github.com/rios0rios0/gitforge/blob/main/LICENSE">
        <img src="https://img.shields.io/github/license/rios0rios0/gitforge.svg?style=for-the-badge&logo=github" alt="License"/></a>
    <a href="https://github.com/rios0rios0/gitforge/actions/workflows/default.yaml">
        <img src="https://img.shields.io/github/actions/workflow/status/rios0rios0/gitforge/default.yaml?branch=main&style=for-the-badge&logo=github" alt="Build Status"/></a>
    <a href="https://sonarcloud.io/summary/overall?id=rios0rios0_gitforge">
        <img src="https://img.shields.io/sonar/coverage/rios0rios0_gitforge?server=https%3A%2F%2Fsonarcloud.io&style=for-the-badge&logo=sonarqubecloud" alt="Coverage"/></a>
    <a href="https://sonarcloud.io/summary/overall?id=rios0rios0_gitforge">
        <img src="https://img.shields.io/sonar/quality_gate/rios0rios0_gitforge?server=https%3A%2F%2Fsonarcloud.io&style=for-the-badge&logo=sonarqubecloud" alt="Quality Gate"/></a>
    <a href="https://www.bestpractices.dev/projects/11944">
        <img src="https://img.shields.io/cii/level/11944?style=for-the-badge&logo=opensourceinitiative" alt="OpenSSF Best Practices"/></a>
</p>

A shared Go library providing common abstractions for working with Git hosting platforms (forges). Used by [autobump](https://github.com/rios0rios0/autobump) and [autoupdate](https://github.com/rios0rios0/autoupdate).

## Features

- **Multi-Provider Support**: GitHub, GitLab, Azure DevOps, and Codeberg (Forgejo) with a unified interface
- **Repository Discovery**: Automatically list repositories from organizations/groups
- **Pull Request Management**: Create PRs/MRs and check for existing ones
- **File Operations**: Read files, list trees, check file existence via API
- **Local Git Operations**: Branch, commit, push (SSH/HTTPS) via go-git
- **SSH Client Configuration**: SSH connections that honour `ssh_config` host aliases, `known_hosts` and SSH agent keys the way the `ssh` binary does (see [SSH client configuration](#ssh-client-configuration))
- **Commit Signing**: GPG signing with keyring export and passphrase support, and SSH signing via `ssh-keygen`
- **Changelog Processing**: Full Keep-a-Changelog toolkit (version calculation, deduplication, entry insertion)
- **Configuration**: Token resolution (inline, `${ENV_VAR}`, file path), config file discovery
- **Registry Pattern**: Factory-based provider and discoverer registries

## Installation

```bash
go get github.com/rios0rios0/gitforge/v4
```

Import the packages from `github.com/rios0rios0/gitforge/v4/...`. Go requires a module released at v2 or later to carry its major version in the module path, so the path changes with every major release.

## Architecture

Bounded contexts live under `pkg/`, each owning its own `domain/` and `infrastructure/` packages:

```
gitforge/
├── pkg/
│   ├── changelog/        # Keep-a-Changelog processing (version calculation, deduplication, insertion)
│   ├── config/           # YAML configuration loading, token resolution, config file discovery
│   ├── git/              # Local git operations (go-git), URL parsing, SSH/HTTPS push
│   ├── global/           # Shared interfaces (ForgeProvider, FileAccessProvider, etc.) and entities
│   ├── providers/        # GitHub, GitLab, Azure DevOps, Codeberg implementations
│   ├── registry/         # Provider and discoverer registries
│   └── signing/          # GPG and SSH commit signing
└── test/                 # Shared test doubles and builders
```

## Provider Interfaces

The library uses Go interface composition:

- **`ForgeProvider`**: Core interface (URL matching, discovery, PR creation, auth)
- **`FileAccessProvider`**: Extends with API-based file operations (read, list, tags, branch creation)
- **`LocalGitAuthProvider`**: Extends with local go-git authentication (service type, transport, auth methods)
- **`ReviewProvider`**: Extends with pull request review operations (diffs, comments, threads, merge, review submission)
- **`MirrorProvider`**: Extends with repository migration/mirroring

Each concrete provider implements the first three interfaces. GitHub and Azure DevOps also implement `ReviewProvider`; Codeberg also implements `MirrorProvider`. Consumers type-assert to the level they need.

Two optional single-method interfaces sit beside that hierarchy, also reached by type assertion:

- **`OpenPullRequestFinder`** (GitHub, Azure DevOps): returns the open pull request of a source branch, where `PullRequestExists` only reports that one exists
- **`OrganizationRepositoryLister`** (Azure DevOps): lists a whole organization in one strict call and fails instead of skipping a part of it, where `DiscoverRepositories` stays best-effort

Discovered repositories report `IsFork`, `IsArchived` and `IsDisabled` (disabled, or under maintenance on Azure DevOps). Providers never filter on them; skipping such repositories is the consumer's policy.

## SSH Client Configuration

go-git is a pure-Go SSH client and never runs the `ssh` binary, so the configuration `ssh` would read does not apply to its connections on its own. A consumer calls `InstallSSHTransport()` (`pkg/git/infrastructure`) once at startup, and every SSH push, clone and remote listing then behaves the way `git` does on that machine:

- **Host aliases.** The `HostName` and `Port` an alias names in `~/.ssh/config` or `/etc/ssh/ssh_config` are dialed, read as one OpenSSH configuration: the first file that sets a keyword wins, and `IdentityFile` accumulates. Under WSL, when git runs Windows' `ssh.exe` (through `GIT_SSH_COMMAND`, `core.sshCommand`, `GIT_SSH`, or an `ssh` on `PATH` that is or wraps it), the Windows profile's `.ssh/config` is read too.
- **Keys.** An agent auth method offers the keys an alias's `IdentityFile` names first, the agent's others unless `IdentitiesOnly` is set, and unencrypted key files the agent does not hold last, as OpenSSH does. When `IdentitiesOnly` leaves an alias no key to offer, the transport warns once for that alias, and the warning ends `add that key to the SSH agent`.
- **Host keys.** Hosts are verified against `SSH_KNOWN_HOSTS`, or `~/.ssh/known_hosts`, `/etc/ssh/ssh_known_hosts` and, under WSL, the Windows profiles' `known_hosts`. The server is asked only for the host key types those files record, so a host recorded with its Ed25519 key is not failed with `knownhosts: key mismatch` because Go's SSH library asked for ECDSA first.

A consumer with a way of its own to provide a key calls `InstallSSHTransportWith(SSHTransportOptions{MissingKeyRemedy: ...})` instead, and that warning offers it as an alternative, after `, or `: `MissingKeyRemedy: "configure it as ssh_key_path"` makes it end `add that key to the SSH agent, or configure it as ssh_key_path`. With the zero `SSHTransportOptions`, `InstallSSHTransportWith` is `InstallSSHTransport()`.

Building blocks for a consumer's own auth methods:

- **`NewSSHAgentAuthFromSocket(path)`**: an agent auth method for any agent socket, which lists the keys and signs over connections of its own, retrying a listing a relay dropped.
- **`DetectSSHAgentSockets()`**: the agent sockets that exist (`SSH_AUTH_SOCK`, then 1Password's `~/.1password/agent.sock`).
- **`KnownHostsCallback()`**: the host key callback auth methods should carry, as the transport reads the host key types from the same files.
- **`RunningUnderWSL()`** and **`SSHConfigSearchDescription()`**: for a consumer's own error messages.

## License

See [LICENSE](LICENSE) file for details.
