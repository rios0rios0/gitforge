package gitlab

import (
	"context"
	"fmt"
	"strconv"

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
	log "github.com/sirupsen/logrus"
	gl "gitlab.com/gitlab-org/api/client-go"
)

func (p *Provider) DiscoverRepositories(
	ctx context.Context,
	group string,
) ([]globalEntities.Repository, error) {
	if p.client == nil {
		return nil, errClientNotInitialized
	}

	repos, err := p.discoverGroupProjects(ctx, group)
	if err != nil {
		log.Warnf("Failed to list group projects for %q, falling back to user projects: %v", group, err)
		return p.discoverUserProjects(ctx, group)
	}
	return repos, nil
}

func (p *Provider) discoverGroupProjects(
	ctx context.Context,
	group string,
) ([]globalEntities.Repository, error) {
	var allRepos []globalEntities.Repository
	includeSubgroups := true
	opts := &gl.ListGroupProjectsOptions{
		ListOptions:      gl.ListOptions{PerPage: perPage},
		IncludeSubGroups: &includeSubgroups,
	}

	for {
		projects, resp, err := p.client.Groups.ListGroupProjects(
			group, opts, gl.WithContext(ctx),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to list group projects: %w", err)
		}

		for _, proj := range projects {
			allRepos = append(allRepos, gitlabProjectToDomain(proj, group))
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allRepos, nil
}

func (p *Provider) discoverUserProjects(
	ctx context.Context,
	user string,
) ([]globalEntities.Repository, error) {
	var allRepos []globalEntities.Repository
	owned := true
	opts := &gl.ListProjectsOptions{
		ListOptions: gl.ListOptions{PerPage: perPage},
		Owned:       &owned,
	}

	for {
		projects, resp, err := p.client.Projects.ListProjects(
			opts, gl.WithContext(ctx),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to list projects for %q: %w", user, err)
		}

		for _, proj := range projects {
			allRepos = append(allRepos, gitlabProjectToDomain(proj, user))
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allRepos, nil
}

func gitlabProjectToDomain(proj *gl.Project, org string) globalEntities.Repository {
	defaultBranch := "main"
	if proj.DefaultBranch != "" {
		defaultBranch = proj.DefaultBranch
	}
	return globalEntities.Repository{
		ID:            strconv.FormatInt(proj.ID, 10),
		Name:          proj.Path,
		Organization:  org,
		DefaultBranch: "refs/heads/" + defaultBranch,
		RemoteURL:     proj.HTTPURLToRepo,
		SSHURL:        proj.SSHURLToRepo,
		ProviderName:  providerName,
	}
}
