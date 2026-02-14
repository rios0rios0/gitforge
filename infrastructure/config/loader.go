package config

import (
	"fmt"
	"net/url"
	"os"

	"github.com/rios0rios0/gitforge/support"
)

// ReadData reads data from a file path or a URL.
// If the path is a valid URL, it downloads the content; otherwise, it reads from disk.
func ReadData(configPath string) ([]byte, error) {
	uri, err := url.Parse(configPath)
	if err != nil || uri.Scheme == "" || uri.Host == "" {
		var data []byte
		data, err = os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		return data, nil
	}
	return support.DownloadFile(configPath)
}
