package azuredevops

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

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
