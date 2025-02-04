package saltstack

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"html/template"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/ekristen/cast/pkg/sysinfo"
	"github.com/ekristen/cast/pkg/utils"

	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/packet"
)

type Mode int

const (
	Package Mode = iota
	Binary
	OneDir
)

type Meta struct {
	MajorVersion string
	MinorVersion string
	Version      string
	OS           *sysinfo.OS
}

func (m *Meta) Render(val string) (string, error) {
	tmpl, err := template.New("template").Parse(val)
	if err != nil {
		return "", err
	}

	var content bytes.Buffer
	if err := tmpl.Execute(&content, m); err != nil {
		return "", err
	}

	return content.String(), nil
}

type Config struct {
	Path string
}

type Installer struct {
	Mode   Mode
	Config *Config

	log *logrus.Entry
}

func New(config *Config) *Installer {
	return &Installer{
		Mode:   OneDir,
		Config: config,

		log: logrus.WithField("component", "saltstack-installer"),
	}
}

func NewConfig() *Config {
	return &Config{}
}

func (i *Installer) SetMode(mode Mode) {
	i.Mode = mode
}

func (i *Installer) GetBinary() string {
	switch i.Mode {
	case Package:
		return "/usr/bin/salt-call"
	case OneDir:
		return filepath.Join(i.Config.Path, "salt", "salt-call")
	default:
		return ""
	}
}

func (i *Installer) GetMode() Mode {
	return i.Mode
}

func (i *Installer) Run(ctx context.Context) error {
	err := os.MkdirAll(i.Config.Path, 0755)
	if err != nil {
		return err
	}

	metadata := Meta{
		Version:      Version,
		MajorVersion: strings.Split(Version, ".")[0],
		MinorVersion: strings.Split(Version, ".")[1],
		OS:           sysinfo.GetOSInfo(),
	}

	switch i.Mode {
	case Binary:
		return fmt.Errorf("binary mode no longer supported")
	case Package:
		return i.installPackage(ctx, metadata)
	case OneDir:
		return i.installOneDir(ctx, metadata)
	default:
		return fmt.Errorf("unsupported install mode")
	}
}

func (i *Installer) installOneDir(ctx context.Context, metadata Meta) error {
	log := i.log.WithField("handler", "install-onedir")

	tarFile := filepath.Join(i.Config.Path, "salt.tar.xz")
	hashFile := filepath.Join(i.Config.Path, "salt.tar.xz.sha256")

	log.Info("downloading tar.gz file")
	onedirURL, err := metadata.Render(OneDirURL)
	if err != nil {
		return err
	}
	log.Debugf("binary url: %s", onedirURL)
	if err := utils.DownloadFile(ctx, onedirURL, tarFile, nil, nil); err != nil {
		return err
	}

	log.Info("downloading hash file")
	hashURL, err := metadata.Render(OneDirHashURL)
	if err != nil {
		return err
	}
	log.Debugf("hash url: %s", hashURL)
	if err := utils.DownloadFile(ctx, hashURL, hashFile, nil, nil); err != nil {
		return err
	}

	log.Info("validating tar.gz.file")
	if err := i.validateFile(tarFile, "sha256", sha256.New); err != nil {
		return err
	}

	log.Info("extracting file")
	if err := utils.ExtractArchive(tarFile, i.Config.Path); err != nil {
		return err
	}

	i.log.Info("install-onedir done")

	return nil
}

func (i *Installer) installPackage(ctx context.Context, metadata Meta) error {
	switch metadata.OS.Vendor {
	case "ubuntu":
		i.log.Debug("checking salt install on ubuntu")

		runAptGetUpdate := false
		runAptGetInstall := false

		if err := i.installPackageKey(ctx); err != nil {
			return err
		}

		exists, err := utils.FileExists("/etc/apt/sources.list.d/saltstack.list")
		if err != nil {
			return err
		}

		if exists {
			i.log.Debug("old saltstack.list exists, renaming to salt.list")
			if err := os.Rename("/etc/apt/sources.list.d/saltstack.list", "/etc/apt/sources.list.d/salt.list"); err != nil {
				return err
			}
			runAptGetUpdate = true
		}

		exists, err = utils.FileExists("/etc/apt/sources.list.d/salt.list")
		if err != nil {
			return err
		}

		aptRepo, err := metadata.Render(APTRepo)
		if err != nil {
			return err
		}

		i.log.Debugf("apt repo: %s", aptRepo)

		if err := os.WriteFile("/etc/apt/sources.list.d/salt.list", []byte(aptRepo), 0644); err != nil {
			return err
		}

		saltCallExists, err := utils.FileExists("/usr/bin/salt-call")
		if err != nil {
			return err
		}

		if !saltCallExists {
			runAptGetInstall = true
		}

		if runAptGetUpdate || runAptGetInstall {
			i.log.Info("updating apt")
			if err := i.runCommand(ctx, "apt-get", "update"); err != nil {
				i.log.WithError(err).Error("unable to run apt-get update")
				return err
			}
		}

		if runAptGetInstall {
			i.log.Info("installing saltstack")
			args := []string{
				"install",
				"-o", "Dpkg::Options::=--force-confdef",
				"-o", "Dpkg::Options::=--force-confold",
				"-y",
				"--allow-change-held-packages",
				"--no-install-suggests",
				"salt-common",
			}
			if err := i.runCommand(ctx, "apt-get", args...); err != nil {
				i.log.WithError(err).Error("unable to apt-get install salt-common")
				return err
			}
		}

		exists, err = utils.FileExists("/usr/bin/salt-call")
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("salt-call not found at /usr/bin/salt-call")
		}

		i.log.Info("salt installed properly")
	default:
		return fmt.Errorf("unsupported operating system: %s", metadata.OS.Vendor)
	}

	return nil
}

func (i *Installer) installPackageKey(ctx context.Context) error {
	log := i.log.WithField("handler", "install-package-key")

	log.Info("downloading saltstack package key")

	if err := os.MkdirAll(filepath.Base(RepoKeyFile), 0755); err != nil {
		return err
	}

	if err := utils.DownloadFile(ctx, RepoKeyURL, RepoKeyFile, nil, nil); err != nil {
		return err
	}

	log.Info("saltstack package key downloaded")

	return nil
}

func (i *Installer) runCommand(ctx context.Context, command string, args ...string) error {
	logger := i.log.WithField("command", command)

	logger.Debug("running command")

	cmd := exec.Command(command, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if startErr := cmd.Start(); startErr != nil {
		return startErr
	}

	go func() {
		<-ctx.Done()

		glogger := i.log.WithField("pid", cmd.Process.Pid)

		glogger.Warnf("parent context signaled done, killing %s process", command)

		if err := cmd.Process.Kill(); err != nil {
			glogger.Fatal(err)
			return
		}

		glogger.Warnf("%s killed", command)
	}()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		pre := scanner.Text()
		pre = strings.TrimPrefix(pre, "# ")
		logger.Trace(pre)
	}

	if waitErr := cmd.Wait(); waitErr != nil {
		return waitErr
	}

	if cmd.ProcessState.ExitCode() > 0 {
		logger.Errorf("process exited %d", cmd.ProcessState.ExitCode())
		return fmt.Errorf("process exited %d", cmd.ProcessState.ExitCode())
	}

	return nil
}

func (i *Installer) validateFile(filename string, hashSuffix string, hashFunc func() hash.Hash) error {
	i.log.WithField("filename", filename).Info("validating file checksum")

	if exists, err := utils.FileExists(fmt.Sprintf("%s.valid", filename)); err != nil {
		return err
	} else if exists {
		return nil
	}

	hasher := hashFunc()
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(hasher, f); err != nil {
		return err
	}

	actual := fmt.Sprintf("%x", hasher.Sum(nil))

	expectedBytes, err := os.ReadFile(fmt.Sprintf("%s.%s", filename, hashSuffix))
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
		return fmt.Errorf("hashes do not match")
	}
}

func (i *Installer) validateSignature(filename string) error {
	i.log.WithField("filename", filename).Info("validating file signature")

	if exists, err := utils.FileExists(fmt.Sprintf("%s.valid", filename)); err != nil {
		return err
	} else if exists {
		return nil
	}

	fileContent, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	// Get a Reader for the signature file
	sigFile, err := os.Open(fmt.Sprintf("%s.asc", filename))
	if err != nil {
		return err
	}

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

	block, err = armor.Decode(bytes.NewReader([]byte(PublicKey)))
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
