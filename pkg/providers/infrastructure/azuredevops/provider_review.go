package azuredevops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
	log "github.com/sirupsen/logrus"

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

// --- ReviewProvider ---

func (p *Provider) ListOpenPullRequests(
	ctx context.Context,
	repo globalEntities.Repository,
) ([]globalEntities.PullRequestDetail, error) {
	baseURL := buildBaseURL(repo.Organization)
	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/pullrequests?searchCriteria.status=active&api-version=%s",
		repo.Project, resolveRepoIdentifier(repo), apiVersion,
	)

	resp, err := p.doRequest(ctx, baseURL, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list open pull requests: %w", err)
	}

	var result struct {
		Value []struct {
			PullRequestID int    `json:"pullRequestId"`
			Title         string `json:"title"`
			Status        string `json:"status"`
			IsDraft       bool   `json:"isDraft"`
			SourceRefName string `json:"sourceRefName"`
			TargetRefName string `json:"targetRefName"`
			URL           string `json:"url"`
			CreatedBy     struct {
				DisplayName string `json:"displayName"`
			} `json:"createdBy"`
		} `json:"value"`
	}
	if unmarshalErr := json.Unmarshal(resp, &result); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse pull requests response: %w", unmarshalErr)
	}

	var prs []globalEntities.PullRequestDetail
	for _, pr := range result.Value {
		if pr.IsDraft {
			continue
		}
		prs = append(prs, globalEntities.PullRequestDetail{
			PullRequest: globalEntities.PullRequest{
				ID:     pr.PullRequestID,
				Title:  pr.Title,
				URL:    pr.URL,
				Status: pr.Status,
			},
			SourceBranch: strings.TrimPrefix(pr.SourceRefName, "refs/heads/"),
			TargetBranch: strings.TrimPrefix(pr.TargetRefName, "refs/heads/"),
			Author:       pr.CreatedBy.DisplayName,
		})
	}

	return prs, nil
}

func (p *Provider) GetPullRequestDiff(
	ctx context.Context,
	repo globalEntities.Repository,
	prID int,
) (string, error) {
	// get the PR details for source/target branches
	baseURL := buildBaseURL(repo.Organization)
	prEndpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/pullrequests/%d?api-version=%s",
		repo.Project, resolveRepoIdentifier(repo), prID, apiVersion,
	)

	prResp, err := p.doRequest(ctx, baseURL, http.MethodGet, prEndpoint, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get pull request details: %w", err)
	}

	var prData struct {
		SourceRefName string `json:"sourceRefName"`
		TargetRefName string `json:"targetRefName"`
	}
	if unmarshalErr := json.Unmarshal(prResp, &prData); unmarshalErr != nil {
		return "", fmt.Errorf("failed to parse pull request details: %w", unmarshalErr)
	}

	sourceBranch := strings.TrimPrefix(prData.SourceRefName, "refs/heads/")
	targetBranch := strings.TrimPrefix(prData.TargetRefName, "refs/heads/")

	// get changed files
	files, err := p.GetPullRequestFiles(ctx, repo, prID)
	if err != nil {
		return "", err
	}

	// for each file, fetch content at both branches and compute diff
	var fullDiff strings.Builder
	var errs []error
	for _, f := range files {
		diff, diffErr := p.computeFileDiff(ctx, repo, f, sourceBranch, targetBranch)
		if diffErr != nil {
			errs = append(errs, diffErr)
		}
		fullDiff.WriteString(diff)
	}

	return fullDiff.String(), errors.Join(errs...)
}

func (p *Provider) computeFileDiff(
	ctx context.Context,
	repo globalEntities.Repository,
	f globalEntities.PullRequestFile,
	sourceBranch, targetBranch string,
) (string, error) {
	switch f.Status {
	case "deleted":
		oldContent, fetchErr := p.getFileContentAtVersion(ctx, repo, f.Path, targetBranch)
		if fetchErr != nil {
			return "", fmt.Errorf("failed to fetch deleted file %s: %w", f.Path, fetchErr)
		}
		return buildUnifiedDiff(f.Path, "/dev/null", oldContent, ""), nil
	case "added":
		newContent, fetchErr := p.getFileContentAtVersion(ctx, repo, f.Path, sourceBranch)
		if fetchErr != nil {
			return "", fmt.Errorf("failed to fetch added file %s: %w", f.Path, fetchErr)
		}
		return buildUnifiedDiff("/dev/null", f.Path, "", newContent), nil
	default:
		oldPath := f.Path
		if f.OldPath != "" {
			oldPath = f.OldPath
		}
		oldContent, fetchErr := p.getFileContentAtVersion(ctx, repo, oldPath, targetBranch)
		if fetchErr != nil {
			return "", fmt.Errorf("failed to fetch old version of %s: %w", oldPath, fetchErr)
		}
		newContent, fetchErr := p.getFileContentAtVersion(ctx, repo, f.Path, sourceBranch)
		if fetchErr != nil {
			return "", fmt.Errorf("failed to fetch new version of %s: %w", f.Path, fetchErr)
		}
		return buildUnifiedDiff(oldPath, f.Path, oldContent, newContent), nil
	}
}

func (p *Provider) getFileContentAtVersion(
	ctx context.Context,
	repo globalEntities.Repository,
	path string,
	version string,
) (string, error) {
	baseURL := buildBaseURL(repo.Organization)
	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/items?path=%s&versionDescriptor.version=%s&versionDescriptor.versionType=branch&api-version=%s",
		repo.Project,
		resolveRepoIdentifier(repo),
		url.QueryEscape(path),
		url.QueryEscape(version),
		apiVersion,
	)

	resp, err := p.doRequest(ctx, baseURL, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}

	return string(resp), nil
}

type diffLineOp struct {
	op   diffmatchpatch.Operation
	text string
}

type hunkRange struct{ start, end int }

func buildUnifiedDiff(oldPath, newPath, oldContent, newContent string) string {
	if oldContent == newContent {
		return ""
	}

	const contextSize = 3

	dmp := diffmatchpatch.New()
	a, b, lineArray := dmp.DiffLinesToChars(oldContent, newContent)
	diffs := dmp.DiffMain(a, b, false)
	diffs = dmp.DiffCharsToLines(diffs, lineArray)

	var ops []diffLineOp
	for _, d := range diffs {
		for _, line := range splitLines(d.Text) {
			ops = append(ops, diffLineOp{op: d.Type, text: line})
		}
	}

	changes := groupHunkRanges(ops, contextSize)
	if len(changes) == 0 {
		return ""
	}

	normOld := strings.TrimPrefix(oldPath, "/")
	normNew := strings.TrimPrefix(newPath, "/")

	var result strings.Builder
	writeDiffHeader(&result, oldPath, newPath, normOld, normNew)
	for _, ch := range changes {
		writeHunk(&result, ops, max(ch.start-contextSize, 0), min(ch.end+contextSize, len(ops)))
	}

	return result.String()
}

func groupHunkRanges(ops []diffLineOp, contextSize int) []hunkRange {
	var changes []hunkRange
	for i, op := range ops {
		if op.op != diffmatchpatch.DiffEqual {
			if len(changes) > 0 && i-changes[len(changes)-1].end <= 2*contextSize {
				changes[len(changes)-1].end = i + 1
			} else {
				changes = append(changes, hunkRange{start: i, end: i + 1})
			}
		}
	}
	return changes
}

func writeDiffHeader(result *strings.Builder, oldPath, newPath, normOld, normNew string) {
	switch {
	case oldPath == "/dev/null":
		fmt.Fprintf(result, "diff --git a/%s b/%s\n", normNew, normNew)
		result.WriteString("--- /dev/null\n")
		fmt.Fprintf(result, "+++ b/%s\n", normNew)
	case newPath == "/dev/null":
		fmt.Fprintf(result, "diff --git a/%s b/%s\n", normOld, normOld)
		fmt.Fprintf(result, "--- a/%s\n", normOld)
		result.WriteString("+++ /dev/null\n")
	default:
		fmt.Fprintf(result, "diff --git a/%s b/%s\n", normOld, normNew)
		fmt.Fprintf(result, "--- a/%s\n", normOld)
		fmt.Fprintf(result, "+++ b/%s\n", normNew)
	}
}

func writeHunk(result *strings.Builder, ops []diffLineOp, hunkStart, hunkEnd int) {
	oldLine, newLine := 1, 1
	for i := range hunkStart {
		switch ops[i].op {
		case diffmatchpatch.DiffEqual:
			oldLine++
			newLine++
		case diffmatchpatch.DiffDelete:
			oldLine++
		case diffmatchpatch.DiffInsert:
			newLine++
		}
	}

	oldStart, newStart := oldLine, newLine
	oldCount, newCount := 0, 0
	var hunkBody strings.Builder
	for i := hunkStart; i < hunkEnd; i++ {
		switch ops[i].op {
		case diffmatchpatch.DiffEqual:
			hunkBody.WriteString(" " + ops[i].text + "\n")
			oldCount++
			newCount++
		case diffmatchpatch.DiffDelete:
			hunkBody.WriteString("-" + ops[i].text + "\n")
			oldCount++
		case diffmatchpatch.DiffInsert:
			hunkBody.WriteString("+" + ops[i].text + "\n")
			newCount++
		}
	}

	if oldCount == 0 {
		oldStart = 0
	}
	if newCount == 0 {
		newStart = 0
	}

	fmt.Fprintf(result, "@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount)
	result.WriteString(hunkBody.String())
}

func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func (p *Provider) GetPullRequestFiles(
	ctx context.Context,
	repo globalEntities.Repository,
	prID int,
) ([]globalEntities.PullRequestFile, error) {
	baseURL := buildBaseURL(repo.Organization)

	// get the latest iteration
	iterEndpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/pullrequests/%d/iterations?api-version=%s",
		repo.Project, resolveRepoIdentifier(repo), prID, apiVersion,
	)

	iterResp, err := p.doRequest(ctx, baseURL, http.MethodGet, iterEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get pull request iterations: %w", err)
	}

	var iterResult struct {
		Value []struct {
			ID int `json:"id"`
		} `json:"value"`
	}
	if unmarshalErr := json.Unmarshal(iterResp, &iterResult); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse iterations response: %w", unmarshalErr)
	}

	if len(iterResult.Value) == 0 {
		return nil, nil
	}

	latestIter := iterResult.Value[len(iterResult.Value)-1].ID

	// get changes for the latest iteration
	changesEndpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/pullrequests/%d/iterations/%d/changes?api-version=%s",
		repo.Project, resolveRepoIdentifier(repo), prID, latestIter, apiVersion,
	)

	changesResp, err := p.doRequest(ctx, baseURL, http.MethodGet, changesEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get pull request changes: %w", err)
	}

	var changesResult struct {
		ChangeEntries []struct {
			ChangeType string `json:"changeType"`
			Item       struct {
				Path string `json:"path"`
			} `json:"item"`
			OriginalPath string `json:"originalPath"`
		} `json:"changeEntries"`
	}
	if unmarshalErr := json.Unmarshal(changesResp, &changesResult); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse changes response: %w", unmarshalErr)
	}

	var files []globalEntities.PullRequestFile
	for _, change := range changesResult.ChangeEntries {
		status := mapADOChangeType(change.ChangeType)
		files = append(files, globalEntities.PullRequestFile{
			Path:    change.Item.Path,
			OldPath: change.OriginalPath,
			Status:  status,
		})
	}

	return files, nil
}

func (p *Provider) PostPullRequestComment(
	ctx context.Context,
	repo globalEntities.Repository,
	prID int,
	body string,
) error {
	baseURL := buildBaseURL(repo.Organization)
	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/pullrequests/%d/threads?api-version=%s",
		repo.Project, resolveRepoIdentifier(repo), prID, apiVersion,
	)

	threadBody := map[string]any{
		"comments": []map[string]any{
			{
				"parentCommentId": 0,
				"content":         body,
				"commentType":     1,
			},
		},
		"status": 1,
	}

	_, err := p.doRequest(ctx, baseURL, http.MethodPost, endpoint, threadBody)
	if err != nil {
		return fmt.Errorf("failed to post pull request comment: %w", err)
	}

	return nil
}

func (p *Provider) PostPullRequestThreadComment(
	ctx context.Context,
	repo globalEntities.Repository,
	prID int,
	filePath string,
	line int,
	body string,
) error {
	baseURL := buildBaseURL(repo.Organization)
	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/pullrequests/%d/threads?api-version=%s",
		repo.Project, resolveRepoIdentifier(repo), prID, apiVersion,
	)

	threadBody := map[string]any{
		"comments": []map[string]any{
			{
				"parentCommentId": 0,
				"content":         body,
				"commentType":     1,
			},
		},
		"threadContext": map[string]any{
			"filePath": filePath,
			"rightFileStart": map[string]int{
				"line":   line,
				"offset": 1,
			},
			"rightFileEnd": map[string]int{
				"line":   line,
				"offset": 1,
			},
		},
		"status": 1,
	}

	// look up the latest iteration so ADO can anchor the comment to the correct diff;
	// without this the UI shows "This file no longer exists in the latest pull request changes".
	iterationID, iterErr := p.getLatestPullRequestIterationID(ctx, repo, prID)
	if iterErr != nil {
		log.WithError(iterErr).
			WithField("prID", prID).
			Warn("failed to look up latest PR iteration; posting thread without iterationContext")
	} else {
		prThreadContext := map[string]any{
			"iterationContext": map[string]int{
				"firstComparingIteration":  iterationID,
				"secondComparingIteration": iterationID,
			},
		}

		// look up the changeTrackingId for the target file so ADO can follow the line
		// across iterations.
		changeTrackingID, found, changeErr := p.getChangeTrackingID(
			ctx, repo, prID, iterationID, filePath,
		)
		switch {
		case changeErr != nil:
			log.WithError(changeErr).
				WithFields(log.Fields{"prID": prID, "filePath": filePath}).
				Warn("failed to look up changeTrackingId; posting thread without it")
		case !found:
			log.WithFields(log.Fields{"prID": prID, "filePath": filePath}).
				Warn("no matching change entry found; posting thread without changeTrackingId")
		case len(changeTrackingID) == 0:
			// Defensive: ADO returned a `changeEntries` row that
			// matched the file but had no `changeTrackingId` field
			// (or an explicit `null`). The path lookup is correct so
			// this isn't a "no matching entry" case, but the value
			// is unusable — distinguish the warning from the
			// no-match branch above so an operator scanning logs can
			// tell which side of the API contract broke.
			log.WithFields(log.Fields{"prID": prID, "filePath": filePath}).
				Warn("matching change entry has empty changeTrackingId; posting thread without it")
		default:
			prThreadContext["changeTrackingId"] = changeTrackingID
		}

		threadBody["pullRequestThreadContext"] = prThreadContext
	}

	_, err := p.doRequest(ctx, baseURL, http.MethodPost, endpoint, threadBody)
	if err != nil {
		return fmt.Errorf("failed to post pull request thread comment: %w", err)
	}

	return nil
}

// getLatestPullRequestIterationID returns the largest iteration ID for the given PR.
// Iterations are append-only, so the largest ID corresponds to the latest push.
func (p *Provider) getLatestPullRequestIterationID(
	ctx context.Context,
	repo globalEntities.Repository,
	prID int,
) (int, error) {
	baseURL := buildBaseURL(repo.Organization)
	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/pullrequests/%d/iterations?api-version=%s",
		repo.Project, resolveRepoIdentifier(repo), prID, apiVersion,
	)

	resp, err := p.doRequest(ctx, baseURL, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get pull request iterations: %w", err)
	}

	var result struct {
		Value []struct {
			ID int `json:"id"`
		} `json:"value"`
	}
	if unmarshalErr := json.Unmarshal(resp, &result); unmarshalErr != nil {
		return 0, fmt.Errorf("failed to parse iterations response: %w", unmarshalErr)
	}

	if len(result.Value) == 0 {
		return 0, errors.New("no iterations found for pull request")
	}

	latest := 0
	for _, iter := range result.Value {
		if iter.ID > latest {
			latest = iter.ID
		}
	}

	return latest, nil
}

// getChangeTrackingID returns the changeTrackingId for the change entry whose item.path
// matches the requested filePath in the given PR iteration. The boolean is true when a
// matching path entry was found, regardless of whether that entry carried a usable
// `changeTrackingId` — the caller is expected to check the returned value's length
// before embedding it in the thread payload. ADO's API returns paths with a leading
// slash (e.g. "/README.md") while callers may pass paths without one — both forms are
// normalised before comparison.
//
// The value is returned as `json.RawMessage` so the original JSON shape (int / string /
// other) round-trips byte-for-byte into the thread create payload that gets sent back
// to ADO. Returning `any` would force an intermediate `json.Unmarshal` which decodes
// numbers into `float64` and would lose precision on a large numeric ID — pinned per
// Copilot review on PR #85 thread `PRRT_kwDORQWb3M5-6QR6`. Pinned per Copilot review
// on PR #85 thread `PRRT_kwDORQWb3M5-6QRd`.
func (p *Provider) getChangeTrackingID(
	ctx context.Context,
	repo globalEntities.Repository,
	prID int,
	iterationID int,
	filePath string,
) (json.RawMessage, bool, error) {
	baseURL := buildBaseURL(repo.Organization)
	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/pullrequests/%d/iterations/%d/changes?api-version=%s",
		repo.Project, resolveRepoIdentifier(repo), prID, iterationID, apiVersion,
	)

	resp, err := p.doRequest(ctx, baseURL, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get pull request iteration changes: %w", err)
	}

	var result struct {
		ChangeEntries []struct {
			ChangeTrackingID json.RawMessage `json:"changeTrackingId"`
			Item             struct {
				Path string `json:"path"`
			} `json:"item"`
		} `json:"changeEntries"`
	}
	if unmarshalErr := json.Unmarshal(resp, &result); unmarshalErr != nil {
		return nil, false, fmt.Errorf("failed to parse iteration changes response: %w", unmarshalErr)
	}

	target := strings.TrimPrefix(filePath, "/")
	for _, entry := range result.ChangeEntries {
		if strings.TrimPrefix(entry.Item.Path, "/") != target {
			continue
		}
		// Path matched. The raw bytes are returned even if empty — the
		// caller distinguishes "matched without ID" from "matched with
		// ID" by checking `len(returned) > 0` so the warning shape on
		// the caller side stays accurate (Copilot thread
		// `PRRT_kwDORQWb3M5-6QRd`).
		return entry.ChangeTrackingID, true, nil
	}

	return nil, false, nil
}

func (p *Provider) GetPullRequestCheckStatus(
	ctx context.Context,
	repo globalEntities.Repository,
	prID int,
) (bool, error) {
	baseURL := buildBaseURL(repo.Organization)
	endpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/pullrequests/%d/statuses?api-version=%s",
		repo.Project, resolveRepoIdentifier(repo), prID, apiVersion,
	)

	resp, err := p.doRequest(ctx, baseURL, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, fmt.Errorf("failed to get pull request statuses: %w", err)
	}

	var result struct {
		Value []struct {
			State string `json:"state"`
		} `json:"value"`
	}
	if unmarshalErr := json.Unmarshal(resp, &result); unmarshalErr != nil {
		return false, fmt.Errorf("failed to parse statuses response: %w", unmarshalErr)
	}

	// if no statuses, consider passed (no CI configured)
	if len(result.Value) == 0 {
		return true, nil
	}

	for _, status := range result.Value {
		if status.State != "succeeded" {
			return false, nil
		}
	}

	return true, nil
}

func (p *Provider) MergePullRequest(
	ctx context.Context,
	repo globalEntities.Repository,
	prID int,
	strategy string,
) error {
	baseURL := buildBaseURL(repo.Organization)

	// first get the PR to obtain the last merge source commit
	getEndpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/pullrequests/%d?api-version=%s",
		repo.Project, resolveRepoIdentifier(repo), prID, apiVersion,
	)

	prResp, err := p.doRequest(ctx, baseURL, http.MethodGet, getEndpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to get pull request for merge: %w", err)
	}

	var prData struct {
		LastMergeSourceCommit struct {
			CommitID string `json:"commitId"`
		} `json:"lastMergeSourceCommit"`
	}
	if unmarshalErr := json.Unmarshal(prResp, &prData); unmarshalErr != nil {
		return fmt.Errorf("failed to parse pull request data: %w", unmarshalErr)
	}

	// complete the PR
	updateEndpoint := fmt.Sprintf(
		"/%s/_apis/git/repositories/%s/pullrequests/%d?api-version=%s",
		repo.Project, resolveRepoIdentifier(repo), prID, apiVersion,
	)

	body := map[string]any{
		"status":                "completed",
		"lastMergeSourceCommit": prData.LastMergeSourceCommit,
		"completionOptions": map[string]any{
			"deleteSourceBranch": false,
			"mergeStrategy":      mapADOMergeStrategy(strategy),
		},
	}

	_, err = p.doRequest(ctx, baseURL, http.MethodPatch, updateEndpoint, body)
	if err != nil {
		return fmt.Errorf("failed to complete pull request: %w", err)
	}

	return nil
}

const (
	adoMergeStrategySquash        = 1
	adoMergeStrategyNoFastForward = 2
	adoMergeStrategyRebase        = 3
)

func mapADOMergeStrategy(strategy string) int {
	strategyMap := map[string]int{
		"squash": adoMergeStrategySquash,
		"merge":  adoMergeStrategyNoFastForward,
		"rebase": adoMergeStrategyRebase,
	}

	if val, ok := strategyMap[strategy]; ok {
		return val
	}

	return adoMergeStrategySquash
}

func mapADOChangeType(changeType string) string {
	changeTypeMap := map[string]string{
		"add":    "added",
		"edit":   "modified",
		"delete": "deleted",
		"rename": "renamed",
	}

	if status, ok := changeTypeMap[strings.ToLower(changeType)]; ok {
		return status
	}

	return "modified"
}
