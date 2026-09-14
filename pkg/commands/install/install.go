package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rancher/wrangler/v3/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"

	"github.com/ekristen/cast/pkg/cache"
	"github.com/ekristen/cast/pkg/commands"
	"github.com/ekristen/cast/pkg/common"
	"github.com/ekristen/cast/pkg/distro"
	"github.com/ekristen/cast/pkg/installer"
	"github.com/ekristen/cast/pkg/saltstack"
	"github.com/ekristen/cast/pkg/state"
)

func Execute(ctx context.Context, cmd *cli.Command) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("install does not support this operating system (%s) at this time", runtime.GOOS)
	}

	ctx = signals.SetupSignalHandler(ctx)

	log := logrus.WithField("command", "install")

	if cmd.Args().Len() != 1 {
		return fmt.Errorf("please provide a distro alias to install")
	}

	cachePath := cmd.String("cache-path")
	if cmd.Bool("dev") {
		cachePath = filepath.Join(os.TempDir(), cmd.String("cache-path"))
	}

	cache, err := cache.New(cachePath)
	if err != nil {
		return err
	}

	isLocal := false
	distroName := cmd.Args().First()

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

	// Load saved installation state
	savedState, err := state.Load()
	if err != nil {
		log.WithError(err).Debug("failed to load saved state, continuing without it")
		savedState = &state.State{Installations: make(map[string]state.InstallState)}
	}

	// Use distroName as the key for state lookup
	distroKey := distroName

	distroData := map[string]string{
		"User": cmd.String("user"),
	}

	pillars := cmd.StringSlice("variable")
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

		dist, err = distro.NewLocal(ctx, distroName, &distroVersion, cmd.Bool("pre-release"), cmd.String("github-token"), distroData)
		if err != nil {
			return err
		}
	} else {
		dist, err = distro.NewGitHub(ctx, distroName, &distroVersion,
			cmd.Bool("no-os-check"),
			cmd.Bool("pre-release"), cmd.String("github-token"), distroData)
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

	saltState := cmd.String("saltstack-state")
	mode := cmd.String("mode")

	// Check if --mode was explicitly provided; if not, try to use saved mode
	if !cmd.IsSet("mode") {
		if saved, ok := savedState.GetInstallState(distroKey); ok && saved.Mode != "" {
			mode = saved.Mode
			log.Infof("using saved mode from previous installation: %s", mode)
		}
	}

	if saltState == "" {
		log.Infof("installing using mode: %s", mode)
		saltState, err = dist.GetModeState(mode)
		if err != nil {
			return err
		}
	} else {
		log.Infof("installing using state: %s", saltState)
	}

	fileRoot := cmd.String("saltstack-file-root")
	if fileRoot == "" {
		fileRoot = filepath.Join(distroCache.GetPath(), "source")
	}

	ssim := saltstack.Package
	if cmd.String("saltstack-install-mode") != "package" {
		return fmt.Errorf("due to changes with salt, the only install method is temporarily via package and apt")
	}

	config := &installer.Config{
		Mode:                 installer.LocalInstallMode,
		CachePath:            installerCache.GetPath(),
		NoRootCheck:          cmd.Bool("no-root-check"),
		SaltStackUser:        cmd.String("user"),
		SaltStackState:       saltState,
		SaltStackTest:        cmd.Bool("saltstack-test"),
		SaltStackFileRoot:    fileRoot,
		SaltStackLogLevel:    cmd.String("saltstack-log-level"),
		SaltStackPillars:     dist.GetSaltstackPillars(),
		SaltStackInstallMode: ssim,
	}

	instance := installer.New(ctx, config)

	if err := instance.Run(); err != nil {
		// Display failure message if defined
		if msg := dist.GetFailureMessage(); msg != "" {
			fmt.Println()
			fmt.Println(msg)
		}
		return err
	}

	// Save installation state after successful install
	savedState.SetInstallState(distroKey, state.InstallState{
		DistroName: distroName,
		Version:    distroVersion,
		Mode:       mode,
	})
	if err := savedState.Save(); err != nil {
		log.WithError(err).Warn("failed to save installation state")
	}

	// Display success message if defined
	if msg := dist.GetSuccessMessage(); msg != "" {
		fmt.Println()
		fmt.Println(msg)
	}

	return nil
}

func init() {
	flags := []cli.Flag{
		&cli.StringFlag{
			Name:    "github-token",
			Usage:   "Used to authenticate to the GitHub API",
			Sources: cli.EnvVars("GITHUB_TOKEN", "CAST_GITHUB_TOKEN"),
		},
		&cli.BoolFlag{
			Name:  "pre-release",
			Usage: "Include pre-release versions as valid install targets",
		},
		&cli.StringFlag{
			Name:    "mode",
			Usage:   "If the distro supports a mode, you can specify it for install",
			Value:   "default",
			Sources: cli.EnvVars("CAST_MODE"),
		},
		&cli.StringFlag{
			Name:    "user",
			Usage:   "The user to install against (cannot be root)",
			Sources: cli.EnvVars("SUDO_USER", "CAST_SUDO_USER"),
		},
		&cli.StringFlag{
			Name:    "cache-path",
			Usage:   "The path where the tool caches files",
			Value:   "/var/cache/cast",
			Sources: cli.EnvVars("CAST_CACHE_PATH"),
		},
		&cli.BoolFlag{
			Name:    "no-cache",
			Usage:   "Do not use any cached files",
			Sources: cli.EnvVars("CAST_NO_CACHE"),
		},
		&cli.StringSliceFlag{
			Name:    "variable",
			Usage:   "Variable to be made available for saltstack pillar templates",
			Aliases: []string{"var"},
			Sources: cli.EnvVars("CAST_VARIABLE"),
		},
		// Hidden Flags
		&cli.BoolFlag{
			Name:    "dev",
			Usage:   "(dev) Enable Development Mode",
			Sources: cli.EnvVars("CAST_DEVELOPMENT_MODE"),
			Hidden:  true,
		},
		&cli.BoolFlag{
			Name:    "no-root-check",
			Usage:   "(dev) disable checking if user is root",
			Sources: cli.EnvVars("CAST_NO_ROOT_CHECK"),
			Hidden:  true,
		},
		&cli.BoolFlag{
			Name:    "saltstack-test",
			Usage:   "Enable SaltStack Test Mode",
			Aliases: []string{"st"},
			Sources: cli.EnvVars("CAST_SALTSTACK_TEST"),
			Hidden:  true,
		},
		&cli.StringFlag{
			Name:    "saltstack-state",
			Usage:   "Specific SaltStack State to use as entrypoint",
			Aliases: []string{"ss"},
			Sources: cli.EnvVars("CAST_SALTSTACK_STATE"),
			Hidden:  true,
		},
		&cli.StringFlag{
			Name:    "saltstack-file-root",
			Usage:   "Use a specific directory for the file root for SaltStack",
			Aliases: []string{"ssfr"},
			Sources: cli.EnvVars("CAST_SALTSTACK_FILE_ROOT"),
			Hidden:  true,
		},
		&cli.StringFlag{
			Name:    "saltstack-log-level",
			Usage:   "Log level for Saltstack",
			Value:   "info",
			Aliases: []string{"ssll"},
			Sources: cli.EnvVars("CAST_SALTSTACK_LOG_LEVEL"),
			Hidden:  true,
		},
		&cli.StringFlag{
			Name:    "saltstack-install-mode",
			Usage:   "Install Mode for Saltstack",
			Value:   "package",
			Aliases: []string{"ssim"},
			Sources: cli.EnvVars("CAST_SALTSTACK_INSTALL_MODE"),
			Hidden:  true,
		},
		&cli.BoolFlag{
			Name:  "no-os-check",
			Usage: "Disable OS check",
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
