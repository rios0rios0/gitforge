package infrastructure

import (
	"fmt"

	"github.com/go-git/go-git/v5"
	log "github.com/sirupsen/logrus"
)

// WorktreeIsClean returns true when the working tree has no uncommitted changes
// (staged or unstaged). This is the go-git equivalent of `git status --porcelain`
// returning empty output.
// Exported for use by autoupdate (github.com/rios0rios0/autoupdate).
func WorktreeIsClean(workTree *git.Worktree) (bool, error) {
	status, err := workTree.Status()
	if err != nil {
		return false, fmt.Errorf("could not get worktree status: %w", err)
	}

	return status.IsClean(), nil
}

// StageAll adds all changes in the working tree to the index, equivalent to
// `git add -A`. It stages new, modified, and deleted files.
// Exported for use by autoupdate (github.com/rios0rios0/autoupdate).
func StageAll(workTree *git.Worktree) error {
	log.Info("Staging all changes")

	err := workTree.AddWithOptions(&git.AddOptions{All: true})
	if err != nil {
		return fmt.Errorf("could not stage all changes: %w", err)
	}

	return nil
}
