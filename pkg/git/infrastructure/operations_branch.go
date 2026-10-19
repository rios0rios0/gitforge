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
// order and falling back to the default SSH agent (which covers public repositories
// over HTTPS too) when every explicit method fails. The fallback carries the host key
// callback installed via SetDefaultSSHHostKeyCallback, so a consumer that discovered
// known_hosts its own way gets the same verification here as in the explicit methods.
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

	refs, err := remote.List(&git.ListOptions{Auth: defaultSSHAuthForRemote(remote)})
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

// CreateAndSwitchBranch creates a new branch at hash and makes it the current branch.
// Exported for use by autobump (github.com/rios0rios0/autobump).
//
// When hash is the commit HEAD already points to -- what every caller passes -- the
// switch needs nothing but HEAD itself: the branch is created and HEAD is pointed at
// it, leaving the index and the working tree exactly as they are, as `git switch -c`
// does. A go-git checkout is not used for that, in either of its forms. The plain one
// rejects the switch when the index differs from the working tree for reasons that
// would not stop git, such as line-ending normalisation after a native clone; and the
// forced one resets the working tree hard, which discards local changes and deletes
// every untracked and ignored file -- in an operator's own checkout, their `.env`,
// build output and editor settings -- even though no file needs to change.
//
// A hash HEAD does not point to needs the working tree to change, so it gets a regular
// checkout, which refuses when local changes are in the way instead of discarding them.
func CreateAndSwitchBranch(
	repo *git.Repository,
	workTree *git.Worktree,
	branchName string,
	hash plumbing.Hash,
) error {
	log.Infof("Creating and switching to new branch '%s'", branchName)
	branch := plumbing.NewBranchReferenceName(branchName)
	if err := repo.Storer.SetReference(plumbing.NewHashReference(branch, hash)); err != nil {
		return fmt.Errorf("could not create branch: %w", err)
	}

	if head, err := repo.Head(); err != nil || head.Hash() != hash {
		return CheckoutBranch(workTree, branchName)
	}

	log.Infof("Switching to branch '%s'", branchName)
	if err := repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, branch)); err != nil {
		return fmt.Errorf("could not checkout branch: %w", err)
	}
	return nil
}

// CheckoutBranch switches to the given branch. It never forces the checkout: a forced
// go-git checkout resets the working tree hard, deleting untracked and ignored files.
// Exported for use by autobump (github.com/rios0rios0/autobump).
func CheckoutBranch(w *git.Worktree, branchName string) error {
	log.Infof("Switching to branch '%s'", branchName)
	err := w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branchName),
	})
	if err != nil {
		return fmt.Errorf("could not checkout branch: %w", err)
	}
	return nil
}
