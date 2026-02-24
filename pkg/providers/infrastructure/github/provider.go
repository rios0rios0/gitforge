package github

import (
	"fmt"
	"net/url"

	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	gh "github.com/google/go-github/v66/github"
	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
	log "github.com/sirupsen/logrus"
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
