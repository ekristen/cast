package release

import (
	"context"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/ekristen/cast/pkg/commands"
	"github.com/ekristen/cast/pkg/common"
	"github.com/ekristen/cast/pkg/release"
)

func Execute(ctx context.Context, cmd *cli.Command) error {
	if cmd.String("dir") != "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		if err := os.Chdir(cmd.String("dir")); err != nil {
			return err
		}
		defer os.Chdir(cwd)
	}

	config := &release.RunConfig{
		DistDir:      cmd.String("dist"),
		RmDist:       cmd.Bool("rm-dist"),
		ConfigFile:   cmd.String("config"),
		GitHubToken:  cmd.String("github-token"),
		Tag:          cmd.String("tag"),
		CosignKey:    cmd.String("cosign-key"),
		LegacySign:   cmd.Bool("legacy-pgp-sign"),
		LegacyPGPKey: cmd.String("legacy-pgp-key"),
		DryRun:       cmd.Bool("dry-run"),
	}

	if err := release.Run(ctx, config); err != nil {
		return err
	}

	return nil
}

func init() {
	flags := []cli.Flag{
		&cli.StringFlag{
			Name:  "dist",
			Value: "dist",
		},
		&cli.StringFlag{
			Name:  "config",
			Value: ".cast.yml",
		},
		&cli.StringFlag{
			Name:    "github-token",
			Sources: cli.EnvVars("GITHUB_TOKEN"),
		},
		&cli.StringFlag{
			Name:  "cosign-key",
			Value: "cosign.key",
		},
		&cli.BoolFlag{
			Name: "legacy-pgp-sign",
		},
		&cli.StringFlag{
			Name:  "legacy-pgp-key",
			Value: "pgp.key",
		},
		&cli.BoolFlag{
			Name: "rm-dist",
		},
		&cli.BoolFlag{
			Name: "dry-run",
		},
		&cli.StringFlag{
			Name:   "tag",
			Hidden: true,
		},
		&cli.StringFlag{
			Name:   "dir",
			Hidden: true,
		},
	}

	cmd := &cli.Command{
		Name:   "release",
		Usage:  "generate release",
		Flags:  append(flags, commands.GlobalFlags()...),
		Before: commands.GlobalBefore,
		Action: Execute,
	}

	common.RegisterCommand(cmd)
}
