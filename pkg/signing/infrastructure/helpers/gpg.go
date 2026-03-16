package helpers

import (
	"bytes"
	"context"
	"encoding/base64"
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

const armoredPGPHeader = "-----BEGIN PGP"

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
// Supports armored (ASCII) and base64-encoded armored key formats.
// Exported for use by autobump (github.com/rios0rios0/autobump).
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

	resolved, err := resolveGPGKeyFormat(gpgKeyData)
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(resolved), nil
}

// resolveGPGKeyFormat detects the format of GPG key data and normalizes it to armored PGP.
// It supports: raw armored PGP, and base64-encoded armored PGP.
func resolveGPGKeyFormat(data []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(data)

	if len(trimmed) == 0 {
		return nil, errors.New(
			"GPG key file is empty; ensure the file contains a valid armored PGP key " +
				"(exported with: gpg --export-secret-key --armor <KEY_ID>)",
		)
	}

	// already armored
	if strings.HasPrefix(string(trimmed), armoredPGPHeader) {
		return trimmed, nil
	}

	// try base64 decode (strip internal newlines/spaces from line-wrapped encodings)
	cleaned := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			return -1
		}
		return r
	}, string(trimmed))
	decoded, err := base64.StdEncoding.DecodeString(cleaned)
	if err == nil && strings.HasPrefix(string(bytes.TrimSpace(decoded)), armoredPGPHeader) {
		log.Info("GPG key was base64-encoded; decoded successfully")
		return bytes.TrimSpace(decoded), nil
	}

	return nil, fmt.Errorf(
		"GPG key file does not contain valid armored PGP data (file size: %d bytes); "+
			"ensure the key is exported with: gpg --export-secret-key --armor <KEY_ID>",
		len(trimmed),
	)
}

// GetGpgKey returns a GPG key entity from the given reader, decrypting it with the provided passphrase.
// If passphrase is empty, it prompts interactively (falling back to empty passphrase in non-TTY environments).
// Exported for use by autobump (github.com/rios0rios0/autobump).
func GetGpgKey(gpgKeyReader io.Reader, passphrase string) (*openpgp.Entity, error) {
	entityList, err := openpgp.ReadArmoredKeyRing(gpgKeyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	entity := entityList[0]
	if entity == nil {
		return nil, ErrCannotFindPrivKeyMatchingFingerprint
	}

	passphraseBytes := []byte(passphrase)
	if passphrase == "" {
		passphraseBytes, err = promptPassphrase()
		if err != nil {
			return nil, err
		}
	}

	if entity.PrivateKey == nil {
		return nil, ErrCannotFindPrivKey
	}

	err = entity.PrivateKey.Decrypt(passphraseBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt GPG key: %w", err)
	}

	log.Info("Successfully decrypted GPG key")
	return entity, nil
}

// promptPassphrase reads a passphrase from the terminal, falling back to empty passphrase
// when no TTY is available (e.g. in CI environments).
func promptPassphrase() ([]byte, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		log.Info("No TTY detected (CI environment), using empty passphrase")
		return []byte(""), nil
	}

	fmt.Print("Enter the passphrase for your GPG key: ") //nolint:forbidigo // this line is not for debugging
	passphrase, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		if errors.Is(err, syscall.ENOTTY) {
			log.Info("No TTY detected, using empty passphrase")
			return []byte(""), nil
		}
		return nil, fmt.Errorf("failed to read passphrase: %w", err)
	}
	fmt.Println() //nolint:forbidigo // this line is not for debugging
	return passphrase, nil
}
