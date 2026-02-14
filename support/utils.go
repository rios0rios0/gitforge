package support

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const DownloadTimeout = 30

// ReadLines reads a whole file into memory and returns lines as a string slice.
func ReadLines(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	err = scanner.Err()
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return lines, nil
}

// WriteLines writes the lines to the given file.
func WriteLines(filePath string, lines []string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, line := range lines {
		fmt.Fprintln(writer, line)
	}

	err = writer.Flush()
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}
	return nil
}

// DownloadFile downloads a file from the given URL and returns its content as bytes.
func DownloadFile(url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DownloadTimeout*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}

// StripUsernameFromURL removes the username from a URL if present.
// For example: https://user@dev.azure.com/org/project -> https://dev.azure.com/org/project
func StripUsernameFromURL(rawURL string) string {
	if !strings.HasPrefix(rawURL, "https://") && !strings.HasPrefix(rawURL, "http://") {
		return rawURL
	}

	const skipping = "://"
	schemeEnd := strings.Index(rawURL, skipping) + len(skipping)
	atIndex := strings.Index(rawURL[schemeEnd:], "@")
	if atIndex == -1 {
		return rawURL
	}

	return rawURL[:schemeEnd] + rawURL[schemeEnd+atIndex+1:]
}
