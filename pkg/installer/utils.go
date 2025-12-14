package installer

import (
	"errors"
	"io"
	"os"

	"github.com/ekristen/cast/pkg/httputil"
)

func DownloadFile(url string, dest string) error {
	// Get the data using a proxy-aware client
	client := httputil.NewClient()
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

func Exists(name string) (bool, error) {
	_, err := os.Stat(name)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}
