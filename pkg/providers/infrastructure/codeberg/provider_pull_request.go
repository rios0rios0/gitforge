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

func (p *Provider) PullRequestExists(
	ctx context.Context,
	repo globalEntities.Repository,
	sourceBranch string,
) (bool, error) {
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
			return false, fmt.Errorf("failed to list pull requests: %w", err)
		}

		var prs []forgejoPR
		if unmarshalErr := json.Unmarshal(resp, &prs); unmarshalErr != nil {
			return false, fmt.Errorf("failed to parse pull requests response: %w", unmarshalErr)
		}

		if len(prs) == 0 {
			return false, nil
		}

		for _, pr := range prs {
			if pr.Head.Ref == branch {
				return true, nil
			}
		}

		if len(prs) < limit {
			return false, nil
		}

		page++
	}
}
