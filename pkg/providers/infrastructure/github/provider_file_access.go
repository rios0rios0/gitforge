package github

import (
	"context"
	"fmt"
	"strings"

	gh "github.com/google/go-github/v66/github"
	globalDomain "github.com/rios0rios0/gitforge/pkg/global/domain"
	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

func (p *Provider) GetFileContent(
	ctx context.Context,
	repo globalEntities.Repository,
	path string,
) (string, error) {
	fileContent, _, _, err := p.client.Repositories.GetContents(
		ctx, repo.Organization, repo.Name, path,
		&gh.RepositoryContentGetOptions{},
	)
	if err != nil {
		return "", fmt.Errorf("failed to get file %q: %w", path, err)
	}
	if fileContent == nil {
		return "", fmt.Errorf("path %q is a directory, not a file", path)
	}

	content, err := fileContent.GetContent()
	if err != nil {
		return "", fmt.Errorf("failed to decode file content: %w", err)
	}

	return content, nil
}

func (p *Provider) ListFiles(
	ctx context.Context,
	repo globalEntities.Repository,
	pattern string,
) ([]globalEntities.File, error) {
	tree, _, err := p.client.Git.GetTree(
		ctx, repo.Organization, repo.Name,
		strings.TrimPrefix(repo.DefaultBranch, "refs/heads/"),
		true,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo tree: %w", err)
	}

	var files []globalEntities.File
	for _, entry := range tree.Entries {
		if pattern != "" && !strings.HasSuffix(entry.GetPath(), pattern) {
			continue
		}
		files = append(files, globalEntities.File{
			Path:     entry.GetPath(),
			ObjectID: entry.GetSHA(),
			IsDir:    entry.GetType() == "tree",
		})
	}

	return files, nil
}

func (p *Provider) GetTags(
	ctx context.Context,
	repo globalEntities.Repository,
) ([]string, error) {
	var allTags []string
	opts := &gh.ListOptions{PerPage: perPage}

	for {
		tags, resp, err := p.client.Repositories.ListTags(
			ctx, repo.Organization, repo.Name, opts,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to list tags: %w", err)
		}

		for _, tag := range tags {
			allTags = append(allTags, tag.GetName())
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	globalDomain.SortVersionsDescending(allTags)
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
	owner := repo.Organization
	repoName := repo.Name

	baseBranch := strings.TrimPrefix(input.BaseBranch, "refs/heads/")
	baseRef, _, err := p.client.Git.GetRef(
		ctx, owner, repoName, "refs/heads/"+baseBranch,
	)
	if err != nil {
		return fmt.Errorf("failed to get base branch ref: %w", err)
	}
	baseSHA := baseRef.Object.GetSHA()

	baseCommit, _, err := p.client.Git.GetCommit(
		ctx, owner, repoName, baseSHA,
	)
	if err != nil {
		return fmt.Errorf("failed to get base commit: %w", err)
	}

	var treeEntries []*gh.TreeEntry
	for _, change := range input.Changes {
		content := change.Content
		path := strings.TrimPrefix(change.Path, "/")
		mode := blobMode
		entryType := blobType
		treeEntries = append(treeEntries, &gh.TreeEntry{
			Path:    &path,
			Mode:    &mode,
			Type:    &entryType,
			Content: &content,
		})
	}

	newTree, _, err := p.client.Git.CreateTree(
		ctx, owner, repoName, baseCommit.Tree.GetSHA(), treeEntries,
	)
	if err != nil {
		return fmt.Errorf("failed to create tree: %w", err)
	}

	newCommit, _, err := p.client.Git.CreateCommit(
		ctx, owner, repoName,
		&gh.Commit{
			Message: &input.CommitMessage,
			Tree:    newTree,
			Parents: []*gh.Commit{{SHA: &baseSHA}},
		},
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to create commit: %w", err)
	}

	branchRef := "refs/heads/" + input.BranchName
	_, _, err = p.client.Git.CreateRef(
		ctx, owner, repoName,
		&gh.Reference{
			Ref:    &branchRef,
			Object: &gh.GitObject{SHA: newCommit.SHA},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	return nil
}
