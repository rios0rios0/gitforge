package entities

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
	IsFork        bool
	IsArchived    bool
	// IsDisabled reports a repository that cannot be used at the moment: disabled
	// on Azure DevOps (or under maintenance there), or disabled on GitHub. Providers
	// only report it; deciding to skip such a repository is the consumer's policy.
	IsDisabled bool
}
