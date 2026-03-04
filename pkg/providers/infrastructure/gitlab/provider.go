package gitlab

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
	log "github.com/sirupsen/logrus"
	gl "gitlab.com/gitlab-org/api/client-go"
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
func NewProvider(token string) globalEntities.ForgeProvider {
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

func (p *Provider) GetServiceType() globalEntities.ServiceType {
	return globalEntities.GITLAB
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

func (p *Provider) CloneURL(repo globalEntities.Repository) string {
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
