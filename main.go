package main

import (
	"os"
	"path"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/ekristen/cast/pkg/common"

	_ "github.com/ekristen/cast/pkg/commands/init"
	_ "github.com/ekristen/cast/pkg/commands/install"
	_ "github.com/ekristen/cast/pkg/commands/release"
	_ "github.com/ekristen/cast/pkg/commands/test"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			// log panics forces exit
			if _, ok := r.(*logrus.Entry); ok {
				os.Exit(1)
			}
			panic(r)
		}
	}()

	app := cli.NewApp()
	app.Name = path.Base(os.Args[0])
	app.Usage = common.AppVersion.Name
	app.Version = common.AppVersion.Summary
	app.Authors = []*cli.Author{
		{
			Name:  "Erik Kristensen",
			Email: "erik@erikkristensen.com",
		},
	}

	app.Commands = common.GetCommands()
	app.CommandNotFound = func(context *cli.Context, command string) {
		logrus.Fatalf("Command %s not found.", command)
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}
