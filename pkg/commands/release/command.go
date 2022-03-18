package release

import (
	"os"

	"github.com/urfave/cli/v2"

	"github.com/ekristen/cast/pkg/commands"
	"github.com/ekristen/cast/pkg/common"
	"github.com/ekristen/cast/pkg/release"
)

func Execute(c *cli.Context) error {
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

	config := &release.RunConfig{
		DistDir:      c.Path("dist"),
		RmDist:       c.Bool("rm-dist"),
		ConfigFile:   c.Path("config"),
		GitHubToken:  c.String("github-token"),
		Tag:          c.String("tag"),
		CosignKey:    c.Path("cosign-key"),
		LegacySign:   c.Bool("legacy-pgp-sign"),
		LegacyPGPKey: c.Path("legacy-pgp-key"),
		DryRun:       c.Bool("dry-run"),
	}

	if err := release.Run(c.Context, config); err != nil {
		return err
	}

	return nil
}

func init() {
	flags := []cli.Flag{
		&cli.PathFlag{
			Name:  "dist",
			Value: "dist",
		},
		&cli.PathFlag{
			Name:  "config",
			Value: ".cast.yml",
		},
		&cli.StringFlag{
			Name:    "github-token",
			EnvVars: []string{"GITHUB_TOKEN"},
		},
		&cli.PathFlag{
			Name:  "cosign-key",
			Value: "cosign.key",
		},
		&cli.BoolFlag{
			Name: "legacy-pgp-sign",
		},
		&cli.PathFlag{
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
		&cli.PathFlag{
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
