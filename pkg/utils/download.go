package utils

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/ekristen/cast/pkg/httputil"
)

func DownloadFile(ctx context.Context, url string, dest string, httpClient *http.Client, headers map[string]string) error {
	if httpClient == nil {
		httpClient = httputil.NewClient()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	for k, v := range headers {
		req.Header.Add(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 399 {
		return fmt.Errorf("received error code %d attempting to download", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func DownloadFileToBytes(ctx context.Context, url string, httpClient *http.Client, headers map[string]string) ([]byte, error) {
	if httpClient == nil {
		httpClient = httputil.NewClient()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Add(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 399 {
		return nil, fmt.Errorf("received error code %d attempting to download", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
