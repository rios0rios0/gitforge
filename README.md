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

- **Multi-Provider Support**: GitHub, GitLab, and Azure DevOps with a unified interface
- **Repository Discovery**: Automatically list repositories from organizations/groups
- **Pull Request Management**: Create PRs/MRs and check for existing ones
- **File Operations**: Read files, list trees, check file existence via API
- **Local Git Operations**: Branch, commit, push (SSH/HTTPS) via go-git
- **Commit Signing**: GPG signing with keyring export and passphrase support
- **Changelog Processing**: Full Keep-a-Changelog toolkit (version calculation, deduplication, entry insertion)
- **Configuration**: Token resolution (inline, `${ENV_VAR}`, file path), config file discovery
- **Registry Pattern**: Factory-based provider and discoverer registries

## Installation

```bash
go get github.com/rios0rios0/gitforge
```

## Architecture

```
gitforge/
├── domain/
│   ├── entities/         # Core business objects (Repository, PullRequest, etc.)
│   └── repositories/     # Provider interfaces (ForgeProvider, FileAccessProvider, etc.)
├── infrastructure/
│   ├── providers/        # GitHub, GitLab, Azure DevOps implementations
│   ├── git/              # Local git operations (go-git)
│   ├── signing/          # GPG and SSH commit signing
│   ├── config/           # YAML configuration loading
│   └── registry/         # Provider and discoverer registries
└── support/              # Utility functions (file I/O, HTTP, URL manipulation)
```

## Provider Interfaces

The library uses Go interface composition:

- **`ForgeProvider`**: Core interface (URL matching, discovery, PR creation, auth)
- **`FileAccessProvider`**: Extends with API-based file operations (read, list, tags, branch creation)
- **`LocalGitAuthProvider`**: Extends with local go-git authentication (service type, transport, auth methods)

Each concrete provider implements all three interfaces. Consumers type-assert to the level they need.

## License

See [LICENSE](LICENSE) file for details.
