package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	gh "github.com/google/go-github/v66/github"
	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
	log "github.com/sirupsen/logrus"
)

func (p *Provider) DiscoverRepositories(
	ctx context.Context,
	org string,
) ([]globalEntities.Repository, error) {
	repos, err := p.discoverOrgRepos(ctx, org)
	if err != nil {
		var ghErr *gh.ErrorResponse
		if errors.As(err, &ghErr) && ghErr.Response.StatusCode == http.StatusNotFound {
			log.Debugf("Not an organization %q, falling back to user repos: %v", org, err)
			return p.discoverUserRepos(ctx, org)
		}
		return nil, err
	}
	return repos, nil
}

func (p *Provider) discoverOrgRepos(
	ctx context.Context,
	org string,
) ([]globalEntities.Repository, error) {
	var allRepos []globalEntities.Repository
	opts := &gh.RepositoryListByOrgOptions{
		ListOptions: gh.ListOptions{PerPage: perPage},
	}

	for {
		repos, resp, err := p.client.Repositories.ListByOrg(ctx, org, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list org repos: %w", err)
		}

		for _, r := range repos {
			allRepos = append(allRepos, githubRepoToDomain(r, org))
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allRepos, nil
}

func (p *Provider) discoverUserRepos(
	ctx context.Context,
	user string,
) ([]globalEntities.Repository, error) {
	var allRepos []globalEntities.Repository
	opts := &gh.RepositoryListByUserOptions{
		ListOptions: gh.ListOptions{PerPage: perPage},
		Type:        "all",
	}

	for {
		repos, resp, err := p.client.Repositories.ListByUser(ctx, user, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list user repos for %q: %w", user, err)
		}

		for _, r := range repos {
			allRepos = append(allRepos, githubRepoToDomain(r, user))
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allRepos, nil
}

func githubRepoToDomain(r *gh.Repository, org string) globalEntities.Repository {
	defaultBranch := "main"
	if r.DefaultBranch != nil {
		defaultBranch = *r.DefaultBranch
	}
	return globalEntities.Repository{
		ID:            strconv.FormatInt(r.GetID(), 10),
		Name:          r.GetName(),
		Organization:  org,
		DefaultBranch: "refs/heads/" + defaultBranch,
		RemoteURL:     r.GetCloneURL(),
		SSHURL:        r.GetSSHURL(),
		ProviderName:  providerName,
		IsFork:        r.GetFork(),
		IsArchived:    r.GetArchived(),
	}
}
