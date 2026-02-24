package github

import (
	"context"
	"fmt"

	gh "github.com/google/go-github/v66/github"
	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

// --- ReviewProvider ---

func (p *Provider) ListOpenPullRequests(
	ctx context.Context,
	repo globalEntities.Repository,
) ([]globalEntities.PullRequestDetail, error) {
	var allPRs []globalEntities.PullRequestDetail
	opts := &gh.PullRequestListOptions{
		State:       "open",
		ListOptions: gh.ListOptions{PerPage: perPage},
	}

	for {
		prs, resp, err := p.client.PullRequests.List(
			ctx, repo.Organization, repo.Name, opts,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to list open pull requests: %w", err)
		}

		for _, pr := range prs {
			allPRs = append(allPRs, globalEntities.PullRequestDetail{
				PullRequest: globalEntities.PullRequest{
					ID:     pr.GetNumber(),
					Title:  pr.GetTitle(),
					URL:    pr.GetHTMLURL(),
					Status: pr.GetState(),
				},
				SourceBranch: pr.GetHead().GetRef(),
				TargetBranch: pr.GetBase().GetRef(),
				Author:       pr.GetUser().GetLogin(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allPRs, nil
}

func (p *Provider) GetPullRequestDiff(
	ctx context.Context,
	repo globalEntities.Repository,
	prID int,
) (string, error) {
	diff, _, err := p.client.PullRequests.GetRaw(
		ctx, repo.Organization, repo.Name, prID, gh.RawOptions{Type: gh.Diff},
	)
	if err != nil {
		return "", fmt.Errorf("failed to get pull request diff: %w", err)
	}

	return diff, nil
}

func (p *Provider) GetPullRequestFiles(
	ctx context.Context,
	repo globalEntities.Repository,
	prID int,
) ([]globalEntities.PullRequestFile, error) {
	var allFiles []globalEntities.PullRequestFile
	opts := &gh.ListOptions{PerPage: perPage}

	for {
		files, resp, err := p.client.PullRequests.ListFiles(
			ctx, repo.Organization, repo.Name, prID, opts,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to list pull request files: %w", err)
		}

		for _, f := range files {
			allFiles = append(allFiles, globalEntities.PullRequestFile{
				Path:      f.GetFilename(),
				OldPath:   f.GetPreviousFilename(),
				Status:    f.GetStatus(),
				Additions: f.GetAdditions(),
				Deletions: f.GetDeletions(),
				Patch:     f.GetPatch(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allFiles, nil
}

func (p *Provider) PostPullRequestComment(
	ctx context.Context,
	repo globalEntities.Repository,
	prID int,
	body string,
) error {
	_, _, err := p.client.Issues.CreateComment(
		ctx, repo.Organization, repo.Name, prID,
		&gh.IssueComment{Body: &body},
	)
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
	event := "COMMENT"
	_, _, err := p.client.PullRequests.CreateReview(
		ctx, repo.Organization, repo.Name, prID,
		&gh.PullRequestReviewRequest{
			Event: &event,
			Comments: []*gh.DraftReviewComment{
				{
					Path: &filePath,
					Line: &line,
					Body: &body,
				},
			},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to post pull request thread comment: %w", err)
	}

	return nil
}
