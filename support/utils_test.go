package support_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rios0rios0/gitforge/support"
)

func TestReadLines(t *testing.T) {
	t.Parallel()

	t.Run("should read file lines correctly", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test.txt")
		err := os.WriteFile(filePath, []byte("line1\nline2\nline3"), 0o600)
		if err != nil {
			t.Fatal(err)
		}

		// when
		lines, err := support.ReadLines(filePath)

		// then
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(lines) != 3 {
			t.Errorf("expected 3 lines, got %d", len(lines))
		}
		if lines[0] != "line1" || lines[1] != "line2" || lines[2] != "line3" {
			t.Errorf("unexpected lines: %v", lines)
		}
	})

	t.Run("should return error when file does not exist", func(t *testing.T) {
		t.Parallel()

		// given
		filePath := "/tmp/nonexistent_file_12345.txt"

		// when
		_, err := support.ReadLines(filePath)

		// then
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestWriteLines(t *testing.T) {
	t.Parallel()

	t.Run("should write and read back lines correctly", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "output.txt")
		lines := []string{"hello", "world"}

		// when
		err := support.WriteLines(filePath, lines)

		// then
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		readBack, err := support.ReadLines(filePath)
		if err != nil {
			t.Fatalf("unexpected error reading back: %v", err)
		}
		if len(readBack) != 2 {
			t.Errorf("expected 2 lines, got %d", len(readBack))
		}
	})
}

func TestStripUsernameFromURL(t *testing.T) {
	t.Parallel()

	t.Run("should strip username from HTTPS URL", func(t *testing.T) {
		t.Parallel()

		// given
		rawURL := "https://user@dev.azure.com/org/project"

		// when
		result := support.StripUsernameFromURL(rawURL)

		// then
		expected := "https://dev.azure.com/org/project"
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	})

	t.Run("should return URL unchanged when no username present", func(t *testing.T) {
		t.Parallel()

		// given
		rawURL := "https://dev.azure.com/org/project"

		// when
		result := support.StripUsernameFromURL(rawURL)

		// then
		if result != rawURL {
			t.Errorf("expected %q, got %q", rawURL, result)
		}
	})

	t.Run("should return non-HTTP URL unchanged", func(t *testing.T) {
		t.Parallel()

		// given
		rawURL := "git@github.com:owner/repo.git"

		// when
		result := support.StripUsernameFromURL(rawURL)

		// then
		if result != rawURL {
			t.Errorf("expected %q, got %q", rawURL, result)
		}
	})
}

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
		data, err := support.DownloadFile(server.URL + "/test.txt")

		// then
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(data) != expected {
			t.Errorf("expected %q, got %q", expected, string(data))
		}
	})

	t.Run("should return error when server returns non-200 status", func(t *testing.T) {
		t.Parallel()

		// given
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		// when
		_, err := support.DownloadFile(server.URL + "/test.txt")

		// then
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("should return error for invalid URL", func(t *testing.T) {
		t.Parallel()

		// given
		invalidURL := "http://127.0.0.1:0/nonexistent"

		// when
		_, err := support.DownloadFile(invalidURL)

		// then
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
