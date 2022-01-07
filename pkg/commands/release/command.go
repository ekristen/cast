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

	if err := release.Run(c.Context, c.Path("config"), c.String("github-token"), c.String("tag")); err != nil {
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
		&cli.StringFlag{
			Name:    "github-token",
			EnvVars: []string{"GITHUB_TOKEN"},
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
