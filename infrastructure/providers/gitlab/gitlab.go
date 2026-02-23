package gitlab

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	log "github.com/sirupsen/logrus"
	gl "gitlab.com/gitlab-org/api/client-go"

	"github.com/rios0rios0/gitforge/domain/entities"
	"github.com/rios0rios0/gitforge/domain/repositories"
	"github.com/rios0rios0/gitforge/support"
)

const (
	providerName = "gitlab"
	perPage      = 100
)

var errClientNotInitialized = errors.New("gitlab client not initialized")

// Provider implements ForgeProvider, FileAccessProvider, and LocalGitAuthProvider for GitLab.
type Provider struct {
	token  string
	client *gl.Client
}

// NewProvider creates a new GitLab provider with the given token.
func NewProvider(token string) repositories.ForgeProvider {
	client, err := gl.NewClient(token)
	if err != nil {
		return &Provider{token: token, client: nil}
	}
	return &Provider{
		token:  token,
		client: client,
	}
}

func (p *Provider) Name() string      { return providerName }
func (p *Provider) AuthToken() string { return p.token }

func (p *Provider) MatchesURL(rawURL string) bool {
	return strings.Contains(rawURL, "gitlab.com")
}

// --- LocalGitAuthProvider ---

func (p *Provider) GetServiceType() entities.ServiceType {
	return entities.GITLAB
}

func (p *Provider) PrepareCloneURL(url string) string {
	return url
}

func (p *Provider) ConfigureTransport() {
	// GitLab doesn't need special transport configuration
}

func (p *Provider) GetAuthMethods(username string) []transport.AuthMethod {
	var authMethods []transport.AuthMethod

	if p.token != "" {
		if username == "" {
			username = "oauth2"
		}
		log.Infof("Using access token to authenticate with GitLab")
		authMethods = append(authMethods, &http.BasicAuth{
			Username: username,
			Password: p.token,
		})
	}

	return authMethods
}

// --- ForgeProvider: Discovery ---

func (p *Provider) DiscoverRepositories(
	ctx context.Context,
	group string,
) ([]entities.Repository, error) {
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
) ([]entities.Repository, error) {
	var allRepos []entities.Repository
	opts := &gl.ListGroupProjectsOptions{
		ListOptions:      gl.ListOptions{PerPage: perPage},
		IncludeSubGroups: new(true),
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
) ([]entities.Repository, error) {
	var allRepos []entities.Repository
	opts := &gl.ListProjectsOptions{
		ListOptions: gl.ListOptions{PerPage: perPage},
		Owned:       new(true),
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

func gitlabProjectToDomain(proj *gl.Project, org string) entities.Repository {
	defaultBranch := "main"
	if proj.DefaultBranch != "" {
		defaultBranch = proj.DefaultBranch
	}
	return entities.Repository{
		ID:            strconv.FormatInt(proj.ID, 10),
		Name:          proj.Path,
		Organization:  org,
		DefaultBranch: "refs/heads/" + defaultBranch,
		RemoteURL:     proj.HTTPURLToRepo,
		SSHURL:        proj.SSHURLToRepo,
		ProviderName:  providerName,
	}
}

// --- ForgeProvider: Pull Requests ---

func (p *Provider) CreatePullRequest(
	ctx context.Context,
	repo entities.Repository,
	input entities.PullRequestInput,
) (*entities.PullRequest, error) {
	if p.client == nil {
		return nil, errClientNotInitialized
	}

	pid := repo.Organization + "/" + repo.Name
	sourceBranch := strings.TrimPrefix(input.SourceBranch, "refs/heads/")
	targetBranch := strings.TrimPrefix(input.TargetBranch, "refs/heads/")

	mr, _, err := p.client.MergeRequests.CreateMergeRequest(
		pid,
		&gl.CreateMergeRequestOptions{
			Title:              new(input.Title),
			Description:        new(input.Description),
			SourceBranch:       new(sourceBranch),
			TargetBranch:       new(targetBranch),
			RemoveSourceBranch: new(true),
		},
		gl.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create merge request: %w", err)
	}

	return &entities.PullRequest{
		ID:     int(mr.IID),
		Title:  mr.Title,
		URL:    mr.WebURL,
		Status: mr.State,
	}, nil
}

func (p *Provider) PullRequestExists(
	ctx context.Context,
	repo entities.Repository,
	sourceBranch string,
) (bool, error) {
	if p.client == nil {
		return false, errClientNotInitialized
	}

	pid := repo.Organization + "/" + repo.Name
	state := "opened"
	mrs, _, err := p.client.MergeRequests.ListProjectMergeRequests(
		pid,
		&gl.ListProjectMergeRequestsOptions{
			SourceBranch: new(sourceBranch),
			State:        new(state),
		},
		gl.WithContext(ctx),
	)
	if err != nil {
		return false, fmt.Errorf("failed to list merge requests: %w", err)
	}

	return len(mrs) > 0, nil
}

func (p *Provider) CloneURL(repo entities.Repository) string {
	remoteURL := repo.RemoteURL
	if remoteURL == "" {
		remoteURL = fmt.Sprintf(
			"https://gitlab.com/%s/%s.git",
			repo.Organization, repo.Name,
		)
	}

	parsed, err := url.Parse(remoteURL)
	if err != nil {
		return remoteURL
	}

	parsed.User = url.UserPassword("oauth2", p.token)

	return parsed.String()
}

// --- FileAccessProvider ---

func (p *Provider) GetFileContent(
	ctx context.Context,
	repo entities.Repository,
	path string,
) (string, error) {
	if p.client == nil {
		return "", errClientNotInitialized
	}

	branch := strings.TrimPrefix(repo.DefaultBranch, "refs/heads/")
	raw, _, err := p.client.RepositoryFiles.GetRawFile(
		repo.Organization+"/"+repo.Name, path,
		&gl.GetRawFileOptions{Ref: new(branch)},
		gl.WithContext(ctx),
	)
	if err != nil {
		return "", fmt.Errorf("failed to get file %q: %w", path, err)
	}

	return string(raw), nil
}

func (p *Provider) ListFiles(
	ctx context.Context,
	repo entities.Repository,
	pattern string,
) ([]entities.File, error) {
	if p.client == nil {
		return nil, errClientNotInitialized
	}

	branch := strings.TrimPrefix(repo.DefaultBranch, "refs/heads/")
	recursive := true
	var allFiles []entities.File
	opts := &gl.ListTreeOptions{
		ListOptions: gl.ListOptions{PerPage: perPage},
		Ref:         new(branch),
		Recursive:   &recursive,
	}

	for {
		nodes, resp, err := p.client.Repositories.ListTree(
			repo.Organization+"/"+repo.Name,
			opts,
			gl.WithContext(ctx),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to list tree: %w", err)
		}

		for _, node := range nodes {
			if pattern != "" && !strings.HasSuffix(node.Path, pattern) {
				continue
			}
			allFiles = append(allFiles, entities.File{
				Path:     node.Path,
				ObjectID: node.ID,
				IsDir:    node.Type == "tree",
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allFiles, nil
}

func (p *Provider) GetTags(
	ctx context.Context,
	repo entities.Repository,
) ([]string, error) {
	if p.client == nil {
		return nil, errClientNotInitialized
	}

	var allTags []string
	opts := &gl.ListTagsOptions{
		ListOptions: gl.ListOptions{PerPage: perPage},
	}

	pid := repo.Organization + "/" + repo.Name
	for {
		tags, resp, err := p.client.Tags.ListTags(
			pid, opts, gl.WithContext(ctx),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to list tags: %w", err)
		}

		for _, tag := range tags {
			allTags = append(allTags, tag.Name)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	support.SortVersionsDescending(allTags)
	return allTags, nil
}

func (p *Provider) HasFile(
	ctx context.Context,
	repo entities.Repository,
	path string,
) bool {
	_, err := p.GetFileContent(ctx, repo, path)
	return err == nil
}

func (p *Provider) CreateBranchWithChanges(
	ctx context.Context,
	repo entities.Repository,
	input entities.BranchInput,
) error {
	if p.client == nil {
		return errClientNotInitialized
	}

	pid := repo.Organization + "/" + repo.Name
	baseBranch := strings.TrimPrefix(input.BaseBranch, "refs/heads/")

	_, _, err := p.client.Branches.CreateBranch(pid, &gl.CreateBranchOptions{
		Branch: new(input.BranchName),
		Ref:    new(baseBranch),
	}, gl.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	var actions []*gl.CommitActionOptions
	for _, change := range input.Changes {
		action := gl.FileUpdate
		switch change.ChangeType {
		case "add":
			action = gl.FileCreate
		case "delete":
			action = gl.FileDelete
		}
		filePath := strings.TrimPrefix(change.Path, "/")
		content := change.Content
		actions = append(actions, &gl.CommitActionOptions{
			Action:   &action,
			FilePath: &filePath,
			Content:  &content,
		})
	}

	_, _, err = p.client.Commits.CreateCommit(
		pid,
		&gl.CreateCommitOptions{
			Branch:        new(input.BranchName),
			CommitMessage: new(input.CommitMessage),
			Actions:       actions,
		},
		gl.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("failed to create commit: %w", err)
	}

	return nil
}
