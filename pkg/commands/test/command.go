package test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/urfave/cli/v2"

	"github.com/ekristen/cast/pkg/commands"
	"github.com/ekristen/cast/pkg/common"
	"github.com/ekristen/cast/pkg/config"
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
		defer os.Chdir(cwd)
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

	args := []string{
		"run", "-i", "--rm",
		`--name=cast-state`,
		fmt.Sprintf("--volume=%s:/srv/salt/%s", basePath, cfg.Manifest.Name),
		`--cap-add=SYS_ADMIN`,
		c.String("image"),
		"salt-call", "-l", "debug", "--local", "--retcode-passthrough",
		"--state-output=mixed", "state.sls", state,
	}

	cmd := exec.CommandContext(c.Context, "docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
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
			Value: "ghcr.io/ekristen/cast-tools/saltstack-tester:focal-3004",
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
