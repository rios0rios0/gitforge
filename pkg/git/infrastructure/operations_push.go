package infrastructure

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
	log "github.com/sirupsen/logrus"
)

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
func (o *GitOperations) PushChangesHTTPS(
	repo *git.Repository,
	username string,
	refSpec config.RefSpec,
) error {
	log.Info("Pushing local changes to remote repository through HTTPS")
	pushOptions := &git.PushOptions{
		RefSpecs:   []config.RefSpec{refSpec},
		RemoteName: "origin",
	}

	service, err := o.GetRemoteServiceType(repo)
	if err != nil {
		return err
	}
	authMethods, err := o.GetAuthMethods(service, username)
	if err != nil {
		return err
	}

	var lastErr error
	for _, auth := range authMethods {
		pushOptions.Auth = auth
		pushErr := repo.Push(pushOptions)
		if pushErr == nil {
			return nil
		}
		lastErr = pushErr
	}
	if lastErr != nil {
		return fmt.Errorf("could not push changes to remote repository: %w", lastErr)
	}
	return errors.New("could not push changes to remote repository: no authentication methods attempted")
}

// PushWithTransportDetection pushes the given refSpec to the origin remote,
// auto-detecting the transport (SSH or HTTPS) from the remote URL.
//
// For SSH remotes (git@ or ssh://), the push uses system SSH keys and the
// authMethods parameter is ignored.
//
// For HTTPS/HTTP remotes, authMethods are tried in order until one succeeds.
// An empty authMethods slice returns an error for HTTPS remotes.
//
// Exported for use by autobump (github.com/rios0rios0/autobump) and
// autoupdate (github.com/rios0rios0/autoupdate).
func PushWithTransportDetection(
	repo *git.Repository,
	refSpec config.RefSpec,
	authMethods []transport.AuthMethod,
) error {
	remoteCfg, err := repo.Remote("origin")
	if err != nil {
		return fmt.Errorf("failed to get origin remote: %w", err)
	}

	urls := remoteCfg.Config().URLs
	if len(urls) == 0 {
		return errors.New("origin remote has no URLs configured")
	}
	remoteURL := urls[0]

	switch {
	case strings.HasPrefix(remoteURL, "git@") || strings.HasPrefix(remoteURL, "ssh://"):
		log.Info("Pushing to remote via SSH")
		return PushChangesSSH(repo, refSpec)

	case strings.HasPrefix(remoteURL, "https://"):
		log.Info("Pushing to remote via HTTPS")
		return pushWithAuthRetry(repo, refSpec, authMethods)

	case strings.HasPrefix(remoteURL, "http://"):
		log.Warn("Pushing over plaintext HTTP — credentials may be exposed; consider switching to HTTPS")
		return pushWithAuthRetry(repo, refSpec, authMethods)

	default:
		return fmt.Errorf("unsupported remote URL scheme: %s", remoteURL)
	}
}

// pushWithAuthRetry tries each auth method in sequence until one succeeds.
func pushWithAuthRetry(
	repo *git.Repository,
	refSpec config.RefSpec,
	authMethods []transport.AuthMethod,
) error {
	if len(authMethods) == 0 {
		return errors.New("no authentication methods provided for HTTPS push")
	}

	var lastErr error
	for _, method := range authMethods {
		lastErr = repo.Push(&git.PushOptions{
			RefSpecs:   []config.RefSpec{refSpec},
			RemoteName: "origin",
			Auth:       method,
		})
		if lastErr == nil {
			return nil
		}
		log.Debugf("Push attempt failed with auth method %T: %v", method, lastErr)
	}

	return fmt.Errorf("all push attempts failed, last error: %w", lastErr)
}
