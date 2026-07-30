package azuredevops

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
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

	// httpStatusNonAuthoritative is the status Azure DevOps serves its sign-in
	// page with. It sits inside the 2xx success window but never carries an API
	// payload, so it has to be recognized before the body is parsed.
	httpStatusNonAuthoritative = 203
	httpStatusRedirectMin      = 300
	httpStatusRedirectMax      = 400

	// JSON payload keys reused across multiple API requests.
	jsonKeyItem            = "item"
	jsonKeyPath            = "path"
	jsonKeyContent         = "content"
	jsonKeyName            = "name"
	jsonKeySourceRefName   = "sourceRefName"
	jsonKeyTargetRefName   = "targetRefName"
	jsonKeyTitle           = "title"
	jsonKeyComments        = "comments"
	jsonKeyParentCommentID = "parentCommentId"
	jsonKeyCommentType     = "commentType"
	jsonKeyStatus          = "status"
	jsonKeyFilePath        = "filePath"
	jsonKeyLine            = "line"

	// prStatusAbandoned is the pull request status that closes a PR on Azure DevOps.
	prStatusAbandoned = "abandoned"

	// logFieldPRID is the structured-log field name for the pull request ID.
	logFieldPRID = "prID"
)

// Provider implements ForgeProvider, FileAccessProvider, and LocalGitAuthProvider for Azure DevOps.
type Provider struct {
	token      string
	httpClient *http.Client

	// reviewerIDMu guards reviewerIDCache and reviewerIDOnces so concurrent
	// callers from different organizations can each lazily resolve their own
	// reviewer ID. The cache is keyed by organization because the bot's
	// identity is org-scoped on Azure DevOps; a single Provider can be reused
	// across orgs with the same PAT, so a per-org [sync.Once] is required —
	// reusing a single Once across orgs would silently return the first org's
	// cached ID for every subsequent organization.
	reviewerIDMu     sync.Mutex
	reviewerIDOnces  map[string]*sync.Once
	reviewerIDCache  map[string]string
	reviewerIDErrors map[string]error
}

// NewProvider creates a new Azure DevOps provider with the given PAT.
func NewProvider(token string) globalEntities.ForgeProvider {
	return &Provider{
		token: token,
		httpClient: &http.Client{
			Timeout: httpTimeout,
			// Azure DevOps answers an unauthenticated REST call with a redirect to
			// a sign-in page rather than 401. Following that redirect turns a clear
			// authentication failure into an HTML page served as 203, which looks
			// like success to every status check downstream, so the redirect is
			// handed back to the caller untouched instead.
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		reviewerIDOnces:  make(map[string]*sync.Once),
		reviewerIDCache:  make(map[string]string),
		reviewerIDErrors: make(map[string]error),
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

func (p *Provider) SSHCloneURL(repo globalEntities.Repository, sshAlias string) string {
	host := "ssh.dev.azure.com"
	if sshAlias != "" {
		host = fmt.Sprintf("dev.azure.com-%s", sshAlias)
	}
	return fmt.Sprintf("git@%s:v3/%s/%s/%s", host, repo.Organization, repo.Project, repo.Name)
}
