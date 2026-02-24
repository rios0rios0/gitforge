package domain_test

import (
	"os"
	"path/filepath"
	"testing"

	domain "github.com/rios0rios0/gitforge/pkg/config/domain"
	"github.com/rios0rios0/gitforge/pkg/config/domain/entities"
)

func TestResolveToken(t *testing.T) {
	t.Parallel()

	t.Run("should return empty string when raw is empty", func(t *testing.T) {
		t.Parallel()

		// given
		raw := ""

		// when
		result := domain.ResolveToken(raw)

		// then
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("should return empty for unset environment variable", func(t *testing.T) {
		t.Parallel()

		// given
		raw := "${GITFORGE_NONEXISTENT_VAR_12345}"

		// when
		result := domain.ResolveToken(raw)

		// then
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("should read token from file when path exists", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		tokenFile := filepath.Join(tmpDir, "token.txt")
		err := os.WriteFile(tokenFile, []byte("  file-token  \n"), 0o600)
		if err != nil {
			t.Fatal(err)
		}

		// when
		result := domain.ResolveToken(tokenFile)

		// then
		if result != "file-token" {
			t.Errorf("expected %q, got %q", "file-token", result)
		}
	})

	t.Run("should return inline token when not a file path", func(t *testing.T) {
		t.Parallel()

		// given
		raw := "ghp_abc123"

		// when
		result := domain.ResolveToken(raw)

		// then
		if result != "ghp_abc123" {
			t.Errorf("expected %q, got %q", "ghp_abc123", result)
		}
	})
}

func TestResolveTokenEnvVar(t *testing.T) {
	// given — cannot use t.Parallel with t.Setenv
	t.Setenv("GITFORGE_TEST_TOKEN", "my-secret-token")
	raw := "${GITFORGE_TEST_TOKEN}"

	// when
	result := domain.ResolveToken(raw)

	// then
	if result != "my-secret-token" {
		t.Errorf("expected %q, got %q", "my-secret-token", result)
	}
}

func TestFindConfigFileFound(t *testing.T) {
	// given — cannot use t.Parallel with t.Chdir
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, ".testapp.yaml")
	err := os.WriteFile(configFile, []byte("providers: []"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	// when
	t.Chdir(tmpDir)
	result, err := domain.FindConfigFile("testapp")

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != ".testapp.yaml" {
		t.Errorf("expected %q, got %q", ".testapp.yaml", result)
	}
}

func TestFindConfigFileNotFound(t *testing.T) {
	// given — cannot use t.Parallel with t.Chdir
	tmpDir := t.TempDir()

	// when
	t.Chdir(tmpDir)
	_, err := domain.FindConfigFile("nonexistent_app_xyz")

	// then
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestValidateProviders(t *testing.T) {
	t.Parallel()

	t.Run("should return nil when providers are valid", func(t *testing.T) {
		t.Parallel()

		// given
		providers := []entities.ProviderConfig{
			{Type: "github", Token: "ghp_test", Organizations: []string{"my-org"}},
		}

		// when
		err := domain.ValidateProviders(providers)

		// then
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("should return error when type is missing", func(t *testing.T) {
		t.Parallel()

		// given
		providers := []entities.ProviderConfig{
			{Type: "", Token: "ghp_test", Organizations: []string{"my-org"}},
		}

		// when
		err := domain.ValidateProviders(providers)

		// then
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("should return error when token is missing", func(t *testing.T) {
		t.Parallel()

		// given
		providers := []entities.ProviderConfig{
			{Type: "github", Token: "", Organizations: []string{"my-org"}},
		}

		// when
		err := domain.ValidateProviders(providers)

		// then
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("should return error when organizations are empty", func(t *testing.T) {
		t.Parallel()

		// given
		providers := []entities.ProviderConfig{
			{Type: "github", Token: "ghp_test", Organizations: []string{}},
		}

		// when
		err := domain.ValidateProviders(providers)

		// then
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
