package openapi

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func Fetch(source string) ([]byte, error) {
	if !strings.HasPrefix(source, "http://") && !strings.HasPrefix(source, "https://") {
		return nil, fmt.Errorf("spec source must be an http or https URL, got %q", source)
	}
	return fetchHTTP(source)
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
