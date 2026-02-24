//go:build unit

package infrastructure_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	infrastructure "github.com/rios0rios0/gitforge/pkg/config/infrastructure"
)

func TestDownloadFile(t *testing.T) {
	t.Parallel()

	t.Run("should download file from valid URL", func(t *testing.T) {
		t.Parallel()

		// given
		expected := "file-content-from-server"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(expected))
		}))
		defer server.Close()

		// when
		data, err := infrastructure.DownloadFile(server.URL + "/test.txt")

		// then
		require.NoError(t, err)
		assert.Equal(t, expected, string(data))
	})

	t.Run("should return error when server returns non-200 status", func(t *testing.T) {
		t.Parallel()

		// given
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		// when
		_, err := infrastructure.DownloadFile(server.URL + "/test.txt")

		// then
		require.Error(t, err)
	})

	t.Run("should return error for invalid URL", func(t *testing.T) {
		t.Parallel()

		// given
		invalidURL := "http://127.0.0.1:0/nonexistent"

		// when
		_, err := infrastructure.DownloadFile(invalidURL)

		// then
		require.Error(t, err)
	})
}
