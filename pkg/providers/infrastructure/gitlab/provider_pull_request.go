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

// findOpenMergeRequestIID returns the IID of the opened merge request whose source
// branch is sourceBranch. The boolean reports whether such a merge request was found.
func (p *Provider) findOpenMergeRequestIID(
	ctx context.Context,
	repo globalEntities.Repository,
	sourceBranch string,
) (int64, bool, error) {
	if p.client == nil {
		return 0, false, errClientNotInitialized
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
		return 0, false, fmt.Errorf("failed to list merge requests: %w", err)
	}

	if len(mrs) == 0 {
		return 0, false, nil
	}

	return mrs[0].IID, true, nil
}

func (p *Provider) PullRequestExists(
	ctx context.Context,
	repo globalEntities.Repository,
	sourceBranch string,
) (bool, error) {
	_, found, err := p.findOpenMergeRequestIID(ctx, repo, sourceBranch)
	return found, err
}

func (p *Provider) ClosePullRequest(
	ctx context.Context,
	repo globalEntities.Repository,
	sourceBranch string,
) (bool, error) {
	iid, found, err := p.findOpenMergeRequestIID(ctx, repo, sourceBranch)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}

	pid := repo.Organization + "/" + repo.Name
	stateEvent := "close"
	if _, _, updateErr := p.client.MergeRequests.UpdateMergeRequest(
		pid, iid,
		&gl.UpdateMergeRequestOptions{StateEvent: &stateEvent},
		gl.WithContext(ctx),
	); updateErr != nil {
		return false, fmt.Errorf("failed to close merge request %d: %w", iid, updateErr)
	}

	return true, nil
}
