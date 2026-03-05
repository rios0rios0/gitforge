package infrastructure

import (
	"errors"
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"
	log "github.com/sirupsen/logrus"
)

// CloneRepo clones a remote repository into the given directory.
// It prepares the clone URL via the adapter finder (if one matches the URL)
// and tries each provided authentication method until one succeeds.
func (o *GitOperations) CloneRepo(
	url, dir string,
	authMethods []transport.AuthMethod,
) (*git.Repository, error) {
	adapter := o.adapterFinder.GetAdapterByURL(url)

	cloneURL := url
	if adapter != nil {
		cloneURL = adapter.PrepareCloneURL(url)
		adapter.ConfigureTransport()
	}

	log.Infof("Cloning %s into %s", cloneURL, dir)
	cloneOptions := &git.CloneOptions{URL: cloneURL}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication methods provided for cloning %s", url)
	}

	var lastErr error
	for _, auth := range authMethods {
		cloneOptions.Auth = auth
		repo, cloneErr := git.PlainClone(dir, false, cloneOptions)
		if cloneErr == nil {
			log.Infof("Successfully cloned %s", url)
			return repo, nil
		}
		lastErr = cloneErr
	}

	if lastErr != nil {
		return nil, fmt.Errorf("failed to clone %s: %w", url, lastErr)
	}
	return nil, errors.New("failed to clone: no authentication methods attempted")
}
