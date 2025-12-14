package release

import (
	"context"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"

	"github.com/ekristen/cast/pkg/commands"
	"github.com/ekristen/cast/pkg/common"
)

const template = `release:
  github:
    owner: <owner> # Set this to the owner of the GitHub Repository
    repository: <repo> # Set this to the repository name
manifest:
  version: 2
  name: <distro-name> # Set this to the name of your distribution
  modes:
    - name: server
      state: <distro-name>.server
      default: true
  supported_os:
    - id: ubuntu
      release: 20.04
      codename: focal
`

func Execute(ctx context.Context, cmd *cli.Command) error {
	if _, err := os.Stat(".cast.yml"); !os.IsNotExist(err) {
		return fmt.Errorf("file .cast.yml already exists")
	}

	if err := os.WriteFile(".cast.yml", []byte(template), 0644); err != nil {
		return err
	}

	logrus.Info("generated .cast.yml")

	return nil
}

func init() {
	flags := []cli.Flag{}

	cmd := &cli.Command{
		Name:   "init",
		Usage:  "generates a .cast.yml file",
		Flags:  append(flags, commands.GlobalFlags()...),
		Before: commands.GlobalBefore,
		Action: Execute,
	}

	common.RegisterCommand(cmd)
}
