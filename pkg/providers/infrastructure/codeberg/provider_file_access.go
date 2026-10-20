package codeberg

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
	globalHelpers "github.com/rios0rios0/gitforge/v4/pkg/global/domain/helpers"
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
	entries, err := p.listTreeEntries(ctx, repo)
	if err != nil {
		return nil, err
	}

	var files []globalEntities.File
	for _, entry := range entries {
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

// treesPerPage is the `per_page` asked for on every page of a Git trees listing:
// Forgejo's DEFAULT_GIT_TREES_PER_PAGE, documented as both the default and the
// most it serves, 1000 unless an instance lowers it — see
// https://forgejo.org/docs/latest/admin/config-cheat-sheet/. listTreeEntries does
// not rely on the server serving that many.
const treesPerPage = 1000

// forgejoTreePage is one page of a Git trees listing.
type forgejoTreePage struct {
	SHA        string             `json:"sha"`
	Tree       []forgejoTreeEntry `json:"tree"`
	Truncated  bool               `json:"truncated"`
	TotalCount int                `json:"total_count"`
}

// listTreeEntries returns every entry of the tree on the repository's default
// branch, read page by page.
//
// Forgejo pages its Git trees listing by `page` and `per_page` and reports in
// `total_count` how many entries the whole listing holds, so reading only the
// first page returned at most one page of a larger repository. The walk reads
// every page in turn, the first by branch and the rest from the SHA the first
// page reported, so a push in the middle cannot mix two trees. It ends once it
// holds `total_count` entries or a page comes back with `truncated` unset:
// Forgejo sets that flag on every page of a listing longer than one page, the
// last included, so the flag alone cannot mark the end. A page that brings no
// new path before then means the listing cannot be completed — a server that
// ignores `page`, for one — and is an error rather than a partial list.
func (p *Provider) listTreeEntries(
	ctx context.Context,
	repo globalEntities.Repository,
) ([]forgejoTreeEntry, error) {
	ref := strings.TrimPrefix(repo.DefaultBranch, "refs/heads/")
	var entries []forgejoTreeEntry
	listed := make(map[string]struct{})
	for page := 1; ; page++ {
		tree, err := p.fetchTreePage(ctx, repo, ref, page)
		if err != nil {
			return nil, err
		}

		added := 0
		for _, entry := range tree.Tree {
			if _, seen := listed[entry.Path]; seen {
				continue
			}
			listed[entry.Path] = struct{}{}
			entries = append(entries, entry)
			added++
		}

		if !tree.Truncated || (tree.TotalCount > 0 && len(entries) >= tree.TotalCount) {
			return entries, nil
		}
		if added == 0 {
			return nil, fmt.Errorf(
				"%w: tree page %d brought no entry not already read", errIncompleteListing, page,
			)
		}
		if tree.SHA != "" {
			ref = tree.SHA
		}
	}
}

// fetchTreePage reads one page of the recursive tree listing of ref.
func (p *Provider) fetchTreePage(
	ctx context.Context,
	repo globalEntities.Repository,
	ref string,
	page int,
) (forgejoTreePage, error) {
	endpoint := fmt.Sprintf(
		"/api/v1/repos/%s/%s/git/trees/%s?recursive=true&page=%d&per_page=%d",
		repo.Organization, repo.Name, ref, page, treesPerPage,
	)

	resp, err := p.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return forgejoTreePage{}, fmt.Errorf("failed to get repo tree: %w", err)
	}

	var tree forgejoTreePage
	if unmarshalErr := json.Unmarshal(resp, &tree); unmarshalErr != nil {
		return forgejoTreePage{}, fmt.Errorf("failed to parse tree response: %w", unmarshalErr)
	}

	return tree, nil
}

func (p *Provider) GetTags(
	ctx context.Context,
	repo globalEntities.Repository,
) ([]string, error) {
	var allTags []string
	walkErr := walkPages(
		"tags of "+repo.Organization+"/"+repo.Name,
		func(page int) ([]forgejoTag, error) {
			endpoint := fmt.Sprintf(
				"/api/v1/repos/%s/%s/tags?page=%d&limit=%d",
				repo.Organization, repo.Name, page, perPage,
			)
			resp, err := p.doRequest(ctx, http.MethodGet, endpoint, nil)
			if err != nil {
				return nil, err
			}

			var tags []forgejoTag
			if unmarshalErr := json.Unmarshal(resp, &tags); unmarshalErr != nil {
				return nil, fmt.Errorf("failed to parse tags response: %w", unmarshalErr)
			}
			return tags, nil
		},
		func(tag forgejoTag) string { return tag.Name },
		func(tag forgejoTag) bool {
			allTags = append(allTags, tag.Name)
			return false
		},
	)
	if walkErr != nil {
		return nil, fmt.Errorf("failed to list tags: %w", walkErr)
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
