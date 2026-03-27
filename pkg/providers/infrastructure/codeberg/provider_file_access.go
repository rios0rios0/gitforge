package codeberg

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
	globalHelpers "github.com/rios0rios0/gitforge/pkg/global/domain/helpers"
)

type forgejoFileContent struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
	Type     string `json:"type"`
}

type forgejoTreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
}

type forgejoTag struct {
	Name string `json:"name"`
}

func (p *Provider) GetFileContent(
	ctx context.Context,
	repo globalEntities.Repository,
	path string,
) (string, error) {
	endpoint := fmt.Sprintf(
		"/api/v1/repos/%s/%s/contents/%s",
		repo.Organization, repo.Name, path,
	)

	resp, err := p.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get file %q: %w", path, err)
	}

	var fc forgejoFileContent
	if unmarshalErr := json.Unmarshal(resp, &fc); unmarshalErr != nil {
		return "", fmt.Errorf("failed to parse file content response: %w", unmarshalErr)
	}

	if fc.Type == "dir" {
		return "", fmt.Errorf("path %q is a directory, not a file", path)
	}

	if fc.Encoding == "base64" {
		decoded, decodeErr := base64.StdEncoding.DecodeString(fc.Content)
		if decodeErr != nil {
			return "", fmt.Errorf("failed to decode base64 content: %w", decodeErr)
		}
		return string(decoded), nil
	}

	return fc.Content, nil
}

func (p *Provider) ListFiles(
	ctx context.Context,
	repo globalEntities.Repository,
	pattern string,
) ([]globalEntities.File, error) {
	branch := strings.TrimPrefix(repo.DefaultBranch, "refs/heads/")
	endpoint := fmt.Sprintf(
		"/api/v1/repos/%s/%s/git/trees/%s?recursive=true",
		repo.Organization, repo.Name, branch,
	)

	resp, err := p.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo tree: %w", err)
	}

	var result struct {
		Tree []forgejoTreeEntry `json:"tree"`
	}
	if unmarshalErr := json.Unmarshal(resp, &result); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse tree response: %w", unmarshalErr)
	}

	var files []globalEntities.File
	for _, entry := range result.Tree {
		if pattern != "" && !strings.HasSuffix(entry.Path, pattern) {
			continue
		}
		files = append(files, globalEntities.File{
			Path:     entry.Path,
			ObjectID: entry.SHA,
			IsDir:    entry.Type == "tree",
		})
	}

	return files, nil
}

func (p *Provider) GetTags(
	ctx context.Context,
	repo globalEntities.Repository,
) ([]string, error) {
	var allTags []string
	page := 1

	for {
		endpoint := fmt.Sprintf(
			"/api/v1/repos/%s/%s/tags?page=%d&limit=%d",
			repo.Organization, repo.Name, page, perPage,
		)

		resp, err := p.doRequest(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to list tags: %w", err)
		}

		var tags []forgejoTag
		if unmarshalErr := json.Unmarshal(resp, &tags); unmarshalErr != nil {
			return nil, fmt.Errorf("failed to parse tags response: %w", unmarshalErr)
		}

		if len(tags) == 0 {
			break
		}

		for _, tag := range tags {
			allTags = append(allTags, tag.Name)
		}

		if len(tags) < perPage {
			break
		}
		page++
	}

	globalHelpers.SortVersionsDescending(allTags)
	return allTags, nil
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
	baseBranch := strings.TrimPrefix(input.BaseBranch, "refs/heads/")

	// create the branch from the base
	createBranchEndpoint := fmt.Sprintf(
		"/api/v1/repos/%s/%s/branches",
		repo.Organization, repo.Name,
	)
	branchBody := map[string]string{
		"new_branch_name": input.BranchName,
		"old_branch_name": baseBranch,
	}
	if _, err := p.doRequest(ctx, http.MethodPost, createBranchEndpoint, branchBody); err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	// apply each file change
	for _, change := range input.Changes {
		filePath := strings.TrimPrefix(change.Path, "/")
		fileEndpoint := fmt.Sprintf(
			"/api/v1/repos/%s/%s/contents/%s",
			repo.Organization, repo.Name, filePath,
		)

		changeType := strings.ToLower(strings.TrimSpace(change.ChangeType))

		// handle delete operations explicitly
		if changeType == "delete" {
			fileBody := map[string]string{
				"message": input.CommitMessage,
				"branch":  input.BranchName,
			}

			if _, err := p.doRequest(ctx, http.MethodDelete, fileEndpoint, fileBody); err != nil {
				return fmt.Errorf("failed to delete file %q: %w", filePath, err)
			}

			continue
		}

		// treat unknown, non-empty change types as an error
		if changeType != "" && changeType != "add" && changeType != "edit" && changeType != "create" &&
			changeType != "update" {
			return fmt.Errorf("unsupported change type %q for file %q", change.ChangeType, filePath)
		}

		// default behavior: create or update file content
		encoded := base64.StdEncoding.EncodeToString([]byte(change.Content))
		fileBody := map[string]string{
			"content": encoded,
			"message": input.CommitMessage,
			"branch":  input.BranchName,
		}

		if _, err := p.doRequest(ctx, http.MethodPost, fileEndpoint, fileBody); err != nil {
			return fmt.Errorf("failed to create/update file %q: %w", filePath, err)
		}
	}

	return nil
}
