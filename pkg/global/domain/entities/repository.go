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
}
