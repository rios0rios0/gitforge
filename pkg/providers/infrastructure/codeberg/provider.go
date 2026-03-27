package codeberg

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing/transport"
	gohttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
	log "github.com/sirupsen/logrus"
)

const (
	providerName    = "codeberg"
	defaultBaseURL  = "https://codeberg.org"
	perPage         = 50
	httpTimeout     = 30 * time.Second
	httpStatusOKMin = 200
	httpStatusOKMax = 300
)

// Provider implements ForgeProvider, LocalGitAuthProvider, and MirrorProvider for Codeberg (Forgejo).
type Provider struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

// NewProvider creates a new Codeberg provider with the given API token.
func NewProvider(token string) globalEntities.ForgeProvider {
	return &Provider{
		token:   token,
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: httpTimeout,
		},
	}
}

func (p *Provider) Name() string      { return providerName }
func (p *Provider) AuthToken() string { return p.token }

func (p *Provider) MatchesURL(rawURL string) bool {
	return strings.Contains(rawURL, "codeberg.org")
}

// --- LocalGitAuthProvider ---

func (p *Provider) GetServiceType() globalEntities.ServiceType {
	return globalEntities.CODEBERG
}

func (p *Provider) PrepareCloneURL(cloneURL string) string {
	return cloneURL
}

func (p *Provider) ConfigureTransport() {
	// Codeberg doesn't need special transport configuration
}

func (p *Provider) GetAuthMethods(_ string) []transport.AuthMethod {
	var authMethods []transport.AuthMethod

	if p.token != "" {
		log.Infof("Using access token to authenticate with Codeberg")
		authMethods = append(authMethods, &gohttp.BasicAuth{
			Username: "token",
			Password: p.token,
		})
	}

	return authMethods
}

func (p *Provider) CloneURL(repo globalEntities.Repository) string {
	remoteURL := repo.RemoteURL
	if remoteURL == "" {
		remoteURL = fmt.Sprintf(
			"%s/%s/%s.git",
			p.baseURL, repo.Organization, repo.Name,
		)
	}

	parsed, err := url.Parse(remoteURL)
	if err != nil {
		return remoteURL
	}

	parsed.User = url.UserPassword("token", p.token)

	return parsed.String()
}

func (p *Provider) SSHCloneURL(repo globalEntities.Repository, sshAlias string) string {
	host := "codeberg.org"
	if sshAlias != "" {
		host = fmt.Sprintf("codeberg.org-%s", sshAlias)
	}
	return fmt.Sprintf("git@%s:%s/%s.git", host, repo.Organization, repo.Name)
}
