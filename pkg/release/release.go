package release

import (
	"bytes"
	"context"
	"crypto/sha256"
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

	"github.com/Masterminds/semver"
	"github.com/ekristen/cast/pkg/config"
	"github.com/ekristen/cast/pkg/git"
	"github.com/ekristen/cast/pkg/utils"
	"github.com/google/go-github/v76/github"
	"github.com/pkg/errors"
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

type RunConfig struct {
	DistDir      string
	RmDist       bool
	ConfigFile   string
	GitHubToken  string
	Tag          string
	CosignKey    string
	LegacySign   bool
	LegacyPGPKey string
	DryRun       bool
}

func Run(ctx context.Context, runConfig *RunConfig) (err error) {
	var dl *http.Client
	var gh *github.Client

	log := logrus.WithField("component", "release").WithField("handler", "run")

	if exists, err := utils.FileExists(runConfig.DistDir); err != nil {
		return errors.Wrap(err, "error checking if file exists")
	} else if exists && !runConfig.RmDist {
		return fmt.Errorf("dist exists and --rm-dist not specified")
	} else if exists && runConfig.RmDist {
		if err := os.RemoveAll(runConfig.DistDir); err != nil {
			return errors.Wrap(err, "unable to remove dist dir")
		}
	}

	cfg, err := config.Load(runConfig.ConfigFile)
	if err != nil {
		return errors.Wrap(err, "unable to load config")
	}

	if err := cfg.Validate(); err != nil {
		return errors.Wrap(err, "config validation failure")
	}

	if !git.IsRepo(ctx) {
		return fmt.Errorf("not a git repo")
	}

	if runConfig.Tag == "" {
		runConfig.Tag, err = git.Clean(git.Run(ctx, "describe", "--tags"))
		if err != nil {
			return errors.Wrap(err, "unable to obtain git tag")
		}
	}

	v, err := semver.NewVersion(strings.TrimPrefix(runConfig.Tag, "v"))
	if err != nil {
		return errors.Wrap(err, "unable to parse semver")
	}

	if exists, err := utils.FileExists(runConfig.CosignKey); err != nil {
		return errors.Wrap(err, "unable to open cosign key")
	} else if !exists {
		return fmt.Errorf("cosign.key not found")
	}

	cosignPub := strings.ReplaceAll(runConfig.CosignKey, ".key", ".pub")
	if exists, err := utils.FileExists(cosignPub); err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("cosign.pub not found")
	}

	if runConfig.GitHubToken != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: runConfig.GitHubToken},
		)

		gh = github.NewClient(oauth2.NewClient(ctx, ts))
		dl = &http.Client{
			Transport: &transport{token: runConfig.GitHubToken, underlyingTransport: http.DefaultTransport},
		}
	} else {
		return fmt.Errorf("unable to perform release without a valid github token")
	}

	log.Info("fetching current release")

	release, _, err := gh.Repositories.GetReleaseByTag(ctx, cfg.Release.GitHub.Owner, cfg.Release.GitHub.Repo, runConfig.Tag)
	if err != nil && !strings.Contains(err.Error(), "404 Not Found") {
		return errors.Wrap(err, "unable to fetch release by tag")
	}

	if release != nil {
		return fmt.Errorf("release already exists, use --overwrite to delete the release first")
	}

	releaseOpts := &github.RepositoryRelease{
		TagName:    github.String(runConfig.Tag),
		Name:       github.String(runConfig.Tag),
		Prerelease: github.Bool(v.Prerelease() != ""),
	}

	log.Debug("release config", releaseOpts)

	release, _, err = gh.Repositories.CreateRelease(ctx, cfg.Release.GitHub.Owner, cfg.Release.GitHub.Repo, releaseOpts)
	if err != nil {
		return errors.Wrap(err, "unable to create release for tag")
	}

	// TODO: check if valid release

	/*
		dir, err := ioutil.TempDir(os.TempDir(), "release-")
		if err != nil {
			return errors.Wrap(err, "unable to create temp directory")
		}
		defer os.RemoveAll(dir)
	*/
	dir := runConfig.DistDir
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errors.Wrap(err, "unable to mkdirp")
	}

	artifacts := []Artifact{}

	log.Info("generating manifest.yml")

	mfilename := filepath.Join(dir, "manifest.yml")

	md, err := yaml.Marshal(cfg.Manifest)
	if err != nil {
		return errors.Wrap(err, "unable to parse manifest")
	}

	// TODO: validate manifest?

	if err := ioutil.WriteFile(mfilename, md, 0644); err != nil {
		return errors.Wrap(err, "unable to write manifest file")
	}

	artifacts = append(artifacts, Artifact{
		Name: "manifest.yml",
		Path: mfilename,
	})

	checksumsFile, err := os.OpenFile(filepath.Join(dir, "checksums.txt"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return errors.Wrap(err, "unable to open checksum file")
	}

	artifacts = append(artifacts, Artifact{
		Name: "checksums.txt",
		Path: checksumsFile.Name(),
	})

	log.Info("downloading tarball")

	filename, err := downloadFile(release.GetTarballURL(), dir, dl, nil)
	if err != nil {
		return errors.Wrap(err, "unable to download tarball")
	}

	log.Debugf("tarball filename: %s", filename)

	log.Info("checksumming tarball")
	tf, err := os.Open(filename)
	if err != nil {
		return errors.Wrap(err, "unable to open tarball for checksumming")
	}
	defer tf.Close()

	hasher := sha512.New()
	if _, err := io.Copy(hasher, tf); err != nil {
		return errors.Wrap(err, "unable to copy file to hasher512")
	}

	checksum := fmt.Sprintf("%x", hasher.Sum(nil))

	checksumFileLine := fmt.Sprintf("%s %s\n", checksum, filepath.Base(filename))

	if _, err := checksumsFile.Write([]byte(checksumFileLine)); err != nil {
		return err
	}

	log.Info("check-summing manifest.yml")

	d, err := os.ReadFile(mfilename)
	if err != nil {
		return errors.Wrap(err, "unable to open manifest file")
	}

	hasher = sha512.New()
	hasher.Write(d)
	checksum = fmt.Sprintf("%x", hasher.Sum(nil))

	if _, err := checksumsFile.Write(
		[]byte(fmt.Sprintf("%s %s\n", checksum, filepath.Base(mfilename))),
	); err != nil {
		return err
	}

	if err := checksumsFile.Close(); err != nil {
		return err
	}

	for _, f := range cfg.Release.ExtraFiles {
		artifacts = append(artifacts, Artifact{
			Name: filepath.Base(f),
			Path: filepath.Join(dir, filepath.Base(f)),
		})

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

	artifacts = append(artifacts, Artifact{
		Name: "cosign.pub",
		Path: cosignPub,
	})

	if runConfig.LegacySign {
		log.Info("legacy: downloading legacy tar.gz")
		archiveURL := fmt.Sprintf("https://github.com/%s/%s/archive/%s.tar.gz", cfg.Release.GitHub.Owner, cfg.Release.GitHub.Repo, runConfig.Tag)

		log.WithField("url", archiveURL).Debug("legacy: archive url")

		filename, err := downloadFile(archiveURL, dir, dl, nil)
		if err != nil {
			return errors.Wrap(err, "unable to download legacy archive file")
		}

		log.WithField("filename", filename).Debug("legacy: tarball filename")

		targzFilename := filepath.Base(filename)
		log.WithField("filename", targzFilename).Debug("legacy: tar.gz filename")

		targzNewFilename := targzFilename
		if !strings.Contains(targzNewFilename, runConfig.Tag) {
			targzNewFilename = strings.ReplaceAll(targzNewFilename, v.String(), runConfig.Tag)
		}

		targzSigFilename := fmt.Sprintf("%s.asc", targzNewFilename)

		log.Info("legacy: hashing legacy tar.gz")

		tf2, err := os.Open(filename)
		if err != nil {
			return errors.Wrap(err, "unable to open legacy tar.gz file")
		}
		defer tf2.Close()

		hasher256 := sha256.New()
		if _, err := io.Copy(hasher256, tf2); err != nil {
			return errors.Wrap(err, "unable to copy to hasher256")
		}
		checksum256 := fmt.Sprintf("%x", hasher256.Sum(nil))

		log.WithField("sha256", checksum256).Debug("legacy: sha256 value")

		checksum256FileLine := fmt.Sprintf("%s  /tmp/%s\n", checksum256, targzNewFilename)

		legacyChecksumFilename := fmt.Sprintf("%s.sha256", targzNewFilename)
		legacyChecksumPath := filepath.Join(dir, legacyChecksumFilename)
		if err := ioutil.WriteFile(legacyChecksumPath, []byte(checksum256FileLine), 0644); err != nil {
			return err
		}

		log.Info("legacy: writing checksum file")

		legacyChecksumSignFilename := fmt.Sprintf("%s.asc", legacyChecksumFilename)

		pgpPrivateKey, err := ioutil.ReadFile(runConfig.LegacyPGPKey)
		if err != nil {
			return err
		}

		log.Info("legacy: signing checksum file")
		if err := utils.GPGSign(dir, legacyChecksumFilename, legacyChecksumSignFilename, pgpPrivateKey, false); err != nil {
			return errors.Wrap(err, "unable to sign checksum file")
		}

		log.Info("legacy: signing tar.gz")
		if err := utils.GPGSign(dir, targzFilename, targzSigFilename, pgpPrivateKey, true); err != nil {
			return errors.Wrap(err, "unable to sign archive file")
		}

		legacyArtifacts := []Artifact{
			{
				Name: legacyChecksumFilename,
				Path: legacyChecksumPath,
			},
			{
				Name: legacyChecksumSignFilename,
				Path: filepath.Join(dir, legacyChecksumSignFilename),
			},
			{
				Name: targzSigFilename,
				Path: filepath.Join(dir, targzSigFilename),
			},
			{
				Name: "pgp.pub",
				Path: "pgp.pub",
			},
		}

		artifacts = append(artifacts, legacyArtifacts...)
	}

	for _, a := range artifacts {
		log.WithField("file", a.Name).Infof("uploading release asset")

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
	args := []string{"sign-blob", "--yes", "--tlog-upload=false", "--key=cosign.key", "--output-signature=" + out, file}

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
