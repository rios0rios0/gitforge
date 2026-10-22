package github

import (
	"errors"
	"fmt"
	"net/url"

	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	gh "github.com/google/go-github/v74/github"
	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
	log "github.com/sirupsen/logrus"
)

const (
	providerName = "github"
	perPage      = 100
	blobMode     = "100644"
	blobType     = "blob"
	treeType     = "tree"

	// defaultBranchName is the fallback branch name when GitHub does not report one.
	defaultBranchName = "main"

	// Pull request state values reported by the GitHub API.
	prStateOpen   = "open"
	prStateClosed = "closed"
	prStateMerged = "merged"
)

// errIncompleteListing reports a listing that could not be read to its end: a
// page or tree level the API marked as cut short, or a walk that stopped making
// progress. Returning what was read so far would hand callers a partial listing
// they could not tell apart from a complete one.
var errIncompleteListing = errors.New("listing could not be read completely")

// Provider implements ForgeProvider, FileAccessProvider, and LocalGitAuthProvider for GitHub.
type Provider struct {
	token  string
	client *gh.Client
}

// NewProvider creates a new GitHub provider with the given token.
func NewProvider(token string) globalEntities.ForgeProvider {
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

func (p *Provider) GetServiceType() globalEntities.ServiceType {
	return globalEntities.GITHUB
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

func (p *Provider) CloneURL(repo globalEntities.Repository) string {
	remoteURL := repo.RemoteURL
	if remoteURL == "" {
		remoteURL = fmt.Sprintf(
			"https://github.com/%s/%s.git",
			repo.Organization, repo.Name,
		)
	}

	parsed, err := url.Parse(remoteURL)
	if err != nil {
		return remoteURL
	}

	parsed.User = url.UserPassword("x-access-token", p.token)

	return parsed.String()
}

func (p *Provider) SSHCloneURL(repo globalEntities.Repository, sshAlias string) string {
	host := "github.com"
	if sshAlias != "" {
		host = fmt.Sprintf("github.com-%s", sshAlias)
	}
	return fmt.Sprintf("git@%s:%s/%s.git", host, repo.Organization, repo.Name)
}
