package distro

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ekristen/cast/pkg/common"
	"github.com/ekristen/cast/pkg/utils"

	"github.com/google/go-github/v41/github"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v3"

	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/packet"
)

// 1. Parse distro into owner/repo
// 2. Verify repo existw
// 3. Obtain latest release (or specified release)
// 4. Check latest release for manifest
// 5. If no manifest, assume v0, which uses archive file
// 6. If manifest, parse

// Release is either made up of v0
// -- archive.tar.gz
// -- archive.tar.gz.sha256
// -- archive.tar.gz.sha256.asc (gpg)

// Release is made up of v1
// - manifest.yaml
// - archive.tar.gz
// - archive.tar.gz.sha256
// - archive.tar.gz.sha256.sig (cosign)

type Distro struct {
	Owner   string
	Repo    string
	Version string
	Name    string

	Alias   string
	IsAlias bool

	Manifest *Manifest

	IncludePreReleases bool

	ctx      context.Context
	log      *logrus.Entry
	http     *http.Client
	dlHttp   *http.Client
	github   *github.Client
	releases []*github.RepositoryRelease
	selected *github.RepositoryRelease

	archiveName string
}

type transport struct {
	token               string
	underlyingTransport http.RoundTripper
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("Authorization", fmt.Sprintf("token %s", t.token))
	return t.underlyingTransport.RoundTrip(req)
}

func New(ctx context.Context, distro string, version *string, includePreReleases bool, githubToken string) (*Distro, error) {
	var d *Distro
	if v, ok := aliases[distro]; ok {
		d = v
		d.Alias = distro
		d.IsAlias = true
	} else {
		parts := strings.Split(distro, "/")
		if len(parts) != 2 {
			return nil, fmt.Errorf("incorrect distro format, expect owner/repo")
		}

		d = &Distro{
			Owner: parts[0],
			Repo:  parts[1],
		}
	}

	d.IncludePreReleases = includePreReleases

	if version != nil {
		d.Version = *version
	}

	d.Name = fmt.Sprintf("%s_%s", d.Owner, d.Repo)

	d.ctx = ctx

	if githubToken != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: githubToken},
		)
		d.http = oauth2.NewClient(d.ctx, ts)
		d.github = github.NewClient(d.http)

		d.dlHttp = &http.Client{
			Transport: &transport{token: githubToken, underlyingTransport: http.DefaultTransport},
		}
	} else {
		d.github = github.NewClient(nil)
	}

	d.log = logrus.WithField("component", "distro").WithField("owner", d.Owner).WithField("repo", d.Repo)

	if err := d.fetchReleases(d.ctx); err != nil {
		return nil, err
	}

	if err := d.verifyRelease(); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *Distro) GetName() string {
	return d.Name
}

func (d *Distro) GetRelease() *github.RepositoryRelease {
	return d.selected
}

func (d *Distro) GetReleaseName() string {
	return *d.selected.TagName
}

func (d *Distro) GetReleaseAssets() []*github.ReleaseAsset {
	return d.selected.Assets
}

func (d *Distro) GetModeState(mode string) (string, error) {
	for _, m := range d.Manifest.Modes {
		if mode == "" && m.Default {
			return m.State, nil
		} else if mode != "" && m.Name == mode {
			return m.State, nil
		}
	}

	return "", fmt.Errorf("unable to resolve state from mode: %s", mode)
}

func (d *Distro) GetCachePath() string {
	return filepath.Join(d.GetName(), d.GetReleaseName())
}

func (d *Distro) Download(dir string) error {
	if err := d.DownloadAssets(dir); err != nil {
		return err
	}
	if err := d.ValidateAssets(dir); err != nil {
		return err
	}
	if err := d.ExtractArchiveFile(dir); err != nil {
		return err
	}
	return nil
}

func (d *Distro) DownloadAssets(dir string) error {
	d.archiveName = fmt.Sprintf("%s.tar.gz", d.GetReleaseName())
	if d.Manifest.Version == 1 {
		d.archiveName = fmt.Sprintf("%s-%s.tar.gz", d.Repo, d.GetReleaseName())

		if d.Name == "remux" {
			d.archiveName = fmt.Sprintf("%s-%s-%s.tar.gz", d.Owner, d.Repo, d.GetReleaseName())
		}
	}

	archiveURL := d.selected.GetTarballURL()
	if d.Manifest.Version == 1 {
		archiveURL = fmt.Sprintf("https://github.com/%s/%s/archive/%s.tar.gz", d.Owner, d.Repo, d.selected.GetTagName())
	}

	if err := d.downloadArchiveFile(archiveURL, d.archiveName, dir); err != nil {
		return err
	}

	for _, a := range d.selected.Assets {
		assetURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/assets/%d", d.Owner, d.Repo, a.GetID())
		// a.GetBrowserDownloadURL()

		if err := d.downloadReleaseAsset(assetURL, a.GetName(), dir); err != nil {
			return err
		}
	}

	return nil
}

func (d *Distro) ValidateAssets(dir string) error {
	switch d.Manifest.Version {
	case 1:
		// original
		for _, a := range d.selected.Assets {
			if strings.HasSuffix(a.GetName(), ".sha256") {
				if err := d.validateFile(dir, strings.TrimSuffix(a.GetName(), ".sha256")); err != nil {
					return err
				}
			}
		}

		for _, a := range d.selected.Assets {
			if strings.HasSuffix(a.GetName(), ".asc") && !strings.HasSuffix(a.GetName(), ".sha256.asc") {
				if err := d.validatePGPSignature(dir, strings.TrimSuffix(a.GetName(), ".asc")); err != nil {
					return err
				}
			}
		}
	case 2:
		// new
	}

	return nil
}

func (d *Distro) downloadArchiveFile(url, filename, dir string) error {
	log := d.log.WithField("version", d.GetReleaseName())
	dst := filepath.Join(dir, filename)

	/*
		if !i.config.NoCache {
			exists, err := utils.FileExists(dst)
			if err != nil {
				return err
			}
			if exists {
				log.Info("downloading archive file (cached)")
				return nil
			}
		}
	*/

	log.WithField("url", url).Debug("tarball url")
	log.Info("downloading archive file")

	if err := utils.DownloadFile(url, dst, d.dlHttp, nil); err != nil {
		return err
	}

	return nil
}

func (d *Distro) ExtractArchiveFile(dir string) error {
	d.log.WithField("version", d.GetReleaseName()).Info("extracting archive file")

	src := filepath.Join(dir, d.archiveName)
	dst := filepath.Join(dir, "source")
	if d.Manifest.Base != "" {
		dst = filepath.Join(dir, "source", d.Manifest.Base)
	}

	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()

		switch {

		// if no more files are found return
		case err == io.EOF:
			return nil

		// return any other error
		case err != nil:
			return err

		// if the header is nil, just skip it (not sure how this happens)
		case header == nil:
			continue
		}

		// the target location where the dir/file should be created
		strippedName := strings.Split(header.Name, "/")[1:]
		if len(strippedName) == 0 {
			continue
		}

		/*
			if d.Manifest.Version == 1 {
				strippedName = strippedName[1:]
				if len(strippedName) == 0 {
					continue
				}
			}
		*/

		target := filepath.Join(append([]string{dst}, strippedName...)...)

		log := d.log.WithField("stripped", filepath.Join(strippedName...)).WithField("target", target)

		// the following switch could also be done using fi.Mode(), not sure if there
		// a benefit of using one vs. the other.
		// fi := header.FileInfo()

		// check the file type
		switch header.Typeflag {

		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			log.Debug("extracting directory")

			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}

		// if it's a file create it
		case tar.TypeReg:
			log.Debug("extracting file")

			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			// copy over contents
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}

			// manually close here after each file operation; defering would cause each file close
			// to wait until all operations have completed.
			f.Close()
		}
	}
}

func (d *Distro) downloadReleaseAsset(url, filename, dir string) error {
	dst := filepath.Join(dir, filename)

	/*
		if !i.config.NoCache {
			exists, err := Exists(dst)
			if err != nil {
				return err
			}
			if exists {
				log.Infof("downloading release file (cached)")
				return nil
			}
		}
	*/

	d.log.WithField("filename", filename).Infof("downloading release file")

	headers := map[string]string{
		"accept": "application/octet-stream",
	}

	if err := utils.DownloadFile(url, dst, d.dlHttp, headers); err != nil {
		return err
	}

	return nil
}

func (d *Distro) fetchReleases(ctx context.Context) error {
	d.log.Debugf("fetching releases")

	allreleases, _, err := d.github.Repositories.ListReleases(ctx, d.Owner, d.Repo, &github.ListOptions{})
	if err != nil {
		d.log.WithError(err).Error("error listing releases from github")
		return err
	}

	if len(allreleases) == 0 {
		err := fmt.Errorf("repository has no releases")
		d.log.Error("repository has no releases")
		return err
	}

	d.releases = allreleases

	d.selected = d.releases[0]

	return nil
}

func (d *Distro) verifyRelease() error {
	for _, asset := range d.selected.Assets {
		if *asset.Name == "manifest.yaml" || *asset.Name == "manifest.json" {
			assetURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/assets/%d", d.Owner, d.Repo, asset.GetID())
			// asset.GetBrowserDownloadURL()

			headers := map[string]string{
				"accept": "application/octet-stream",
			}

			contents, err := utils.DownloadFileToBytes(assetURL, d.dlHttp, headers)
			if err != nil {
				return err
			}

			if err := yaml.Unmarshal(contents, &d.Manifest); err != nil {
				return err
			}

			continue
		}
	}

	if d.Manifest == nil && !d.IsAlias {
		return fmt.Errorf("no manifest found for release")
	}

	if d.Manifest == nil && d.IsAlias {
		d.Manifest = manifests[d.Alias]
	}

	return nil
}

func (d *Distro) validateFile(dir string, filename string) error {
	d.log.WithField("filename", filename).Info("validating file checksum")

	filename = filepath.Join(dir, filename)

	if exists, err := utils.FileExists(fmt.Sprintf("%s.valid", filename)); err != nil {
		return err
	} else if exists {
		return nil
	}

	hasher := sha256.New()
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(hasher, f); err != nil {
		return err
	}

	actual := fmt.Sprintf("%x", hasher.Sum(nil))

	expectedBytes, err := ioutil.ReadFile(fmt.Sprintf("%s.sha256", filename))
	if err != nil {
		return err
	}

	expected := strings.Split(string(expectedBytes), " ")[0]

	if actual == expected {
		if _, err := os.Create(fmt.Sprintf("%s.valid", filename)); err != nil {
			return err
		}

		return nil
	} else {
		return fmt.Errorf("hashes do not match: expected: %s, actual: %s", expected, actual)
	}
}

func (d *Distro) validatePGPSignature(dir, filename string) error {
	d.log.WithField("filename", filename).Info("validating file pgp signature")

	filename = filepath.Join(dir, filename)

	if exists, err := utils.FileExists(fmt.Sprintf("%s.valid", filename)); err != nil {
		return err
	} else if exists {
		return nil
	}

	fileContent, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	// Get a Reader for the signature file
	sigFile, err := os.Open(fmt.Sprintf("%s.asc", filename))
	if err != nil {
		return err
	}
	defer sigFile.Close()

	defer func() {
		if err := sigFile.Close(); err != nil {
			panic(err)
		}
	}()

	block, err := armor.Decode(sigFile)
	if err != nil {
		return fmt.Errorf("error decoding signature file: %s", err)
	}
	if block.Type != "PGP SIGNATURE" {
		return errors.New("not an armored signature or message")
	}

	// Read the signature file
	pack, err := packet.Read(block.Body)
	if err != nil {
		return err
	}

	// Was it really a signature file ? If yes, get the Signature
	signature, ok := pack.(*packet.Signature)
	if !ok {
		return errors.New("not a valid signature file")
	}

	block, err = armor.Decode(bytes.NewReader([]byte(common.PGPPublicKey)))
	if err != nil {
		return fmt.Errorf("error decoding public key: %s", err)
	}
	if block.Type != "PGP PUBLIC KEY BLOCK" {
		return errors.New("not an armored public key")
	}

	// Read the key
	pack, err = packet.Read(block.Body)
	if err != nil {
		return fmt.Errorf("error reading public key: %s", err)
	}

	// Was it really a public key file ? If yes, get the PublicKey
	publicKey, ok := pack.(*packet.PublicKey)
	if !ok {
		return errors.New("invalid public key")
	}

	// Get the hash method used for the signature
	hash := signature.Hash.New()

	// Hash the content of the file (if the file is big, that's where you have to change the code to avoid getting the whole file in memory, by reading and writting in small chunks)
	_, err = hash.Write(fileContent)
	if err != nil {
		return err
	}

	// Check the signature
	err = publicKey.VerifySignature(hash, signature)
	if err != nil {
		return err
	}

	// Mark file as Valid
	if _, err := os.Create(fmt.Sprintf("%s.valid", filename)); err != nil {
		return err
	}

	return nil
}
