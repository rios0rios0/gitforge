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
- **Credentials**: A token for a forge from its environment variables or, failing that, from an already signed-in `gh` or `az` CLI (see [Credentials](#credentials))
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
│   ├── credentials/      # Forge tokens from environment variables or the forge's own CLI (gh, az)
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

## Credentials

A consumer that has no token configured for a forge asks `pkg/credentials` for one. `NewDefaultCredentialResolver()` (`pkg/credentials/infrastructure`) tries the forge's environment variables first, so an explicitly exported token always wins, then the forge's own CLI, so someone who already ran `gh auth login` or `az login` does not have to export a second token:

| Forge | Environment variables, in order | CLI fallback | Sign in with |
|-------|---------------------------------|--------------|--------------|
| GitHub | `GH_TOKEN`, `GITHUB_TOKEN` | `gh auth token --hostname github.com` | `gh auth login` |
| Azure DevOps | `AZURE_DEVOPS_EXT_PAT`, `SYSTEM_ACCESSTOKEN` | `az account get-access-token --resource 499b84ac-1321-427f-aa17-267ca6975798` | `az login` |
| GitLab | `GITLAB_TOKEN`, `GL_TOKEN` | -- | -- |
| Codeberg | `CODEBERG_TOKEN` | -- | -- |

```go
credential, err := credentialInfra.NewDefaultCredentialResolver().Resolve(ctx, credentialEntities.CredentialRequest{
    ServiceType:  globalEntities.AZUREDEVOPS,
    Organization: "my-org", // Azure DevOps only: selects the organization's Microsoft Entra tenant
})
if err != nil {
    // errors.Is(err, credentialEntities.ErrNoCredential); the text names every source tried:
    // "no Azure DevOps credential available: AZURE_DEVOPS_EXT_PAT and SYSTEM_ACCESSTOKEN are not set; ..."
}
log.Infof("Using the %s token from the %s", globalEntities.AZUREDEVOPS.DisplayName(), credential.Source)
```

- **GitHub** asks `gh` for `github.com` by name: the GitHub provider only talks to `api.github.com`, and a bare `gh auth token` answers for `GH_HOST`, which may be a GitHub Enterprise server.
- **Azure DevOps** tokens are issued for the Azure DevOps resource, which Azure DevOps accepts wherever it accepts a personal access token. With an `Organization`, the organization's tenant is read from its public `connectionData` endpoint (no credential is sent) and passed as `--tenant`, because the signed-in account's default subscription can belong to another tenant.
- **GitLab** has no CLI fallback: `glab` has no command that prints a token.
- The CLI runs by the absolute path `PATH` lookup returns, from the user's home directory, with no standard input and a 30-second limit (`DefaultCLITimeout`). Its standard output (the token) never appears in an error; a failure reports its standard error and the command to sign in.
- **Send the token only to the forge's API.** The providers call fixed hosts (`api.github.com`, `dev.azure.com`), which is safe. Do not hand it to git-over-HTTPS authentication for a remote's own host: forges are recognised by a substring of the remote URL, so a remote such as `https://github.com.example.net/...` would receive the user's token.

Each source is its own `CredentialResolver` (`EnvCredentialResolver`, `CLICredentialResolver`), combined with `NewChainCredentialResolver`; `test/doubles` has stubs for the resolver, the CLI runner and the tenant lookup.

## License

See [LICENSE](LICENSE) file for details.
