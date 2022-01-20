package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ekristen/cast/pkg/cache"
	"github.com/ekristen/cast/pkg/commands"
	"github.com/ekristen/cast/pkg/common"
	"github.com/ekristen/cast/pkg/distro"
	"github.com/ekristen/cast/pkg/installer"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func Execute(c *cli.Context) error {
	ctx := signals.SetupSignalHandler(c.Context)

	log := logrus.WithField("command", "install")

	if c.Args().Len() != 1 {
		return fmt.Errorf("please provide a distro alias to install")
	}

	cachePath := c.Path("cache-path")
	if c.Bool("dev") {
		cachePath = filepath.Join(os.TempDir(), c.Path("cache-path"))
	}

	cache, err := cache.New(cachePath)
	if err != nil {
		return err
	}

	distroName := c.Args().First()
	distroParts := strings.Split(distroName, "@")
	distroVersion := ""
	if len(distroParts) == 2 {
		distroName = distroParts[0]
		distroVersion = distroParts[1]
	}

	distro, err := distro.New(ctx, distroName, &distroVersion, c.Bool("pre-release"), c.String("github-token"))
	if err != nil {
		return err
	}

	log.Info("distro validated successfully")

	distroCache, err := cache.NewSubpath(distro.GetCachePath())
	if err != nil {
		return err
	}

	if err := distro.Download(distroCache.GetPath()); err != nil {
		return err
	}

	log.Info("distro downloaded successfully")

	installerCache, err := cache.NewSubpath("installer")
	if err != nil {
		return err
	}

	state := c.String("saltstack-state")
	if state == "" {
		mode := c.String("mode")
		log.Infof("installing using mode: %s", mode)
		state, err = distro.GetModeState(mode)
		if err != nil {
			return err
		}
	} else {
		log.Infof("installing using state: %s", state)
	}

	fileRoot := c.Path("saltstack-file-root")
	if fileRoot == "" {
		fileRoot = filepath.Join(distroCache.GetPath(), "source")
	}

	config := &installer.Config{
		Mode:              installer.LocalInstallMode,
		CachePath:         installerCache.GetPath(),
		NoRootCheck:       c.Bool("no-root-check"),
		SaltStackUser:     c.String("user"),
		SaltStackState:    state,
		SaltStackTest:     c.Bool("saltstack-test"),
		SaltStackFileRoot: fileRoot,
		SaltStackLogLevel: c.String("saltstack-log-level"),
	}

	instance := installer.New(ctx, config)

	if err := instance.Run(); err != nil {
		return err
	}

	return nil
}

func init() {
	flags := []cli.Flag{
		&cli.StringFlag{
			Name:    "github-token",
			Usage:   "Used to authenticate to the GitHub API",
			EnvVars: []string{"GITHUB_TOKEN"},
		},
		/*
			&cli.StringFlag{
				Name:  "ssh-host",
				Usage: "Remote SSH Host",
			},
			&cli.IntFlag{
				Name:  "ssh-port",
				Usage: "Port of the Remote SSH Host",
				Value: 22,
			},
		*/
		&cli.BoolFlag{
			Name:  "pre-release",
			Usage: "Include pre-release versions as valid install targets",
		},
		&cli.StringFlag{
			Name:  "mode",
			Usage: "If the distro supports a mode, you can specify it for install",
		},
		&cli.StringFlag{
			Name:    "user",
			Usage:   "The user to install against (cannot be root)",
			EnvVars: []string{"SUDO_USER"},
		},
		&cli.PathFlag{
			Name:  "cache-path",
			Usage: "The path where the tool caches files",
			Value: "/var/cache/cast",
		},
		&cli.BoolFlag{
			Name:  "no-cache",
			Usage: "Do not use any cached files",
		},
		// Hidden Flags
		&cli.BoolFlag{
			Name:   "dev",
			Usage:  "(dev) Enable Development Mode",
			Hidden: true,
		},
		&cli.BoolFlag{
			Name:   "no-root-check",
			Usage:  "(dev) disable checking if user is root",
			Hidden: true,
		},
		&cli.BoolFlag{
			Name:    "saltstack-test",
			Aliases: []string{"st"},
			EnvVars: []string{"CAST_SALTSTACK_TEST"},
			Usage:   "Enable SaltStack Test Mode",
			Hidden:  true,
		},
		&cli.StringFlag{
			Name:    "saltstack-state",
			Aliases: []string{"ss"},
			Usage:   "Specific SaltStack State to use as entrypoint",
			Hidden:  true,
		},
		&cli.StringFlag{
			Name:   "saltstack-file-root",
			Usage:  "Use a specific directory for the file root for SaltStack",
			Hidden: true,
		},
		&cli.StringFlag{
			Name:   "saltstack-log-level",
			Usage:  "Log level for Saltstack",
			Value:  "info",
			Hidden: true,
		},
	}

	cmd := &cli.Command{
		Name:   "install",
		Usage:  "install a cast compatible distro",
		Flags:  append(flags, commands.GlobalFlags()...),
		Before: commands.GlobalBefore,
		Action: Execute,
	}

	common.RegisterCommand(cmd)
}
