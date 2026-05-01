package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	gh "github.com/google/go-github/v66/github"
	log "github.com/sirupsen/logrus"

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

// ErrThreadStatusUpdateUnsupported is returned by the GitHub provider when callers
// attempt to update the status of a review thread. GitHub has no direct REST
// equivalent of Azure DevOps' thread status field; thread resolution is exposed
// only via the GraphQL resolveReviewThread mutation, which is not yet wired up.
var ErrThreadStatusUpdateUnsupported = errors.New(
	"updating pull request thread status is not supported on GitHub",
)

// GitHub PR review event strings accepted by PullRequests.CreateReview.
// Defined as constants so the verdict-mapping switch and the inline-comment
// path share a single source of truth.
const (
	reviewEventApprove        = "APPROVE"
	reviewEventRequestChanges = "REQUEST_CHANGES"
	reviewEventComment        = "COMMENT"
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
				IsDraft:      pr.GetDraft(),
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

// PostPullRequestComment posts an issue-level comment on the pull request via
// the Issues REST API. GitHub's REST surface does not expose a per-comment
// "thread status" field analogous to Azure DevOps, so any
// entities.WithThreadStatus value supplied by the caller is silently ignored
// here. The variadic argument exists purely so callers can write provider-
// agnostic code against the ReviewProvider interface.
func (p *Provider) PostPullRequestComment(
	ctx context.Context,
	repo globalEntities.Repository,
	prID int,
	body string,
	_ ...globalEntities.CommentOption,
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

// PostPullRequestThreadComment posts an inline review comment on a specific
// file and line via the GitHub Reviews REST API. GitHub does not expose a
// per-thread status field comparable to Azure DevOps, so any
// entities.WithThreadStatus value supplied by the caller is silently ignored
// here. The variadic argument exists purely so callers can write provider-
// agnostic code against the ReviewProvider interface.
func (p *Provider) PostPullRequestThreadComment(
	ctx context.Context,
	repo globalEntities.Repository,
	prID int,
	filePath string,
	line int,
	body string,
	_ ...globalEntities.CommentOption,
) (int, error) {
	event := reviewEventComment
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

// SubmitPullRequestReview records a native PR review on GitHub via the
// PullRequests.CreateReview endpoint so the verdict shows up in the platform's
// reviewer panel. The verdict is mapped to the GitHub `event` field per the
// table on the ReviewProvider interface; ReviewVerdictWaitingForAuthor has no
// native equivalent on GitHub and is collapsed to REQUEST_CHANGES.
//
// A self-review attempt (the authenticated identity is the PR author) returns
// HTTP 422 from GitHub with a body whose `message` matches selfReviewErrFragment.
// That specific case is logged at warn level and swallowed so the caller's
// fallback comment path still has a chance to surface the verdict — failing the
// whole review here would cause silent regressions on bot-authored PRs (e.g.
// autobump runs). Any other 422 (missing fields, invalid PR state, etc.) is
// returned as a wrapped error so genuine validation failures stay visible.
//
// A ReviewVerdictComment with an empty body is skipped without an API call:
// GitHub rejects empty COMMENT reviews with 422 ("Body is too short") and
// nothing meaningful would surface anyway.
func (p *Provider) SubmitPullRequestReview(
	ctx context.Context,
	repo globalEntities.Repository,
	prID int,
	sub globalEntities.ReviewSubmission,
) error {
	event, ok := mapVerdictToReviewEvent(sub.Verdict)
	if !ok {
		return fmt.Errorf("unsupported review verdict %q", sub.Verdict)
	}

	if event == reviewEventComment && sub.Body == "" {
		return nil
	}

	body := sub.Body
	req := &gh.PullRequestReviewRequest{Event: &event}
	if body != "" {
		req.Body = &body
	}

	_, _, err := p.client.PullRequests.CreateReview(
		ctx, repo.Organization, repo.Name, prID, req,
	)
	if err != nil {
		if isSelfReviewError(err) {
			log.WithFields(log.Fields{
				"repo":    repo.Organization + "/" + repo.Name,
				"prID":    prID,
				"verdict": sub.Verdict,
			}).Warnf(
				"GitHub rejected native review submission (self-review): %v",
				err,
			)
			return nil
		}
		return fmt.Errorf("failed to submit pull request review: %w", err)
	}

	return nil
}

// selfReviewErrFragment is the substring GitHub puts in the 422 response body
// when the authenticated user tries to review their own pull request. Matching
// the string keeps the swallow narrow so unrelated 422 validation failures
// (missing fields, invalid PR state, etc.) still surface as errors.
const selfReviewErrFragment = "Can not approve your own pull request"

// isSelfReviewError reports whether err is a GitHub 422 caused by a self-review
// attempt. It checks both the typed ErrorResponse message and the raw body so
// fixture / replay payloads where only one is populated still match.
func isSelfReviewError(err error) bool {
	var ghErr *gh.ErrorResponse
	if !errors.As(err, &ghErr) || ghErr.Response == nil {
		return false
	}
	if ghErr.Response.StatusCode != http.StatusUnprocessableEntity {
		return false
	}
	return strings.Contains(ghErr.Message, selfReviewErrFragment) ||
		strings.Contains(err.Error(), selfReviewErrFragment)
}

// mapVerdictToReviewEvent translates a gitforge ReviewVerdict to the GitHub
// `event` string accepted by CreateReview. WaitingForAuthor collapses to
// REQUEST_CHANGES because GitHub does not have a separate "waiting on author"
// state; surfacing it as REQUEST_CHANGES at least keeps the verdict visible.
func mapVerdictToReviewEvent(v globalEntities.ReviewVerdict) (string, bool) {
	switch v {
	case globalEntities.ReviewVerdictApprove:
		return reviewEventApprove, true
	case globalEntities.ReviewVerdictRequestChanges,
		globalEntities.ReviewVerdictWaitingForAuthor:
		return reviewEventRequestChanges, true
	case globalEntities.ReviewVerdictComment:
		return reviewEventComment, true
	}
	return "", false
}

// GetPullRequestStatus returns the GitHub pull request state. GitHub uses
// "open" or "closed" for `state`; closed PRs that were merged are reported as
// "merged" so callers can distinguish abandoned PRs from merged ones.
//
// The merged signal is read off `merged_at` (`MergedAt`) rather than the
// `merged` boolean. The boolean is reliably set on the single-PR `GET
// /repos/.../pulls/{N}` response this method uses, but `merged_at` is
// the canonical timestamp populated whenever the PR was merged at any
// point — using the timestamp avoids a class of false negatives on
// fixture / replay payloads where `merged` is omitted (the Go client's
// `GetMerged()` returns the zero value for a missing field, which would
// silently report a merged PR as `closed`). Pinned per Copilot review on
// PR #86 thread `PRRT_kwDORQWb3M5-6QA0`.
func (p *Provider) GetPullRequestStatus(
	ctx context.Context,
	repo globalEntities.Repository,
	prID int,
) (string, error) {
	pr, _, err := p.client.PullRequests.Get(ctx, repo.Organization, repo.Name, prID)
	if err != nil {
		return "", fmt.Errorf("failed to get pull request: %w", err)
	}

	if pr.GetState() == "closed" && !pr.GetMergedAt().IsZero() {
		return "merged", nil
	}

	return pr.GetState(), nil
}
