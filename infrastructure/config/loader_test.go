package config_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/gitforge/infrastructure/config"
)

func TestReadData(t *testing.T) {
	t.Parallel()

	t.Run("should read data from local file", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "config.yaml")
		expected := "providers:\n  - type: github\n"
		err := os.WriteFile(filePath, []byte(expected), 0o600)
		require.NoError(t, err)

		// when
		data, err := config.ReadData(filePath)

		// then
		require.NoError(t, err)
		assert.Equal(t, expected, string(data))
	})

	t.Run("should return error when file does not exist", func(t *testing.T) {
		t.Parallel()

		// given
		filePath := "/tmp/nonexistent_config_file_xyz.yaml"

		// when
		_, err := config.ReadData(filePath)

		// then
		require.Error(t, err)
	})

	t.Run("should read data from URL", func(t *testing.T) {
		t.Parallel()

		// given
		expected := "remote-config-content"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(expected))
		}))
		defer server.Close()

		// when
		data, err := config.ReadData(server.URL + "/config.yaml")

		// then
		require.NoError(t, err)
		assert.Equal(t, expected, string(data))
	})

	t.Run("should return error when URL returns non-200 status", func(t *testing.T) {
		t.Parallel()

		// given
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		// when
		_, err := config.ReadData(server.URL + "/missing.yaml")

		// then
		require.Error(t, err)
	})

	t.Run("should treat plain string without scheme as file path", func(t *testing.T) {
		t.Parallel()

		// given
		path := "not-a-url"

		// when
		_, err := config.ReadData(path)

		// then
		require.Error(t, err)
	})
}
