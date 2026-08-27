package helpers

import (
	"errors"
	"os"
	"path/filepath"
)

// ErrConfigFileNotFound is returned when no configuration file can be located.
var ErrConfigFileNotFound = errors.New("config file not found")

// FindConfigFile searches for a configuration file in standard locations.
// The appName parameter controls the file name patterns (e.g. "autobump" -> ".autobump.yaml").
// Returns the path to the first file found, or ErrConfigFileNotFound if none is found.
// Exported for use by autobump and autoupdate to locate their configuration files.
//
// The working directory is searched before the home directory. For a configuration that belongs
// to the operator rather than to whatever directory they happen to be standing in, that order is
// wrong -- use FindGlobalConfigFile instead.
func FindConfigFile(appName string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}

	locations := []string{
		".",
		".config",
		"configs",
	}
	if homeDir != "" {
		locations = append(
			locations,
			homeDir,
			filepath.Join(homeDir, ".config"),
		)
	}

	return findConfigFileIn(locations, appName)
}

// FindGlobalConfigFile searches for an operator-level configuration file, looking only in the
// user's home directory.
//
// It exists because FindConfigFile searches the working directory first, and a tool that acts
// *on* a repository usually runs with that repository as the working directory. A repository may
// legitimately carry its own `.<appName>.yaml` holding per-project overrides, so asking
// FindConfigFile for the operator's configuration in that situation answers with the project's
// file instead. Nothing about that looks wrong: the project file's own settings still apply, and
// only the settings a tool honours exclusively from the operator's configuration go missing,
// silently and with no error to notice.
//
// Callers should fall back to FindConfigFile, or to their own default, when this returns
// ErrConfigFileNotFound. An operator who keeps no configuration in their home directory has no
// operator-level settings to lose, so the wider search costs them nothing.
//
// Returns the path to the first file found, or ErrConfigFileNotFound if none is found.
func FindGlobalConfigFile(appName string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", ErrConfigFileNotFound
	}

	return findConfigFileIn(
		[]string{homeDir, filepath.Join(homeDir, ".config")},
		appName,
	)
}

// findConfigFileIn returns the first existing configuration file across the given locations,
// trying every supported name in each location before moving to the next one.
func findConfigFileIn(locations []string, appName string) (string, error) {
	patterns := []string{
		"." + appName + ".yaml",
		"." + appName + ".yml",
		appName + ".yaml",
		appName + ".yml",
	}

	for _, loc := range locations {
		for _, pat := range patterns {
			p := filepath.Join(loc, pat)
			if _, statErr := os.Stat(p); statErr == nil {
				return p, nil
			}
		}
	}

	return "", ErrConfigFileNotFound
}
