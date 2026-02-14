package github

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	gh "github.com/google/go-github/v66/github"
	log "github.com/sirupsen/logrus"

	"github.com/rios0rios0/gitforge/domain/entities"
	"github.com/rios0rios0/gitforge/domain/repositories"
	"github.com/rios0rios0/gitforge/support"
)

const (
	providerName = "github"
	perPage      = 100
	blobMode     = "100644"
	blobType     = "blob"
)

// Provider implements ForgeProvider, FileAccessProvider, and LocalGitAuthProvider for GitHub.
type Provider struct {
	token  string
	client *gh.Client
}

// NewProvider creates a new GitHub provider with the given token.
func NewProvider(token string) repositories.ForgeProvider {
	client := gh.NewClient(nil).WithAuthToken(token)
	return &Provider{
		token:  token,
		client: client,
	}
}

func (p *Provider) Name() string      { return providerName }
func (p *Provider) AuthToken() string { return p.token }

func (p *Provider) MatchesURL(rawURL string) bool {
	return strings.Contains(rawURL, "github.com")
}

// --- LocalGitAuthProvider ---

func (p *Provider) GetServiceType() entities.ServiceType {
	return entities.GITHUB
}

func (p *Provider) PrepareCloneURL(url string) string {
	return url
}

func (p *Provider) ConfigureTransport() {
	// GitHub doesn't need special transport configuration
}

func (p *Provider) GetAuthMethods(_ string) []transport.AuthMethod {
	var authMethods []transport.AuthMethod

	if p.token != "" {
		log.Infof("Using access token to authenticate with GitHub")
		authMethods = append(authMethods, &http.BasicAuth{
			Username: "x-access-token",
			Password: p.token,
		})
	}

	return authMethods
}

// --- ForgeProvider: Discovery ---

func (p *Provider) DiscoverRepositories(
	ctx context.Context,
	org string,
) ([]entities.Repository, error) {
	repos, err := p.discoverOrgRepos(ctx, org)
	if err != nil {
		log.Warnf("Failed to list org repos for %q, falling back to user repos: %v", org, err)
		return p.discoverUserRepos(ctx, org)
	}
	return repos, nil
}

func (p *Provider) discoverOrgRepos(
	ctx context.Context,
	org string,
) ([]entities.Repository, error) {
	var allRepos []entities.Repository
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
) ([]entities.Repository, error) {
	var allRepos []entities.Repository
	opts := &gh.RepositoryListByUserOptions{
		ListOptions: gh.ListOptions{PerPage: perPage},
		Type:        "owner",
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

func githubRepoToDomain(r *gh.Repository, org string) entities.Repository {
	defaultBranch := "main"
	if r.DefaultBranch != nil {
		defaultBranch = *r.DefaultBranch
	}
	return entities.Repository{
		ID:            strconv.FormatInt(r.GetID(), 10),
		Name:          r.GetName(),
		Organization:  org,
		DefaultBranch: "refs/heads/" + defaultBranch,
		RemoteURL:     r.GetCloneURL(),
		SSHURL:        r.GetSSHURL(),
		ProviderName:  providerName,
	}
}

// --- ForgeProvider: Pull Requests ---

func (p *Provider) CreatePullRequest(
	ctx context.Context,
	repo entities.Repository,
	input entities.PullRequestInput,
) (*entities.PullRequest, error) {
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

	return &entities.PullRequest{
		ID:     pr.GetNumber(),
		Title:  pr.GetTitle(),
		URL:    pr.GetHTMLURL(),
		Status: pr.GetState(),
	}, nil
}

func (p *Provider) PullRequestExists(
	ctx context.Context,
	repo entities.Repository,
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

func (p *Provider) CloneURL(repo entities.Repository) string {
	remoteURL := repo.RemoteURL
	if remoteURL == "" {
		remoteURL = fmt.Sprintf(
			"https://github.com/%s/%s.git",
			repo.Organization, repo.Name,
		)
	}
	return strings.Replace(
		remoteURL,
		"https://",
		"https://x-access-token:"+p.token+"@",
		1,
	)
}

// --- FileAccessProvider ---

func (p *Provider) GetFileContent(
	ctx context.Context,
	repo entities.Repository,
	path string,
) (string, error) {
	fileContent, _, _, err := p.client.Repositories.GetContents(
		ctx, repo.Organization, repo.Name, path,
		&gh.RepositoryContentGetOptions{},
	)
	if err != nil {
		return "", fmt.Errorf("failed to get file %q: %w", path, err)
	}
	if fileContent == nil {
		return "", fmt.Errorf("path %q is a directory, not a file", path)
	}

	content, err := fileContent.GetContent()
	if err != nil {
		return "", fmt.Errorf("failed to decode file content: %w", err)
	}

	return content, nil
}

func (p *Provider) ListFiles(
	ctx context.Context,
	repo entities.Repository,
	pattern string,
) ([]entities.File, error) {
	tree, _, err := p.client.Git.GetTree(
		ctx, repo.Organization, repo.Name,
		strings.TrimPrefix(repo.DefaultBranch, "refs/heads/"),
		true,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo tree: %w", err)
	}

	var files []entities.File
	for _, entry := range tree.Entries {
		if pattern != "" && !strings.HasSuffix(entry.GetPath(), pattern) {
			continue
		}
		files = append(files, entities.File{
			Path:     entry.GetPath(),
			ObjectID: entry.GetSHA(),
			IsDir:    entry.GetType() == "tree",
		})
	}

	return files, nil
}

func (p *Provider) GetTags(
	ctx context.Context,
	repo entities.Repository,
) ([]string, error) {
	var allTags []string
	opts := &gh.ListOptions{PerPage: perPage}

	for {
		tags, resp, err := p.client.Repositories.ListTags(
			ctx, repo.Organization, repo.Name, opts,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to list tags: %w", err)
		}

		for _, tag := range tags {
			allTags = append(allTags, tag.GetName())
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
	owner := repo.Organization
	repoName := repo.Name

	baseBranch := strings.TrimPrefix(input.BaseBranch, "refs/heads/")
	baseRef, _, err := p.client.Git.GetRef(
		ctx, owner, repoName, "refs/heads/"+baseBranch,
	)
	if err != nil {
		return fmt.Errorf("failed to get base branch ref: %w", err)
	}
	baseSHA := baseRef.Object.GetSHA()

	baseCommit, _, err := p.client.Git.GetCommit(
		ctx, owner, repoName, baseSHA,
	)
	if err != nil {
		return fmt.Errorf("failed to get base commit: %w", err)
	}

	var treeEntries []*gh.TreeEntry
	for _, change := range input.Changes {
		content := change.Content
		path := strings.TrimPrefix(change.Path, "/")
		mode := blobMode
		entryType := blobType
		treeEntries = append(treeEntries, &gh.TreeEntry{
			Path:    &path,
			Mode:    &mode,
			Type:    &entryType,
			Content: &content,
		})
	}

	newTree, _, err := p.client.Git.CreateTree(
		ctx, owner, repoName, baseCommit.Tree.GetSHA(), treeEntries,
	)
	if err != nil {
		return fmt.Errorf("failed to create tree: %w", err)
	}

	newCommit, _, err := p.client.Git.CreateCommit(
		ctx, owner, repoName,
		&gh.Commit{
			Message: &input.CommitMessage,
			Tree:    newTree,
			Parents: []*gh.Commit{{SHA: &baseSHA}},
		},
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to create commit: %w", err)
	}

	branchRef := "refs/heads/" + input.BranchName
	_, _, err = p.client.Git.CreateRef(
		ctx, owner, repoName,
		&gh.Reference{
			Ref:    &branchRef,
			Object: &gh.GitObject{SHA: newCommit.SHA},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	return nil
}
