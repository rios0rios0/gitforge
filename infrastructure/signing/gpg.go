package signing

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/ProtonMail/go-crypto/openpgp"
	log "github.com/sirupsen/logrus"
	"golang.org/x/term"
)

var (
	ErrCannotFindPrivKey                    = errors.New("cannot find private key")
	ErrCannotFindPrivKeyMatchingFingerprint = errors.New(
		"cannot find private key matching fingerprint",
	)
)

// ExportGpgKey exports a GPG key from the keyring to a file.
func ExportGpgKey(ctx context.Context, gpgKeyID string, gpgKeyExportPath string) error {
	// TODO: until today Go is not capable to read the key from the keyring (kbx)
	cmd := exec.CommandContext(
		ctx,
		"gpg",
		"--export-secret-key",
		"--output",
		gpgKeyExportPath,
		"--armor",
		gpgKeyID,
	)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to execute command GPG: %w", err)
	}
	return nil
}

// GetGpgKeyReader returns a reader for the GPG key.
// The appName parameter is used for default key path generation (e.g. "autobump" -> ~/.gnupg/autobump-{keyID}.asc).
func GetGpgKeyReader(ctx context.Context, gpgKeyID string, gpgKeyPath string, appName string) (io.Reader, error) {
	if gpgKeyPath == "" {
		gpgKeyPath = os.ExpandEnv(fmt.Sprintf("$HOME/.gnupg/%s-%s.asc", appName, gpgKeyID))
		log.Warnf("No key path provided, attempting to read (%s) at: %s", gpgKeyID, gpgKeyPath)

		if _, err := os.Stat(gpgKeyPath); os.IsNotExist(err) {
			err = ExportGpgKey(ctx, gpgKeyID, gpgKeyPath)
			if err != nil {
				return nil, err
			}
		}
	}

	gpgKeyData, err := os.ReadFile(gpgKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	return strings.NewReader(string(gpgKeyData)), nil
}

// GetGpgKey returns a GPG key entity from the given reader,
// prompting for the passphrase to decrypt the key.
func GetGpgKey(gpgKeyReader io.Reader) (*openpgp.Entity, error) {
	entityList, err := openpgp.ReadArmoredKeyRing(gpgKeyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	entity := entityList[0]
	if entity == nil {
		return nil, ErrCannotFindPrivKeyMatchingFingerprint
	}

	fmt.Print("Enter the passphrase for your GPG key: ") //nolint:forbidigo // this line is not for debugging
	var passphrase []byte
	passphrase, err = term.ReadPassword(0)
	if err != nil {
		if errors.Is(err, syscall.ENOTTY) {
			passphrase = []byte("")
		} else {
			return nil, fmt.Errorf("failed to read passphrase: %w", err)
		}
	}
	fmt.Println() //nolint:forbidigo // this line is not for debugging

	if entity.PrivateKey == nil {
		return nil, ErrCannotFindPrivKey
	}

	err = entity.PrivateKey.Decrypt(passphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt GPG key: %w", err)
	}

	log.Info("Successfully decrypted GPG key")
	return entity, nil
}
