package entities

// BranchStatus represents the status of a branch with respect to pull requests.
type BranchStatus int

const (
	BranchCreated      BranchStatus = iota // Branch was newly created
	BranchExistsWithPR                     // Branch exists and PR exists - skip entirely
	BranchExistsNoPR                       // Branch exists but no PR - need to create PR
)
