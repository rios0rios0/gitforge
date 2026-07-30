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
	"strings"
)

// ErrAuthentication indicates Azure DevOps rejected the personal access token.
// It is returned instead of a parse failure whenever the API answers with its
// sign-in page, so callers can tell a credential problem apart from a genuine
// protocol error with [errors.Is].
var ErrAuthentication = errors.New("authentication failed")

// isSignInResponse reports whether the response is the Azure DevOps sign-in
// page rather than an API payload.
//
// Azure DevOps does not answer an unauthenticated REST call with 401. It
// answers with a redirect to a sign-in page, and that page is then served as
// 203 Non-Authoritative Information with an HTML body. Neither shape is ever
// produced by a successful API call, so either one means the token was
// missing, expired, or not accepted for the requested resource.
//
// Both arms are deliberately narrow. The redirect arm requires a Location
// header so that a bodyless 3xx such as 304 Not Modified still falls through to
// the regular status handling, and the 203 arm requires an HTML content type
// because endpoints such as file content return arbitrary bodies that must keep
// flowing through untouched. Media types are case-insensitive, so the content
// type is folded before it is matched.
//
// It only inspects the status line and the headers, never the body, so the
// caller can decide on a sign-in response without buffering the HTML page it
// carries.
func isSignInResponse(resp *http.Response) bool {
	isRedirect := resp.StatusCode >= httpStatusRedirectMin &&
		resp.StatusCode < httpStatusRedirectMax &&
		resp.Header.Get("Location") != ""
	if isRedirect {
		return true
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))

	return resp.StatusCode == httpStatusNonAuthoritative &&
		strings.Contains(contentType, "text/html")
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

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Decided before the body is read: a sign-in page carries nothing the error
	// needs, so there is no reason to buffer it.
	if isSignInResponse(resp) {
		return nil, nil, fmt.Errorf(
			"%w: Azure DevOps returned its sign-in page (status %d) for %s %s "+
				"instead of an API response, which means the personal access token "+
				"is missing, expired, or lacks the scopes this call requires",
			ErrAuthentication, resp.StatusCode, method, endpoint,
		)
	}

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
