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

func (p *Provider) PullRequestExists(
	ctx context.Context,
	repo globalEntities.Repository,
	sourceBranch string,
) (bool, error) {
	prs, _, err := p.client.PullRequests.List(
		ctx, repo.Organization, repo.Name,
		&gh.PullRequestListOptions{
			Head:  repo.Organization + ":" + sourceBranch,
			State: "open",
		},
	)
	if err != nil {
		return false, fmt.Errorf("failed to list pull requests: %w", err)
	}

	return len(prs) > 0, nil
}
