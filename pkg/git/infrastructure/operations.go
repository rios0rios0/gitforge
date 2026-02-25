package infrastructure

import (
	"errors"
	"fmt"

	"github.com/go-git/go-git/v5"
	gitEntities "github.com/rios0rios0/gitforge/pkg/git/domain/entities"
	log "github.com/sirupsen/logrus"
)

const (
	DefaultGitTag               = "0.1.0"
	MaxAcceptableInitialCommits = 5
)

var (
	ErrNoAuthMethodFound  = errors.New("no authentication method found")
	ErrAuthNotImplemented = errors.New("authentication method not implemented")
	ErrNoRemoteURL        = errors.New("no remote URL found for repository")
	ErrNoTagsFound        = errors.New("no tags found in Git history")
)

// GitOperations encapsulates git operations with an adapter finder for service type resolution.
type GitOperations struct {
	adapterFinder gitEntities.AdapterFinder
}

// NewGitOperations creates a new GitOperations with the given adapter finder.
func NewGitOperations(finder gitEntities.AdapterFinder) *GitOperations {
	return &GitOperations{adapterFinder: finder}
}

// OpenRepo opens a git repository at the given path.
// Exported for use by autobump (github.com/rios0rios0/autobump).
func OpenRepo(projectPath string) (*git.Repository, error) {
	log.Infof("Opening repository at %s", projectPath)
	repo, err := git.PlainOpen(projectPath)
	if err != nil {
		return nil, fmt.Errorf("could not open repository: %w", err)
	}
	return repo, nil
}
