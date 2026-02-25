package infrastructure

import (
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	log "github.com/sirupsen/logrus"

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

// GetRemoteRepoURL returns the URL of the remote repository.
// Exported for use by autobump (github.com/rios0rios0/autobump).
func GetRemoteRepoURL(repo *git.Repository) (string, error) {
	remote, err := repo.Remote("origin")
	if err != nil {
		return "", fmt.Errorf("could not get remote: %w", err)
	}

	if len(remote.Config().URLs) > 0 {
		return remote.Config().URLs[0], nil
	}

	return "", ErrNoRemoteURL
}

// GetAmountCommits returns the number of commits in the repository.
func GetAmountCommits(repo *git.Repository) (int, error) {
	commits, err := repo.Log(&git.LogOptions{})
	if err != nil {
		return 0, fmt.Errorf("could not get commits: %w", err)
	}

	amountCommits := 0
	err = commits.ForEach(func(_ *object.Commit) error {
		amountCommits++
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("could not count commits: %w", err)
	}

	return amountCommits, nil
}

// GetLatestTag finds the latest tag in the Git history.
// Exported for use by autobump (github.com/rios0rios0/autobump).
func GetLatestTag(repo *git.Repository) (*globalEntities.LatestTag, error) {
	tags, err := repo.Tags()
	if err != nil {
		return nil, fmt.Errorf("could not get tags: %w", err)
	}

	var latestTag *plumbing.Reference
	_ = tags.ForEach(func(tag *plumbing.Reference) error {
		latestTag = tag
		return nil
	})

	numCommits, commitErr := GetAmountCommits(repo)
	if commitErr != nil {
		log.Warnf("Could not count commits, assuming 0: %v", commitErr)
	}
	if latestTag == nil {
		if numCommits >= MaxAcceptableInitialCommits {
			log.Warnf("No tags found in Git history, falling back to '%s'", DefaultGitTag)
			version, _ := semver.NewVersion(DefaultGitTag)
			return &globalEntities.LatestTag{
				Tag:  version,
				Date: time.Now(),
			}, nil
		}

		log.Warn("This project seems be a new project, the CHANGELOG should be committed by itself.")
		return nil, ErrNoTagsFound
	}

	commit, err := repo.CommitObject(latestTag.Hash())
	if err != nil {
		return nil, fmt.Errorf("could not get commit for tag: %w", err)
	}
	latestTagDate := commit.Committer.When

	version, _ := semver.NewVersion(latestTag.Name().Short())
	return &globalEntities.LatestTag{
		Tag:  version,
		Date: latestTagDate,
	}, nil
}
