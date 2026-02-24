package gitlab

import (
	"context"
	"fmt"
	"strings"

	globalDomain "github.com/rios0rios0/gitforge/pkg/global/domain"
	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
	gl "gitlab.com/gitlab-org/api/client-go"
)

func (p *Provider) GetFileContent(
	ctx context.Context,
	repo globalEntities.Repository,
	path string,
) (string, error) {
	if p.client == nil {
		return "", errClientNotInitialized
	}

	branch := strings.TrimPrefix(repo.DefaultBranch, "refs/heads/")
	raw, _, err := p.client.RepositoryFiles.GetRawFile(
		repo.Organization+"/"+repo.Name, path,
		&gl.GetRawFileOptions{Ref: new(branch)},
		gl.WithContext(ctx),
	)
	if err != nil {
		return "", fmt.Errorf("failed to get file %q: %w", path, err)
	}

	return string(raw), nil
}

func (p *Provider) ListFiles(
	ctx context.Context,
	repo globalEntities.Repository,
	pattern string,
) ([]globalEntities.File, error) {
	if p.client == nil {
		return nil, errClientNotInitialized
	}

	branch := strings.TrimPrefix(repo.DefaultBranch, "refs/heads/")
	recursive := true
	var allFiles []globalEntities.File
	opts := &gl.ListTreeOptions{
		ListOptions: gl.ListOptions{PerPage: perPage},
		Ref:         new(branch),
		Recursive:   &recursive,
	}

	for {
		nodes, resp, err := p.client.Repositories.ListTree(
			repo.Organization+"/"+repo.Name,
			opts,
			gl.WithContext(ctx),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to list tree: %w", err)
		}

		for _, node := range nodes {
			if pattern != "" && !strings.HasSuffix(node.Path, pattern) {
				continue
			}
			allFiles = append(allFiles, globalEntities.File{
				Path:     node.Path,
				ObjectID: node.ID,
				IsDir:    node.Type == "tree",
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allFiles, nil
}

func (p *Provider) GetTags(
	ctx context.Context,
	repo globalEntities.Repository,
) ([]string, error) {
	if p.client == nil {
		return nil, errClientNotInitialized
	}

	var allTags []string
	opts := &gl.ListTagsOptions{
		ListOptions: gl.ListOptions{PerPage: perPage},
	}

	pid := repo.Organization + "/" + repo.Name
	for {
		tags, resp, err := p.client.Tags.ListTags(
			pid, opts, gl.WithContext(ctx),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to list tags: %w", err)
		}

		for _, tag := range tags {
			allTags = append(allTags, tag.Name)
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
	if p.client == nil {
		return errClientNotInitialized
	}

	pid := repo.Organization + "/" + repo.Name
	baseBranch := strings.TrimPrefix(input.BaseBranch, "refs/heads/")

	_, _, err := p.client.Branches.CreateBranch(pid, &gl.CreateBranchOptions{
		Branch: new(input.BranchName),
		Ref:    new(baseBranch),
	}, gl.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	var actions []*gl.CommitActionOptions
	for _, change := range input.Changes {
		action := gl.FileUpdate
		switch change.ChangeType {
		case "add":
			action = gl.FileCreate
		case "delete":
			action = gl.FileDelete
		}
		filePath := strings.TrimPrefix(change.Path, "/")
		content := change.Content
		actions = append(actions, &gl.CommitActionOptions{
			Action:   &action,
			FilePath: &filePath,
			Content:  &content,
		})
	}

	_, _, err = p.client.Commits.CreateCommit(
		pid,
		&gl.CreateCommitOptions{
			Branch:        new(input.BranchName),
			CommitMessage: new(input.CommitMessage),
			Actions:       actions,
		},
		gl.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("failed to create commit: %w", err)
	}

	return nil
}
