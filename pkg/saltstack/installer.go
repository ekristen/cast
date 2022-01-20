package saltstack

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha512"
	"errors"
	"fmt"
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
)

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
	}

	return ""
}

func (i *Installer) GetMode() Mode {
	return i.Mode
}

func (i *Installer) Run(ctx context.Context) error {
	os.MkdirAll(i.Config.Path, 0755)

	switch i.Mode {
	case Binary:
		return i.installBinary(ctx)
	case Package:
		return i.installPackage(ctx)
	default:
		return fmt.Errorf("unsupported install mode")
	}
}

func (i *Installer) installBinary(ctx context.Context) error {
	log := i.log.WithField("handler", "install-binary")

	tarfile := filepath.Join(i.Config.Path, "saltstack-binary.tar.gz")
	hashfile := filepath.Join(i.Config.Path, "saltstack-binary.tar.gz.sha512")
	sigfile := filepath.Join(i.Config.Path, "saltstack-binary.tar.gz.sha512.asc")

	log.Info("downloading tar.gz file")
	if err := utils.DownloadFile(ctx, BinaryURL, tarfile, nil, nil); err != nil {
		return err
	}

	log.Info("downloading sha512 file")
	if err := utils.DownloadFile(ctx, HashURL, hashfile, nil, nil); err != nil {
		return err
	}

	log.Info("downloading signature file")
	if err := utils.DownloadFile(ctx, SigURL, sigfile, nil, nil); err != nil {
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

func (i *Installer) installPackage(ctx context.Context) error {
	osinfo := sysinfo.GetOSInfo()

	switch osinfo.Vendor {
	case "ubuntu":
		i.log.Debug("checking salt install on ubuntu")

		runAptGetUpdate := false
		runAptGetInstall := false

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

		if !exists {
			aptlist := fmt.Sprintf(UbuntuRepo, osinfo.Architecture, osinfo.Version, osinfo.Architecture, osinfo.Codename)
			if err := ioutil.WriteFile("/etc/apt/sources.list.d/salt.list", []byte(aptlist), 0644); err != nil {
				return err
			}
			runAptGetInstall = true
		} else {
			i.log.Debug("salt configured with apt")
		}

		if runAptGetUpdate || runAptGetInstall {
			if err := i.runCommand(ctx, "apt-get", "update"); err != nil {
				return err
			}
		}

		if runAptGetInstall {
			args := []string{"install", "-o", `Dpkg::Options::="--force-confdef"`, "-o", `Dpkg::Options::="--force-confold"`, "-y", "--allow-change-held-packages", "salt-common"}
			if err := i.runCommand(ctx, "apt-get", args...); err != nil {
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
		return fmt.Errorf("unsupported operating system: %s", osinfo.Vendor)
	}

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
		m := strings.TrimPrefix(scanner.Text(), "# ")
		log.Trace(m)
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
