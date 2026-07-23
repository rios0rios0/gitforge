package github

import (
	"context"
	"fmt"
	"strings"

	gh "github.com/google/go-github/v66/github"
	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

func (p *Provider) CreatePullRequest(
	ctx context.Context,
	repo globalEntities.Repository,
	input globalEntities.PullRequestInput,
) (*globalEntities.PullRequest, error) {
	sourceBranch := strings.TrimPrefix(input.SourceBranch, "refs/heads/")
	targetBranch := strings.TrimPrefix(input.TargetBranch, "refs/heads/")
	maintainerCanModify := true

	pr, _, err := p.client.PullRequests.Create(
		ctx, repo.Organization, repo.Name,
		&gh.NewPullRequest{
			Title:               &input.Title,
			Head:                &sourceBranch,
			Base:                &targetBranch,
			Body:                &input.Description,
			MaintainerCanModify: &maintainerCanModify,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create pull request: %w", err)
	}

	return &globalEntities.PullRequest{
		ID:     pr.GetNumber(),
		Title:  pr.GetTitle(),
		URL:    pr.GetHTMLURL(),
		Status: pr.GetState(),
	}, nil
}

// findOpenPullRequestNumber returns the number of the open pull request whose head is
// sourceBranch. The boolean reports whether such a pull request was found.
func (p *Provider) findOpenPullRequestNumber(
	ctx context.Context,
	repo globalEntities.Repository,
	sourceBranch string,
) (int, bool, error) {
	prs, _, err := p.client.PullRequests.List(
		ctx, repo.Organization, repo.Name,
		&gh.PullRequestListOptions{
			Head:  repo.Organization + ":" + sourceBranch,
			State: prStateOpen,
		},
	)
	if err != nil {
		return 0, false, fmt.Errorf("failed to list pull requests: %w", err)
	}

	if len(prs) == 0 {
		return 0, false, nil
	}

	return prs[0].GetNumber(), true, nil
}

func (p *Provider) PullRequestExists(
	ctx context.Context,
	repo globalEntities.Repository,
	sourceBranch string,
) (bool, error) {
	_, found, err := p.findOpenPullRequestNumber(ctx, repo, sourceBranch)
	return found, err
}

func (p *Provider) ClosePullRequest(
	ctx context.Context,
	repo globalEntities.Repository,
	sourceBranch string,
) (bool, error) {
	number, found, err := p.findOpenPullRequestNumber(ctx, repo, sourceBranch)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}

	state := prStateClosed
	if _, _, editErr := p.client.PullRequests.Edit(
		ctx, repo.Organization, repo.Name, number,
		&gh.PullRequest{State: &state},
	); editErr != nil {
		return false, fmt.Errorf("failed to close pull request %d: %w", number, editErr)
	}

	return true, nil
}
