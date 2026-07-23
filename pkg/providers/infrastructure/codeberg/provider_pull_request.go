package codeberg

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

type forgejoPR struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	HTMLURL string `json:"html_url"`
	State   string `json:"state"`
	Head    struct {
		Label string `json:"label"`
		Ref   string `json:"ref"`
	} `json:"head"`
}

func (p *Provider) CreatePullRequest(
	ctx context.Context,
	repo globalEntities.Repository,
	input globalEntities.PullRequestInput,
) (*globalEntities.PullRequest, error) {
	sourceBranch := strings.TrimPrefix(input.SourceBranch, "refs/heads/")
	targetBranch := strings.TrimPrefix(input.TargetBranch, "refs/heads/")

	endpoint := fmt.Sprintf(
		"/api/v1/repos/%s/%s/pulls",
		repo.Organization, repo.Name,
	)

	body := map[string]any{
		"title": input.Title,
		"head":  sourceBranch,
		"base":  targetBranch,
		"body":  input.Description,
	}

	resp, err := p.doRequest(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create pull request: %w", err)
	}

	var pr forgejoPR
	if unmarshalErr := json.Unmarshal(resp, &pr); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse pull request response: %w", unmarshalErr)
	}

	return &globalEntities.PullRequest{
		ID:     pr.Number,
		Title:  pr.Title,
		URL:    pr.HTMLURL,
		Status: pr.State,
	}, nil
}

// findOpenPullRequestNumber returns the number of the open pull request whose head ref
// is sourceBranch. The boolean reports whether such a pull request was found.
func (p *Provider) findOpenPullRequestNumber(
	ctx context.Context,
	repo globalEntities.Repository,
	sourceBranch string,
) (int, bool, error) {
	branch := strings.TrimPrefix(sourceBranch, "refs/heads/")

	page := 1
	const limit = 50

	for {
		endpoint := fmt.Sprintf(
			"/api/v1/repos/%s/%s/pulls?state=open&page=%d&limit=%d",
			repo.Organization, repo.Name, page, limit,
		)

		resp, err := p.doRequest(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return 0, false, fmt.Errorf("failed to list pull requests: %w", err)
		}

		var prs []forgejoPR
		if unmarshalErr := json.Unmarshal(resp, &prs); unmarshalErr != nil {
			return 0, false, fmt.Errorf("failed to parse pull requests response: %w", unmarshalErr)
		}

		if len(prs) == 0 {
			return 0, false, nil
		}

		for _, pr := range prs {
			if pr.Head.Ref == branch {
				return pr.Number, true, nil
			}
		}

		if len(prs) < limit {
			return 0, false, nil
		}

		page++
	}
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

	endpoint := fmt.Sprintf(
		"/api/v1/repos/%s/%s/pulls/%d",
		repo.Organization, repo.Name, number,
	)
	body := map[string]any{"state": "closed"}

	if _, closeErr := p.doRequest(ctx, http.MethodPatch, endpoint, body); closeErr != nil {
		return false, fmt.Errorf("failed to close pull request %d: %w", number, closeErr)
	}

	return true, nil
}
