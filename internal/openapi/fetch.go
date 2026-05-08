package openapi

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Fetch retrieves raw spec bytes from an http/https URL, a file:// URL, or a local path.
func Fetch(source string) ([]byte, error) {
	switch {
	case strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://"):
		return fetchHTTP(source)
	case strings.HasPrefix(source, "file://"):
		return os.ReadFile(strings.TrimPrefix(source, "file://"))
	default:
		return os.ReadFile(source)
	}
}

func fetchHTTP(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url) //nolint:noctx
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}
