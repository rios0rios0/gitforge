package git

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	log "github.com/sirupsen/logrus"

	"github.com/rios0rios0/gitforge/domain/entities"
	domainRepos "github.com/rios0rios0/gitforge/domain/repositories"
)

// AdapterFinder provides adapter lookup capabilities without circular dependencies.
type AdapterFinder interface {
	GetAdapterByServiceType(serviceType entities.ServiceType) domainRepos.LocalGitAuthProvider
	GetAdapterByURL(url string) domainRepos.LocalGitAuthProvider
}

// adapterFinder is the package-level adapter finder, set by the application at startup.
var adapterFinder AdapterFinder //nolint:gochecknoglobals // required to break import cycle

// SetAdapterFinder sets the adapter finder used by git utilities.
func SetAdapterFinder(finder AdapterFinder) {
	adapterFinder = finder
}

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

// OpenRepo opens a git repository at the given path.
func OpenRepo(projectPath string) (*git.Repository, error) {
	log.Infof("Opening repository at %s", projectPath)
	repo, err := git.PlainOpen(projectPath)
	if err != nil {
		return nil, fmt.Errorf("could not open repository: %w", err)
	}
	return repo, nil
}

// CheckBranchExists checks if a given Git branch exists (local or remote).
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

	return CheckoutBranch(workTree, branchName)
}

// CheckoutBranch switches to the given branch.
func CheckoutBranch(w *git.Worktree, branchName string) error {
	log.Infof("Switching to branch '%s'", branchName)
	err := w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/heads/" + branchName),
	})
	if err != nil {
		return fmt.Errorf("could not checkout branch: %w", err)
	}
	return nil
}

// CommitChanges commits the changes in the given worktree with optional GPG signing.
func CommitChanges(
	workTree *git.Worktree,
	commitMessage string,
	signKey *openpgp.Entity,
	name string,
	email string,
) (plumbing.Hash, error) {
	log.Info("Committing changes")

	signoff := fmt.Sprintf("\n\nSigned-off-by: %s <%s>", name, email)
	commitMessage += signoff

	commit, err := workTree.Commit(commitMessage, &git.CommitOptions{SignKey: signKey})
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("could not commit changes: %w", err)
	}
	return commit, nil
}

// PushChangesSSH pushes the changes to the remote repository over SSH.
func PushChangesSSH(repo *git.Repository, refSpec config.RefSpec) error {
	log.Info("Pushing local changes to remote repository through SSH")
	err := repo.Push(&git.PushOptions{
		RefSpecs: []config.RefSpec{refSpec},
	})
	if err != nil {
		return fmt.Errorf("could not push changes to remote repository: %w", err)
	}
	return nil
}

// PushChangesHTTPS pushes the changes to the remote repository over HTTPS.
// It tries each authentication method returned by the adapter until one succeeds.
func PushChangesHTTPS(
	repo *git.Repository,
	username string,
	refSpec config.RefSpec,
) error {
	log.Info("Pushing local changes to remote repository through HTTPS")
	pushOptions := &git.PushOptions{
		RefSpecs:   []config.RefSpec{refSpec},
		RemoteName: "origin",
	}

	service, err := GetRemoteServiceType(repo)
	if err != nil {
		return err
	}
	authMethods, err := GetAuthMethods(service, username)
	if err != nil {
		return err
	}

	for _, auth := range authMethods {
		pushOptions.Auth = auth
		err = repo.Push(pushOptions)

		if err == nil {
			return nil
		}
	}

	if err != nil {
		return fmt.Errorf("could not push changes to remote repository: %w", err)
	}
	return nil
}

// GetAuthMethods returns the authentication methods for the given service type.
// It delegates to the appropriate adapter based on the service type.
func GetAuthMethods(
	service entities.ServiceType,
	username string,
) ([]transport.AuthMethod, error) {
	adapter := adapterFinder.GetAdapterByServiceType(service)
	if adapter == nil {
		log.Errorf("No authentication mechanism implemented for service type '%v'", service)
		return nil, ErrAuthNotImplemented
	}

	adapter.ConfigureTransport()

	authMethods := adapter.GetAuthMethods(username)

	if len(authMethods) == 0 {
		log.Error("No authentication credentials found for any authentication method")
		return nil, ErrNoAuthMethodFound
	}

	return authMethods, nil
}

// GetRemoteServiceType returns the type of the remote service (e.g. GitHub, GitLab).
func GetRemoteServiceType(repo *git.Repository) (entities.ServiceType, error) {
	cfg, err := repo.Config()
	if err != nil {
		return entities.UNKNOWN, fmt.Errorf("could not get repository config: %w", err)
	}

	var firstRemote string
	for _, remote := range cfg.Remotes {
		firstRemote = remote.URLs[0]
		break
	}

	return GetServiceTypeByURL(firstRemote), nil
}

// GetServiceTypeByURL returns the type of the remote service by URL.
func GetServiceTypeByURL(remoteURL string) entities.ServiceType {
	adapter := adapterFinder.GetAdapterByURL(remoteURL)
	if adapter != nil {
		return adapter.GetServiceType()
	}

	switch {
	case strings.Contains(remoteURL, "bitbucket.org"):
		return entities.BITBUCKET
	case strings.Contains(remoteURL, "git-codecommit"):
		return entities.CODECOMMIT
	default:
		return entities.UNKNOWN
	}
}

// GetRemoteRepoURL returns the URL of the remote repository.
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
func GetLatestTag(repo *git.Repository) (*entities.LatestTag, error) {
	tags, err := repo.Tags()
	if err != nil {
		return nil, fmt.Errorf("could not get tags: %w", err)
	}

	var latestTag *plumbing.Reference
	_ = tags.ForEach(func(tag *plumbing.Reference) error {
		latestTag = tag
		return nil
	})

	numCommits, _ := GetAmountCommits(repo)
	if latestTag == nil {
		if numCommits >= MaxAcceptableInitialCommits {
			log.Warnf("No tags found in Git history, falling back to '%s'", DefaultGitTag)
			version, _ := semver.NewVersion(DefaultGitTag)
			return &entities.LatestTag{
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
	return &entities.LatestTag{
		Tag:  version,
		Date: latestTagDate,
	}, nil
}
