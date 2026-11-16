package azuredevops

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	log "github.com/sirupsen/logrus"

	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
)

type adoProject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type adoRepository struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	RemoteURL       string     `json:"remoteUrl"`
	SSHURL          string     `json:"sshUrl"`
	DefaultBranch   string     `json:"defaultBranch"`
	IsFork          bool       `json:"isFork"`
	IsDisabled      bool       `json:"isDisabled"`
	IsInMaintenance bool       `json:"isInMaintenance"`
	Project         adoProject `json:"project"`
}

// Ensure the optional strict lister is implemented at compile time.
var _ globalEntities.OrganizationRepositoryLister = (*Provider)(nil)

// toDomain maps a repository of the REST API onto the shared entity. A
// repository under maintenance is reported as disabled: neither can be cloned or
// pushed to while the state lasts.
func (r adoRepository) toDomain(organization, project string) globalEntities.Repository {
	return globalEntities.Repository{
		ID:            r.ID,
		Name:          r.Name,
		Organization:  organization,
		Project:       project,
		DefaultBranch: r.DefaultBranch,
		RemoteURL:     r.RemoteURL,
		SSHURL:        r.SSHURL,
		ProviderName:  providerName,
		IsFork:        r.IsFork,
		IsDisabled:    r.IsDisabled || r.IsInMaintenance,
	}
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
			// Kept best-effort for the consumers that rely on it, but no longer
			// silent: a caller that must see the whole organization uses
			// ListOrganizationRepositories, which fails instead.
			log.WithError(repoErr).WithField("project", proj.Name).
				Warn("Skipping a project whose repositories could not be listed")
			continue
		}
		for _, r := range projRepos {
			repos = append(repos, r.toDomain(extractOrgName(baseURL), proj.Name))
		}
	}

	return repos, nil
}

// ListOrganizationRepositories lists every repository of org with the
// organization-level endpoint, which returns each repository's project with it,
// so the listing needs no per-project calls and only the Code (Read) scope. Any
// failure, including a sign-in page or an unreadable page, is returned instead
// of narrowing the result.
func (p *Provider) ListOrganizationRepositories(
	ctx context.Context,
	org string,
) ([]globalEntities.Repository, error) {
	baseURL := normalizeOrgURL(org)
	organization := extractOrgName(baseURL)

	var repos []globalEntities.Repository
	continuationToken := ""
	for {
		endpoint := "/_apis/git/repositories?api-version=" + apiVersion
		if continuationToken != "" {
			endpoint += "&continuationToken=" + url.QueryEscape(continuationToken)
		}

		resp, headers, err := p.doRequestWithHeaders(ctx, baseURL, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to list the repositories of %s: %w", organization, err)
		}

		var result struct {
			Value []adoRepository `json:"value"`
		}
		if unmarshalErr := json.Unmarshal(resp, &result); unmarshalErr != nil {
			return nil, fmt.Errorf("failed to parse repositories response: %w", unmarshalErr)
		}

		for _, r := range result.Value {
			repos = append(repos, r.toDomain(organization, r.Project.Name))
		}

		continuationToken = headers.Get(paginationHeader)
		if continuationToken == "" {
			return repos, nil
		}
	}
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
