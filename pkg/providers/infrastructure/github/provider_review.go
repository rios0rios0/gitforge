package github

import (
	"context"
	"errors"
	"fmt"

	gh "github.com/google/go-github/v66/github"
	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

// ErrThreadStatusUpdateUnsupported is returned by the GitHub provider when callers
// attempt to update the status of a review thread. GitHub has no direct REST
// equivalent of Azure DevOps' thread status field; thread resolution is exposed
// only via the GraphQL resolveReviewThread mutation, which is not yet wired up.
var ErrThreadStatusUpdateUnsupported = errors.New(
	"updating pull request thread status is not supported on GitHub",
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

func (p *Provider) GetPullRequestCheckStatus(
	ctx context.Context,
	repo globalEntities.Repository,
	prID int,
) (bool, error) {
	// get the PR to find the head SHA
	pr, _, err := p.client.PullRequests.Get(ctx, repo.Organization, repo.Name, prID)
	if err != nil {
		return false, fmt.Errorf("failed to get pull request: %w", err)
	}

	headSHA := pr.GetHead().GetSHA()

	// get combined status for the head commit
	combinedStatus, _, err := p.client.Repositories.GetCombinedStatus(
		ctx, repo.Organization, repo.Name, headSHA, nil,
	)
	if err != nil {
		return false, fmt.Errorf("failed to get combined status: %w", err)
	}

	// also check check suites (GitHub Actions uses check runs, not commit statuses)
	checkSuites, _, err := p.client.Checks.ListCheckSuitesForRef(
		ctx, repo.Organization, repo.Name, headSHA,
		&gh.ListCheckSuiteOptions{},
	)
	if err != nil {
		return false, fmt.Errorf("failed to list check suites: %w", err)
	}

	// if there are no statuses and no check suites, consider it as passed (no CI configured)
	hasStatuses := combinedStatus.GetTotalCount() > 0
	hasCheckSuites := checkSuites.GetTotal() > 0

	if !hasStatuses && !hasCheckSuites {
		return true, nil
	}

	// check combined status (legacy status API)
	if hasStatuses && combinedStatus.GetState() != "success" {
		return false, nil
	}

	// check all check suites (GitHub Actions)
	for _, suite := range checkSuites.CheckSuites {
		if suite.GetStatus() != "completed" {
			return false, nil
		}
		if suite.GetConclusion() != "success" && suite.GetConclusion() != "neutral" {
			return false, nil
		}
	}

	return true, nil
}

func (p *Provider) MergePullRequest(
	ctx context.Context,
	repo globalEntities.Repository,
	prID int,
	strategy string,
) error {
	mergeMethod := strategy
	if mergeMethod == "" {
		mergeMethod = "squash"
	}

	_, _, err := p.client.PullRequests.Merge(
		ctx, repo.Organization, repo.Name, prID,
		"",
		&gh.PullRequestOptions{MergeMethod: mergeMethod},
	)
	if err != nil {
		return fmt.Errorf("failed to merge pull request: %w", err)
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
) (int, error) {
	event := "COMMENT"
	review, _, err := p.client.PullRequests.CreateReview(
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
		return 0, fmt.Errorf("failed to post pull request thread comment: %w", err)
	}

	// GitHub returns a review ID rather than a thread ID; expose it under the
	// same "thread ID" abstraction so callers have a stable handle to reference
	// the inline review later.
	return int(review.GetID()), nil
}

// UpdatePullRequestThreadStatus is not supported on GitHub via the REST API.
// GitHub exposes thread resolution only through the GraphQL resolveReviewThread
// mutation; until that is wired up, this method always returns
// ErrThreadStatusUpdateUnsupported.
func (p *Provider) UpdatePullRequestThreadStatus(
	_ context.Context,
	_ globalEntities.Repository,
	_, _ int,
	_ string,
) error {
	return ErrThreadStatusUpdateUnsupported
}

// GetPullRequestStatus returns the GitHub pull request state. GitHub uses
// "open" or "closed" for `state`; closed PRs that were merged are reported as
// "merged" so callers can distinguish abandoned PRs from merged ones.
func (p *Provider) GetPullRequestStatus(
	ctx context.Context,
	repo globalEntities.Repository,
	prID int,
) (string, error) {
	pr, _, err := p.client.PullRequests.Get(ctx, repo.Organization, repo.Name, prID)
	if err != nil {
		return "", fmt.Errorf("failed to get pull request: %w", err)
	}

	if pr.GetState() == "closed" && pr.GetMerged() {
		return "merged", nil
	}

	return pr.GetState(), nil
}
