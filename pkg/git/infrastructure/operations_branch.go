package infrastructure

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
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

// ListRemoteBranches returns the short names of every branch present on the origin
// remote. It queries the remote directly rather than reading remote-tracking
// references, so the result reflects the current server state even when the local
// clone is stale.
//
// Exported for use by autobump (github.com/rios0rios0/autobump) and
// autoupdate (github.com/rios0rios0/autoupdate).
func ListRemoteBranches(repo *git.Repository, authMethods []transport.AuthMethod) ([]string, error) {
	remote, err := repo.Remote(originRemoteName)
	if err != nil {
		return nil, fmt.Errorf("failed to get origin remote: %w", err)
	}

	refs, err := listRemoteRefs(remote, authMethods)
	if err != nil {
		return nil, err
	}

	branches := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref.Name().IsBranch() {
			branches = append(branches, ref.Name().Short())
		}
	}
	return branches, nil
}

// listRemoteRefs lists the remote references, trying each authentication method in
// order and falling back to an unauthenticated listing (which covers SSH agent and
// public repositories) when every explicit method fails.
func listRemoteRefs(
	remote *git.Remote,
	authMethods []transport.AuthMethod,
) ([]*plumbing.Reference, error) {
	var lastErr error
	for _, method := range authMethods {
		refs, err := remote.List(&git.ListOptions{Auth: method})
		if err == nil {
			return refs, nil
		}
		lastErr = err
		log.Debugf("Remote listing failed with auth method %T: %v", method, err)
	}

	refs, err := remote.List(&git.ListOptions{})
	if err == nil {
		return refs, nil
	}
	if lastErr == nil {
		lastErr = err
	}
	return nil, fmt.Errorf("could not list remote references: %w", lastErr)
}

// DeleteRemoteBranch deletes the given branch on the origin remote by pushing an
// empty source to its ref. go-git drops the matching remote-tracking reference as
// part of that push, so only the remote side is handled here; a local branch of the
// same name survives and must be removed with DeleteLocalBranch.
//
// Exported for use by autobump (github.com/rios0rios0/autobump) and
// autoupdate (github.com/rios0rios0/autoupdate).
func DeleteRemoteBranch(
	repo *git.Repository,
	branchName string,
	authMethods []transport.AuthMethod,
) error {
	log.Infof("Deleting remote branch '%s'", branchName)

	refSpec := config.RefSpec(":refs/heads/" + branchName)
	if err := PushWithTransportDetection(repo, refSpec, authMethods); err != nil {
		return fmt.Errorf("could not delete remote branch %q: %w", branchName, err)
	}

	return nil
}

// DeleteLocalBranch removes the local refs/heads/<branchName> reference when it exists.
//
// This is the necessary companion to DeleteRemoteBranch: deleting a branch on the
// remote leaves the local branch untouched, and CheckBranchExists reports a branch as
// existing when it finds either one. A caller that deletes a branch remotely and then
// asks whether it exists would still be told "yes", and so would never recreate it.
//
// A branch that is absent is not an error. The currently checked-out branch is never
// removed, because dropping HEAD's reference leaves the repository unusable.
//
// Exported for use by autobump (github.com/rios0rios0/autobump) and
// autoupdate (github.com/rios0rios0/autoupdate).
func DeleteLocalBranch(repo *git.Repository, branchName string) error {
	refName := plumbing.ReferenceName("refs/heads/" + branchName)

	if head, err := repo.Head(); err == nil && head.Name() == refName {
		log.Debugf("Not deleting local branch '%s': it is currently checked out", branchName)
		return nil
	}

	_, err := repo.Reference(refName, false)
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("could not read local branch %q: %w", branchName, err)
	}

	if err = repo.Storer.RemoveReference(refName); err != nil {
		return fmt.Errorf("could not delete local branch %q: %w", branchName, err)
	}

	log.Debugf("Deleted local branch '%s'", branchName)
	return nil
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
