package gitlab

import (
	"context"
	"fmt"
	"strings"

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
	gl "gitlab.com/gitlab-org/api/client-go"
)

func (p *Provider) CreatePullRequest(
	ctx context.Context,
	repo globalEntities.Repository,
	input globalEntities.PullRequestInput,
) (*globalEntities.PullRequest, error) {
	if p.client == nil {
		return nil, errClientNotInitialized
	}

	pid := repo.Organization + "/" + repo.Name
	sourceBranch := strings.TrimPrefix(input.SourceBranch, "refs/heads/")
	targetBranch := strings.TrimPrefix(input.TargetBranch, "refs/heads/")

	title := input.Title
	description := input.Description
	removeSourceBranch := true
	mr, _, err := p.client.MergeRequests.CreateMergeRequest(
		pid,
		&gl.CreateMergeRequestOptions{
			Title:              &title,
			Description:        &description,
			SourceBranch:       &sourceBranch,
			TargetBranch:       &targetBranch,
			RemoveSourceBranch: &removeSourceBranch,
		},
		gl.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create merge request: %w", err)
	}

	return &globalEntities.PullRequest{
		ID:     int(mr.IID),
		Title:  mr.Title,
		URL:    mr.WebURL,
		Status: mr.State,
	}, nil
}

func (p *Provider) PullRequestExists(
	ctx context.Context,
	repo globalEntities.Repository,
	sourceBranch string,
) (bool, error) {
	if p.client == nil {
		return false, errClientNotInitialized
	}

	pid := repo.Organization + "/" + repo.Name
	state := "opened"
	mrs, _, err := p.client.MergeRequests.ListProjectMergeRequests(
		pid,
		&gl.ListProjectMergeRequestsOptions{
			SourceBranch: &sourceBranch,
			State:        &state,
		},
		gl.WithContext(ctx),
	)
	if err != nil {
		return false, fmt.Errorf("failed to list merge requests: %w", err)
	}

	return len(mrs) > 0, nil
}
