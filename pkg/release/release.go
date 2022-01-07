package release

import (
	"bytes"
	"context"
	"crypto/sha512"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ekristen/cast/pkg/config"
	"github.com/ekristen/cast/pkg/git"
	"github.com/google/go-github/v41/github"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v3"
)

// 1. Check if git repo
// 2. Check if on tag
// 3. Download tar.gz
// 4. Checksum tar.gz
// 5. Write manifest
// 6. Checksum manifest
// 7. Sign manifest
// 8. Upload checkum, manifest, extra_files
// 9. Done
var (
	dispositionRegexp = regexp.MustCompile(`^\w+; filename=(.+)$`)
)

type Artifact struct {
	Name string `json:"name,omitempty"`
	Path string `json:"path,omitempty"`
}

func Run(ctx context.Context, configFile string, githubToken string, tag string) (err error) {
	log := logrus.WithField("component", "release").WithField("handler", "run")

	cfg, err := config.Load(configFile)
	if err != nil {
		return err
	}

	if !git.IsRepo(ctx) {
		return fmt.Errorf("not a git repo")
	}

	if tag == "" {
		tag, err = git.Clean(git.Run(ctx, "describe", "--tags"))
		if err != nil {
			return err
		}
	}

	var dl *http.Client
	var gh *github.Client

	if githubToken != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: githubToken},
		)

		gh = github.NewClient(oauth2.NewClient(ctx, ts))
		dl = &http.Client{
			Transport: &transport{token: githubToken, underlyingTransport: http.DefaultTransport},
		}
	} else {
		gh = github.NewClient(nil)
	}

	log.Info("fetching current release")

	release, _, err := gh.Repositories.GetReleaseByTag(ctx, cfg.Release.GitHub.Owner, cfg.Release.GitHub.Repo, tag)
	if err != nil && !strings.Contains(err.Error(), "404 Not Found") {
		return err
	}

	if release != nil {
		return fmt.Errorf("release already exists, use --overwrite to delete the release first")
	}

	release, _, err = gh.Repositories.CreateRelease(ctx, cfg.Release.GitHub.Owner, cfg.Release.GitHub.Repo, &github.RepositoryRelease{
		TagName: github.String(tag),
		Name:    github.String(tag),
	})

	dir, err := ioutil.TempDir(os.TempDir(), "release-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	artifacts := []Artifact{}

	log.Info("generating manifest.yml")

	mfilename := filepath.Join(dir, "manifest.yml")

	md, err := yaml.Marshal(cfg.Manifest)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(mfilename, md, 0644); err != nil {
		return err
	}

	artifacts = append(artifacts, Artifact{
		Name: "manifest.yml",
		Path: mfilename,
	})

	checksumsFile, err := os.OpenFile(filepath.Join(dir, "checksums.txt"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	artifacts = append(artifacts, Artifact{
		Name: "checksums.txt",
		Path: checksumsFile.Name(),
	})

	log.Info("checksumming tarball")

	filename, err := downloadFile(release.GetTarballURL(), dir, dl, nil)
	if err != nil {
		return err
	}

	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	hasher := sha512.New()
	hasher.Write(b)
	checksum := fmt.Sprintf("%x", hasher.Sum(nil))

	checksumsFile.Write([]byte(fmt.Sprintf("%s %s\n", checksum, filepath.Base(filename))))

	log.Info("checksumming manifest.yml")

	d, err := ioutil.ReadFile(mfilename)
	if err != nil {
		return err
	}

	hasher = sha512.New()
	hasher.Write(d)
	checksum = fmt.Sprintf("%x", hasher.Sum(nil))

	checksumsFile.Write([]byte(fmt.Sprintf("%s %s\n", checksum, filepath.Base(mfilename))))

	if err := checksumsFile.Close(); err != nil {
		return err
	}

	for _, f := range cfg.Release.ExtraFiles {
		copyFile(f, filepath.Join(dir, filepath.Base(f)))
	}

	log.Info("signing checksums.txt")

	if err := signFile(ctx, checksumsFile.Name()); err != nil {
		return err
	}

	artifacts = append(artifacts, Artifact{
		Name: "checksums.txt.sig",
		Path: fmt.Sprintf("%s.sig", checksumsFile.Name()),
	})

	for _, a := range artifacts {
		log.Infof("uploading release asset: %s", a.Name)

		f, err := os.Open(a.Path)
		if err != nil {
			return err
		}

		if _, _, err := gh.Repositories.UploadReleaseAsset(
			ctx,
			cfg.Release.GitHub.Owner,
			cfg.Release.GitHub.Repo,
			release.GetID(),
			&github.UploadOptions{
				Name: a.Name,
			},
			f,
		); err != nil {
			return err
		}
	}

	return nil
}

func signFile(ctx context.Context, file string) error {
	out := fmt.Sprintf("%s.sig", file)
	args := []string{"sign-blob", "--key=cosign.key", "--output-signature=" + out, file}

	var b bytes.Buffer

	cmd := exec.CommandContext(ctx, "cosign", args...)
	cmd.Stderr = &b
	cmd.Stdout = &b

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sign: %s failed: %w: %s", "cosign", err, b.String())
	}

	return nil
}

func downloadFile(url string, dest string, httpClient *http.Client, headers map[string]string) (string, error) {
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	for k, v := range headers {
		req.Header.Add(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 399 {
		return "", fmt.Errorf("received error code %d attempting to download", resp.StatusCode)
	}

	fn := dispositionRegexp.FindStringSubmatch(resp.Header.Get("content-disposition"))[1]
	fullname := filepath.Join(dest, fn)

	out, err := os.Create(fullname)
	if err != nil {
		return "", err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return fullname, err
}

func copyFile(src, dst string) (err error) {
	sfi, err := os.Stat(src)
	if err != nil {
		return
	}
	if !sfi.Mode().IsRegular() {
		// cannot copy non-regular files (e.g., directories,
		// symlinks, devices, etc.)
		return fmt.Errorf("CopyFile: non-regular source file %s (%q)", sfi.Name(), sfi.Mode().String())
	}
	dfi, err := os.Stat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return
		}
	} else {
		if !(dfi.Mode().IsRegular()) {
			return fmt.Errorf("CopyFile: non-regular destination file %s (%q)", dfi.Name(), dfi.Mode().String())
		}
		if os.SameFile(sfi, dfi) {
			return
		}
	}
	if err = os.Link(src, dst); err == nil {
		return
	}
	err = copyFileContents(src, dst)
	return
}

// copyFileContents copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file.
func copyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}
