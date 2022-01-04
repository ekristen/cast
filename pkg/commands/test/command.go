package test

import (
	"github.com/ekristen/cast/pkg/commands"
	"github.com/ekristen/cast/pkg/common"
	"github.com/urfave/cli/v2"
)

func Execute(c *cli.Context) error {

	return nil
}

func init() {
	flags := []cli.Flag{}

	cmd := &cli.Command{
		Name:   "test",
		Usage:  "test",
		Flags:  append(flags, commands.GlobalFlags()...),
		Before: commands.GlobalBefore,
		Action: Execute,
	}

	common.RegisterCommand(cmd)
}
