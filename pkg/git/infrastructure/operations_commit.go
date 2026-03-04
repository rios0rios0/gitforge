package infrastructure

import (
	"context"
	"fmt"
	"io"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	log "github.com/sirupsen/logrus"

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

// CommitChanges commits the changes in the given worktree with optional signing.
// The repo parameter is required when using a CommitSigner that produces post-commit signatures (e.g. SSH).
// Exported for use by autobump (github.com/rios0rios0/autobump).
func CommitChanges(
	repo *git.Repository,
	workTree *git.Worktree,
	commitMessage string,
	signer globalEntities.CommitSigner,
	name string,
	email string,
) (plumbing.Hash, error) {
	log.Info("Committing changes")

	signoff := fmt.Sprintf("\n\nSigned-off-by: %s <%s>", name, email)
	commitMessage += signoff

	hash, err := workTree.Commit(commitMessage, &git.CommitOptions{})
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("could not commit changes: %w", err)
	}

	if signer != nil {
		hash, err = applySignature(repo, hash, signer)
		if err != nil {
			return plumbing.ZeroHash, err
		}
	}

	return hash, nil
}

// applySignature reads an unsigned commit, signs it with the provided signer, and stores the signed version.
func applySignature(
	repo *git.Repository,
	hash plumbing.Hash,
	signer globalEntities.CommitSigner,
) (plumbing.Hash, error) {
	commitObj, err := repo.CommitObject(hash)
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("could not read commit for signing: %w", err)
	}

	unsignedObj := repo.Storer.NewEncodedObject()
	if err = commitObj.Encode(unsignedObj); err != nil {
		return plumbing.ZeroHash, fmt.Errorf("could not encode commit for signing: %w", err)
	}

	reader, err := unsignedObj.Reader()
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("could not read encoded commit: %w", err)
	}
	defer func() {
		if cerr := reader.Close(); cerr != nil {
			log.WithError(cerr).Warn("failed to close commit reader after signing")
		}
	}()

	content, err := io.ReadAll(reader)
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("could not read commit content: %w", err)
	}

	signature, err := signer.Sign(context.Background(), content)
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("commit signing failed: %w", err)
	}

	commitObj.PGPSignature = signature
	signedObj := repo.Storer.NewEncodedObject()
	if err = commitObj.Encode(signedObj); err != nil {
		return plumbing.ZeroHash, fmt.Errorf("could not encode signed commit: %w", err)
	}

	newHash, err := repo.Storer.SetEncodedObject(signedObj)
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("could not store signed commit: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("could not get HEAD for signing: %w", err)
	}

	newRef := plumbing.NewHashReference(head.Name(), newHash)
	if err = repo.Storer.SetReference(newRef); err != nil {
		return plumbing.ZeroHash, fmt.Errorf("could not update HEAD after signing: %w", err)
	}

	log.Info("Successfully applied signature to commit")
	return newHash, nil
}
