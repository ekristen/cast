package installer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
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
	outFile    string

	command string

	pillarJSON string
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
	SaltStackPillars     map[string]string
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

	i.log.Info("preparing pillar data")
	pillarJSON, err := json.Marshal(i.config.SaltStackPillars)
	if err != nil {
		return err
	}
	i.pillarJSON = string(pillarJSON)

	i.log.Debug("configuring saltstack installer")
	sconfig := saltstack.NewConfig()
	sconfig.Path = filepath.Join(i.config.CachePath, "saltstack")

	i.log.Info("running saltstack installer")
	sinstaller := saltstack.New(sconfig)
	sinstaller.SetMode(saltstack.Binary)

	if err := sinstaller.Run(i.ctx); err != nil {
		return err
	}

	i.command = sinstaller.GetBinary()
	if i.command == "" {
		return fmt.Errorf("unable to resolve salt binary to use")
	}

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
	i.outFile = filepath.Join(i.logRoot, "results.yaml")

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
		fmt.Sprintf(`pillar=%s`, i.pillarJSON),
	}

	if i.config.SaltStackTest {
		args = append(args, "test=True")
	}
	if !strings.HasSuffix(i.command, "-call") {
		args = append([]string{"call"}, args...)
	}

	i.log.Debugf("running command %s %s", i.command, args)

	logFile, err := os.OpenFile(i.logFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		i.log.WithError(err).Error("unable to open log file for writing")
		return err
	}
	defer logFile.Close()

	var out bytes.Buffer

	cmd := exec.CommandContext(i.ctx, i.command, args...)
	cmd.Stdout = &out

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	teeStderr := io.TeeReader(stderr, logFile)
	scanner := bufio.NewScanner(teeStderr)

	done := make(chan struct{})

	/*
		go func() {
			<-i.ctx.Done()

			if cmd == nil || cmd.Process == nil {
				return
			}

			log := i.log.WithField("pid", cmd.Process.Pid)

			log.Warn("parent context signaled done, killing salt-call process")

			if err := cmd.Process.Kill(); err != nil {
				log.Fatal(err)
				return
			}

			log.Warn("salt-call killed")
			log.WithField("log", i.logFile).Info("log file location")
		}()
	*/

	go func() {
		inStateExecution := false
		inStateFailure := false
		inStateStartTime := ""

		for scanner.Scan() {
			m := strings.TrimPrefix(scanner.Text(), "# ")

			log := i.log.WithField("component", "saltstack")

			if !inStateExecution {
				if beginRegexp.MatchString(m) {
					matches := beginRegexp.FindAllStringSubmatch(m, -1)

					fields := logrus.Fields{
						"state":      matches[0][1],
						"time_begin": matches[0][2],
					}
					inStateStartTime = matches[0][2]

					log.WithFields(fields).Trace(m)

					i.log.WithFields(fields).Debug("running state")
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
						"time_begin": inStateStartTime,
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

		i.log.Debug("signaling stderr read is complete")
		done <- struct{}{}
	}()

	i.log.Debug("executing cmd.start")
	if err := cmd.Start(); err != nil {
		return err
	}

	i.log.Debug("waiting for stderr read to complete")

	<-done

	i.log.Debug("reading from stderr is done")

	// Note: we do not look for error here because
	// we do it via the exit code down lower
	cmd.Wait()

	// TODO: write out to a file

	if _, err := logFile.Write(out.Bytes()); err != nil {
		i.log.WithError(err).Error("unable to write to log file")
	}
	if err := ioutil.WriteFile(i.outFile, out.Bytes(), 0640); err != nil {
		i.log.WithError(err).Error("unable to write to out file")
	}

	i.log.WithField("file", i.logFile).Info("log file location")
	i.log.WithField("file", i.outFile).Info("results file location")

	switch code := cmd.ProcessState.ExitCode(); {
	// This is hit when salt-call encounters an error
	case code == 1:
		i.log.WithField("code", code).Error("salt-call finished with errors")

		var results saltstack.LocalResultsErrors
		if err := yaml.Unmarshal(out.Bytes(), &results); err != nil {
			fmt.Println(out.String())
			return err
		}

		i.log.Warn(out.String())

		return fmt.Errorf(out.String())
	// This is hit when we kill salt-call because of a signals
	// handler trap on the main cli process
	case code == -1:
		i.log.Warn("salt-call terminated")

		return fmt.Errorf("salt-call terminated")
	case code == 2:
		if err := i.parseAndLogResults(out.Bytes()); err != nil {
			return err
		}

		i.log.Info("salt-call completed but had failed states")

		return fmt.Errorf("salt-call completed but had failed states")
	case code == 0:
		if err := i.parseAndLogResults(out.Bytes()); err != nil {
			return err
		}

		i.log.Info("salt-call completed successfully")
	}

	return nil
}

func (i *Installer) parseAndLogResults(in []byte) error {
	var results saltstack.LocalResults
	if err := yaml.Unmarshal(in, &results); err != nil {
		fmt.Println(in)
		return err
	}

	success, failed := 0, 0

	var firstFailedState saltstack.Result

	for _, r := range results.Local {
		switch r.Result {
		case true:
			success++
		case false:
			if failed == 0 {
				firstFailedState = r
			}
			failed++
		}
	}

	if failed > 0 {
		space := regexp.MustCompile(`\s+`)
		i.log.WithFields(logrus.Fields{
			"sls":     firstFailedState.SLS,
			"run_num": firstFailedState.RunNumber,
			"comment": strings.ReplaceAll(fmt.Sprintf("%q", space.ReplaceAllString(firstFailedState.Comment, " ")), `"`, ""),
		}).Warn("first failed state")
	}

	i.log.WithFields(logrus.Fields{
		"total":   len(results.Local),
		"success": success,
		"failed":  failed,
	}).Info("statistics")

	return nil
}
