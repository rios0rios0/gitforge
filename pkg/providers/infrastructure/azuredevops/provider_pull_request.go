package azuredevops

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	log "github.com/sirupsen/logrus"

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

// resolveRepoIdentifier returns repo.ID if non-empty, otherwise falls back to repo.Name.
// The Azure DevOps API accepts both the repository UUID and the repository name in URL paths.
func resolveRepoIdentifier(repo globalEntities.Repository) string {
	if repo.ID != "" {
		return repo.ID
	}
	log.WithField("repoName", repo.Name).
		Warn("Repository ID is empty, falling back to repository name for API calls")
	return url.PathEscape(repo.Name)
}

// ensureRefsPrefix prepends "refs/heads/" to a branch name if it does not already start with "refs/".
// The Azure DevOps API requires fully qualified ref names (e.g. "refs/heads/main").
func ensureRefsPrefix(branch string) string {
	if strings.HasPrefix(branch, "refs/") {
		return branch
	}
	return "refs/heads/" + branch
}

func (p *Provider) CreatePullRequest(
	ctx context.Context,
	repo globalEntities.Repository,
	input globalEntities.PullRequestInput,
) (*globalEntities.PullRequest, error) {
	baseURL := buildBaseURL(repo.Organization)
	repoIdentifier := resolveRepoIdentifier(repo)

	body := map[string]any{
		jsonKeySourceRefName: ensureRefsPrefix(input.SourceBranch),
		jsonKeyTargetRefName: ensureRefsPrefix(input.TargetBranch),
		jsonKeyTitle:         input.Title,
		"description":        input.Description,
	}

	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/pullrequests?api-version=%s",
		repo.Project, repoIdentifier, apiVersion,
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
			repo.Project, repoIdentifier, pr.ID, apiVersion,
		)
		_, _ = p.doRequest(ctx, baseURL, http.MethodPatch, updateEndpoint, updateBody)
	}

	return pr, nil
}

// activePullRequestsEndpoint builds the endpoint listing the active pull requests
// whose source branch is sourceBranch.
func activePullRequestsEndpoint(repo globalEntities.Repository, sourceBranch string) string {
	return fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/pullrequests?searchCriteria.sourceRefName=%s&searchCriteria.status=active&api-version=%s",
		repo.Project,
		resolveRepoIdentifier(repo),
		url.QueryEscape(ensureRefsPrefix(sourceBranch)),
		apiVersion,
	)
}

func (p *Provider) PullRequestExists(
	ctx context.Context,
	repo globalEntities.Repository,
	sourceBranch string,
) (bool, error) {
	baseURL := buildBaseURL(repo.Organization)
	endpoint := activePullRequestsEndpoint(repo, sourceBranch)

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

// findActivePullRequestID returns the ID of the active pull request whose source branch
// is sourceBranch. The boolean reports whether such a pull request was found.
func (p *Provider) findActivePullRequestID(
	ctx context.Context,
	repo globalEntities.Repository,
	sourceBranch string,
) (int, bool, error) {
	baseURL := buildBaseURL(repo.Organization)
	endpoint := activePullRequestsEndpoint(repo, sourceBranch)

	resp, err := p.doRequest(ctx, baseURL, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, false, err
	}

	var result struct {
		Value []struct {
			PullRequestID int `json:"pullRequestId"`
		} `json:"value"`
	}
	if unmarshalErr := json.Unmarshal(resp, &result); unmarshalErr != nil {
		return 0, false, fmt.Errorf("failed to parse PR list response: %w", unmarshalErr)
	}

	if len(result.Value) == 0 {
		return 0, false, nil
	}

	return result.Value[0].PullRequestID, true, nil
}

func (p *Provider) ClosePullRequest(
	ctx context.Context,
	repo globalEntities.Repository,
	sourceBranch string,
) (bool, error) {
	prID, found, err := p.findActivePullRequestID(ctx, repo, sourceBranch)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}

	baseURL := buildBaseURL(repo.Organization)
	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/pullrequests/%d?api-version=%s",
		repo.Project, resolveRepoIdentifier(repo), prID, apiVersion,
	)
	body := map[string]any{jsonKeyStatus: prStatusAbandoned}

	if _, abandonErr := p.doRequest(
		ctx, baseURL, http.MethodPatch, endpoint, body,
	); abandonErr != nil {
		return false, fmt.Errorf("failed to abandon pull request %d: %w", prID, abandonErr)
	}

	log.WithField(logFieldPRID, prID).Info("Abandoned pull request")
	return true, nil
}
