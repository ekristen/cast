package utils

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
)

func DownloadFile(url string, dest string, httpClient *http.Client, headers map[string]string) error {
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	req, err := http.NewRequest("GET", url, nil)
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

func DownloadFileToBytes(url string, httpClient *http.Client, headers map[string]string) ([]byte, error) {
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if headers != nil {
		for k, v := range headers {
			req.Header.Add(k, v)
		}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 399 {
		return nil, fmt.Errorf("received error code %d attempting to download", resp.StatusCode)
	}

	return ioutil.ReadAll(resp.Body)
}
