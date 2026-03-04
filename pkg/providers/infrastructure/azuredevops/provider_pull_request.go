package azuredevops

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

func (p *Provider) CreatePullRequest(
	ctx context.Context,
	repo globalEntities.Repository,
	input globalEntities.PullRequestInput,
) (*globalEntities.PullRequest, error) {
	baseURL := buildBaseURL(repo.Organization)

	body := map[string]any{
		"sourceRefName": input.SourceBranch,
		"targetRefName": input.TargetBranch,
		"title":         input.Title,
		"description":   input.Description,
	}

	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/pullrequests?api-version=%s",
		repo.Project, repo.ID, apiVersion,
	)

	resp, err := p.doRequest(ctx, baseURL, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create PR: %w", err)
	}

	var prResp struct {
		PullRequestID int    `json:"pullRequestId"`
		Title         string `json:"title"`
		URL           string `json:"url"`
		Status        string `json:"status"`
	}
	if unmarshalErr := json.Unmarshal(resp, &prResp); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse PR response: %w", unmarshalErr)
	}

	pr := &globalEntities.PullRequest{
		ID:     prResp.PullRequestID,
		Title:  prResp.Title,
		URL:    prResp.URL,
		Status: prResp.Status,
	}

	if input.AutoComplete {
		updateBody := map[string]any{
			"autoCompleteSetBy": map[string]string{"id": "me"},
		}
		updateEndpoint := fmt.Sprintf(
			"/%s/_apis/git/repositories/%s/pullrequests/%d?api-version=%s",
			repo.Project, repo.ID, pr.ID, apiVersion,
		)
		_, _ = p.doRequest(ctx, baseURL, http.MethodPatch, updateEndpoint, updateBody)
	}

	return pr, nil
}

func (p *Provider) PullRequestExists(
	ctx context.Context,
	repo globalEntities.Repository,
	sourceBranch string,
) (bool, error) {
	baseURL := buildBaseURL(repo.Organization)
	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/pullrequests?searchCriteria.sourceRefName=refs/heads/%s&searchCriteria.status=active&api-version=%s",
		repo.Project,
		repo.ID,
		sourceBranch,
		apiVersion,
	)

	resp, err := p.doRequest(ctx, baseURL, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, err
	}

	var result struct {
		Count int `json:"count"`
	}
	if unmarshalErr := json.Unmarshal(resp, &result); unmarshalErr != nil {
		return false, fmt.Errorf("failed to parse PR list response: %w", unmarshalErr)
	}

	return result.Count > 0, nil
}
