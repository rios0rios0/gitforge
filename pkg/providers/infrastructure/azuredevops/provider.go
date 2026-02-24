package azuredevops

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing/protocol/packp/capability"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gohttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
	log "github.com/sirupsen/logrus"
)

const (
	providerName     = "azuredevops"
	apiVersion       = "7.0"
	httpTimeout      = 30 * time.Second
	httpStatusOKMin  = 200
	httpStatusOKMax  = 300
	paginationHeader = "X-Ms-Continuationtoken"
	allZeroObjectID  = "0000000000000000000000000000000000000000"
)

// Provider implements ForgeProvider, FileAccessProvider, and LocalGitAuthProvider for Azure DevOps.
type Provider struct {
	token      string
	httpClient *http.Client
}

// NewProvider creates a new Azure DevOps provider with the given PAT.
func NewProvider(token string) globalEntities.ForgeProvider {
	return &Provider{
		token: token,
		httpClient: &http.Client{
			Timeout: httpTimeout,
		},
	}
}

func (p *Provider) Name() string      { return providerName }
func (p *Provider) AuthToken() string { return p.token }

func (p *Provider) MatchesURL(rawURL string) bool {
	return strings.Contains(rawURL, "dev.azure.com")
}

// --- LocalGitAuthProvider ---

func (p *Provider) GetServiceType() globalEntities.ServiceType {
	return globalEntities.AZUREDEVOPS
}

func (p *Provider) PrepareCloneURL(rawURL string) string {
	return stripUsernameFromURL(rawURL)
}

func (p *Provider) ConfigureTransport() {
	transport.UnsupportedCapabilities = []capability.Capability{ //nolint:reassign // required for Azure DevOps
		capability.ThinPack,
	}
}

func (p *Provider) GetAuthMethods(_ string) []transport.AuthMethod {
	var authMethods []transport.AuthMethod

	if p.token != "" {
		log.Infof("Using access token to authenticate with Azure DevOps")
		authMethods = append(authMethods, &gohttp.BasicAuth{
			Username: "pat",
			Password: p.token,
		})
	}

	return authMethods
}

func (p *Provider) CloneURL(repo globalEntities.Repository) string {
	remoteURL := repo.RemoteURL
	if remoteURL == "" {
		remoteURL = fmt.Sprintf(
			"https://dev.azure.com/%s/%s/_git/%s",
			repo.Organization, repo.Project, repo.Name,
		)
	}

	parsed, err := url.Parse(remoteURL)
	if err != nil {
		return remoteURL
	}

	parsed.User = url.UserPassword("pat", p.token)

	return parsed.String()
}
