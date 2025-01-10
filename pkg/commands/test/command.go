package test

import (
	"errors"
	"fmt"
	"github.com/ekristen/cast/pkg/commands"
	"github.com/ekristen/cast/pkg/common"
	"github.com/ekristen/cast/pkg/config"
	"github.com/urfave/cli/v2"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func Execute(c *cli.Context) error {
	if c.Args().Len() != 1 {
		return fmt.Errorf("expect a single argument")
	}

	if c.Path("dir") != "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		if err := os.Chdir(c.Path("dir")); err != nil {
			return err
		}
		defer func(dir string) {
			_ = os.Chdir(dir)
		}(cwd)
	}

	state := c.Args().First()

	cfg, err := config.Load(c.Path("config"))
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// docker run -it --rm --name="sift-state-${STATE}" -v `pwd`/sift:/srv/salt/sift --cap-add SYS_ADMIN teamdfir/sift-saltstack-tester:${DISTRO} \
	//  salt-call -l debug --local --retcode-passthrough --state-output=mixed state.sls ${STATE} pillar="{sift_user: root}"

	basePath := cwd
	if cfg.Manifest.Base != "" {
		basePath = filepath.Join(basePath, cfg.Manifest.Base)
	}

	name := fmt.Sprintf("cast-test-state-%s", randomString(9))

	args := []string{
		"run", "-i", "--rm",
		fmt.Sprintf(`--name=%s`, name),
		fmt.Sprintf("--volume=%s:/srv/salt/%s", basePath, cfg.Manifest.Name),
		`--cap-add=SYS_ADMIN`,
		fmt.Sprintf("--platform=%s", c.String("platform")),
		c.String("image"),
		"salt-call", "-l", "debug", "--local", "--retcode-passthrough",
		"--state-output=mixed", "state.sls", state,
	}

	cmd := exec.CommandContext(c.Context, "docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			os.Exit(exitError.ExitCode())
		}
		return err
	}

	return nil
}

const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

func randomString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func init() {
	flags := []cli.Flag{
		&cli.PathFlag{
			Name:  "config",
			Value: ".cast.yml",
		},
		&cli.PathFlag{
			Name:   "dir",
			Hidden: true,
		},
		&cli.StringFlag{
			Name:    "user",
			Usage:   "The user to install against (cannot be root)",
			EnvVars: []string{"SUDO_USER"},
		},
		&cli.StringFlag{
			Name:  "image",
			Value: "ghcr.io/ekristen/cast-tools/saltstack-tester:24.04-3006",
		},
		&cli.StringFlag{
			Name:  "platform",
			Value: fmt.Sprintf("linux/%s", runtime.GOARCH),
		},
	}

	cmd := &cli.Command{
		Name:   "test-state",
		Usage:  "test a state",
		Flags:  append(flags, commands.GlobalFlags()...),
		Before: commands.GlobalBefore,
		Action: Execute,
	}

	common.RegisterCommand(cmd)
}
