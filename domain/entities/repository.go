package entities

import (
	"context"
	"time"

	"github.com/Masterminds/semver/v3"
)

// ServiceType represents the type of Git hosting service.
type ServiceType int

const (
	UNKNOWN ServiceType = iota
	GITHUB
	GITLAB
	AZUREDEVOPS
	BITBUCKET
	CODECOMMIT
)

// LatestTag holds information about the latest Git tag.
type LatestTag struct {
	Tag  *semver.Version
	Date time.Time
}

// BranchStatus represents the status of a branch with respect to pull requests.
type BranchStatus int

const (
	BranchCreated      BranchStatus = iota // Branch was newly created
	BranchExistsWithPR                     // Branch exists and PR exists - skip entirely
	BranchExistsNoPR                       // Branch exists but no PR - need to create PR
)

// Repository represents a Git repository on any hosting provider.
type Repository struct {
	ID            string
	Name          string
	Organization  string
	Project       string // Used by Azure DevOps; empty for GitHub/GitLab
	DefaultBranch string
	RemoteURL     string
	SSHURL        string
	ProviderName  string
}

// RepositoryDiscoverer can list repositories from a Git hosting provider.
type RepositoryDiscoverer interface {
	// Name returns the provider identifier (e.g. "github").
	Name() string
	// DiscoverRepositories lists all repositories in an organization or group.
	DiscoverRepositories(ctx context.Context, org string) ([]Repository, error)
}
