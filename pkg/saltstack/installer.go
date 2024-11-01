package saltstack

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha512"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
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
		Mode:   Package,
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
	case Binary:
		return filepath.Join(i.Config.Path, "salt")
	case Package:
		return "/usr/bin/salt-call"
	case OneDir:
		return filepath.Join(i.Config.Path, "salt-call")
	}

	return ""
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
		return i.installBinary(ctx, metadata)
	case Package:
		return i.installPackage(ctx, metadata)
	case OneDir:
		return i.installOneDir(ctx, metadata)
	default:
		return fmt.Errorf("unsupported install mode")
	}
}

func (i *Installer) installOneDir(ctx context.Context, metadata Meta) error {
	// log := i.log.WithField("handler", "install-onedir")

	return nil
}

func (i *Installer) installBinary(ctx context.Context, metadata Meta) error {
	log := i.log.WithField("handler", "install-binary")

	tarfile := filepath.Join(i.Config.Path, "saltstack-binary.tar.gz")
	hashfile := filepath.Join(i.Config.Path, "saltstack-binary.tar.gz.sha512")
	sigfile := filepath.Join(i.Config.Path, "saltstack-binary.tar.gz.sha512.asc")

	log.Info("downloading tar.gz file")
	binaryURL, err := metadata.Render(BinaryURL)
	if err != nil {
		return err
	}
	log.Debugf("binary url: %s", binaryURL)
	if err := utils.DownloadFile(ctx, binaryURL, tarfile, nil, nil); err != nil {
		return err
	}

	log.Info("downloading sha512 file")
	hashURL, err := metadata.Render(HashURL)
	if err != nil {
		return err
	}
	log.Debugf("hash url: %s", hashURL)
	if err := utils.DownloadFile(ctx, hashURL, hashfile, nil, nil); err != nil {
		return err
	}

	log.Info("downloading signature file")
	sigURL, err := metadata.Render(SigURL)
	if err != nil {
		return err
	}
	log.Debugf("sig url: %s", sigURL)
	if err := utils.DownloadFile(ctx, sigURL, sigfile, nil, nil); err != nil {
		return err
	}

	log.Info("validating tar.gz.file")
	if err := i.validateFile(tarfile); err != nil {
		return err
	}

	log.Info("validating signature")
	if err := i.validateSignature(hashfile); err != nil {
		return err
	}

	log.Info("extracting file")
	if err := utils.ExtractTarballGz(tarfile, i.Config.Path); err != nil {
		return err
	}

	i.log.Info("install-binary done")

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
	log := i.log.WithField("command", command)

	log.Debug("running command")

	cmd := exec.Command(command, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	cmd.Start()

	go func() {
		<-ctx.Done()

		log := i.log.WithField("pid", cmd.Process.Pid)

		log.Warnf("parent context signaled done, killing %s process", command)

		if err := cmd.Process.Kill(); err != nil {
			log.Fatal(err)
			return
		}

		log.Warnf("%s killed", command)
	}()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		pre := scanner.Text()
		pre = strings.TrimPrefix(pre, "# ")
		log.Trace(pre)
	}

	cmd.Wait()

	if cmd.ProcessState.ExitCode() > 0 {
		log.Errorf("process exited %d", cmd.ProcessState.ExitCode())
		return fmt.Errorf("process exited %d", cmd.ProcessState.ExitCode())
	}

	return nil
}

func (i *Installer) validateFile(filename string) error {
	i.log.WithField("filename", filename).Info("validating file checksum")

	if exists, err := utils.FileExists(fmt.Sprintf("%s.valid", filename)); err != nil {
		return err
	} else if exists {
		return nil
	}

	hasher := sha512.New()
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(hasher, f); err != nil {
		return err
	}

	actual := fmt.Sprintf("%x", hasher.Sum(nil))

	expectedBytes, err := ioutil.ReadFile(fmt.Sprintf("%s.sha512", filename))
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

	fileContent, err := ioutil.ReadFile(filename)
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
