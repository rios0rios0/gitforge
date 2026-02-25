package infrastructure

import (
	"errors"
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
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
