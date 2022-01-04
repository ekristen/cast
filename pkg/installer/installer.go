package installer

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/ekristen/cast/pkg/saltstack"
)

type Mode int

const (
	LocalInstallMode Mode = iota
	LocalUpgradeMode
	LocalRemoveMode
	RemoteInstallMode
	RemoteUpgradeMode
	RemoteRemoveMode
)

type Installer struct {
	ctx context.Context
	log *logrus.Entry

	Mode Mode

	config *Config

	configRoot string
	logRoot    string
	logFile    string

	command string
}

type Config struct {
	Mode                 Mode
	CachePath            string
	NoRootCheck          bool
	SaltStackUser        string
	SaltStackState       string
	SaltStackTest        bool
	SaltStackLogLevel    string
	SaltStackFileRoot    string
	SaltStackInstallMode saltstack.Mode
}

func New(ctx context.Context, config *Config) *Installer {
	return &Installer{
		ctx:     ctx,
		log:     logrus.WithField("component", "installer"),
		config:  config,
		command: "salt-call",
	}
}

func (i *Installer) Run() (err error) {
	i.log.Info("checking if install can progress")
	if err := i.checks(); err != nil {
		return err
	}

	i.log.Debug("configuring the installer")
	if err := i.setup(); err != nil {
		return err
	}

	i.log.Debug("configuring saltstack installer")
	sconfig := saltstack.NewConfig()
	sconfig.Path = filepath.Join(i.config.CachePath, "saltstack")

	i.log.Info("running saltstack installer")
	sinstaller := saltstack.New(sconfig)
	if err := sinstaller.Run(); err != nil {
		return err
	}

	i.command = sinstaller.GetBinary()

	i.log.Info("running cast installer")

	i.log.Infof("installing as user: %s", i.config.SaltStackUser)

	switch i.config.Mode {
	case LocalInstallMode:
		i.log.Info("performing local install")
		return i.localRun()
	default:
		return fmt.Errorf("unsupported install mode: %d", i.Mode)
	}
}

func (i *Installer) checks() error {
	if os.Geteuid() == 0 {
		sudoUser := os.Getenv("SUDO_USER")
		if i.config.SaltStackUser == "" && sudoUser == "" {
			return fmt.Errorf("--user was not provided, or install was not ran with sudo")
		}
	}

	return nil
}

func (i *Installer) setup() error {
	i.logRoot = filepath.Join(i.config.CachePath, "logs")
	if err := os.MkdirAll(i.logRoot, 0755); err != nil {
		return err
	}
	i.logFile = filepath.Join(i.logRoot, "saltstack.log")

	i.configRoot = filepath.Join(i.config.CachePath, "salt")
	if err := os.MkdirAll(i.configRoot, 0755); err != nil {
		return err
	}

	data := []byte("enable_fqdns_grains: False\n")
	if err := os.WriteFile(filepath.Join(i.configRoot, "minion"), data, 0644); err != nil {
		return err
	}

	return nil
}

func (i *Installer) localRun() error {
	if err := i.runSaltstack(); err != nil {
		return err
	}

	return nil
}

func (i *Installer) runSaltstack() error {
	i.log.Info("starting saltstack run")

	beginRegexp := regexp.MustCompile(`\[INFO\s+\] Running state \[(.+)\] at time (.*)$`)
	endRegexp := regexp.MustCompile(`\[INFO\s+\] Completed state \[(.*)\] at time (.*) (\()?duration_in_ms=([\d.]+)(\))?$`)
	execRegexp := regexp.MustCompile(`\[INFO\s+\] Executing state \[(.*)\] for \[(.*)\]$`)
	resRegexp := regexp.MustCompile(`^\[.*$`)

	args := []string{
		"--config-dir", i.configRoot,
		"--local",
		"--retcode-passthrough",
		"-l", i.config.SaltStackLogLevel,
		"--out", "yaml",
		"--file-root", i.config.SaltStackFileRoot,
		"--no-color",
		"state.apply",
		i.config.SaltStackState,
		fmt.Sprintf(`pillar={sift_user: "%s"}`, i.config.SaltStackUser),
	}

	if i.config.SaltStackTest {
		args = append(args, "test=True")
	}
	if !strings.HasSuffix(i.command, "-call") {
		args = append([]string{"call"}, args...)
	}

	i.log.Debugf("running command %s %s", i.command, args)

	logFile, err := os.OpenFile(i.logFile, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		i.log.WithError(err).Error("unable to open log file for writing")
		return err
	}
	defer logFile.Close()

	cmd := exec.Command(i.command, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, _ := cmd.StderrPipe()
	if err != nil {
		return err
	}

	cmd.Start()

	go func() {
		<-i.ctx.Done()

		log := i.log.WithField("pid", cmd.Process.Pid)

		log.Warn("parent context signaled done, killing salt-call process")

		if err := cmd.Process.Kill(); err != nil {
			log.Fatal(err)
			return
		}

		log.Warn("salt-call killed")
		log.WithField("log", i.logFile).Info("log file location")
	}()

	inStateExecution := false
	inStateFailure := false
	inStateStartTime := ""

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		m := strings.TrimPrefix(scanner.Text(), "# ")

		_, err := logFile.Write([]byte(m))
		if err != nil {
			i.log.WithError(err).Warn("unable to write to log file")
		}

		log := i.log.WithField("component", "saltstack")

		if !inStateExecution {
			if beginRegexp.MatchString(m) {
				matches := beginRegexp.FindAllStringSubmatch(m, -1)

				fields := logrus.Fields{
					"state":      matches[0][1],
					"time_start": matches[0][2],
				}
				inStateStartTime = matches[0][2]

				log.WithFields(fields).Trace(m)

				i.log.WithFields(fields).Info("running state")
				inStateExecution = true
			} else {
				i.log.WithField("component", "saltstack").Trace(m)
			}
		} else {
			if m == "" {
				continue
			}

			if execRegexp.MatchString(m) {
				matches := execRegexp.FindAllStringSubmatch(m, -1)
				log.Trace(m)
				i.log.Infof("Executing %s for %s", matches[0][1], matches[0][2])
			} else if !resRegexp.MatchString(m) {
				log.Trace(m)
				if strings.HasSuffix(m, "Failure!") {
					inStateFailure = true
					i.log.Warnf("Result: %s", m)
				} else {
					i.log.Debugf("Result: %s", m)
				}
			} else if endRegexp.MatchString(m) {
				matches := endRegexp.FindAllStringSubmatch(m, -1)

				duration := matches[0][3]
				if len(matches[0]) > 3 {
					duration = matches[0][4]
				}

				fields := logrus.Fields{
					"state":      matches[0][1],
					"time_start": inStateStartTime,
					"time_end":   matches[0][2],
					"duration":   duration,
				}

				inStateStartTime = ""

				log.WithFields(fields).Trace(m)

				if inStateFailure {
					i.log.WithFields(fields).Error("state failed")
				} else {
					i.log.WithFields(fields).Info("state completed")
				}

				inStateExecution = false
				inStateFailure = false
			} else {
				i.log.WithField("component", "saltstack").Trace(m)
			}
		}
	}

	y, err := io.ReadAll(stdout)
	if err != nil {
		return err
	}

	cmd.Wait()

	i.log.WithField("log", i.logFile).Info("log file location")

	switch cmd.ProcessState.ExitCode() {
	// This is hit when salt-call encounters an error
	case 1:
		var results saltstack.LocalResultsErrors
		if err := yaml.Unmarshal(y, &results); err != nil {
			return err
		}

		i.log.Warn(string(y))

		i.log.Error("salt-call finished with errors")
	// This is hit when we kill salt-call because of a signals
	// handler trap on the main cli process
	case -1:
		i.log.Warn("salt-call terminated")
	default:
		var results saltstack.LocalResults
		if err := yaml.Unmarshal(y, &results); err != nil {
			return err
		}

		i.log.WithFields(logrus.Fields{
			"total": len(results.Local),
		}).Info("statistics")

		i.log.WithField("exitcode", cmd.ProcessState.ExitCode()).Info("salt-call completed successfully")
	}

	return nil
}
