package infrastructure

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	log "github.com/sirupsen/logrus"
)

// CheckBranchExists checks if a given Git branch exists (local or remote).
// Exported for use by autobump (github.com/rios0rios0/autobump).
func CheckBranchExists(repo *git.Repository, branchName string) (bool, error) {
	refs, err := repo.References()
	if err != nil {
		return false, fmt.Errorf("could not get repo references: %w", err)
	}

	branchExists := false
	remoteBranchName := "origin/" + branchName
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		refName := ref.Name().String()
		shortName := ref.Name().Short()

		if ref.Name().IsBranch() && shortName == branchName {
			branchExists = true
		}
		if strings.HasPrefix(refName, "refs/remotes/") && shortName == remoteBranchName {
			branchExists = true
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("could not check if branch exists: %w", err)
	}
	return branchExists, nil
}

// CreateAndSwitchBranch creates a new branch and switches to it.
// Exported for use by autobump (github.com/rios0rios0/autobump).
func CreateAndSwitchBranch(
	repo *git.Repository,
	workTree *git.Worktree,
	branchName string,
	hash plumbing.Hash,
) error {
	log.Infof("Creating and switching to new branch '%s'", branchName)
	ref := plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/"+branchName), hash)
	err := repo.Storer.SetReference(ref)
	if err != nil {
		return fmt.Errorf("could not create branch: %w", err)
	}

	// Force checkout is safe here because the new branch points to the same
	// commit as HEAD — no files need to change. This avoids go-git rejecting
	// the checkout due to index discrepancies (e.g. line-ending normalisation
	// after a native git clone) that would not block a real branch switch.
	return checkoutBranchWithForce(workTree, branchName, true)
}

// CheckoutBranch switches to the given branch.
// Exported for use by autobump (github.com/rios0rios0/autobump).
func CheckoutBranch(w *git.Worktree, branchName string) error {
	return checkoutBranchWithForce(w, branchName, false)
}

// checkoutBranchWithForce switches to the given branch, optionally forcing the
// checkout even when the worktree has unstaged changes. Force is safe when
// switching to a branch that points to the same commit (e.g. a newly created
// branch from HEAD), because no files need to change.
func checkoutBranchWithForce(w *git.Worktree, branchName string, force bool) error {
	log.Infof("Switching to branch '%s'", branchName)
	err := w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/heads/" + branchName),
		Force:  force,
	})
	if err != nil {
		return fmt.Errorf("could not checkout branch: %w", err)
	}
	return nil
}
