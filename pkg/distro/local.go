package distro

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/ekristen/cast/pkg/sysinfo"
	"gopkg.in/yaml.v3"

	cp "github.com/otiai10/copy"
	"github.com/sirupsen/logrus"
)

type LocalDistro struct {
	Owner   string
	Repo    string
	Version string
	Name    string
	Dir     string
	SaltDir string

	Alias   string
	IsAlias bool
	IsLocal bool

	Manifest *Manifest

	IncludePreReleases bool

	ctx context.Context
	log *logrus.Entry

	data interface{}
}

func NewLocal(ctx context.Context, distro string, version *string, includePreReleases bool, githubToken string, data interface{}) (Distro, error) {
	d := &LocalDistro{
		Owner: "local",
		Repo:  path.Base(distro),
	}
	d.IncludePreReleases = includePreReleases

	if version != nil {
		d.Version = *version
	}

	d.Name = fmt.Sprintf("%s_%s", d.Owner, d.Repo)

	distroPath, err := filepath.Abs(distro)
	if err != nil {
		return nil, err
	}
	d.Dir = distroPath

	d.ctx = ctx
	d.data = data
	d.log = logrus.WithField("component", "distro").WithField("type", "local").WithField("owner", d.Owner).WithField("repo", d.Repo)

	if err := d.verifyRelease(); err != nil {
		return nil, err
	}

	d.SaltDir = filepath.Join("source", d.Manifest.Name)
	if d.Manifest.Base != "" {
		d.SaltDir = filepath.Join(d.SaltDir, d.Manifest.Base)
	}
	fmt.Println(d.SaltDir)

	return d, nil
}

func (d *LocalDistro) GetSaltstackPillars() (pillars map[string]string) {
	pillars = d.Manifest.Saltstack.Pillars
	return d.Manifest.Saltstack.Pillars
}

func (d *LocalDistro) GetName() string {
	return d.Name
}

func (d *LocalDistro) GetReleaseName() string {
	return "local"
}

func (d *LocalDistro) GetModeState(mode string) (string, error) {
	for _, m := range d.Manifest.Modes {
		if (mode == "" || mode == "default") && m.Default {
			return m.State, nil
		} else if mode != "" && m.Name == mode {
			return m.State, nil
		}
	}

	return "", fmt.Errorf("unable to resolve state from mode: %s", mode)
}

func (d *LocalDistro) GetCachePath() string {
	cachePath := filepath.Join(d.GetName(), d.GetReleaseName())
	d.log.Debugf("cache path: %s", cachePath)
	return cachePath
}

func (d *LocalDistro) GetCacheSaltStackSourcePath() string {
	fileRootPath := filepath.Join("source", d.Manifest.Name)
	d.log.Debugf("salstack file root path: %s", fileRootPath)
	return fileRootPath
}

func (d *LocalDistro) Download(dir string) error {
	saltstackFileRootPath := d.GetCacheSaltStackSourcePath()
	if err := os.MkdirAll(saltstackFileRootPath, 0755); err != nil {
		return err
	}
	return cp.Copy(d.SaltDir, saltstackFileRootPath)
}

func (d *LocalDistro) verifyRelease() error {
	contents, err := ioutil.ReadFile(fmt.Sprintf("%s/.cast.yml", d.Dir))
	if err != nil {
		return err
	}

	var cfg LocalConfig
	if err := yaml.Unmarshal(contents, &cfg); err != nil {
		return err
	}

	d.Manifest = cfg.Manifest

	isSupported := len(d.Manifest.SupportedOS) == 0

	if !isSupported {
		d.log.Info("checking operating system support")
	}

	osinfo := sysinfo.GetOSInfo()
	for _, s := range d.Manifest.SupportedOS {
		mustmatch := 0
		match := 0

		if s.ID != "" {
			mustmatch++
		}
		if s.Codename != "" {
			mustmatch++
		}
		if s.Release != "" {
			mustmatch++
		}

		if s.ID != "" && strings.EqualFold(s.ID, osinfo.Vendor) {
			match++
		}
		if s.Codename != "" && strings.EqualFold(s.Codename, osinfo.Codename) {
			match++
		}
		if s.Release != "" && strings.EqualFold(s.Release, osinfo.Release) {
			match++
		}

		if match == mustmatch {
			isSupported = true
		}
	}

	if !isSupported {
		return fmt.Errorf("operating system is not supported")
	}

	d.log.Info("operating system is supported")

	d.log.Info("rendering manifest")
	if err := d.Manifest.Render(d.data); err != nil {
		return err
	}

	return nil
}
