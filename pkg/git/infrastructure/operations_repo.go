package infrastructure

import (
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	log "github.com/sirupsen/logrus"

	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
)

// GetRemoteRepoURL returns the URL of the remote repository.
// Exported for use by autobump (github.com/rios0rios0/autobump).
func GetRemoteRepoURL(repo *git.Repository) (string, error) {
	remote, err := repo.Remote(originRemoteName)
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

// GetLatestTag finds the latest release tag in the Git history: the highest tag that
// reads as a semantic version, together with the date of the commit it points to.
// Exported for use by autobump (github.com/rios0rios0/autobump).
//
// The highest version, not the tag the repository lists last. Tags are listed in no
// particular version order -- `1.10.0` sorts before `1.9.0` as text -- and a tag that is
// not a version, such as the four-part tags a fork carries from its upstream, used to
// come back with a nil version that callers printed as `<nil>`. Tags that are not
// versions are ignored, so a repository holding nothing else is treated like one
// without tags.
func GetLatestTag(repo *git.Repository) (*globalEntities.LatestTag, error) {
	tags, err := repo.Tags()
	if err != nil {
		return nil, fmt.Errorf("could not get tags: %w", err)
	}

	var latestTag *plumbing.Reference
	var latestVersion *semver.Version
	_ = tags.ForEach(func(tag *plumbing.Reference) error {
		// A tag that does not parse as a version is skipped, not reported: it says
		// nothing about which release is the latest.
		version, parseErr := semver.NewVersion(tag.Name().Short())
		if parseErr == nil && (latestVersion == nil || version.GreaterThan(latestVersion)) {
			latestTag, latestVersion = tag, version
		}
		return nil
	})

	numCommits, commitErr := GetAmountCommits(repo)
	if commitErr != nil {
		log.Warnf("Could not count commits, assuming 0: %v", commitErr)
	}
	if latestTag == nil {
		if numCommits >= MaxAcceptableInitialCommits {
			log.Warnf("No version tags found in Git history, falling back to '%s'", DefaultGitTag)
			version, _ := semver.NewVersion(DefaultGitTag)
			return &globalEntities.LatestTag{
				Tag:  version,
				Date: time.Now(),
			}, nil
		}

		log.Warn("This project seems to be a new project; the CHANGELOG should be committed by itself.")
		return nil, ErrNoTagsFound
	}

	latestTagDate, err := tagCommitDate(repo, latestTag)
	if err != nil {
		return nil, err
	}

	return &globalEntities.LatestTag{
		Tag:  latestVersion,
		Date: latestTagDate,
	}, nil
}

// tagCommitDate returns the committer date of the commit a tag points to. An
// annotated tag points to a tag object rather than a commit, so it is peeled first.
func tagCommitDate(repo *git.Repository, tag *plumbing.Reference) (time.Time, error) {
	if annotated, err := repo.TagObject(tag.Hash()); err == nil {
		commit, commitErr := annotated.Commit()
		if commitErr != nil {
			return time.Time{}, fmt.Errorf("could not get commit for tag: %w", commitErr)
		}
		return commit.Committer.When, nil
	}

	commit, err := repo.CommitObject(tag.Hash())
	if err != nil {
		return time.Time{}, fmt.Errorf("could not get commit for tag: %w", err)
	}
	return commit.Committer.When, nil
}
