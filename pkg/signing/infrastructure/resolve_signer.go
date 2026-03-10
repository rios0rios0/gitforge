package infrastructure

import (
	"context"

	logger "github.com/sirupsen/logrus"

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
	"github.com/rios0rios0/gitforge/pkg/signing/infrastructure/helpers"
)

// ResolveSignerFromGitConfig determines if and how commits should be signed
// based on git configuration values. Returns nil if signing is not configured
// (gpgSign is not "true").
//
// Parameters map to standard git config values:
//   - gpgSign: commit.gpgsign
//   - signingFormat: gpg.format ("ssh" for SSH signing, anything else for GPG)
//   - signingKey: user.signingkey (key ID for GPG, file path for SSH)
//   - gpgKeyPath: optional file path to exported GPG key (empty = auto-detect from keyring)
//   - gpgPassphrase: GPG key passphrase (empty = prompt interactively)
//   - appName: application name used for GPG key path generation (e.g., "autobump")
func ResolveSignerFromGitConfig(
	gpgSign, signingFormat, signingKey, gpgKeyPath, gpgPassphrase, appName string,
) (globalEntities.CommitSigner, error) {
	if gpgSign != "true" {
		return nil, nil
	}

	switch {
	case signingFormat == "ssh":
		logger.Info("Signing commit with SSH key")
		sshKeyPath, err := helpers.ReadSSHSigningKey(signingKey)
		if err != nil {
			return nil, err
		}
		return NewSSHSigner(sshKeyPath), nil

	default:
		logger.Info("Signing commit with GPG key")
		gpgKeyReader, err := helpers.GetGpgKeyReader(
			context.Background(), signingKey, gpgKeyPath, appName,
		)
		if err != nil {
			return nil, err
		}

		signKey, err := helpers.GetGpgKey(gpgKeyReader, gpgPassphrase)
		if err != nil {
			return nil, err
		}
		return NewGPGSigner(signKey), nil
	}
}
