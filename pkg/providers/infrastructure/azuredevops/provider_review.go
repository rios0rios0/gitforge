package azuredevops

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

// --- ReviewProvider ---

func (p *Provider) ListOpenPullRequests(
	ctx context.Context,
	repo globalEntities.Repository,
) ([]globalEntities.PullRequestDetail, error) {
	baseURL := buildBaseURL(repo.Organization)
	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/pullrequests?searchCriteria.status=active&api-version=%s",
		repo.Project, resolveRepoIdentifier(repo), apiVersion,
	)

	resp, err := p.doRequest(ctx, baseURL, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list open pull requests: %w", err)
	}

	var result struct {
		Value []struct {
			PullRequestID int    `json:"pullRequestId"`
			Title         string `json:"title"`
			Status        string `json:"status"`
			SourceRefName string `json:"sourceRefName"`
			TargetRefName string `json:"targetRefName"`
			URL           string `json:"url"`
			CreatedBy     struct {
				DisplayName string `json:"displayName"`
			} `json:"createdBy"`
		} `json:"value"`
	}
	if unmarshalErr := json.Unmarshal(resp, &result); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse pull requests response: %w", unmarshalErr)
	}

	var prs []globalEntities.PullRequestDetail
	for _, pr := range result.Value {
		prs = append(prs, globalEntities.PullRequestDetail{
			PullRequest: globalEntities.PullRequest{
				ID:     pr.PullRequestID,
				Title:  pr.Title,
				URL:    pr.URL,
				Status: pr.Status,
			},
			SourceBranch: strings.TrimPrefix(pr.SourceRefName, "refs/heads/"),
			TargetBranch: strings.TrimPrefix(pr.TargetRefName, "refs/heads/"),
			Author:       pr.CreatedBy.DisplayName,
		})
	}

	return prs, nil
}

func (p *Provider) GetPullRequestDiff(
	ctx context.Context,
	repo globalEntities.Repository,
	prID int,
) (string, error) {
	files, err := p.GetPullRequestFiles(ctx, repo, prID)
	if err != nil {
		return "", err
	}

	var diff strings.Builder
	for _, f := range files {
		if f.Patch != "" {
			diff.WriteString(f.Patch)
			diff.WriteString("\n")
		}
	}

	return diff.String(), nil
}

func (p *Provider) GetPullRequestFiles(
	ctx context.Context,
	repo globalEntities.Repository,
	prID int,
) ([]globalEntities.PullRequestFile, error) {
	baseURL := buildBaseURL(repo.Organization)

	// get the latest iteration
	iterEndpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/pullrequests/%d/iterations?api-version=%s",
		repo.Project, resolveRepoIdentifier(repo), prID, apiVersion,
	)

	iterResp, err := p.doRequest(ctx, baseURL, http.MethodGet, iterEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get pull request iterations: %w", err)
	}

	var iterResult struct {
		Value []struct {
			ID int `json:"id"`
		} `json:"value"`
	}
	if unmarshalErr := json.Unmarshal(iterResp, &iterResult); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse iterations response: %w", unmarshalErr)
	}

	if len(iterResult.Value) == 0 {
		return nil, nil
	}

	latestIter := iterResult.Value[len(iterResult.Value)-1].ID

	// get changes for the latest iteration
	changesEndpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/pullrequests/%d/iterations/%d/changes?api-version=%s",
		repo.Project, resolveRepoIdentifier(repo), prID, latestIter, apiVersion,
	)

	changesResp, err := p.doRequest(ctx, baseURL, http.MethodGet, changesEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get pull request changes: %w", err)
	}

	var changesResult struct {
		ChangeEntries []struct {
			ChangeType string `json:"changeType"`
			Item       struct {
				Path string `json:"path"`
			} `json:"item"`
			OriginalPath string `json:"originalPath"`
		} `json:"changeEntries"`
	}
	if unmarshalErr := json.Unmarshal(changesResp, &changesResult); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse changes response: %w", unmarshalErr)
	}

	var files []globalEntities.PullRequestFile
	for _, change := range changesResult.ChangeEntries {
		status := mapADOChangeType(change.ChangeType)
		files = append(files, globalEntities.PullRequestFile{
			Path:    change.Item.Path,
			OldPath: change.OriginalPath,
			Status:  status,
		})
	}

	return files, nil
}

func (p *Provider) PostPullRequestComment(
	ctx context.Context,
	repo globalEntities.Repository,
	prID int,
	body string,
) error {
	baseURL := buildBaseURL(repo.Organization)
	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/pullrequests/%d/threads?api-version=%s",
		repo.Project, resolveRepoIdentifier(repo), prID, apiVersion,
	)

	threadBody := map[string]any{
		"comments": []map[string]any{
			{
				"parentCommentId": 0,
				"content":         body,
				"commentType":     1,
			},
		},
		"status": 1,
	}

	_, err := p.doRequest(ctx, baseURL, http.MethodPost, endpoint, threadBody)
	if err != nil {
		return fmt.Errorf("failed to post pull request comment: %w", err)
	}

	return nil
}

func (p *Provider) PostPullRequestThreadComment(
	ctx context.Context,
	repo globalEntities.Repository,
	prID int,
	filePath string,
	line int,
	body string,
) error {
	baseURL := buildBaseURL(repo.Organization)
	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/pullrequests/%d/threads?api-version=%s",
		repo.Project, resolveRepoIdentifier(repo), prID, apiVersion,
	)

	threadBody := map[string]any{
		"comments": []map[string]any{
			{
				"parentCommentId": 0,
				"content":         body,
				"commentType":     1,
			},
		},
		"threadContext": map[string]any{
			"filePath": filePath,
			"rightFileStart": map[string]int{
				"line":   line,
				"offset": 1,
			},
			"rightFileEnd": map[string]int{
				"line":   line,
				"offset": 1,
			},
		},
		"status": 1,
	}

	_, err := p.doRequest(ctx, baseURL, http.MethodPost, endpoint, threadBody)
	if err != nil {
		return fmt.Errorf("failed to post pull request thread comment: %w", err)
	}

	return nil
}

func (p *Provider) GetPullRequestCheckStatus(
	ctx context.Context,
	repo globalEntities.Repository,
	prID int,
) (bool, error) {
	baseURL := buildBaseURL(repo.Organization)
	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/pullrequests/%d/statuses?api-version=%s",
		repo.Project, resolveRepoIdentifier(repo), prID, apiVersion,
	)

	resp, err := p.doRequest(ctx, baseURL, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, fmt.Errorf("failed to get pull request statuses: %w", err)
	}

	var result struct {
		Value []struct {
			State string `json:"state"`
		} `json:"value"`
	}
	if unmarshalErr := json.Unmarshal(resp, &result); unmarshalErr != nil {
		return false, fmt.Errorf("failed to parse statuses response: %w", unmarshalErr)
	}

	// if no statuses, consider passed (no CI configured)
	if len(result.Value) == 0 {
		return true, nil
	}

	for _, status := range result.Value {
		if status.State != "succeeded" {
			return false, nil
		}
	}

	return true, nil
}

func (p *Provider) MergePullRequest(
	ctx context.Context,
	repo globalEntities.Repository,
	prID int,
	strategy string,
) error {
	baseURL := buildBaseURL(repo.Organization)

	// first get the PR to obtain the last merge source commit
	getEndpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/pullrequests/%d?api-version=%s",
		repo.Project, resolveRepoIdentifier(repo), prID, apiVersion,
	)

	prResp, err := p.doRequest(ctx, baseURL, http.MethodGet, getEndpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to get pull request for merge: %w", err)
	}

	var prData struct {
		LastMergeSourceCommit struct {
			CommitID string `json:"commitId"`
		} `json:"lastMergeSourceCommit"`
	}
	if unmarshalErr := json.Unmarshal(prResp, &prData); unmarshalErr != nil {
		return fmt.Errorf("failed to parse pull request data: %w", unmarshalErr)
	}

	// complete the PR
	updateEndpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/pullrequests/%d?api-version=%s",
		repo.Project, resolveRepoIdentifier(repo), prID, apiVersion,
	)

	body := map[string]any{
		"status":                "completed",
		"lastMergeSourceCommit": prData.LastMergeSourceCommit,
		"completionOptions": map[string]any{
			"deleteSourceBranch": false,
			"mergeStrategy":      mapADOMergeStrategy(strategy),
		},
	}

	_, err = p.doRequest(ctx, baseURL, http.MethodPatch, updateEndpoint, body)
	if err != nil {
		return fmt.Errorf("failed to complete pull request: %w", err)
	}

	return nil
}

const (
	adoMergeStrategySquash        = 1
	adoMergeStrategyNoFastForward = 2
	adoMergeStrategyRebase        = 3
)

func mapADOMergeStrategy(strategy string) int {
	strategyMap := map[string]int{
		"squash": adoMergeStrategySquash,
		"merge":  adoMergeStrategyNoFastForward,
		"rebase": adoMergeStrategyRebase,
	}

	if val, ok := strategyMap[strategy]; ok {
		return val
	}

	return adoMergeStrategySquash
}

func mapADOChangeType(changeType string) string {
	changeTypeMap := map[string]string{
		"add":    "added",
		"edit":   "modified",
		"delete": "deleted",
		"rename": "renamed",
	}

	if status, ok := changeTypeMap[strings.ToLower(changeType)]; ok {
		return status
	}

	return "modified"
}
