package infrastructure

import (
	"errors"
	"fmt"
	"net/url"

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

	log.Infof("Cloning %s into %s", sanitizeURL(cloneURL), dir)
	cloneOptions := &git.CloneOptions{URL: cloneURL}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication methods provided for cloning %s", sanitizeURL(cloneURL))
	}

	var lastErr error
	for _, auth := range authMethods {
		cloneOptions.Auth = auth
		repo, cloneErr := git.PlainClone(dir, false, cloneOptions)
		if cloneErr == nil {
			log.Infof("Successfully cloned %s", sanitizeURL(cloneURL))
			return repo, nil
		}
		lastErr = cloneErr
	}

	if lastErr != nil {
		return nil, fmt.Errorf("failed to clone %s: %w", sanitizeURL(cloneURL), lastErr)
	}
	return nil, errors.New("failed to clone: no authentication methods attempted")
}

// sanitizeURL strips embedded credentials from a URL before logging.
func sanitizeURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	parsed.User = nil
	return parsed.String()
}
