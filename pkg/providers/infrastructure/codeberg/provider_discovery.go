package codeberg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
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
	var allRepos []globalEntities.Repository
	page := 1

	for {
		endpoint := fmt.Sprintf(
			"/api/v1/orgs/%s/repos?page=%d&limit=%d",
			org, page, perPage,
		)

		resp, err := p.doRequest(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}

		var repos []forgejoRepo
		if unmarshalErr := json.Unmarshal(resp, &repos); unmarshalErr != nil {
			return nil, fmt.Errorf("failed to parse repos response: %w", unmarshalErr)
		}

		if len(repos) == 0 {
			break
		}

		for _, r := range repos {
			allRepos = append(allRepos, forgejoRepoToDomain(r, org))
		}

		if len(repos) < perPage {
			break
		}
		page++
	}

	return allRepos, nil
}

func (p *Provider) discoverUserRepos(
	ctx context.Context,
	user string,
) ([]globalEntities.Repository, error) {
	var allRepos []globalEntities.Repository
	page := 1

	for {
		endpoint := fmt.Sprintf(
			"/api/v1/users/%s/repos?page=%d&limit=%d",
			user, page, perPage,
		)

		resp, err := p.doRequest(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to list user repos for %q: %w", user, err)
		}

		var repos []forgejoRepo
		if unmarshalErr := json.Unmarshal(resp, &repos); unmarshalErr != nil {
			return nil, fmt.Errorf("failed to parse repos response: %w", unmarshalErr)
		}

		if len(repos) == 0 {
			break
		}

		for _, r := range repos {
			allRepos = append(allRepos, forgejoRepoToDomain(r, user))
		}

		if len(repos) < perPage {
			break
		}
		page++
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
