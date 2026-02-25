package entities

// PullRequest represents a pull/merge request returned by a provider.
type PullRequest struct {
	ID     int
	Title  string
	URL    string
	Status string
}
