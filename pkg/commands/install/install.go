package install

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/ekristen/cast/pkg/cache"
	"github.com/ekristen/cast/pkg/commands"
	"github.com/ekristen/cast/pkg/common"
	"github.com/ekristen/cast/pkg/distro"
	"github.com/ekristen/cast/pkg/installer"
	"github.com/ekristen/cast/pkg/saltstack"
)

func Execute(c *cli.Context) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("install does not support this operating system (%s) at this time", runtime.GOOS)
	}

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

	isLocal := false
	distroName := c.Args().First()

	if _, err := os.Stat(distroName); !os.IsNotExist(err) {
		isLocal = true
	} else if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	}

	distroParts := strings.Split(distroName, "@")
	distroVersion := ""
	if len(distroParts) == 2 {
		distroName = distroParts[0]
		distroVersion = distroParts[1]
	}

	distroData := map[string]string{
		"User": c.String("user"),
	}

	pillars := c.StringSlice("variable")
	for _, p := range pillars {
		parts := strings.Split(p, "=")
		if len(parts) != 2 {
			log.Warnf("invalid saltstack pillar (%s)", p)
			continue
		}
		distroData[parts[0]] = parts[1]
	}

	var dist distro.Distro
	if isLocal {
		log.WithField("name", distroName).WithField("version", distroVersion).Debug("detected distro information")

		dist, err = distro.NewLocal(ctx, distroName, &distroVersion, c.Bool("pre-release"), c.String("github-token"), distroData)
		if err != nil {
			return err
		}
	} else {
		dist, err = distro.NewGitHub(ctx, distroName, &distroVersion, c.Bool("pre-release"), c.String("github-token"), distroData)
		if err != nil {
			return err
		}
	}

	log.Info("distro validated successfully")

	distroCache, err := cache.NewSubpath(dist.GetCachePath())
	if err != nil {
		return err
	}

	log.WithField("path", distroCache.GetPath()).Debug("distro cache path")

	if err := dist.Download(distroCache.GetPath()); err != nil {
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
		state, err = dist.GetModeState(mode)
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

	ssim := saltstack.Package
	if c.String("saltstack-install-mode") == "binary" {
		ssim = saltstack.Binary
	}

	config := &installer.Config{
		Mode:                 installer.LocalInstallMode,
		CachePath:            installerCache.GetPath(),
		NoRootCheck:          c.Bool("no-root-check"),
		SaltStackUser:        c.String("user"),
		SaltStackState:       state,
		SaltStackTest:        c.Bool("saltstack-test"),
		SaltStackFileRoot:    fileRoot,
		SaltStackLogLevel:    c.String("saltstack-log-level"),
		SaltStackPillars:     dist.GetSaltstackPillars(),
		SaltStackInstallMode: ssim,
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
			EnvVars: []string{"GITHUB_TOKEN", "CAST_GITHUB_TOKEN"},
		},
		&cli.BoolFlag{
			Name:  "pre-release",
			Usage: "Include pre-release versions as valid install targets",
		},
		&cli.StringFlag{
			Name:    "mode",
			Usage:   "If the distro supports a mode, you can specify it for install",
			Value:   "default",
			EnvVars: []string{"CAST_MODE"},
		},
		&cli.StringFlag{
			Name:    "user",
			Usage:   "The user to install against (cannot be root)",
			EnvVars: []string{"SUDO_USER", "CAST_SUDO_USER"},
		},
		&cli.PathFlag{
			Name:    "cache-path",
			Usage:   "The path where the tool caches files",
			Value:   "/var/cache/cast",
			EnvVars: []string{"CAST_CACHE_PATH"},
		},
		&cli.BoolFlag{
			Name:    "no-cache",
			Usage:   "Do not use any cached files",
			EnvVars: []string{"CAST_NO_CACHE"},
		},
		&cli.StringSliceFlag{
			Name:    "variable",
			Usage:   "Variable to be made available for saltstack pillar templates",
			Aliases: []string{"var"},
			EnvVars: []string{"CAST_VARIABLE"},
		},
		// Hidden Flags
		&cli.BoolFlag{
			Name:    "dev",
			Usage:   "(dev) Enable Development Mode",
			EnvVars: []string{"CAST_DEVELOPMENT_MODE"},
			Hidden:  true,
		},
		&cli.BoolFlag{
			Name:    "no-root-check",
			Usage:   "(dev) disable checking if user is root",
			EnvVars: []string{"CAST_NO_ROOT_CHECK"},
			Hidden:  true,
		},
		&cli.BoolFlag{
			Name:    "saltstack-test",
			Usage:   "Enable SaltStack Test Mode",
			Aliases: []string{"st"},
			EnvVars: []string{"CAST_SALTSTACK_TEST"},
			Hidden:  true,
		},
		&cli.StringFlag{
			Name:    "saltstack-state",
			Usage:   "Specific SaltStack State to use as entrypoint",
			Aliases: []string{"ss"},
			EnvVars: []string{"CAST_SALTSTACK_STATE"},
			Hidden:  true,
		},
		&cli.StringFlag{
			Name:    "saltstack-file-root",
			Usage:   "Use a specific directory for the file root for SaltStack",
			Aliases: []string{"ssfr"},
			EnvVars: []string{"CAST_SALTSTACK_FILE_ROOT"},
			Hidden:  true,
		},
		&cli.StringFlag{
			Name:    "saltstack-log-level",
			Usage:   "Log level for Saltstack",
			Value:   "info",
			Aliases: []string{"ssll"},
			EnvVars: []string{"CAST_SALTSTACK_LOG_LEVEL"},
			Hidden:  true,
		},
		&cli.StringFlag{
			Name:    "saltstack-install-mode",
			Usage:   "Install Mode for Saltstack",
			Value:   "binary",
			Aliases: []string{"ssim"},
			EnvVars: []string{"CAST_SALTSTACK_INSTALL_MODE"},
			Hidden:  true,
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
