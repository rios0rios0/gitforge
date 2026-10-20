package codeberg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
)

type forgejoRepo struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	CloneURL      string `json:"clone_url"`
	SSHURL        string `json:"ssh_url"`
	DefaultBranch string `json:"default_branch"`
	Fork          bool   `json:"fork"`
	Archived      bool   `json:"archived"`
	Private       bool   `json:"private"`
	Owner         struct {
		Login string `json:"login"`
	} `json:"owner"`
}

func (p *Provider) DiscoverRepositories(
	ctx context.Context,
	org string,
) ([]globalEntities.Repository, error) {
	repos, err := p.discoverOrgRepos(ctx, org)
	if err != nil {
		// Only fall back to user repositories when the organization is definitively not found (HTTP 404).
		var ae *apiError
		if errors.As(err, &ae) && ae.StatusCode() == http.StatusNotFound {
			return p.discoverUserRepos(ctx, org)
		}

		return nil, fmt.Errorf("failed to discover org repos for %q: %w", org, err)
	}
	return repos, nil
}

func (p *Provider) discoverOrgRepos(
	ctx context.Context,
	org string,
) ([]globalEntities.Repository, error) {
	return p.listRepos(ctx, "/api/v1/orgs/"+org+"/repos", org)
}

func (p *Provider) discoverUserRepos(
	ctx context.Context,
	user string,
) ([]globalEntities.Repository, error) {
	repos, err := p.listRepos(ctx, "/api/v1/users/"+user+"/repos", user)
	if err != nil {
		return nil, fmt.Errorf("failed to list user repos for %q: %w", user, err)
	}
	return repos, nil
}

// listRepos reads every page of the repository listing at path through
// walkPages, mapping each repository onto owner.
func (p *Provider) listRepos(
	ctx context.Context,
	path, owner string,
) ([]globalEntities.Repository, error) {
	var allRepos []globalEntities.Repository
	walkErr := walkPages(
		"repositories of "+owner,
		func(page int) ([]forgejoRepo, error) {
			endpoint := fmt.Sprintf("%s?page=%d&limit=%d", path, page, perPage)
			resp, err := p.doRequest(ctx, http.MethodGet, endpoint, nil)
			if err != nil {
				return nil, err
			}

			var repos []forgejoRepo
			if unmarshalErr := json.Unmarshal(resp, &repos); unmarshalErr != nil {
				return nil, fmt.Errorf("failed to parse repos response: %w", unmarshalErr)
			}
			return repos, nil
		},
		func(r forgejoRepo) int { return r.ID },
		func(r forgejoRepo) bool {
			allRepos = append(allRepos, forgejoRepoToDomain(r, owner))
			return false
		},
	)
	if walkErr != nil {
		return nil, walkErr
	}

	return allRepos, nil
}

func forgejoRepoToDomain(r forgejoRepo, org string) globalEntities.Repository {
	defaultBranch := r.DefaultBranch
	if defaultBranch == "" {
		defaultBranch = "main"
	}
	return globalEntities.Repository{
		ID:            strconv.Itoa(r.ID),
		Name:          r.Name,
		Organization:  org,
		DefaultBranch: "refs/heads/" + defaultBranch,
		RemoteURL:     r.CloneURL,
		SSHURL:        r.SSHURL,
		ProviderName:  providerName,
		IsFork:        r.Fork,
		IsArchived:    r.Archived,
	}
}
