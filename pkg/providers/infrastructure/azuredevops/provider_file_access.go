package azuredevops

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	globalDomain "github.com/rios0rios0/gitforge/pkg/global/domain"
	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

func (p *Provider) GetFileContent(
	ctx context.Context,
	repo globalEntities.Repository,
	path string,
) (string, error) {
	baseURL := buildBaseURL(repo.Organization)
	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/items?path=%s&api-version=%s",
		repo.Project, repo.ID, url.QueryEscape(path), apiVersion,
	)

	resp, err := p.doRequest(ctx, baseURL, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}

	return string(resp), nil
}

func (p *Provider) ListFiles(
	ctx context.Context,
	repo globalEntities.Repository,
	pattern string,
) ([]globalEntities.File, error) {
	baseURL := buildBaseURL(repo.Organization)
	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/items?recursionLevel=Full&api-version=%s",
		repo.Project, repo.ID, apiVersion,
	)

	resp, err := p.doRequest(ctx, baseURL, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Value []struct {
			ObjectID      string `json:"objectId"`
			GitObjectType string `json:"gitObjectType"`
			Path          string `json:"path"`
		} `json:"value"`
	}
	if unmarshalErr := json.Unmarshal(resp, &result); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse items response: %w", unmarshalErr)
	}

	var files []globalEntities.File
	for _, item := range result.Value {
		isDir := item.GitObjectType != "blob"
		if pattern != "" && !strings.HasSuffix(item.Path, pattern) {
			continue
		}
		files = append(files, globalEntities.File{
			Path:     item.Path,
			ObjectID: item.ObjectID,
			IsDir:    isDir,
		})
	}

	return files, nil
}

func (p *Provider) GetTags(
	ctx context.Context,
	repo globalEntities.Repository,
) ([]string, error) {
	baseURL := buildBaseURL(repo.Organization)
	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/refs?filter=tags&api-version=%s",
		repo.Project, repo.ID, apiVersion,
	)

	resp, err := p.doRequest(ctx, baseURL, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Value []struct {
			Name string `json:"name"`
		} `json:"value"`
	}
	if unmarshalErr := json.Unmarshal(resp, &result); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse tags response: %w", unmarshalErr)
	}

	var tags []string
	for _, ref := range result.Value {
		tags = append(tags, strings.TrimPrefix(ref.Name, "refs/tags/"))
	}

	globalDomain.SortVersionsDescending(tags)
	return tags, nil
}

func (p *Provider) HasFile(
	ctx context.Context,
	repo globalEntities.Repository,
	path string,
) bool {
	_, err := p.GetFileContent(ctx, repo, path)
	return err == nil
}

func (p *Provider) CreateBranchWithChanges(
	ctx context.Context,
	repo globalEntities.Repository,
	input globalEntities.BranchInput,
) error {
	baseURL := buildBaseURL(repo.Organization)

	baseCommitID, err := p.getCommitID(ctx, baseURL, repo)
	if err != nil {
		return fmt.Errorf("failed to get base branch commit: %w", err)
	}

	var fileChanges []map[string]any
	for _, change := range input.Changes {
		entry := map[string]any{
			"changeType": change.ChangeType,
			"item": map[string]string{
				"path": change.Path,
			},
			"newContent": map[string]string{
				"content":     base64.StdEncoding.EncodeToString([]byte(change.Content)),
				"contentType": "base64encoded",
			},
		}
		fileChanges = append(fileChanges, entry)
	}

	pushBody := map[string]any{
		"refUpdates": []map[string]string{
			{
				"name":        "refs/heads/" + input.BranchName,
				"oldObjectId": allZeroObjectID,
			},
		},
		"commits": []map[string]any{
			{
				"comment": input.CommitMessage,
				"changes": fileChanges,
				"parents": []string{baseCommitID},
			},
		},
	}

	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/pushes?api-version=%s",
		repo.Project, repo.ID, apiVersion,
	)

	_, err = p.doRequest(ctx, baseURL, http.MethodPost, endpoint, pushBody)
	if err != nil {
		return fmt.Errorf("failed to push changes: %w", err)
	}

	return nil
}

func (p *Provider) getCommitID(
	ctx context.Context,
	baseURL string,
	repo globalEntities.Repository,
) (string, error) {
	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s?api-version=%s",
		repo.Project, repo.ID, apiVersion,
	)

	resp, err := p.doRequest(ctx, baseURL, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}

	var repoInfo struct {
		DefaultBranch string `json:"defaultBranch"`
	}
	if unmarshalErr := json.Unmarshal(resp, &repoInfo); unmarshalErr != nil {
		return "", fmt.Errorf("failed to parse repository info: %w", unmarshalErr)
	}

	branchName := strings.TrimPrefix(repoInfo.DefaultBranch, "refs/heads/")
	branchEndpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/refs?filter=heads/%s&api-version=%s",
		repo.Project, repo.ID, branchName, apiVersion,
	)

	branchResp, branchErr := p.doRequest(
		ctx, baseURL, http.MethodGet, branchEndpoint, nil,
	)
	if branchErr != nil {
		return "", branchErr
	}

	var branchResult struct {
		Value []struct {
			ObjectID string `json:"objectId"`
		} `json:"value"`
	}
	if unmarshalErr := json.Unmarshal(branchResp, &branchResult); unmarshalErr != nil {
		return "", fmt.Errorf("failed to parse branch response: %w", unmarshalErr)
	}
	if len(branchResult.Value) == 0 {
		return "", errors.New("default branch not found")
	}

	return branchResult.Value[0].ObjectID, nil
}
