package codeberg

import (
	"context"
	"fmt"
	"net/http"

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

// MigrateRepository creates a mirror repository on Codeberg by migrating from a source URL.
// When Mirror is true, Codeberg will periodically pull updates from the source.
func (p *Provider) MigrateRepository(
	ctx context.Context,
	input globalEntities.MirrorInput,
) error {
	body := map[string]any{
		"clone_addr":  input.CloneAddr,
		"repo_name":   input.RepoName,
		"repo_owner":  input.RepoOwner,
		"mirror":      input.Mirror,
		"private":     input.Private,
		"description": input.Description,
		"service":     input.Service,
	}

	_, err := p.doRequest(ctx, http.MethodPost, "/api/v1/repos/migrate", body)
	if err != nil {
		return fmt.Errorf("failed to migrate repository %q: %w", input.RepoName, err)
	}

	return nil
}
