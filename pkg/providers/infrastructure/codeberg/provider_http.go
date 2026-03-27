package codeberg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func (p *Provider) doRequest(
	ctx context.Context,
	method, endpoint string,
	body any,
) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", marshalErr)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	fullURL := p.baseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+p.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < httpStatusOKMin || resp.StatusCode >= httpStatusOKMax {
		return nil, fmt.Errorf(
			"API error (status %d): %s",
			resp.StatusCode, string(respBody),
		)
	}

	return respBody, nil
}
