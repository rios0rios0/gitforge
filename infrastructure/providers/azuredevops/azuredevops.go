package azuredevops

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing/protocol/packp/capability"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gohttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	log "github.com/sirupsen/logrus"

	"github.com/rios0rios0/gitforge/domain/entities"
	"github.com/rios0rios0/gitforge/domain/repositories"
	forgeSup "github.com/rios0rios0/gitforge/support"
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
func NewProvider(token string) repositories.ForgeProvider {
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

func (p *Provider) GetServiceType() entities.ServiceType {
	return entities.AZUREDEVOPS
}

func (p *Provider) PrepareCloneURL(rawURL string) string {
	return forgeSup.StripUsernameFromURL(rawURL)
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

// --- ForgeProvider: Discovery ---

func (p *Provider) DiscoverRepositories(
	ctx context.Context,
	org string,
) ([]entities.Repository, error) {
	baseURL := normalizeOrgURL(org)

	projects, err := p.getProjects(ctx, baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get projects: %w", err)
	}

	var repos []entities.Repository
	for _, proj := range projects {
		projRepos, repoErr := p.getRepositories(ctx, baseURL, proj.ID)
		if repoErr != nil {
			continue
		}
		for _, r := range projRepos {
			repos = append(repos, entities.Repository{
				ID:            r.ID,
				Name:          r.Name,
				Organization:  extractOrgName(baseURL),
				Project:       proj.Name,
				DefaultBranch: r.DefaultBranch,
				RemoteURL:     r.RemoteURL,
				SSHURL:        r.SSHURL,
				ProviderName:  providerName,
			})
		}
	}

	return repos, nil
}

// --- ForgeProvider: Pull Requests ---

func (p *Provider) CreatePullRequest(
	ctx context.Context,
	repo entities.Repository,
	input entities.PullRequestInput,
) (*entities.PullRequest, error) {
	baseURL := buildBaseURL(repo.Organization)

	body := map[string]any{
		"sourceRefName": input.SourceBranch,
		"targetRefName": input.TargetBranch,
		"title":         input.Title,
		"description":   input.Description,
	}

	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/pullrequests?api-version=%s",
		repo.Project, repo.ID, apiVersion,
	)

	resp, err := p.doRequest(ctx, baseURL, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create PR: %w", err)
	}

	var prResp struct {
		PullRequestID int    `json:"pullRequestId"`
		Title         string `json:"title"`
		URL           string `json:"url"`
		Status        string `json:"status"`
	}
	if unmarshalErr := json.Unmarshal(resp, &prResp); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse PR response: %w", unmarshalErr)
	}

	pr := &entities.PullRequest{
		ID:     prResp.PullRequestID,
		Title:  prResp.Title,
		URL:    prResp.URL,
		Status: prResp.Status,
	}

	if input.AutoComplete {
		updateBody := map[string]any{
			"autoCompleteSetBy": map[string]string{"id": "me"},
		}
		updateEndpoint := fmt.Sprintf(
			"/%s/_apis/git/repositories/%s/pullrequests/%d?api-version=%s",
			repo.Project, repo.ID, pr.ID, apiVersion,
		)
		_, _ = p.doRequest(ctx, baseURL, http.MethodPatch, updateEndpoint, updateBody)
	}

	return pr, nil
}

func (p *Provider) PullRequestExists(
	ctx context.Context,
	repo entities.Repository,
	sourceBranch string,
) (bool, error) {
	baseURL := buildBaseURL(repo.Organization)
	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/pullrequests?searchCriteria.sourceRefName=refs/heads/%s&searchCriteria.status=active&api-version=%s",
		repo.Project,
		repo.ID,
		sourceBranch,
		apiVersion,
	)

	resp, err := p.doRequest(ctx, baseURL, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, err
	}

	var result struct {
		Count int `json:"count"`
	}
	if unmarshalErr := json.Unmarshal(resp, &result); unmarshalErr != nil {
		return false, fmt.Errorf("failed to parse PR list response: %w", unmarshalErr)
	}

	return result.Count > 0, nil
}

func (p *Provider) CloneURL(repo entities.Repository) string {
	remoteURL := repo.RemoteURL
	if remoteURL == "" {
		remoteURL = fmt.Sprintf(
			"https://dev.azure.com/%s/%s/_git/%s",
			repo.Organization, repo.Project, repo.Name,
		)
	}

	parsed, err := url.Parse(remoteURL)
	if err != nil {
		return strings.Replace(
			remoteURL, "https://", "https://pat:"+p.token+"@", 1,
		)
	}

	parsed.User = url.UserPassword("pat", p.token)

	return parsed.String()
}

// --- FileAccessProvider ---

func (p *Provider) GetFileContent(
	ctx context.Context,
	repo entities.Repository,
	path string,
) (string, error) {
	baseURL := buildBaseURL(repo.Organization)
	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/items?path=%s&api-version=%s",
		repo.Project, repo.ID, url.QueryEscape(path), apiVersion,
	)

	resp, err := p.doRequest(ctx, baseURL, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}

	return string(resp), nil
}

func (p *Provider) ListFiles(
	ctx context.Context,
	repo entities.Repository,
	pattern string,
) ([]entities.File, error) {
	baseURL := buildBaseURL(repo.Organization)
	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/items?recursionLevel=Full&api-version=%s",
		repo.Project, repo.ID, apiVersion,
	)

	resp, err := p.doRequest(ctx, baseURL, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Value []struct {
			ObjectID      string `json:"objectId"`
			GitObjectType string `json:"gitObjectType"`
			Path          string `json:"path"`
		} `json:"value"`
	}
	if unmarshalErr := json.Unmarshal(resp, &result); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse items response: %w", unmarshalErr)
	}

	var files []entities.File
	for _, item := range result.Value {
		isDir := item.GitObjectType != "blob"
		if pattern != "" && !strings.HasSuffix(item.Path, pattern) {
			continue
		}
		files = append(files, entities.File{
			Path:     item.Path,
			ObjectID: item.ObjectID,
			IsDir:    isDir,
		})
	}

	return files, nil
}

func (p *Provider) GetTags(
	ctx context.Context,
	repo entities.Repository,
) ([]string, error) {
	baseURL := buildBaseURL(repo.Organization)
	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/refs?filter=tags&api-version=%s",
		repo.Project, repo.ID, apiVersion,
	)

	resp, err := p.doRequest(ctx, baseURL, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Value []struct {
			Name string `json:"name"`
		} `json:"value"`
	}
	if unmarshalErr := json.Unmarshal(resp, &result); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse tags response: %w", unmarshalErr)
	}

	var tags []string
	for _, ref := range result.Value {
		tags = append(tags, strings.TrimPrefix(ref.Name, "refs/tags/"))
	}

	forgeSup.SortVersionsDescending(tags)
	return tags, nil
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
	baseURL := buildBaseURL(repo.Organization)

	baseCommitID, err := p.getCommitID(ctx, baseURL, repo)
	if err != nil {
		return fmt.Errorf("failed to get base branch commit: %w", err)
	}

	var fileChanges []map[string]any
	for _, change := range input.Changes {
		entry := map[string]any{
			"changeType": change.ChangeType,
			"item": map[string]string{
				"path": change.Path,
			},
			"newContent": map[string]string{
				"content":     base64.StdEncoding.EncodeToString([]byte(change.Content)),
				"contentType": "base64encoded",
			},
		}
		fileChanges = append(fileChanges, entry)
	}

	pushBody := map[string]any{
		"refUpdates": []map[string]string{
			{
				"name":        "refs/heads/" + input.BranchName,
				"oldObjectId": allZeroObjectID,
			},
		},
		"commits": []map[string]any{
			{
				"comment": input.CommitMessage,
				"changes": fileChanges,
				"parents": []string{baseCommitID},
			},
		},
	}

	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/pushes?api-version=%s",
		repo.Project, repo.ID, apiVersion,
	)

	_, err = p.doRequest(ctx, baseURL, http.MethodPost, endpoint, pushBody)
	if err != nil {
		return fmt.Errorf("failed to push changes: %w", err)
	}

	return nil
}

// --- internal helpers ---

type adoProject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type adoRepository struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	RemoteURL     string `json:"remoteUrl"`
	SSHURL        string `json:"sshUrl"`
	DefaultBranch string `json:"defaultBranch"`
}

func (p *Provider) getProjects(
	ctx context.Context,
	baseURL string,
) ([]adoProject, error) {
	var all []adoProject
	continuationToken := ""

	for {
		endpoint := "/_apis/projects?api-version=" + apiVersion
		if continuationToken != "" {
			endpoint += "&continuationToken=" + continuationToken
		}

		resp, headers, err := p.doRequestWithHeaders(
			ctx, baseURL, http.MethodGet, endpoint, nil,
		)
		if err != nil {
			return nil, err
		}

		var result struct {
			Value []adoProject `json:"value"`
		}
		if unmarshalErr := json.Unmarshal(resp, &result); unmarshalErr != nil {
			return nil, fmt.Errorf("failed to parse projects response: %w", unmarshalErr)
		}

		all = append(all, result.Value...)
		continuationToken = headers.Get(paginationHeader)
		if continuationToken == "" {
			break
		}
	}

	return all, nil
}

func (p *Provider) getRepositories(
	ctx context.Context,
	baseURL, projectID string,
) ([]adoRepository, error) {
	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories?api-version=%s",
		projectID, apiVersion,
	)

	resp, err := p.doRequest(ctx, baseURL, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Value []adoRepository `json:"value"`
	}
	if unmarshalErr := json.Unmarshal(resp, &result); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse repositories response: %w", unmarshalErr)
	}

	return result.Value, nil
}

func (p *Provider) getCommitID(
	ctx context.Context,
	baseURL string,
	repo entities.Repository,
) (string, error) {
	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s?api-version=%s",
		repo.Project, repo.ID, apiVersion,
	)

	resp, err := p.doRequest(ctx, baseURL, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}

	var repoInfo struct {
		DefaultBranch string `json:"defaultBranch"`
	}
	if unmarshalErr := json.Unmarshal(resp, &repoInfo); unmarshalErr != nil {
		return "", fmt.Errorf("failed to parse repository info: %w", unmarshalErr)
	}

	branchName := strings.TrimPrefix(repoInfo.DefaultBranch, "refs/heads/")
	branchEndpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/refs?filter=heads/%s&api-version=%s",
		repo.Project, repo.ID, branchName, apiVersion,
	)

	branchResp, branchErr := p.doRequest(
		ctx, baseURL, http.MethodGet, branchEndpoint, nil,
	)
	if branchErr != nil {
		return "", branchErr
	}

	var branchResult struct {
		Value []struct {
			ObjectID string `json:"objectId"`
		} `json:"value"`
	}
	if unmarshalErr := json.Unmarshal(branchResp, &branchResult); unmarshalErr != nil {
		return "", fmt.Errorf("failed to parse branch response: %w", unmarshalErr)
	}
	if len(branchResult.Value) == 0 {
		return "", errors.New("default branch not found")
	}

	return branchResult.Value[0].ObjectID, nil
}

func (p *Provider) doRequest(
	ctx context.Context,
	baseURL, method, endpoint string,
	body any,
) ([]byte, error) {
	resp, _, err := p.doRequestWithHeaders(ctx, baseURL, method, endpoint, body)
	return resp, err
}

func (p *Provider) doRequestWithHeaders(
	ctx context.Context,
	baseURL, method, endpoint string,
	body any,
) ([]byte, http.Header, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			return nil, nil, fmt.Errorf("failed to marshal request body: %w", marshalErr)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	fullURL := baseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	auth := base64.StdEncoding.EncodeToString([]byte(":" + p.token))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req) //nolint:gosec // URL is constructed from trusted config, not user input
	if err != nil {
		return nil, nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < httpStatusOKMin || resp.StatusCode >= httpStatusOKMax {
		return nil, nil, fmt.Errorf(
			"API error (status %d): %s",
			resp.StatusCode, string(respBody),
		)
	}

	return respBody, resp.Header, nil
}

// --- URL helpers ---

func normalizeOrgURL(org string) string {
	org = strings.TrimSuffix(org, "/")
	if !strings.HasPrefix(org, "https://") {
		org = "https://dev.azure.com/" + org
	}
	return org
}

func buildBaseURL(orgName string) string {
	if orgName == "" {
		return "https://dev.azure.com"
	}
	return "https://dev.azure.com/" + strings.Split(orgName, "/")[0]
}

func extractOrgName(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return baseURL
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return u.Host
}
