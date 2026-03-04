package azuredevops

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

type adoProject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type adoRepository struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	RemoteURL     string `json:"remoteUrl"`
	SSHURL        string `json:"sshUrl"`
	DefaultBranch string `json:"defaultBranch"`
}

func (p *Provider) DiscoverRepositories(
	ctx context.Context,
	org string,
) ([]globalEntities.Repository, error) {
	baseURL := normalizeOrgURL(org)

	projects, err := p.getProjects(ctx, baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get projects: %w", err)
	}

	var repos []globalEntities.Repository
	for _, proj := range projects {
		projRepos, repoErr := p.getRepositories(ctx, baseURL, proj.ID)
		if repoErr != nil {
			continue
		}
		for _, r := range projRepos {
			repos = append(repos, globalEntities.Repository{
				ID:            r.ID,
				Name:          r.Name,
				Organization:  extractOrgName(baseURL),
				Project:       proj.Name,
				DefaultBranch: r.DefaultBranch,
				RemoteURL:     r.RemoteURL,
				SSHURL:        r.SSHURL,
				ProviderName:  providerName,
			})
		}
	}

	return repos, nil
}

func (p *Provider) getProjects(
	ctx context.Context,
	baseURL string,
) ([]adoProject, error) {
	var all []adoProject
	continuationToken := ""

	for {
		endpoint := "/_apis/projects?api-version=" + apiVersion
		if continuationToken != "" {
			endpoint += "&continuationToken=" + continuationToken
		}

		resp, headers, err := p.doRequestWithHeaders(
			ctx, baseURL, http.MethodGet, endpoint, nil,
		)
		if err != nil {
			return nil, err
		}

		var result struct {
			Value []adoProject `json:"value"`
		}
		if unmarshalErr := json.Unmarshal(resp, &result); unmarshalErr != nil {
			return nil, fmt.Errorf("failed to parse projects response: %w", unmarshalErr)
		}

		all = append(all, result.Value...)
		continuationToken = headers.Get(paginationHeader)
		if continuationToken == "" {
			break
		}
	}

	return all, nil
}

func (p *Provider) getRepositories(
	ctx context.Context,
	baseURL, projectID string,
) ([]adoRepository, error) {
	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories?api-version=%s",
		projectID, apiVersion,
	)

	resp, err := p.doRequest(ctx, baseURL, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Value []adoRepository `json:"value"`
	}
	if unmarshalErr := json.Unmarshal(resp, &result); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse repositories response: %w", unmarshalErr)
	}

	return result.Value, nil
}
