package helpers_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/gitforge/pkg/config/domain/helpers"
)

const appName = "testapp"

// writeConfig creates an empty configuration file at dir/name, creating dir when needed.
func writeConfig(t *testing.T, dir, name string) string {
	t.Helper()

	require.NoError(t, os.MkdirAll(dir, 0o700))
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte("languages:\n"), 0o600))

	return path
}

func TestFindGlobalConfigFile(t *testing.T) {
	t.Run("should return the file in the home directory when one is present", func(t *testing.T) {
		// given
		home := t.TempDir()
		t.Setenv("HOME", home)
		writeConfig(t, home, "."+appName+".yaml")

		// when
		path, err := helpers.FindGlobalConfigFile(appName)

		// then
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(home, "."+appName+".yaml"), path)
	})

	t.Run("should prefer the dotted YAML name over the other spellings", func(t *testing.T) {
		// given
		home := t.TempDir()
		t.Setenv("HOME", home)
		for _, name := range []string{appName + ".yml", appName + ".yaml", "." + appName + ".yml", "." + appName + ".yaml"} {
			writeConfig(t, home, name)
		}

		// when
		path, err := helpers.FindGlobalConfigFile(appName)

		// then
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(home, "."+appName+".yaml"), path)
	})

	t.Run("should fall through to the .config directory when the home root has none", func(t *testing.T) {
		// given
		home := t.TempDir()
		t.Setenv("HOME", home)
		writeConfig(t, filepath.Join(home, ".config"), appName+".yaml")

		// when
		path, err := helpers.FindGlobalConfigFile(appName)

		// then
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(home, ".config", appName+".yaml"), path)
	})

	// The reason this function exists. A tool acting on a repository runs with that repository as
	// the working directory, and the repository may carry its own config; answering with it drops
	// every operator-level setting without a word.
	t.Run("should ignore a config in the working directory", func(t *testing.T) {
		// given
		home := t.TempDir()
		t.Setenv("HOME", home)
		project := t.TempDir()
		writeConfig(t, project, "."+appName+".yaml")
		t.Chdir(project)

		// when
		path, err := helpers.FindGlobalConfigFile(appName)

		// then
		require.ErrorIs(t, err, helpers.ErrConfigFileNotFound)
		assert.Empty(t, path)
	})

	t.Run("should report not found when the home directory holds no config", func(t *testing.T) {
		// given
		t.Setenv("HOME", t.TempDir())

		// when
		path, err := helpers.FindGlobalConfigFile(appName)

		// then
		require.ErrorIs(t, err, helpers.ErrConfigFileNotFound)
		assert.Empty(t, path)
	})
}

func TestFindConfigFile(t *testing.T) {
	t.Run("should return a config in the working directory before the home one", func(t *testing.T) {
		// given
		home := t.TempDir()
		t.Setenv("HOME", home)
		writeConfig(t, home, "."+appName+".yaml")
		project := t.TempDir()
		expected := writeConfig(t, project, "."+appName+".yaml")
		t.Chdir(project)

		// when
		path, err := helpers.FindConfigFile(appName)

		// then
		require.NoError(t, err)
		resolved, absErr := filepath.Abs(path)
		require.NoError(t, absErr)
		assert.Equal(t, expected, resolved, "the working directory is searched first, by design")
	})

	t.Run("should fall back to the home directory when the working directory has none", func(t *testing.T) {
		// given
		home := t.TempDir()
		t.Setenv("HOME", home)
		writeConfig(t, home, "."+appName+".yaml")
		t.Chdir(t.TempDir())

		// when
		path, err := helpers.FindConfigFile(appName)

		// then
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(home, "."+appName+".yaml"), path)
	})

	t.Run("should report not found when no location holds a config", func(t *testing.T) {
		// given
		t.Setenv("HOME", t.TempDir())
		t.Chdir(t.TempDir())

		// when
		path, err := helpers.FindConfigFile(appName)

		// then
		require.ErrorIs(t, err, helpers.ErrConfigFileNotFound)
		assert.Empty(t, path)
	})
}
